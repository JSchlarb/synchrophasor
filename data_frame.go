package synchrophasor

import (
	"bytes"
	"encoding/binary"
	"io"
	"math/cmplx"
)

// DataFrame represents a data frame
type DataFrame struct {
	C37118
	AssociatedConfig *ConfigFrame
}

// NewDataFrame creates a new data frame
func NewDataFrame(cfg *ConfigFrame) *DataFrame {
	df := &DataFrame{
		AssociatedConfig: cfg,
	}
	df.Sync = (SyncAA << 8) | SyncData
	return df
}

// Pack converts data frame to bytes
func (d *DataFrame) Pack() ([]byte, error) {
	if d.AssociatedConfig == nil {
		return nil, ErrInvalidParameter
	}

	// Calculate frame size
	size := uint16(14)

	for _, pmu := range d.AssociatedConfig.PMUStationList {
		size += 2

		if pmu.FormatPhasorType() {
			size += 8 * pmu.Phnmr
		} else {
			size += 4 * pmu.Phnmr
		}

		if pmu.FormatFreqType() {
			size += 8
		} else {
			size += 4
		}

		if pmu.FormatAnalogType() {
			size += 4 * pmu.Annmr
		} else {
			size += 2 * pmu.Annmr
		}

		// Digital data
		size += 2 * pmu.Dgnmr
	}

	size += 2 // CRC
	d.FrameSize = size

	buf := new(bytes.Buffer)

	// Write header
	if err := writeBinary(buf, d.Sync, d.FrameSize, d.IDCode, d.SOC, d.FracSec); err != nil {
		return nil, err
	}

	// Write data for each PMU
	for _, pmu := range d.AssociatedConfig.PMUStationList {
		if err := binary.Write(buf, binary.BigEndian, pmu.Stat); err != nil {
			return nil, err
		}

		// Phasors
		for j := 0; j < int(pmu.Phnmr); j++ {
			if pmu.FormatPhasorType() {
				// Float format
				if pmu.FormatCoord() {
					// Polar
					mag := float32(cmplx.Abs(pmu.PhasorValues[j]))
					ang := float32(cmplx.Phase(pmu.PhasorValues[j]))
					if err := writeBinary(buf, mag, ang); err != nil {
						return nil, err
					}
				} else {
					// Rectangular
					re := float32(real(pmu.PhasorValues[j]))
					im := float32(imag(pmu.PhasorValues[j]))
					if err := writeBinary(buf, re, im); err != nil {
						return nil, err
					}
				}
			} else {
				// Integer format
				if pmu.FormatCoord() {
					// Polar
					mag := cmplx.Abs(pmu.PhasorValues[j])
					ang := cmplx.Phase(pmu.PhasorValues[j])
					magInt := uint16(mag * 1e5 / float64(pmu.GetPhasorFactor(j)))
					angInt := int16(ang * 1e4)
					if err := writeBinary(buf, magInt, angInt); err != nil {
						return nil, err
					}
				} else {
					// Rectangular
					re := real(pmu.PhasorValues[j])
					im := imag(pmu.PhasorValues[j])
					reInt := int16(re * 1e5 / float64(pmu.GetPhasorFactor(j)))
					imInt := int16(im * 1e5 / float64(pmu.GetPhasorFactor(j)))
					if err := writeBinary(buf, reInt, imInt); err != nil {
						return nil, err
					}
				}
			}
		}

		// Freq and DFreq
		if pmu.FormatFreqType() {
			// Float format
			if err := writeBinary(buf, pmu.Freq, pmu.DFreq); err != nil {
				return nil, err
			}
		} else {
			// Integer format
			freqOffset := pmu.Freq - pmu.GetNominalFrequency()
			freqInt := int16(freqOffset * 1000)
			dfreqInt := int16(pmu.DFreq * 100)
			if err := writeBinary(buf, freqInt, dfreqInt); err != nil {
				return nil, err
			}
		}

		// Analog values
		for j := 0; j < int(pmu.Annmr); j++ {
			if pmu.FormatAnalogType() {
				// Float format
				if err := binary.Write(buf, binary.BigEndian, pmu.AnalogValues[j]); err != nil {
					return nil, err
				}
			} else {
				// Integer format
				analogInt := int16(pmu.AnalogValues[j])
				if err := binary.Write(buf, binary.BigEndian, analogInt); err != nil {
					return nil, err
				}
			}
		}

		// Digital values
		for j := 0; j < int(pmu.Dgnmr); j++ {
			var digWord uint16
			for k := 0; k < 16; k++ {
				if pmu.DigitalValues[j][k] {
					digWord |= 1 << uint(k)
				}
			}
			if err := binary.Write(buf, binary.BigEndian, digWord); err != nil {
				return nil, err
			}
		}
	}

	data := buf.Bytes()
	crc := CalcCRC(data)
	if err := binary.Write(buf, binary.BigEndian, crc); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// Unpack parses bytes into data frame
func (d *DataFrame) Unpack(data []byte) error {
	if d.AssociatedConfig == nil {
		return ErrInvalidParameter
	}

	if len(data) < 16 {
		return ErrInvalidSize
	}

	buf := bytes.NewReader(data)

	// Read header
	if err := readBinary(buf, &d.Sync, &d.FrameSize); err != nil {
		return err
	}

	if d.FrameSize < 16 {
		return ErrInvalidSize
	}

	if err := readBinary(buf, &d.IDCode, &d.SOC, &d.FracSec); err != nil {
		return err
	}

	for _, pmu := range d.AssociatedConfig.PMUStationList {
		// STAT
		if err := binary.Read(buf, binary.BigEndian, &pmu.Stat); err != nil {
			return err
		}

		// Phasors
		for j := 0; j < int(pmu.Phnmr); j++ {
			if pmu.FormatPhasorType() {
				// Float format
				var val1, val2 float32
				if err := readBinary(buf, &val1, &val2); err != nil {
					return err
				}

				if pmu.FormatCoord() {
					// Polar: val1=magnitude, val2=angle
					pmu.PhasorValues[j] = cmplx.Rect(float64(val1), float64(val2))
				} else {
					// Rectangular: val1=real, val2=imaginary
					pmu.PhasorValues[j] = complex(float64(val1), float64(val2))
				}
			} else {
				// Integer format
				if pmu.FormatCoord() {
					// Polar
					var mag uint16
					var ang int16
					if err := readBinary(buf, &mag, &ang); err != nil {
						return err
					}

					magFloat := float64(mag) * float64(pmu.GetPhasorFactor(j)) / 1e5
					angFloat := float64(ang) / 1e4
					pmu.PhasorValues[j] = cmplx.Rect(magFloat, angFloat)
				} else {
					// Rectangular
					var re, im int16
					if err := readBinary(buf, &re, &im); err != nil {
						return err
					}

					reFloat := float64(re) * float64(pmu.GetPhasorFactor(j)) / 1e5
					imFloat := float64(im) * float64(pmu.GetPhasorFactor(j)) / 1e5
					pmu.PhasorValues[j] = complex(reFloat, imFloat)
				}
			}
		}

		// Freq and DFreq
		if pmu.FormatFreqType() {
			// Float format
			if err := readBinary(buf, &pmu.Freq, &pmu.DFreq); err != nil {
				return err
			}
		} else {
			// Integer format
			var freqInt int16
			var dfreqInt int16
			if err := readBinary(buf, &freqInt, &dfreqInt); err != nil {
				return err
			}

			pmu.Freq = pmu.GetNominalFrequency() + float32(freqInt)/1000.0
			pmu.DFreq = float32(dfreqInt) / 100.0
		}

		// Analog values
		for j := 0; j < int(pmu.Annmr); j++ {
			if pmu.FormatAnalogType() {
				// Float format
				if err := binary.Read(buf, binary.BigEndian, &pmu.AnalogValues[j]); err != nil {
					return err
				}
			} else {
				// Integer format
				var analogInt int16
				if err := binary.Read(buf, binary.BigEndian, &analogInt); err != nil {
					return err
				}
				pmu.AnalogValues[j] = float32(analogInt)
			}
		}

		// Digital values
		for j := 0; j < int(pmu.Dgnmr); j++ {
			var digWord uint16
			if err := binary.Read(buf, binary.BigEndian, &digWord); err != nil {
				return err
			}
			for k := 0; k < 16; k++ {
				pmu.DigitalValues[j][k] = (digWord & (1 << uint(k))) != 0
			}
		}
	}

	// Read CRC
	if _, err := buf.Seek(int64(d.FrameSize-2), io.SeekStart); err != nil {
		return err
	}
	if err := binary.Read(buf, binary.BigEndian, &d.CHK); err != nil {
		return err
	}

	// Verify CRC
	crcData := data[:d.FrameSize-2]
	if CalcCRC(crcData) != d.CHK {
		return ErrCRCFailed
	}

	return nil
}

// GetMeasurements returns the measurements in a structured format
func (d *DataFrame) GetMeasurements() map[string]interface{} {
	measurements := make([]map[string]interface{}, 0)

	for _, pmu := range d.AssociatedConfig.PMUStationList {
		measurement := map[string]interface{}{
			"stream_id": pmu.IDCode,
			"stat":      pmu.Stat,
			"phasors":   pmu.PhasorValues,
			"analog":    pmu.AnalogValues,
			"digital":   pmu.DigitalValues,
			"frequency": pmu.Freq,
			"rocof":     pmu.DFreq,
		}
		measurements = append(measurements, measurement)
	}

	timestamp := float64(d.SOC) + float64(d.FracSec&0x00FFFFFF)/float64(d.AssociatedConfig.TimeBase)

	return map[string]interface{}{
		"pmu_id":       d.IDCode,
		"time":         timestamp,
		"measurements": measurements,
	}
}
