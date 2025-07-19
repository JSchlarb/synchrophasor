package synchrophasor

import (
	"encoding/binary"
	"io"
	"strings"
)

const _padLength = 16

// padString pads a string to specified length
func padString(s string) string {
	if len(s) >= _padLength {
		return s[:_padLength]
	}
	return s + strings.Repeat(" ", _padLength-len(s))
}

// writeBinary writes multiple values to a writer using binary.BigEndian
func writeBinary(w io.Writer, values ...interface{}) error {
	for _, v := range values {
		if err := binary.Write(w, binary.BigEndian, v); err != nil {
			return err
		}
	}
	return nil
}

// readBinary reads multiple values from a reader using binary.BigEndian
func readBinary(r io.Reader, values ...interface{}) error {
	for _, v := range values {
		if err := binary.Read(r, binary.BigEndian, v); err != nil {
			return err
		}
	}
	return nil
}
