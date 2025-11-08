package m6502

import (
	"github.com/retroenv/retrodisasm/internal/instruction"
	"github.com/retroenv/retrogolib/arch/cpu/m6502"
)

// GetAddressingParam returns the address of the param if it references an address.
func (ar *Arch6502) GetAddressingParam(param any) (uint16, bool) {
	switch val := param.(type) {
	case m6502.Absolute:
		return uint16(val), true
	case m6502.AbsoluteX:
		return uint16(val), true
	case m6502.AbsoluteY:
		return uint16(val), true
	case m6502.Indirect:
		return uint16(val), true
	case m6502.IndirectX:
		return uint16(val), true
	case m6502.IndirectY:
		return uint16(val), true
	case m6502.ZeroPage:
		return uint16(val), true
	case m6502.ZeroPageX:
		return uint16(val), true
	case m6502.ZeroPageY:
		return uint16(val), true
	default:
		return 0, false
	}
}

// IsAddressingIndexed returns if the opcode is using indexed addressing.
func (ar *Arch6502) IsAddressingIndexed(opcode instruction.Opcode) bool {
	addressing := m6502.AddressingMode(opcode.Addressing())
	switch addressing {
	case m6502.ZeroPageXAddressing, m6502.ZeroPageYAddressing,
		m6502.AbsoluteXAddressing, m6502.AbsoluteYAddressing,
		m6502.IndirectXAddressing, m6502.IndirectYAddressing:
		return true
	default:
		return false
	}
}
