// Package program represents an NES program.
package program

import (
	"io"

	"github.com/retroenv/retrogolib/arch/nes/cartridge"
)

// WriteCallbackFunc is a custom callback function that gets called before the offset is written.
type WriteCallbackFunc func(writer io.Writer) error

// Offset defines the content of an offset in a program that can represent data or code.
type Offset struct {
	// data byte or all opcode bytes that are part of the instruction
	OpcodeBytes []byte
	// WriteCallback is a custom callback function that gets called before the offset is
	// written, this allows custom bank switch code to be written at specific offsets.
	// The output of bundled data bytes will be interrupted.
	WriteCallback WriteCallbackFunc

	Type OffsetType

	Address           uint16
	HasAddressComment bool

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
	PRG     []*PRGBank // PRG-ROM banks
	CHR     CHR        // CHR-ROM data
	RAM     byte       // PRG-RAM offsets
	Trainer []byte

	CodeBaseAddress     uint16
	VectorsStartAddress uint16

	Checksums   Checksums
	Handlers    Handlers
	Battery     byte
	Mirror      cartridge.MirrorMode
	Mapper      byte
	VideoFormat byte

	// keep constants and variables in the banks and global in the app to let the chosen assembler decide
	// how to output them
	Constants map[string]uint16
	Variables map[string]uint16
}

// New creates a new program initialize with a program code size.
func New(cart *cartridge.Cartridge) *Program {
	return &Program{
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

// PrgSize returns the overall size of all PRG banks.
func (p Program) PrgSize() int {
	var size int
	for _, bnk := range p.PRG {
		size += len(bnk.PRG)
	}
	return size
}
