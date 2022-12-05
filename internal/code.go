package disasm

import (
	"fmt"

	"github.com/retroenv/nesgodisasm/internal/program"
)

// processJumpDestinations processes all jump targets and updates the callers with
// the generated jump target label name.
func (dis *Disasm) processJumpDestinations() {
	for target := range dis.branchDestinations {
		offset := dis.addressToOffset(target)
		name := dis.offsets[offset].Label
		if name == "" {
			if dis.offsets[offset].IsType(program.CallTarget) {
				name = fmt.Sprintf("_func_%04x", target)
			} else {
				name = fmt.Sprintf("_label_%04x", target)
			}
			dis.offsets[offset].Label = name
		}

		// if the offset is marked as code but does not have opcode bytes, the jumping target
		// is inside the second or third byte of an instruction.
		if dis.offsets[offset].IsType(program.CodeOffset) && len(dis.offsets[offset].OpcodeBytes) == 0 {
			dis.handleJumpIntoInstruction(offset)
		}

		for _, caller := range dis.offsets[offset].branchFrom {
			offset = dis.addressToOffset(caller)
			dis.offsets[offset].Code = dis.offsets[offset].opcode.Instruction.Name
			dis.offsets[offset].branchingTo = name
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
// has multiple opcodes for the same addressing mode which will result in a different
// bytes being assembled.
func (dis *Disasm) handleUnofficialNop(offset uint16) {
	ins := &dis.offsets[offset]
	ins.Comment = fmt.Sprintf("unofficial nop instruction: %s", ins.Code)
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
