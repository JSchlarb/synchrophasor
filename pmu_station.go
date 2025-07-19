package synchrophasor

// PMUStation represents a PMU station configuration
type PMUStation struct {
	C37118
	STN           string
	Format        uint16
	Phnmr         uint16
	Annmr         uint16
	Dgnmr         uint16
	CHNAMPhasor   []string
	CHNAMAnalog   []string
	CHNAMDigital  []string
	Phunit        []uint32
	Anunit        []uint32
	Dgunit        []uint32
	Fnom          uint16
	CfgCnt        uint16
	Stat          uint16
	PhasorValues  []complex128
	AnalogValues  []float32
	DigitalValues [][]bool
	Freq          float32
	DFreq         float32
}

// NewPMUStation creates a new PMU station with given parameters
func NewPMUStation(name string, idCode uint16, freqType, analogType, phasorType, coordType bool) *PMUStation {
	pmu := &PMUStation{
		STN:           name,
		Phnmr:         0,
		Annmr:         0,
		Dgnmr:         0,
		CHNAMPhasor:   make([]string, 0),
		CHNAMAnalog:   make([]string, 0),
		CHNAMDigital:  make([]string, 0),
		Phunit:        make([]uint32, 0),
		Anunit:        make([]uint32, 0),
		Dgunit:        make([]uint32, 0),
		PhasorValues:  make([]complex128, 0),
		AnalogValues:  make([]float32, 0),
		DigitalValues: make([][]bool, 0),
	}
	pmu.IDCode = idCode
	pmu.SetFormat(freqType, analogType, phasorType, coordType)
	return pmu
}

// SetFormat sets the format word
func (p *PMUStation) SetFormat(freqType, analogType, phasorType, coordType bool) {
	p.Format = 0
	if coordType {
		p.Format |= 1
	}
	if phasorType {
		p.Format |= 1 << 1
	}
	if analogType {
		p.Format |= 1 << 2
	}
	if freqType {
		p.Format |= 1 << 3
	}
}

// FormatCoord returns true if phasor format is polar
func (p *PMUStation) FormatCoord() bool {
	return (p.Format & 0x01) != 0
}

// FormatPhasorType returns true if phasor format is float
func (p *PMUStation) FormatPhasorType() bool {
	return (p.Format & 0x02) != 0
}

// FormatAnalogType returns true if analog format is float
func (p *PMUStation) FormatAnalogType() bool {
	return (p.Format & 0x04) != 0
}

// FormatFreqType returns true if freq/dfreq format is float
func (p *PMUStation) FormatFreqType() bool {
	return (p.Format & 0x08) != 0
}

// AddPhasor adds a phasor channel
func (p *PMUStation) AddPhasor(name string, factor uint32, phType uint8) {
	name = padString(name)
	p.CHNAMPhasor = append(p.CHNAMPhasor, name)
	p.Phunit = append(p.Phunit, (uint32(phType)<<24)|(factor&0x0FFFFFF))
	p.Phnmr++
	p.PhasorValues = append(p.PhasorValues, complex(0, 0))
}

// AddAnalog adds an analog channel
func (p *PMUStation) AddAnalog(name string, factor uint32, anType uint8) {
	name = padString(name)
	p.CHNAMAnalog = append(p.CHNAMAnalog, name)
	p.Anunit = append(p.Anunit, (uint32(anType)<<24)|(factor&0x0FFFFFF))
	p.Annmr++
	p.AnalogValues = append(p.AnalogValues, 0.0)
}

// AddDigital adds a digital channel with 16 bits
func (p *PMUStation) AddDigital(names []string, normal, valid uint16) {
	for _, name := range names {
		name = padString(name)
		p.CHNAMDigital = append(p.CHNAMDigital, name)
	}
	p.Dgunit = append(p.Dgunit, (uint32(normal)<<16)|uint32(valid))
	p.Dgnmr++
	p.DigitalValues = append(p.DigitalValues, make([]bool, 16))
}

// GetPhasorFactor returns the factor for a phasor channel
func (p *PMUStation) GetPhasorFactor(index int) uint32 {
	if index >= len(p.Phunit) {
		return 1
	}
	return p.Phunit[index] & 0x0FFFFFF
}

// GetNominalFrequency returns the nominal frequency based on Fnom setting
func (p *PMUStation) GetNominalFrequency() float32 {
	if p.Fnom == FreqNom50Hz {
		return 50.0
	}
	return 60.0
}
