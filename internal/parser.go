package disasm

import (
	"fmt"

	"github.com/retroenv/nesgodisasm/internal/program"
	. "github.com/retroenv/retrogolib/nes/addressing"
	"github.com/retroenv/retrogolib/nes/cpu"
	"github.com/retroenv/retrogolib/nes/parameter"
)

// followExecutionFlow parses opcodes and follows the execution flow to parse all code.
func (dis *Disasm) followExecutionFlow() error {
	for addr := dis.popTarget(); addr != 0; addr = dis.popTarget() {
		dis.pc = addr
		offset := dis.addressToOffset(dis.pc)
		offsetInfo, inspectCode := dis.initializeOffsetInfo(offset)
		if !inspectCode {
			continue
		}

		instruction := offsetInfo.opcode.Instruction

		if offsetInfo.opcode.Addressing == ImpliedAddressing {
			offsetInfo.Code = instruction.Name
		} else {
			params, err := dis.processParamInstruction(dis.pc, offsetInfo)
			if err != nil {
				return err
			}
			offsetInfo.Code = fmt.Sprintf("%s %s", instruction.Name, params)
		}

		if _, ok := cpu.NotExecutingFollowingOpcodeInstructions[instruction.Name]; !ok {
			opcodeLength := uint16(len(offsetInfo.OpcodeBytes))
			nextTarget := dis.pc + opcodeLength
			dis.addTarget(nextTarget, offsetInfo.context, instruction, false)
		} else {
			dis.checkForJumpEngine(offsetInfo, dis.pc)
		}

		dis.checkInstructionOverlap(offsetInfo, offset)

		if instruction.Name == cpu.NopInstruction && instruction.Unofficial {
			dis.handleUnofficialNop(offset)
			continue
		}

		dis.changeOffsetRangeToCode(offsetInfo.OpcodeBytes, offset)
	}
	return nil
}

// in case the current instruction overlaps with an already existing instruction,
// cut the current one short.
func (dis *Disasm) checkInstructionOverlap(offsetInfo *offset, offset uint16) {
	for i := 1; i < len(offsetInfo.OpcodeBytes) && int(offset)+i < len(dis.offsets); i++ {
		ins := &dis.offsets[offset+uint16(i)]
		if ins.IsType(program.CodeOffset) {
			offsetInfo.OpcodeBytes = offsetInfo.OpcodeBytes[:i]
			return
		}
	}
}

// initializeOffsetInfo initializes the offset info for the given offset and returns
// whether the offset should process inspection for code parameters.
func (dis *Disasm) initializeOffsetInfo(offset uint16) (*offset, bool) {
	offsetInfo := &dis.offsets[offset]

	if offsetInfo.IsType(program.CodeOffset) {
		return offsetInfo, false // was set by CDL
	}

	b := dis.readMemory(dis.pc)
	offsetInfo.OpcodeBytes = make([]byte, 1, 3)
	offsetInfo.OpcodeBytes[0] = b

	if offsetInfo.IsType(program.DataOffset) {
		return offsetInfo, false // was set by CDL
	}

	opcode := cpu.Opcodes[b]
	if opcode.Instruction == nil {
		// consider an unknown instruction as start of data
		offsetInfo.SetType(program.DataOffset)
		return offsetInfo, false
	}

	offsetInfo.opcode = opcode
	return offsetInfo, true
}

// processParamInstruction processes an instruction with parameters.
// Special handling is required as this instruction could branch to a different location.
func (dis *Disasm) processParamInstruction(offset uint16, offsetInfo *offset) (string, error) {
	opcode := offsetInfo.opcode
	param, opcodes := dis.readOpParam(opcode.Addressing)
	offsetInfo.OpcodeBytes = append(offsetInfo.OpcodeBytes, opcodes...)

	paramAsString, err := parameter.String(dis.converter, opcode.Addressing, param)
	if err != nil {
		return "", fmt.Errorf("getting parameter as string: %w", err)
	}

	paramAsString = dis.replaceParamByAlias(offset, opcode, param, paramAsString)

	if _, ok := cpu.BranchingInstructions[opcode.Instruction.Name]; ok {
		addr, ok := param.(Absolute)
		if ok {
			dis.addTarget(uint16(addr), offsetInfo.context, opcode.Instruction, true)
		}
	}

	return paramAsString, nil
}

// replaceParamByAlias replaces the absolute address with an alias name if it can match it to
// a constant, zero page variable or a code reference.
func (dis *Disasm) replaceParamByAlias(offset uint16, opcode cpu.Opcode, param any, paramAsString string) string {
	if _, ok := cpu.BranchingInstructions[opcode.Instruction.Name]; ok {
		return paramAsString
	}

	var address uint16
	switch val := param.(type) {
	case Absolute:
		address = uint16(val)
	case AbsoluteX:
		address = uint16(val)
	case AbsoluteY:
		address = uint16(val)
	case Indirect:
		address = uint16(val)
	case IndirectX:
		address = uint16(val)
	case IndirectY:
		address = uint16(val)
	default:
		return paramAsString
	}

	constantInfo, ok := dis.constants[address]
	if ok {
		return dis.replaceParamByConstant(opcode, paramAsString, address, constantInfo)
	}

	if !dis.addVariableReference(offset, opcode, address) {
		return paramAsString
	}

	// force using absolute address to not generate a different opcode by using zeropage access mode
	// TODO check if other assemblers use the same prefix
	switch opcode.Addressing {
	case ZeroPageAddressing, ZeroPageXAddressing, ZeroPageYAddressing:
		return "z:" + paramAsString
	case AbsoluteAddressing, AbsoluteXAddressing, AbsoluteYAddressing:
		return "a:" + paramAsString
	default: // indirect x, ...
		return paramAsString
	}
}

// popTarget pops the next target to disassemble, if there is no more target to parse, 0 will be returned.
// Return address from function addresses have the lowest priority, to be able to handle
// jump table functions correctly.
func (dis *Disasm) popTarget() uint16 {
	if len(dis.targetsToParse) > 0 {
		addr := dis.targetsToParse[0]
		dis.targetsToParse = dis.targetsToParse[1:]
		return addr
	}

	if len(dis.functionReturnsToParse) > 0 {
		addr := dis.functionReturnsToParse[0]
		dis.functionReturnsToParse = dis.functionReturnsToParse[1:]
		return addr
	}

	return 0
}

// addTarget adds a target to the list to be processed if the address has not been processed yet.
func (dis *Disasm) addTarget(target, context uint16, currentInstruction *cpu.Instruction, isABranchTarget bool) {
	offset := dis.addressToOffset(target)
	offsetInfo := &dis.offsets[offset]

	if currentInstruction != nil && currentInstruction.Name == cpu.JsrInstruction {
		offsetInfo.SetType(program.CallTarget)
		if offsetInfo.context == 0 {
			offsetInfo.context = target // begin a new context
		}
	} else if offsetInfo.context == 0 {
		offsetInfo.context = context // continue current context
	}

	if isABranchTarget {
		offsetInfo.branchFrom = append(offsetInfo.branchFrom, dis.pc)
		dis.branchTargets[target] = struct{}{}
	}

	if _, ok := dis.targetsAdded[target]; ok {
		return
	}
	dis.targetsAdded[target] = struct{}{}

	if currentInstruction != nil && currentInstruction.Name == cpu.JsrInstruction {
		dis.functionReturnsToParse = append(dis.functionReturnsToParse, target)
	} else {
		dis.targetsToParse = append(dis.targetsToParse, target)
	}
}

// checkForJumpEngine checks if the current instruction is the jump inside a jump engine function.
// The function offsets after the call to the jump engine will be used as targets to disassemble as code.
// This can be found in some official games like Super Mario Bros.
func (dis *Disasm) checkForJumpEngine(offsetInfo *offset, jumpAddress uint16) {
	instruction := offsetInfo.opcode.Instruction
	if instruction.Name != cpu.JmpInstruction || offsetInfo.opcode.Addressing != IndirectAddressing {
		return
	}

	// parse all instructions of the function context until the jump
	for addr := offsetInfo.context; addr != 0 && addr > jumpAddress; {
		addr = dis.addressToOffset(addr)
		offsetInfo = &dis.offsets[addr]

		// if current function has a branching instruction beside the final jump, it's probably not one
		// of the common jump engines
		if _, ok := cpu.BranchingInstructions[offsetInfo.opcode.Instruction.Name]; ok {
			return
		}

		addr += uint16(len(offsetInfo.OpcodeBytes))
	}

	// if code reaches this point, no branching instructions beside the final indirect jmp have been found
	// in the function, this makes it a likely jump engine
	dis.jumpEngines[offsetInfo.context] = struct{}{}

	// TODO parse caller of jump engine
}
