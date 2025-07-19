package synchrophasor

import (
	"bytes"
	"encoding/binary"
)

// CommandFrame represents a command frame
type CommandFrame struct {
	C37118
	CMD        uint16
	ExtraFrame []byte
}

// NewCommandFrame creates a new command frame
func NewCommandFrame() *CommandFrame {
	cmd := &CommandFrame{}
	cmd.Sync = (SyncAA << 8) | SyncCmd
	cmd.FrameSize = 18
	return cmd
}

// Pack converts command frame to bytes
func (c *CommandFrame) Pack() ([]byte, error) {
	buf := new(bytes.Buffer)

	// Write header and command
	if err := writeBinary(buf, c.Sync, c.FrameSize, c.IDCode, c.SOC, c.FracSec, c.CMD); err != nil {
		return nil, err
	}

	// Write extra frame if exists
	if c.ExtraFrame != nil {
		buf.Write(c.ExtraFrame)
	}

	// Calculate and write CRC
	data := buf.Bytes()
	crc := CalcCRC(data)
	if err := binary.Write(buf, binary.BigEndian, crc); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// Unpack parses bytes into command frame
func (c *CommandFrame) Unpack(data []byte) error {
	if len(data) < 18 {
		return ErrInvalidSize
	}

	buf := bytes.NewReader(data)

	// Read header and command
	if err := readBinary(buf, &c.Sync, &c.FrameSize); err != nil {
		return err
	}

	if c.FrameSize < 18 {
		return ErrInvalidSize
	}

	if err := readBinary(buf, &c.IDCode, &c.SOC, &c.FracSec, &c.CMD); err != nil {
		return err
	}

	// Read extra frame if exists
	extraSize := int(c.FrameSize) - 18
	if extraSize > 0 && extraSize < 65518 {
		c.ExtraFrame = make([]byte, extraSize)
		if _, err := buf.Read(c.ExtraFrame); err != nil {
			return err
		}
	}

	// Read CRC
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
