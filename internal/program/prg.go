package program

import (
	"errors"

	"github.com/retroenv/nesgodisasm/internal/options"
)

// NewPRGBank creates a new PRG bank.
func NewPRGBank(size int) *PRGBank {
	return &PRGBank{
		PRG:       make([]Offset, size),
		Constants: map[string]uint16{},
		Variables: map[string]uint16{},
	}
}

// PRGBank defines a PRG bank.
type PRGBank struct {
	PRG       []Offset // PRG-ROM banks
	Constants map[string]uint16
	Variables map[string]uint16
}

// GetLastNonZeroByte searches for the last byte in PRG that is not zero.
func (bank PRGBank) GetLastNonZeroByte(options *options.Disassembler) (int, error) {
	endIndex := len(bank.PRG) - 6 // leave space for vectors
	if options.ZeroBytes {
		return endIndex, nil
	}

	start := len(bank.PRG) - 1 - 6 // skip irq pointers

	for i := start; i >= 0; i-- {
		offset := bank.PRG[i]
		if (len(offset.OpcodeBytes) == 0 || offset.OpcodeBytes[0] == 0) && offset.Label == "" {
			continue
		}
		return i + 1, nil
	}
	return 0, errors.New("could not find last zero byte")
}
