package synchrophasor

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"net"
	"strings"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
)

// PMU represents a PMU server
type PMU struct {
	Config1      *Config1Frame
	Config2      *ConfigFrame
	Header       *HeaderFrame
	DataRate     int16
	Socket       net.Listener
	Clients      []net.Conn
	ClientsMutex sync.Mutex
	Running      bool
	SendData     map[net.Conn]bool
	SendDataMux  sync.Mutex
	logger       *log.Logger
	metrics      MetricsRecorder
}

// NewPMU creates a new PMU instance
func NewPMU() *PMU {
	pmu := &PMU{
		Clients:  make([]net.Conn, 0),
		SendData: make(map[net.Conn]bool),
		Running:  false,
	}

	// Initialize with default configuration
	pmu.Config2 = NewConfigFrame()
	pmu.Config2.IDCode = 7
	pmu.Config2.SOC = uint32(time.Now().Unix())
	pmu.Config2.FracSec = 0
	pmu.Config2.TimeBase = 1000000
	pmu.Config2.DataRate = 15

	pmu.Config1 = NewConfig1Frame()
	pmu.Config1.ConfigFrame = *pmu.Config2
	pmu.Config1.Sync = (SyncAA << 8) | SyncCfg1

	return pmu
}

// SetLogger sets the logger for the PMU
func (p *PMU) SetLogger(logger *log.Logger) {
	p.logger = logger
}

// SetMetrics sets the metrics recorder for the PMU
func (p *PMU) SetMetrics(m MetricsRecorder) {
	p.metrics = m
}

// log returns the logger or creates a default one
func (p *PMU) log() *log.Logger {
	if p.logger == nil {
		p.logger = log.New()
	}
	return p.logger
}

// Start starts the PMU server
func (p *PMU) Start(address string) error {
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return err
	}

	p.Socket = listener
	p.Running = true

	p.log().WithField("address", address).Info("PMU server listening")

	// Accept connections
	go func() {
		for p.Running {
			conn, err := p.Socket.Accept()
			if err != nil {
				if p.Running {
					p.log().WithError(err).Error("Error accepting connection")
				}
				continue
			}

			clientAddr := conn.RemoteAddr().String()
			p.log().WithField("client", clientAddr).Info("New PDC client connected")

			p.ClientsMutex.Lock()
			p.Clients = append(p.Clients, conn)
			p.SendData[conn] = false
			p.ClientsMutex.Unlock()

			if p.metrics != nil {
				p.metrics.RecordClientConnected()
			}

			// Handle client in goroutine
			go p.handleClient(conn)
		}
	}()

	go p.dataSender()

	return nil
}

// Stop stops the PMU server
func (p *PMU) Stop() {
	p.Running = false
	if p.Socket != nil {
		_ = p.Socket.Close()
	}

	p.ClientsMutex.Lock()
	for _, conn := range p.Clients {
		_ = conn.Close()
	}
	p.Clients = make([]net.Conn, 0)
	p.ClientsMutex.Unlock()

	p.log().Info("PMU server stopped")
}

// handleClient handles a client connection
func (p *PMU) handleClient(conn net.Conn) {
	clientAddr := conn.RemoteAddr().String()

	defer func() {
		_ = conn.Close()
		p.ClientsMutex.Lock()
		delete(p.SendData, conn)
		// Remove from clients list
		for i, c := range p.Clients {
			if c == conn {
				p.Clients = append(p.Clients[:i], p.Clients[i+1:]...)
				break
			}
		}
		p.ClientsMutex.Unlock()

		// Update metrics
		if p.metrics != nil {
			p.metrics.RecordClientDisconnected()
		}

		p.log().WithField("client", clientAddr).Info("PDC client disconnected")
	}()

	buffer := make([]byte, 65536)

	for p.Running {
		// Set read timeout
		if err := conn.SetReadDeadline(time.Now().Add(1 * time.Second)); err != nil {
			p.log().WithField("client", clientAddr).WithError(err).Error("Error setting read deadline")
			break
		}

		n, err := conn.Read(buffer)
		if err != nil {
			var netErr net.Error
			if errors.As(err, &netErr) && netErr.Timeout() {
				continue
			}
			if err.Error() != "EOF" {
				p.log().WithFields(log.Fields{
					"client": clientAddr,
					"error":  err,
				}).Error("Error reading from client")
			}
			break
		}

		// Update metrics
		if p.metrics != nil {
			p.metrics.RecordBytesReceived(n)
		}

		if n >= 4 {
			frameSize := binary.BigEndian.Uint16(buffer[2:4])
			if n >= int(frameSize) {
				// Process frame
				frame, err := UnpackFrame(buffer[:frameSize], nil)
				if err == nil {
					if cmd, ok := frame.(*CommandFrame); ok {
						p.handleCommand(conn, cmd)
					}
				} else {
					p.log().WithFields(log.Fields{
						"client": clientAddr,
						"error":  err,
					}).Error("Error unpacking frame")
					if p.metrics != nil {
						p.metrics.RecordFrameError("unpack_error")
					}
				}
			}
		}
	}
}

// handleCommand processes a command frame
func (p *PMU) handleCommand(conn net.Conn, cmd *CommandFrame) {
	clientAddr := conn.RemoteAddr().String()
	var response []byte
	var err error
	var cmdName string

	switch cmd.CMD {
	case CmdStart:
		cmdName = "START"
		p.SendDataMux.Lock()
		p.SendData[conn] = true
		p.SendDataMux.Unlock()
		p.log().WithField("client", clientAddr).Info("Started data transmission")

	case CmdStop:
		cmdName = "STOP"
		p.SendDataMux.Lock()
		p.SendData[conn] = false
		p.SendDataMux.Unlock()
		p.log().WithField("client", clientAddr).Info("Stopped data transmission")

	case CmdHeader:
		cmdName = "HEADER"
		p.Header.SetTime(nil, nil)
		response, err = p.Header.Pack()
		if err == nil && p.metrics != nil {
			p.metrics.RecordHeaderFrameSent(len(response))
		}

	case CmdCfg1:
		cmdName = "CONFIG1"
		p.Config1.SetTime(nil, nil)
		response, err = p.Config1.Pack()
		if err == nil && p.metrics != nil {
			p.metrics.RecordConfigFrameSent(len(response))
		}

	case CmdCfg2:
		cmdName = "CONFIG2"
		p.Config2.SetTime(nil, nil)
		response, err = p.Config2.Pack()
		if err == nil && p.metrics != nil {
			p.metrics.RecordConfigFrameSent(len(response))
		}

	default:
		cmdName = fmt.Sprintf("UNKNOWN(0x%04X)", cmd.CMD)
	}

	// Record command metric
	if p.metrics != nil {
		p.metrics.RecordCommand(cmdName)
	}

	p.log().WithFields(log.Fields{
		"client":  clientAddr,
		"command": cmdName,
		"cmd_id":  cmd.IDCode,
	}).Debug("Received command")

	if response != nil && err == nil {
		if _, err := conn.Write(response); err != nil {
			p.log().WithFields(log.Fields{
				"client":  clientAddr,
				"command": cmdName,
				"error":   err,
			}).Error("Error writing response")
		}
	} else if err != nil {
		p.log().WithFields(log.Fields{
			"client":  clientAddr,
			"command": cmdName,
			"error":   err,
		}).Error("Error packing response")
		if p.metrics != nil {
			p.metrics.RecordFrameError("pack_error")
		}
	}
}

// dataSender sends data frames to connected clients
func (p *PMU) dataSender() {
	ticker := time.NewTicker(time.Duration(1000/p.Config2.DataRate) * time.Millisecond)
	defer ticker.Stop()

	counter := 0
	framesSent := 0
	lastRateUpdate := time.Now()

	for p.Running {
		<-ticker.C
		// Create data frame
		df := NewDataFrame(p.Config2)
		df.IDCode = p.Config2.IDCode
		df.SetTime(nil, nil)

		// Update PMU data (example values)
		for _, pmu := range p.Config2.PMUStationList {
			// Update phasor values (example)
			for i := range pmu.PhasorValues {
				angle := float64(counter) * math.Pi / 180.0
				pmu.PhasorValues[i] = complex(30000*math.Cos(angle), 30000*math.Sin(angle))
			}

			// Update frequency based on nominal frequency
			nominalFreq := pmu.GetNominalFrequency()
			pmu.Freq = nominalFreq + 0.5*float32(math.Sin(float64(counter)*0.1))
			pmu.DFreq = 0.05 * float32(math.Cos(float64(counter)*0.1))

			// Update analog values
			for i := range pmu.AnalogValues {
				pmu.AnalogValues[i] = 100.0 * float32(math.Sin(float64(counter)*0.1+float64(i)))
			}
		}

		// Pack data frame
		data, err := df.Pack()
		if err != nil {
			p.log().WithError(err).Error("Error packing data frame")
			if p.metrics != nil {
				p.metrics.RecordFrameError("data_pack_error")
			}
			continue
		}

		// Send to all clients with data enabled
		p.ClientsMutex.Lock()
		activeClients := 0
		for conn := range p.SendData {
			p.SendDataMux.Lock()
			sendEnabled := p.SendData[conn]
			p.SendDataMux.Unlock()

			if sendEnabled {
				activeClients++
				go func(c net.Conn) {
					if err := c.SetWriteDeadline(time.Now().Add(100 * time.Millisecond)); err != nil {
						p.log().WithField("client", c.RemoteAddr().String()).WithError(err).Debug("Error setting write deadline")
						return
					}
					_, err := c.Write(data)
					if err != nil {
						p.log().WithFields(log.Fields{
							"client": c.RemoteAddr().String(),
							"error":  err,
						}).Debug("Error sending data frame")
					}
				}(conn)
			}
		}
		p.ClientsMutex.Unlock()

		if activeClients > 0 {
			framesSent++
			if p.metrics != nil {
				p.metrics.RecordDataFrameSent(len(data))
			}
		}

		// Update rate metric every second
		if time.Since(lastRateUpdate) >= time.Second {
			actualRate := float64(framesSent) / time.Since(lastRateUpdate).Seconds()
			if p.metrics != nil {
				p.metrics.UpdateDataFrameRate(actualRate)
			}
			framesSent = 0
			lastRateUpdate = time.Now()
		}

		counter++
		if counter >= 360 {
			counter = 0
		}
	}
}

// LogConfiguration logs the complete PMU configuration
func (p *PMU) LogConfiguration() {
	if p.Config2 == nil {
		p.log().Warn("No configuration available to log")
		return
	}

	// Log main configuration
	p.log().WithFields(log.Fields{
		"id_code":   p.Config2.IDCode,
		"time_base": p.Config2.TimeBase,
		"data_rate": p.Config2.DataRate,
		"num_pmu":   p.Config2.NumPMU,
	}).Info("PMU Configuration")

	// Log each PMU station
	for i, station := range p.Config2.PMUStationList {
		stationLog := p.log().WithFields(log.Fields{
			"index":             i,
			"station_name":      station.STN,
			"station_id":        station.IDCode,
			"nominal_frequency": station.GetNominalFrequency(),
			"config_count":      station.CfgCnt,
		})

		stationLog = stationLog.WithFields(log.Fields{
			"format": map[string]bool{
				"coord_polar":  station.FormatCoord(),
				"phasor_float": station.FormatPhasorType(),
				"analog_float": station.FormatAnalogType(),
				"freq_float":   station.FormatFreqType(),
			},
		})

		stationLog = stationLog.WithFields(log.Fields{
			"channels": map[string]int{
				"phasor":  int(station.Phnmr),
				"analog":  int(station.Annmr),
				"digital": int(station.Dgnmr),
			},
		})

		stationLog.Info("PMU Station Configuration")

		if len(station.CHNAMPhasor) > 0 {
			for j, name := range station.CHNAMPhasor {
				phUnit := station.Phunit[j]
				phType := (phUnit >> 24) & 0xFF
				phScale := phUnit & 0x0FFFFFF

				p.log().WithFields(log.Fields{
					"station":      station.STN,
					"channel_type": "phasor",
					"index":        j,
					"name":         strings.TrimSpace(name),
					"unit_type":    map[uint32]string{0: "voltage", 1: "current"}[phType],
					"scale_factor": phScale,
				}).Debug("Phasor channel configuration")
			}
		}

		if len(station.CHNAMAnalog) > 0 {
			for j, name := range station.CHNAMAnalog {
				anUnit := station.Anunit[j]
				anType := (anUnit >> 24) & 0xFF
				anScale := anUnit & 0x0FFFFFF

				p.log().WithFields(log.Fields{
					"station":      station.STN,
					"channel_type": "analog",
					"index":        j,
					"name":         strings.TrimSpace(name),
					"unit_type":    anType,
					"scale_factor": anScale,
				}).Debug("Analog channel configuration")
			}
		}

		// Log digital channels
		if len(station.CHNAMDigital) > 0 {
			digitalNames := make([]string, 0)
			for _, name := range station.CHNAMDigital {
				trimmed := strings.TrimSpace(name)
				if trimmed != "" {
					digitalNames = append(digitalNames, trimmed)
				}
			}

			for j, dgUnit := range station.Dgunit {
				normalMask := (dgUnit >> 16) & 0xFFFF
				validMask := dgUnit & 0xFFFF

				p.log().WithFields(log.Fields{
					"station":      station.STN,
					"channel_type": "digital",
					"word_index":   j,
					"channels":     digitalNames[j*16 : min((j+1)*16, len(digitalNames))],
					"normal_mask":  fmt.Sprintf("0x%04X", normalMask),
					"valid_mask":   fmt.Sprintf("0x%04X", validMask),
				}).Debug("Digital channel configuration")
			}
		}
	}

	if p.Header != nil {
		p.log().WithFields(log.Fields{
			"header": p.Header.Data,
		}).Info("PMU Header Information")
	}
}
