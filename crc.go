package synchrophasor

import "github.com/sigurn/crc16"

var ieeeC37118Params = crc16.Params{
	Poly:   0x1021,
	Init:   0xFFFF,
	RefIn:  false,
	RefOut: false,
	XorOut: 0x0000,
	Name:   "CRC-16/IEEE-C37.118",
}

var crcTable = crc16.MakeTable(ieeeC37118Params)

// CalcCRC calculates CRC-CCITT for the given data
func CalcCRC(data []byte) uint16 {
	return crc16.Checksum(data, crcTable)
}
