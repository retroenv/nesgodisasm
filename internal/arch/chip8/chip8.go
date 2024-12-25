// Package chip8 provides a CHIP-8 architecture specific disassembler code.
package chip8

import (
	"github.com/retroenv/nesgodisasm/internal/arch"
	"github.com/retroenv/retrogolib/arch/nes/parameter"
)

var _ arch.Architecture = &Chip8{}

// New returns a new 6502 architecture configuration.
func New(converter parameter.Converter) *Chip8 {
	return &Chip8{
		converter: converter,
	}
}

type Chip8 struct {
	converter parameter.Converter
}

func (c Chip8) Constants() (map[uint16]arch.Constant, error) {
	return map[uint16]arch.Constant{}, nil
}

func (c Chip8) GetAddressingParam(param any) (uint16, bool) {
	panic("implement me")
}

func (c Chip8) HandleDisambiguousInstructions(dis arch.Disasm, address uint16, offsetInfo *arch.Offset) bool {
	panic("implement me")
}

func (c Chip8) Initialize(dis arch.Disasm) error {
	return nil
}

func (c Chip8) IsAddressingIndexed(opcode arch.Opcode) bool {
	panic("implement me")
}

func (c Chip8) LastCodeAddress() uint16 {
	panic("implement me")
}

func (c Chip8) ProcessOffset(dis arch.Disasm, address uint16, offsetInfo *arch.Offset) (bool, error) {
	panic("implement me")
}

func (c Chip8) ProcessVariableUsage(offsetInfo *arch.Offset, reference string) error {
	panic("implement me")
}

func (c Chip8) ReadOpParam(dis arch.Disasm, addressing int, address uint16) (any, []byte, error) {
	panic("implement me")
}
