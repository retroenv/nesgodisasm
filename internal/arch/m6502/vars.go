package m6502

import (
	"fmt"

	"github.com/retroenv/nesgodisasm/internal/arch"
	"github.com/retroenv/retrogolib/arch/cpu/m6502"
	"github.com/retroenv/retrogolib/arch/nes/parameter"
)

func (ar *Arch6502) ProcessVariableUsage(offsetInfo *arch.Offset, reference string) error {
	addressing := m6502.AddressingMode(offsetInfo.Opcode.Addressing())
	converted, err := parameter.String(ar.converter, addressing, reference)
	if err != nil {
		return fmt.Errorf("getting parameter as string: %w", err)
	}

	name := offsetInfo.Opcode.Instruction().Name()
	switch addressing {
	case m6502.ZeroPageAddressing, m6502.ZeroPageXAddressing, m6502.ZeroPageYAddressing:
		offsetInfo.Code = fmt.Sprintf("%s %s", name, converted)
	case m6502.AbsoluteAddressing, m6502.AbsoluteXAddressing, m6502.AbsoluteYAddressing:
		offsetInfo.Code = fmt.Sprintf("%s %s", name, converted)
	case m6502.IndirectAddressing, m6502.IndirectXAddressing, m6502.IndirectYAddressing:
		offsetInfo.Code = fmt.Sprintf("%s %s", name, converted)
	}

	return nil
}
