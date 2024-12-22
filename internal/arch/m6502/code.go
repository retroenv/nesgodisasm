package m6502

import (
	"github.com/retroenv/nesgodisasm/internal/arch"
	"github.com/retroenv/nesgodisasm/internal/program"
	"github.com/retroenv/retrogolib/arch/cpu/m6502"
)

// HandleDisambiguousInstructions translates disambiguous instructions into data bytes as it
// has multiple opcodes for the same addressing mode which can result in different
// bytes being assembled and make the resulting ROM not matching the original.
func (ar *Arch6502) HandleDisambiguousInstructions(dis arch.Disasm, address uint16, offsetInfo arch.Offset) bool {
	instruction := offsetInfo.Opcode().Instruction()
	if !instruction.Unofficial() || address >= m6502.InterruptVectorStartAddress {
		return false
	}

	opts := dis.Options()
	if instruction.Name() != m6502.Nop.Name &&
		instruction.Name() != m6502.Sbc.Name &&
		!opts.NoUnofficialInstructions {

		return false
	}

	code := offsetInfo.Code()
	if code == "" { // in case of branch into unofficial nop instruction detected
		offsetInfo.SetComment("disambiguous instruction: " + offsetInfo.Comment())
	} else {
		offsetInfo.SetComment("disambiguous instruction: " + offsetInfo.Code())
	}

	offsetInfo.SetCode("")
	offsetInfo.SetType(program.CodeAsData)
	dis.ChangeAddressRangeToCodeAsData(address, offsetInfo.Data())
	return true
}
