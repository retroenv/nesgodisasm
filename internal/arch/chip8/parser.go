package chip8

import (
	"fmt"

	"github.com/retroenv/nesgodisasm/internal/arch"
	"github.com/retroenv/nesgodisasm/internal/program"
	"github.com/retroenv/retrogolib/arch/cpu/chip8"
)

// initializeOffsetInfo initializes the offset info and returns
// whether the offset should process inspection for code parameters.
func initializeOffsetInfo(dis arch.Disasm, offsetInfo *arch.Offset) (bool, error) {
	if offsetInfo.IsType(program.CodeOffset) {
		return false, nil // was set by CDL
	}

	pc := dis.ProgramCounter()
	b1, err := dis.ReadMemory(pc)
	if err != nil {
		return false, fmt.Errorf("reading memory at address %04x: %w", pc, err)
	}
	offsetInfo.Data = make([]byte, 1, opcodeSize)
	offsetInfo.Data[0] = b1

	if offsetInfo.IsType(program.DataOffset) {
		return false, nil // was set by CDL
	}

	b2, err := dis.ReadMemory(pc + 1)
	if err != nil {
		return false, fmt.Errorf("reading memory at address %04x: %w", pc+1, err)
	}
	offsetInfo.Data = append(offsetInfo.Data, b2)

	w := uint16(b1)<<8 | uint16(b2)
	firstNibble := (w & 0xF000) >> 12
	opcodes := chip8.Opcodes[int(firstNibble)]
	var opcode chip8.Opcode
	for _, op := range opcodes {
		if op.Info.Mask&w == op.Info.Value {
			opcode = op
			break
		}
	}
	if opcode.Instruction == nil {
		// consider an unknown instruction as start of data
		offsetInfo.SetType(program.DataOffset)
		return false, nil
	}

	op := &Opcode{
		op: opcode,
	}
	offsetInfo.Opcode = op
	return true, nil
}
