package disasm

import (
	"fmt"
	"slices"

	"github.com/retroenv/nesgodisasm/internal/program"
	"github.com/retroenv/retrogolib/arch/cpu/m6502"
)

const (
	funcNaming       = "_func_%04x"
	jumpEngineNaming = "_jump_engine_%04x"
	labelNaming      = "_label_%04x"
)

// processJumpDestinations processes all jump destinations and updates the callers with
// the generated jump destination label name.
func (dis *Disasm) processJumpDestinations() {
	branchDestinations := make([]uint16, 0, len(dis.branchDestinations))
	for dest := range dis.branchDestinations {
		branchDestinations = append(branchDestinations, dest)
	}
	slices.Sort(branchDestinations)

	for _, address := range branchDestinations {
		offsetInfo := dis.mapper.offsetInfo(address)

		name := offsetInfo.Label
		if name == "" {
			switch {
			case offsetInfo.IsType(program.JumpEngine):
				name = fmt.Sprintf(jumpEngineNaming, address)
			case offsetInfo.IsType(program.CallDestination):
				name = fmt.Sprintf(funcNaming, address)
			default:
				name = fmt.Sprintf(labelNaming, address)
			}
			offsetInfo.Label = name
		}

		// if the offset is marked as code but does not have opcode bytes, the jump destination
		// is inside the second or third byte of an instruction.
		if (offsetInfo.IsType(program.CodeOffset) || offsetInfo.IsType(program.CodeAsData)) &&
			len(offsetInfo.OpcodeBytes) == 0 {
			dis.handleJumpIntoInstruction(address)
		}

		for _, bankRef := range offsetInfo.branchFrom {
			offsetInfo = bankRef.mapped.offsetInfo(bankRef.index)
			offsetInfo.branchingTo = name

			// reference can be a function address of a jump engine
			if offsetInfo.IsType(program.CodeOffset) {
				offsetInfo.Code = offsetInfo.opcode.Instruction.Name
			}
		}
	}
}

// handleJumpIntoInstruction converts an instruction that has a jump destination label inside
// its second or third opcode bytes into data.
func (dis *Disasm) handleJumpIntoInstruction(address uint16) {
	// look backwards for instruction start
	address--

	for offsetInfo := dis.mapper.offsetInfo(address); len(offsetInfo.OpcodeBytes) == 0; {
		address--
		offsetInfo = dis.mapper.offsetInfo(address)
	}

	offsetInfo := dis.mapper.offsetInfo(address)
	if offsetInfo.Code == "" { // disambiguous instruction
		offsetInfo.Comment = "branch into instruction detected: " + offsetInfo.Comment
	} else {
		offsetInfo.Comment = "branch into instruction detected: " + offsetInfo.Code
		offsetInfo.Code = ""
	}

	offsetInfo.SetType(program.CodeAsData)
	dis.changeAddressRangeToCodeAsData(address, offsetInfo.OpcodeBytes)
}

// handleUnofficialNop translates disambiguous instructions into data bytes as it
// has multiple opcodes for the same addressing mode which can result in different
// bytes being assembled and make the resulting ROM not matching the original.
func (dis *Disasm) handleDisambiguousInstructions(address uint16, offsetInfo *offset) bool {
	instruction := offsetInfo.opcode.Instruction
	if !instruction.Unofficial || address >= irqStartAddress {
		return false
	}

	if instruction.Name != m6502.Nop.Name &&
		instruction.Name != m6502.Sbc.Name &&
		!dis.noUnofficialInstruction {
		return false
	}

	if offsetInfo.Code == "" { // in case of branch into unofficial nop instruction detected
		offsetInfo.Comment = "disambiguous instruction: " + offsetInfo.Comment
	} else {
		offsetInfo.Comment = "disambiguous instruction: " + offsetInfo.Code
	}

	offsetInfo.Code = ""
	offsetInfo.SetType(program.CodeAsData)
	dis.changeAddressRangeToCodeAsData(address, offsetInfo.OpcodeBytes)
	return true
}

// changeAddressRangeToCode sets a range of code addresses to code types.
func (dis *Disasm) changeAddressRangeToCode(address uint16, data []byte) {
	for i := 0; i < len(data) && int(address)+i < irqStartAddress; i++ {
		offsetInfo := dis.mapper.offsetInfo(address + uint16(i))
		offsetInfo.SetType(program.CodeOffset)
	}
}
