// Package synchrophasor implements IEEE C37.118-2011 protocol for synchrophasor data transfer
package synchrophasor

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"strings"
)

// Frame type constants
const (
	FrameTypeData   = 0
	FrameTypeHeader = 1
	FrameTypeCfg1   = 2
	FrameTypeCfg2   = 3
	FrameTypeCmd    = 4
	FrameTypeCfg3   = 5
)

// Sync byte constants
const (
	SyncAA   = 0xAA
	SyncData = 0x01
	SyncHdr  = 0x11
	SyncCfg1 = 0x21
	SyncCfg2 = 0x31
	SyncCmd  = 0x41
	SyncCfg3 = 0x51
)

// Command codes
const (
	CmdStop   = 0x01
	CmdStart  = 0x02
	CmdHeader = 0x03
	CmdCfg1   = 0x04
	CmdCfg2   = 0x05
	CmdCfg3   = 0x06
	CmdExt    = 0x08
)

// Nominal frequency constants
const (
	FreqNom60Hz = 0
	FreqNom50Hz = 1
)

// Phasor unit types
const (
	PhunitVoltage = 0
	PhunitCurrent = 1
)

// Analog unit types
const (
	AnunitPow  = 0
	AnunitRMS  = 1
	AnunitPeak = 2
)

// Custom error types
var (
	ErrInvalidFrame     = errors.New("invalid frame")
	ErrCRCFailed        = errors.New("CRC check failed")
	ErrInvalidParameter = errors.New("invalid parameter")
	ErrInvalidSize      = errors.New("invalid size")
	ErrNotImpl          = errors.New("function not implemented")
)

// HeaderFrame represents a header frame
type HeaderFrame struct {
	C37118
	Data string
}

// NewHeaderFrame creates a new header frame
func NewHeaderFrame(idCode uint16, info string) *HeaderFrame {
	h := &HeaderFrame{
		Data: info,
	}
	h.Sync = (SyncAA << 8) | SyncHdr
	h.FrameSize = 16
	h.IDCode = idCode
	return h
}

// Pack converts header frame to bytes
func (h *HeaderFrame) Pack() ([]byte, error) {
	// Update frame size
	h.FrameSize = uint16(16 + len(h.Data))

	buf := new(bytes.Buffer)

	// Write header
	if err := writeBinary(buf, h.Sync, h.FrameSize, h.IDCode, h.SOC, h.FracSec); err != nil {
		return nil, err
	}

	// Write data
	buf.WriteString(h.Data)

	// Calculate and write CRC
	data := buf.Bytes()
	crc := CalcCRC(data)
	if err := binary.Write(buf, binary.BigEndian, crc); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// Unpack parses bytes into header frame
func (h *HeaderFrame) Unpack(data []byte) error {
	if len(data) < 16 {
		return ErrInvalidSize
	}

	buf := bytes.NewReader(data)

	// Read header
	if err := readBinary(buf, &h.Sync, &h.FrameSize); err != nil {
		return err
	}

	if h.FrameSize < 16 {
		return ErrInvalidSize
	}

	if err := readBinary(buf, &h.IDCode, &h.SOC, &h.FracSec); err != nil {
		return err
	}

	// Read data
	dataSize := int(h.FrameSize) - 16
	if dataSize > 0 && dataSize < 65000 {
		dataBytes := make([]byte, dataSize)
		if _, err := buf.Read(dataBytes); err != nil {
			return err
		}
		h.Data = string(dataBytes)
	}

	// Skip to CRC position
	if _, err := buf.Seek(int64(h.FrameSize-2), io.SeekStart); err != nil {
		return err
	}
	if err := binary.Read(buf, binary.BigEndian, &h.CHK); err != nil {
		return err
	}

	// Verify CRC
	crcData := data[:h.FrameSize-2]
	if CalcCRC(crcData) != h.CHK {
		return ErrCRCFailed
	}

	return nil
}

// ConfigFrame represents a configuration frame
type ConfigFrame struct {
	C37118
	TimeBase       uint32
	NumPMU         uint16
	DataRate       int16
	PMUStationList []*PMUStation
}

// NewConfigFrame creates a new configuration frame
func NewConfigFrame() *ConfigFrame {
	cfg := &ConfigFrame{
		NumPMU:         0,
		PMUStationList: make([]*PMUStation, 0),
	}
	cfg.Sync = (SyncAA << 8) | SyncCfg2
	return cfg
}

// AddPMUStation adds a PMU station to the configuration
func (c *ConfigFrame) AddPMUStation(pmu *PMUStation) {
	c.PMUStationList = append(c.PMUStationList, pmu)
	c.NumPMU++
}

// GetPMUStationByIDCode returns PMU station by ID code
func (c *ConfigFrame) GetPMUStationByIDCode(idCode uint16) *PMUStation {
	for _, pmu := range c.PMUStationList {
		if pmu.IDCode == idCode {
			return pmu
		}
	}
	return nil
}

// Pack converts configuration frame to bytes
func (c *ConfigFrame) Pack() ([]byte, error) {
	// Calculate frame size
	size := uint16(24) // Base size

	for _, pmu := range c.PMUStationList {
		size += 30                                          // PMU header
		size += 16 * (pmu.Phnmr + pmu.Annmr + 16*pmu.Dgnmr) // Channel names
		size += 4 * (pmu.Phnmr + pmu.Annmr + pmu.Dgnmr)     // Units
	}

	c.FrameSize = size

	buf := new(bytes.Buffer)

	// Write common header
	if err := writeBinary(buf, c.Sync, c.FrameSize, c.IDCode, c.SOC, c.FracSec, c.TimeBase, c.NumPMU); err != nil {
		return nil, err
	}

	// Write PMU stations
	for _, pmu := range c.PMUStationList {
		// Station name (16 bytes)
		stnName := padString(pmu.STN)
		buf.WriteString(stnName)

		// PMU fields
		if err := writeBinary(buf, pmu.IDCode, pmu.Format, pmu.Phnmr, pmu.Annmr, pmu.Dgnmr); err != nil {
			return nil, err
		}

		// Channel names
		for _, name := range pmu.CHNAMPhasor {
			buf.WriteString(padString(name))
		}
		for _, name := range pmu.CHNAMAnalog {
			buf.WriteString(padString(name))
		}
		// Digital: 16 names per digital word
		for i := 0; i < int(pmu.Dgnmr*16); i++ {
			if i < len(pmu.CHNAMDigital) {
				buf.WriteString(padString(pmu.CHNAMDigital[i]))
			} else {
				buf.WriteString(padString(""))
			}
		}

		// Units
		for _, unit := range pmu.Phunit {
			if err := binary.Write(buf, binary.BigEndian, unit); err != nil {
				return nil, err
			}
		}
		for _, unit := range pmu.Anunit {
			if err := binary.Write(buf, binary.BigEndian, unit); err != nil {
				return nil, err
			}
		}
		for _, unit := range pmu.Dgunit {
			if err := binary.Write(buf, binary.BigEndian, unit); err != nil {
				return nil, err
			}
		}

		// Nominal frequency and config count
		if err := writeBinary(buf, pmu.Fnom, pmu.CfgCnt); err != nil {
			return nil, err
		}
	}

	// Data rate
	if err := binary.Write(buf, binary.BigEndian, c.DataRate); err != nil {
		return nil, err
	}

	// Calculate and write CRC
	data := buf.Bytes()
	crc := CalcCRC(data)
	if err := binary.Write(buf, binary.BigEndian, crc); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// unpackPMUStation reads a single PMU station from the buffer
func (c *ConfigFrame) unpackPMUStation(buf *bytes.Reader) (*PMUStation, error) {
	pmu := &PMUStation{}

	// Station name
	stnBytes := make([]byte, 16)
	if _, err := buf.Read(stnBytes); err != nil {
		return nil, err
	}
	pmu.STN = strings.TrimSpace(string(stnBytes))

	// PMU fields
	if err := readBinary(buf, &pmu.IDCode, &pmu.Format); err != nil {
		return nil, err
	}

	var phnmr, annmr, dgnmr uint16
	if err := readBinary(buf, &phnmr, &annmr, &dgnmr); err != nil {
		return nil, err
	}

	if phnmr > 1000 || annmr > 1000 || dgnmr > 100 {
		return nil, ErrInvalidSize
	}

	pmu.Phnmr = phnmr
	pmu.Annmr = annmr
	pmu.Dgnmr = dgnmr

	// Calculate expected channel bytes
	channelBytes := 16 * (phnmr + annmr + 16*dgnmr)

	// Save current position for unit reading
	channelPos := buf.Size() - int64(buf.Len())

	// Skip channel names for now
	if _, err := buf.Seek(int64(channelBytes), io.SeekCurrent); err != nil {
		return nil, err
	}

	// Read units
	pmu.Phunit = make([]uint32, phnmr)
	for j := 0; j < int(phnmr); j++ {
		if err := binary.Read(buf, binary.BigEndian, &pmu.Phunit[j]); err != nil {
			return nil, err
		}
	}

	pmu.Anunit = make([]uint32, annmr)
	for j := 0; j < int(annmr); j++ {
		if err := binary.Read(buf, binary.BigEndian, &pmu.Anunit[j]); err != nil {
			return nil, err
		}
	}

	pmu.Dgunit = make([]uint32, dgnmr)
	for j := 0; j < int(dgnmr); j++ {
		if err := binary.Read(buf, binary.BigEndian, &pmu.Dgunit[j]); err != nil {
			return nil, err
		}
	}

	// Read FNOM and CFGCNT
	if err := readBinary(buf, &pmu.Fnom, &pmu.CfgCnt); err != nil {
		return nil, err
	}

	// Go back and read channel names
	currentPos := buf.Size() - int64(buf.Len())
	if _, err := buf.Seek(channelPos, io.SeekStart); err != nil {
		return nil, err
	}

	// Read channel names
	if err := c.readChannelNames(buf, pmu, phnmr, annmr, dgnmr); err != nil {
		return nil, err
	}

	// Restore position
	if _, err := buf.Seek(currentPos, io.SeekStart); err != nil {
		return nil, err
	}

	// Initialize value arrays
	pmu.PhasorValues = make([]complex128, phnmr)
	pmu.AnalogValues = make([]float32, annmr)
	pmu.DigitalValues = make([][]bool, dgnmr)
	for j := 0; j < int(dgnmr); j++ {
		pmu.DigitalValues[j] = make([]bool, 16)
	}

	return pmu, nil
}

// readChannelNames reads channel names for a PMU station
func (c *ConfigFrame) readChannelNames(buf *bytes.Reader, pmu *PMUStation, phnmr, annmr, dgnmr uint16) error {
	// Read phasor channel names
	pmu.CHNAMPhasor = make([]string, phnmr)
	for j := 0; j < int(phnmr); j++ {
		nameBytes := make([]byte, 16)
		if _, err := buf.Read(nameBytes); err != nil {
			return err
		}
		pmu.CHNAMPhasor[j] = strings.TrimSpace(string(nameBytes))
	}

	// Read analog channel names
	pmu.CHNAMAnalog = make([]string, annmr)
	for j := 0; j < int(annmr); j++ {
		nameBytes := make([]byte, 16)
		if _, err := buf.Read(nameBytes); err != nil {
			return err
		}
		pmu.CHNAMAnalog[j] = strings.TrimSpace(string(nameBytes))
	}

	// Read digital channel names
	pmu.CHNAMDigital = make([]string, 16*dgnmr)
	for j := 0; j < int(16*dgnmr); j++ {
		nameBytes := make([]byte, 16)
		if _, err := buf.Read(nameBytes); err != nil {
			return err
		}
		pmu.CHNAMDigital[j] = strings.TrimSpace(string(nameBytes))
	}

	return nil
}

// Unpack parses bytes into configuration frame
func (c *ConfigFrame) Unpack(data []byte) error {
	if len(data) < 24 {
		return ErrInvalidSize
	}

	buf := bytes.NewReader(data)

	// Read common header
	if err := readBinary(buf, &c.Sync, &c.FrameSize); err != nil {
		return err
	}

	if c.FrameSize < 24 {
		return ErrInvalidSize
	}

	if err := readBinary(buf, &c.IDCode, &c.SOC, &c.FracSec, &c.TimeBase); err != nil {
		return err
	}

	var numPMU uint16
	if err := binary.Read(buf, binary.BigEndian, &numPMU); err != nil {
		return err
	}

	if numPMU > 1000 { // Sanity check
		return ErrInvalidSize
	}

	// Read PMU stations
	for i := 0; i < int(numPMU); i++ {
		pmu, err := c.unpackPMUStation(buf)
		if err != nil {
			return err
		}
		c.AddPMUStation(pmu)
	}

	// Data rate
	if err := binary.Read(buf, binary.BigEndian, &c.DataRate); err != nil {
		return err
	}

	// Read CRC
	if _, err := buf.Seek(int64(c.FrameSize-2), io.SeekStart); err != nil {
		return err
	}
	if err := binary.Read(buf, binary.BigEndian, &c.CHK); err != nil {
		return err
	}

	// Verify CRC
	crcData := data[:c.FrameSize-2]
	if CalcCRC(crcData) != c.CHK {
		return ErrCRCFailed
	}

	return nil
}

// Config1Frame represents a configuration frame version 1, extending the base ConfigFrame type.
type Config1Frame struct {
	ConfigFrame
}

// NewConfig1Frame creates a new configuration frame version 1
func NewConfig1Frame() *Config1Frame {
	cfg := &Config1Frame{}
	cfg.Sync = (SyncAA << 8) | SyncCfg1
	cfg.NumPMU = 0
	cfg.PMUStationList = make([]*PMUStation, 0)
	return cfg
}

// FrameType represents the type of frame
type FrameType int

// GetFrameType extracts frame type from byte data
func GetFrameType(data []byte) (FrameType, error) {
	if len(data) < 2 {
		return -1, ErrInvalidSize
	}

	// Check sync byte
	if data[0] != SyncAA {
		return -1, ErrInvalidFrame
	}

	frameType := (data[1] >> 4) & 0x07
	return FrameType(frameType), nil
}

// UnpackFrame unpacks any frame type from bytes
func UnpackFrame(data []byte, cfg *ConfigFrame) (interface{}, error) {
	frameType, err := GetFrameType(data)
	if err != nil {
		return nil, err
	}

	switch frameType {
	case FrameTypeData:
		if cfg == nil {
			return nil, ErrInvalidParameter
		}
		df := NewDataFrame(cfg)
		err := df.Unpack(data)
		return df, err

	case FrameTypeHeader:
		hf := &HeaderFrame{}
		err := hf.Unpack(data)
		return hf, err

	case FrameTypeCfg1:
		cf := NewConfig1Frame()
		err := cf.Unpack(data)
		return cf, err

	case FrameTypeCfg2:
		cf := NewConfigFrame()
		err := cf.Unpack(data)
		return cf, err

	case FrameTypeCfg3:
		return nil, ErrNotImpl

	case FrameTypeCmd:
		cmd := NewCommandFrame()
		err := cmd.Unpack(data)
		return cmd, err

	default:
		return nil, ErrInvalidFrame
	}
}
