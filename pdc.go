package synchrophasor

import (
	"encoding/binary"
	"net"
)

// PDC represents a PDC client
type PDC struct {
	Socket     net.Conn
	IDCode     uint16
	PMUConfig1 *Config1Frame
	PMUConfig2 *ConfigFrame
	PMUHeader  *HeaderFrame
	Buffer     []byte
}

// NewPDC creates a new PDC instance
func NewPDC(idCode uint16) *PDC {
	return &PDC{
		IDCode: idCode,
		Buffer: make([]byte, 65536),
	}
}

// Connect connects to a PMU
func (p *PDC) Connect(address string) error {
	conn, err := net.Dial("tcp", address)
	if err != nil {
		return err
	}
	p.Socket = conn
	return nil
}

// Disconnect closes the connection
func (p *PDC) Disconnect() {
	if p.Socket != nil {
		_ = p.Socket.Close()
		p.Socket = nil
	}
}

// SendCommand sends a command to PMU
func (p *PDC) SendCommand(cmdCode uint16) error {
	cmd := NewCommandFrame()
	cmd.IDCode = p.IDCode
	cmd.CMD = cmdCode
	cmd.SetTime(nil, nil)

	data, err := cmd.Pack()
	if err != nil {
		return err
	}

	_, err = p.Socket.Write(data)
	return err
}

// Start requests PMU to start sending data
func (p *PDC) Start() error {
	return p.SendCommand(CmdStart)
}

// Stop requests PMU to stop sending data
func (p *PDC) Stop() error {
	return p.SendCommand(CmdStop)
}

// GetHeader requests header frame
func (p *PDC) GetHeader() (*HeaderFrame, error) {
	err := p.SendCommand(CmdHeader)
	if err != nil {
		return nil, err
	}

	frame, err := p.ReadFrame()
	if err != nil {
		return nil, err
	}

	header, ok := frame.(*HeaderFrame)
	if !ok {
		return nil, ErrInvalidFrame
	}

	p.PMUHeader = header
	return header, nil
}

// GetConfig requests configuration frame
func (p *PDC) GetConfig(version int) (*ConfigFrame, error) {
	var cmdCode uint16
	switch version {
	case 1:
		cmdCode = CmdCfg1
	case 2:
		cmdCode = CmdCfg2
	case 3:
		cmdCode = CmdCfg3
	default:
		cmdCode = CmdCfg2
	}

	err := p.SendCommand(cmdCode)
	if err != nil {
		return nil, err
	}

	frame, err := p.ReadFrame()
	if err != nil {
		return nil, err
	}

	switch cfg := frame.(type) {
	case *ConfigFrame:
		p.PMUConfig2 = cfg
		return cfg, nil
	case *Config1Frame:
		p.PMUConfig1 = cfg
		cfg2 := &ConfigFrame{}
		cfg2.C37118 = cfg.C37118
		cfg2.TimeBase = cfg.TimeBase
		cfg2.NumPMU = cfg.NumPMU
		cfg2.DataRate = cfg.DataRate
		cfg2.PMUStationList = cfg.PMUStationList
		p.PMUConfig2 = cfg2
		return cfg2, nil
	default:
		return nil, ErrInvalidFrame
	}
}

// ReadFrame reads a frame from the socket
func (p *PDC) ReadFrame() (interface{}, error) {
	// Read at least SYNC + FRAMESIZE (4 bytes)
	totalRead := 0
	for totalRead < 4 {
		n, err := p.Socket.Read(p.Buffer[totalRead:])
		if err != nil {
			return nil, err
		}
		totalRead += n
	}

	frameSize := binary.BigEndian.Uint16(p.Buffer[2:4])

	for totalRead < int(frameSize) {
		n, err := p.Socket.Read(p.Buffer[totalRead:])
		if err != nil {
			return nil, err
		}
		totalRead += n
	}

	return UnpackFrame(p.Buffer[:frameSize], p.PMUConfig2)
}
