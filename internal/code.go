package disasm

import (
	"fmt"

	"github.com/retroenv/nesgodisasm/internal/program"
)

const (
	funcNaming       = "_func_%04x"
	jumpEngineNaming = "_jump_engine_%04x"
	labelNaming      = "_label_%04x"
)

// processJumpDestinations processes all jump destinations and updates the callers with
// the generated jump destination label name.
func (dis *Disasm) processJumpDestinations() {
	for address := range dis.branchDestinations {
		index := dis.addressToIndex(address)
		offsetInfo := &dis.offsets[index]

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
		if offsetInfo.IsType(program.CodeOffset) && len(offsetInfo.OpcodeBytes) == 0 {
			dis.handleJumpIntoInstruction(index)
		}

		for _, caller := range offsetInfo.branchFrom {
			index = dis.addressToIndex(caller)
			offsetInfo = &dis.offsets[index]
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
func (dis *Disasm) handleJumpIntoInstruction(index uint16) {
	// look backwards for instruction start
	instructionStart := index - 1
	for ; len(dis.offsets[instructionStart].OpcodeBytes) == 0; instructionStart-- {
	}

	offsetInfo := &dis.offsets[instructionStart]
	offsetInfo.Comment = fmt.Sprintf("branch into instruction detected: %s", offsetInfo.Code)
	offsetInfo.Code = ""
	offsetInfo.SetType(program.CodeAsData)
	dis.changeOffsetRangeToData(offsetInfo.OpcodeBytes, instructionStart)
}

// handleUnofficialNop translates unofficial nop codes into data bytes as the instruction
// has multiple opcodes for the same addressing mode which can result in different
// bytes being assembled and make the resulting ROM not match the original.
func (dis *Disasm) handleUnofficialNop(index uint16) {
	offsetInfo := &dis.offsets[index]
	if offsetInfo.Code == "" { // in case of branch into unofficial nop instruction detected
		offsetInfo.Comment = fmt.Sprintf("unofficial nop instruction: %s", offsetInfo.Comment)
	} else {
		offsetInfo.Comment = fmt.Sprintf("unofficial nop instruction: %s", offsetInfo.Code)
	}
	offsetInfo.Code = ""
	offsetInfo.SetType(program.CodeAsData)
	dis.changeOffsetRangeToData(offsetInfo.OpcodeBytes, index)
}

// changeIndexRangeToCode sets a range of code offsets to code types.
func (dis *Disasm) changeIndexRangeToCode(data []byte, index uint16) {
	for i := 0; i < len(data) && int(index)+i < len(dis.offsets); i++ {
		offsetInfo := &dis.offsets[index+uint16(i)]
		offsetInfo.SetType(program.CodeOffset)
	}
}
