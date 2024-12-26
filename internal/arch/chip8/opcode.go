package chip8

import (
	"github.com/retroenv/nesgodisasm/internal/arch"
	"github.com/retroenv/retrogolib/arch/cpu/chip8"
)

const opcodeSize = 2

var _ arch.Opcode = &Opcode{}

type Opcode struct {
	op         chip8.Opcode
	addressing int
}

func (o Opcode) Addressing() int {
	return int(o.op.Info.Mask)
}

func (o Opcode) Instruction() arch.Instruction {
	return Instruction{ins: o.op.Instruction}
}

func (o Opcode) ReadsMemory() bool {
	return o.op.ReadsMemory(chip8.MemoryReadInstructions)
}

func (o Opcode) WritesMemory() bool {
	return o.op.WritesMemory(chip8.MemoryWriteInstructions)
}

func (o Opcode) ReadWritesMemory() bool {
	return o.op.ReadWritesMemory(chip8.MemoryReadWriteInstructions)
}
