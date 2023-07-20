// Package program represents an NES program.
package program

import (
	"errors"

	"github.com/retroenv/nesgodisasm/internal/options"
	"github.com/retroenv/retrogolib/arch/nes/cartridge"
)

// Offset defines the content of an offset in a program that can represent data or code.
type Offset struct {
	OpcodeBytes []byte // data byte or all opcode bytes that are part of the instruction

	Type OffsetType

	Label        string // name of label or subroutine if identified as a jump destination
	Code         string // asm output of this instruction
	Comment      string
	LabelComment string
}

// Handlers defines the handlers that the NES can jump to.
type Handlers struct {
	NMI   string
	Reset string
	IRQ   string
}

// Checksums contains the CRC32 checksums to identify the PRG and CHR parts of the ROM.
type Checksums struct {
	PRG     uint32
	CHR     uint32
	Overall uint32
}

// Program defines an NES program that contains code or data.
type Program struct {
	PRG     []Offset // PRG-ROM banks
	CHR     []byte   // CHR-ROM banks
	RAM     byte     // PRG-RAM banks
	Trainer []byte

	CodeBaseAddress uint16
	Checksums       Checksums
	Handlers        Handlers
	Battery         byte
	Mirror          cartridge.MirrorMode
	Mapper          byte
	VideoFormat     byte

	Constants map[string]uint16
	Variables map[string]uint16
}

// New creates a new program initialize with a program code size.
func New(cart *cartridge.Cartridge) *Program {
	return &Program{
		PRG:       make([]Offset, len(cart.PRG)),
		CHR:       cart.CHR,
		RAM:       cart.RAM,
		Battery:   cart.Battery,
		Mapper:    cart.Mapper,
		Mirror:    cart.Mirror,
		Trainer:   cart.Trainer,
		Constants: map[string]uint16{},
		Variables: map[string]uint16{},
	}
}

// GetLastNonZeroPRGByte searches for the last byte in PRG that is not zero.
func (app *Program) GetLastNonZeroPRGByte(options *options.Disassembler) (int, error) {
	endIndex := len(app.PRG) - 6 // leave space for vectors
	if options.ZeroBytes {
		return endIndex, nil
	}

	start := len(app.PRG) - 1 - 6 // skip irq pointers

	for i := start; i >= 0; i-- {
		offset := app.PRG[i]
		if (len(offset.OpcodeBytes) == 0 || offset.OpcodeBytes[0] == 0) && offset.Label == "" {
			continue
		}
		return i + 1, nil
	}
	return 0, errors.New("could not find last zero byte")
}

// GetLastNonZeroCHRByte searches for the last byte in CHR that is not zero.
func (app *Program) GetLastNonZeroCHRByte() int {
	for i := len(app.CHR) - 1; i >= 0; i-- {
		b := app.CHR[i]
		if b == 0 {
			continue
		}
		return i + 1
	}
	return 0
}
