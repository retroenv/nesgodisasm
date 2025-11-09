package program

import (
	"github.com/retroenv/retrodisasm/internal/options"
)

// NewPRGBank creates a new PRG bank.
func NewPRGBank(size int) *PRGBank {
	return &PRGBank{
		Offsets:   make([]Offset, size),
		Constants: map[string]uint16{},
		Variables: map[string]uint16{},
	}
}

// PRGBank defines a PRG bank.
type PRGBank struct {
	Name string

	Offsets []Offset
	Vectors [3]uint16

	Constants map[string]uint16
	Variables map[string]uint16
}

// LastNonZeroByte searches for the last byte in PRG that is not zero.
func (bank PRGBank) LastNonZeroByte(options options.Disassembler) int {
	endIndex := len(bank.Offsets) - 6 // leave space for vectors
	if options.ZeroBytes {
		return endIndex
	}

	start := len(bank.Offsets) - 1 - 6 // skip irq pointers

	for i := start; i >= 0; i-- {
		offset := bank.Offsets[i]
		if (len(offset.Data) == 0 || offset.Data[0] == 0) && offset.Label == "" {
			continue
		}
		return i + 1
	}

	return endIndex
}
