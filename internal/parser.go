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
	for addr := dis.addressToDisassemble(); addr != 0; addr = dis.addressToDisassemble() {
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

		if _, ok := cpu.NotExecutingFollowingOpcodeInstructions[instruction.Name]; ok {
			dis.checkForJumpEngineJmp(offsetInfo, dis.pc)
		} else {
			opcodeLength := uint16(len(offsetInfo.OpcodeBytes))
			followingOpcodeAddress := dis.pc + opcodeLength
			dis.addAddressToParse(followingOpcodeAddress, offsetInfo.context, addr, instruction, false)
			dis.checkForJumpEngineCall(offsetInfo, dis.pc)
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
			ins.Comment = "branch into instruction detected"
			offsetInfo.Comment = offsetInfo.Code
			offsetInfo.OpcodeBytes = offsetInfo.OpcodeBytes[:i]
			offsetInfo.Code = ""
			offsetInfo.ClearType(program.CodeOffset)
			offsetInfo.SetType(program.CodeAsData | program.DataOffset)
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
			dis.addAddressToParse(uint16(addr), offsetInfo.context, dis.pc, opcode.Instruction, true)
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

	address, ok := getAddressingParam(param)
	if !ok {
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

// addressToDisassemble returns the next address to disassemble, if there are no more addresses to parse,
// 0 will be returned. Return address from function addresses have the lowest priority, to be able to
// handle jump table functions correctly.
func (dis *Disasm) addressToDisassemble() uint16 {
	for {
		if len(dis.offsetsToParse) > 0 {
			addr := dis.offsetsToParse[0]
			dis.offsetsToParse = dis.offsetsToParse[1:]
			return addr
		}

		for len(dis.functionReturnsToParse) > 0 {
			addr := dis.functionReturnsToParse[0]
			dis.functionReturnsToParse = dis.functionReturnsToParse[1:]

			_, ok := dis.functionReturnsToParseAdded[addr]
			// if the address was removed from the set it marks the address as not being parsed anymore,
			// this way is more efficient than iterating the slice to delete the element
			if !ok {
				continue
			}
			delete(dis.functionReturnsToParseAdded, addr)
			return addr
		}

		if !dis.scanForNewJumpEngineEntry() {
			return 0
		}
	}
}

// addAddressToParse adds an address to the list to be processed if the address has not been processed yet.
func (dis *Disasm) addAddressToParse(address, context, from uint16, currentInstruction *cpu.Instruction,
	isABranchDestination bool) {

	offset := dis.addressToOffset(address)
	offsetInfo := &dis.offsets[offset]

	if isABranchDestination && currentInstruction != nil && currentInstruction.Name == cpu.JsrInstruction {
		offsetInfo.SetType(program.CallDestination)
		if offsetInfo.context == 0 {
			offsetInfo.context = address // begin a new context
		}
	} else if offsetInfo.context == 0 {
		offsetInfo.context = context // continue current context
	}

	if isABranchDestination {
		if from > 0 {
			offsetInfo.branchFrom = append(offsetInfo.branchFrom, from)
		}
		dis.branchDestinations[address] = struct{}{}
	}

	if _, ok := dis.offsetsToParseAdded[address]; ok {
		return
	}
	dis.offsetsToParseAdded[address] = struct{}{}

	// add instructions that follow a function call to a special queue with lower priority, to allow the
	// jump engine be detected before trying to parse the data following the call, which in case of a jump
	// engine is not code but pointers to functions.
	if currentInstruction != nil && currentInstruction.Name == cpu.JsrInstruction {
		dis.functionReturnsToParse = append(dis.functionReturnsToParse, address)
		dis.functionReturnsToParseAdded[address] = struct{}{}
	} else {
		dis.offsetsToParse = append(dis.offsetsToParse, address)
	}
}

// getAddressingParam returns the address of the param if it references an address.
func getAddressingParam(param any) (uint16, bool) {
	switch val := param.(type) {
	case Absolute:
		return uint16(val), true
	case AbsoluteX:
		return uint16(val), true
	case AbsoluteY:
		return uint16(val), true
	case Indirect:
		return uint16(val), true
	case IndirectX:
		return uint16(val), true
	case IndirectY:
		return uint16(val), true
	case ZeroPage:
		return uint16(val), true
	case ZeroPageX:
		return uint16(val), true
	case ZeroPageY:
		return uint16(val), true
	default:
		return 0, false
	}
}
