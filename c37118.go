package synchrophasor

import "time"

// C37118 is the base structure for all frame types
type C37118 struct {
	Sync      uint16
	FrameSize uint16
	IDCode    uint16
	SOC       uint32
	FracSec   uint32
	CHK       uint16
}

// SetTime sets SOC and FracSec, calculating them if not provided
func (c *C37118) SetTime(soc *uint32, fracSec *uint32) {
	now := time.Now()

	if soc != nil {
		c.SOC = *soc
	} else {
		c.SOC = uint32(now.Unix())
	}

	if fracSec != nil {
		c.FracSec = *fracSec
	} else {
		nanos := now.Nanosecond()
		fraction := uint32(nanos / 1000)
		// Set time quality and other bits
		c.FracSec = 0x80000000 | (fraction & 0x00FFFFFF)
	}
}

// SetTimeWithQuality sets SOC and FracSec with specific parameters
func (c *C37118) SetTimeWithQuality(
	soc uint32, frSeconds uint32, leapDir string, leapOcc bool, leapPen bool, timeQuality uint8) {
	c.SOC = soc

	c.FracSec = 2

	// Bit 6: Leap second direction
	if leapDir == "-" {
		c.FracSec |= 1
	}
	c.FracSec <<= 1

	// Bit 5: Leap second occurred
	if leapOcc {
		c.FracSec |= 1
	}
	c.FracSec <<= 1

	// Bit 4: Leap second pending
	if leapPen {
		c.FracSec |= 1
	}
	c.FracSec <<= 4 // Shift for time quality bits

	// Bits 3-0: Time quality
	c.FracSec |= uint32(timeQuality & 0x0F)

	// Clear MSB for standard compliance
	c.FracSec ^= 0x80

	// Shift to upper byte and add fraction of second
	c.FracSec <<= 24
	c.FracSec |= frSeconds & 0x00FFFFFF
}
