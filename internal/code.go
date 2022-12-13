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
		offset := dis.addressToOffset(address)
		offsetInfo := &dis.offsets[offset]

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
			dis.handleJumpIntoInstruction(offset)
		}

		for _, caller := range offsetInfo.branchFrom {
			caller = dis.addressToOffset(caller)
			offset := &dis.offsets[caller]
			offset.branchingTo = name

			// reference can be a function address of a jump engine
			if offset.IsType(program.CodeOffset) {
				offset.Code = offset.opcode.Instruction.Name
			}
		}
	}
}

// handleJumpIntoInstruction converts an instruction that has a jump destination label inside
// its second or third opcode bytes into data.
func (dis *Disasm) handleJumpIntoInstruction(offset uint16) {
	// look backwards for instruction start
	instructionStart := offset - 1
	for ; len(dis.offsets[instructionStart].OpcodeBytes) == 0; instructionStart-- {
	}

	ins := &dis.offsets[instructionStart]
	ins.Comment = fmt.Sprintf("branch into instruction detected: %s", ins.Code)
	ins.Code = ""
	ins.SetType(program.CodeAsData)
	dis.changeOffsetRangeToData(ins.OpcodeBytes, instructionStart)
}

// handleUnofficialNop translates unofficial nop codes into data bytes as the instruction
// has multiple opcodes for the same addressing mode which can result in different
// bytes being assembled and make the resulting ROM not match the original.
func (dis *Disasm) handleUnofficialNop(offset uint16) {
	ins := &dis.offsets[offset]
	if ins.Code == "" { // in case of branch into unofficial nop instruction detected
		ins.Comment = fmt.Sprintf("unofficial nop instruction: %s", ins.Comment)
	} else {
		ins.Comment = fmt.Sprintf("unofficial nop instruction: %s", ins.Code)
	}
	ins.Code = ""
	ins.SetType(program.CodeAsData)
	dis.changeOffsetRangeToData(ins.OpcodeBytes, offset)
}

// changeOffsetRangeToCode sets a range of code offsets to code types.
func (dis *Disasm) changeOffsetRangeToCode(data []byte, offset uint16) {
	for i := 0; i < len(data) && int(offset)+i < len(dis.offsets); i++ {
		ins := &dis.offsets[offset+uint16(i)]
		ins.SetType(program.CodeOffset)
	}
}
