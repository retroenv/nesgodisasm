package m6502

import (
	"github.com/retroenv/retrodisasm/internal/arch"
	"github.com/retroenv/retrogolib/arch/cpu/m6502"
)

var _ arch.Opcode = &Opcode{}

type Opcode struct {
	op m6502.Opcode
}

func (o Opcode) Addressing() int {
	return int(o.op.Addressing)
}

func (o Opcode) Instruction() arch.Instruction {
	return Instruction{ins: o.op.Instruction}
}

func (o Opcode) ReadsMemory() bool {
	return o.op.ReadsMemory(m6502.MemoryReadInstructions)
}

func (o Opcode) WritesMemory() bool {
	return o.op.WritesMemory(m6502.MemoryWriteInstructions)
}

func (o Opcode) ReadWritesMemory() bool {
	return o.op.ReadWritesMemory(m6502.MemoryReadWriteInstructions)
}
