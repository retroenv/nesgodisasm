package disasm

import (
	"errors"
	"fmt"

	"github.com/retroenv/nesgodisasm/internal/program"
	. "github.com/retroenv/retrogolib/addressing"
	"github.com/retroenv/retrogolib/arch/cpu/m6502"
	"github.com/retroenv/retrogolib/arch/nes"
	"github.com/retroenv/retrogolib/arch/nes/parameter"
	"github.com/retroenv/retrogolib/cpu"
)

var errInstructionOverlapsIRQHandlers = errors.New("instruction overlaps IRQ handler start")

// followExecutionFlow parses opcodes and follows the execution flow to parse all code.
func (dis *Disasm) followExecutionFlow(bnk *bank) error {
	for address := dis.addressToDisassemble(bnk); address != 0; address = dis.addressToDisassemble(bnk) {
		if _, ok := bnk.offsetsParsed[address]; ok {
			continue
		}
		bnk.offsetsParsed[address] = struct{}{}

		dis.pc = address
		index := dis.addressToIndex(bnk, dis.pc)
		offsetInfo, inspectCode := dis.initializeOffsetInfo(bnk, index)
		if !inspectCode {
			continue
		}

		instruction := offsetInfo.opcode.Instruction

		if offsetInfo.opcode.Addressing == ImpliedAddressing {
			offsetInfo.Code = instruction.Name
		} else {
			params, err := dis.processParamInstruction(bnk, dis.pc, offsetInfo)
			if err != nil {
				if errors.Is(err, errInstructionOverlapsIRQHandlers) {
					dis.handleInstructionIRQOverlap(bnk, address, index, offsetInfo)
					continue
				}
				return err
			}
			offsetInfo.Code = fmt.Sprintf("%s %s", instruction.Name, params)
		}

		if _, ok := m6502.NotExecutingFollowingOpcodeInstructions[instruction.Name]; ok {
			dis.checkForJumpEngineJmp(bnk, offsetInfo, dis.pc)
		} else {
			opcodeLength := uint16(len(offsetInfo.OpcodeBytes))
			followingOpcodeAddress := dis.pc + opcodeLength
			dis.addAddressToParse(bnk, followingOpcodeAddress, offsetInfo.context, address, instruction, false)
			dis.checkForJumpEngineCall(bnk, offsetInfo, dis.pc)
		}

		dis.checkInstructionOverlap(bnk, offsetInfo, index)

		if dis.handleDisambiguousInstructions(bnk, offsetInfo, index) {
			continue
		}

		dis.changeIndexRangeToCode(bnk, offsetInfo.OpcodeBytes, index)
	}
	return nil
}

// in case the current instruction overlaps with an already existing instruction,
// cut the current one short.
func (dis *Disasm) checkInstructionOverlap(bnk *bank, offsetInfo *offset, index uint16) {
	for i := 1; i < len(offsetInfo.OpcodeBytes) && int(index)+i < len(bnk.offsets); i++ {
		offsetInfoFollowing := &bnk.offsets[index+uint16(i)]
		if !offsetInfoFollowing.IsType(program.CodeOffset) {
			continue
		}

		offsetInfoFollowing.Comment = "branch into instruction detected"
		offsetInfo.Comment = offsetInfo.Code
		offsetInfo.OpcodeBytes = offsetInfo.OpcodeBytes[:i]
		offsetInfo.Code = ""
		offsetInfo.ClearType(program.CodeOffset)
		offsetInfo.SetType(program.CodeAsData | program.DataOffset)
		return
	}
}

// initializeOffsetInfo initializes the offset info for the given offset and returns
// whether the offset should process inspection for code parameters.
func (dis *Disasm) initializeOffsetInfo(bnk *bank, index uint16) (*offset, bool) {
	offsetInfo := &bnk.offsets[index]

	if offsetInfo.IsType(program.CodeOffset) {
		return offsetInfo, false // was set by CDL
	}

	b := dis.readMemory(dis.pc)
	offsetInfo.OpcodeBytes = make([]byte, 1, 3)
	offsetInfo.OpcodeBytes[0] = b

	if offsetInfo.IsType(program.DataOffset) {
		return offsetInfo, false // was set by CDL
	}

	opcode := m6502.Opcodes[b]
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
func (dis *Disasm) processParamInstruction(bnk *bank, address uint16, offsetInfo *offset) (string, error) {
	opcode := offsetInfo.opcode
	param, opcodes := dis.readOpParam(opcode.Addressing, dis.pc)
	offsetInfo.OpcodeBytes = append(offsetInfo.OpcodeBytes, opcodes...)

	if address+uint16(len(offsetInfo.OpcodeBytes)) > irqStartAddress {
		return "", errInstructionOverlapsIRQHandlers
	}

	paramAsString, err := parameter.String(dis.converter, opcode.Addressing, param)
	if err != nil {
		return "", fmt.Errorf("getting parameter as string: %w", err)
	}

	paramAsString = dis.replaceParamByAlias(bnk, address, opcode, param, paramAsString)

	if _, ok := m6502.BranchingInstructions[opcode.Instruction.Name]; ok {
		addr, ok := param.(Absolute)
		if ok {
			dis.addAddressToParse(bnk, uint16(addr), offsetInfo.context, dis.pc, opcode.Instruction, true)
		}
	}

	return paramAsString, nil
}

// replaceParamByAlias replaces the absolute address with an alias name if it can match it to
// a constant, zero page variable or a code reference.
func (dis *Disasm) replaceParamByAlias(bnk *bank, address uint16, opcode cpu.Opcode, param any, paramAsString string) string {
	forceVariableUsage := false
	addressReference, addressValid := getAddressingParam(param)
	if !addressValid || addressReference >= irqStartAddress {
		return paramAsString
	}

	if _, ok := m6502.BranchingInstructions[opcode.Instruction.Name]; ok {
		var handleParam bool
		handleParam, forceVariableUsage = checkBranchingParam(addressReference, opcode)
		if !handleParam {
			return paramAsString
		}
	}

	constantInfo, ok := dis.constants[addressReference]
	if ok {
		return dis.replaceParamByConstant(opcode, paramAsString, addressReference, constantInfo)
	}

	if !dis.addVariableReference(bnk, addressReference, address, opcode, forceVariableUsage) {
		return paramAsString
	}

	// force using absolute address to not generate a different opcode by using zeropage access mode
	switch opcode.Addressing {
	case ZeroPageAddressing, ZeroPageXAddressing, ZeroPageYAddressing:
		return dis.options.ZeroPagePrefix + paramAsString
	case AbsoluteAddressing, AbsoluteXAddressing, AbsoluteYAddressing:
		return dis.options.AbsolutePrefix + paramAsString
	default: // indirect x, ...
		return paramAsString
	}
}

// addressToDisassemble returns the next address to disassemble, if there are no more addresses to parse,
// 0 will be returned. Return address from function addresses have the lowest priority, to be able to
// handle jump table functions correctly.
func (dis *Disasm) addressToDisassemble(bnk *bank) uint16 {
	for {
		if len(bnk.offsetsToParse) > 0 {
			address := bnk.offsetsToParse[0]
			bnk.offsetsToParse = bnk.offsetsToParse[1:]
			return address
		}

		for len(bnk.functionReturnsToParse) > 0 {
			address := bnk.functionReturnsToParse[0]
			bnk.functionReturnsToParse = bnk.functionReturnsToParse[1:]

			_, ok := bnk.functionReturnsToParseAdded[address]
			// if the address was removed from the set it marks the address as not being parsed anymore,
			// this way is more efficient than iterating the slice to delete the element
			if !ok {
				continue
			}
			delete(bnk.functionReturnsToParseAdded, address)
			return address
		}

		if !dis.scanForNewJumpEngineEntry(bnk) {
			return 0
		}
	}
}

// addAddressToParse adds an address to the list to be processed if the address has not been processed yet.
func (dis *Disasm) addAddressToParse(bnk *bank, address, context, from uint16, currentInstruction *cpu.Instruction,
	isABranchDestination bool) {

	// ignore branching into addresses before the code base address, for example when generating code in
	// zeropage and branching into it to execute it.
	if address < dis.codeBaseAddress {
		return
	}

	index := dis.addressToIndex(bnk, address)
	offsetInfo := &bnk.offsets[index]

	if isABranchDestination && currentInstruction != nil && currentInstruction.Name == m6502.Jsr.Name {
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
		bnk.branchDestinations[address] = struct{}{}
	}

	if _, ok := bnk.offsetsToParseAdded[address]; ok {
		return
	}
	bnk.offsetsToParseAdded[address] = struct{}{}

	// add instructions that follow a function call to a special queue with lower priority, to allow the
	// jump engine be detected before trying to parse the data following the call, which in case of a jump
	// engine is not code but pointers to functions.
	if currentInstruction != nil && currentInstruction.Name == m6502.Jsr.Name {
		bnk.functionReturnsToParse = append(bnk.functionReturnsToParse, address)
		bnk.functionReturnsToParseAdded[address] = struct{}{}
	} else {
		bnk.offsetsToParse = append(bnk.offsetsToParse, address)
	}
}

// handleInstructionIRQOverlap handles an instruction overlapping with the start of the IRQ handlers.
// The opcodes are cut until the start of the IRQ handlers and the offset is converted to type data.
func (dis *Disasm) handleInstructionIRQOverlap(bnk *bank, address, index uint16, offsetInfo *offset) {
	if address > irqStartAddress {
		return
	}

	keepLength := int(irqStartAddress - address)
	offsetInfo.OpcodeBytes = offsetInfo.OpcodeBytes[:keepLength]

	for i := 0; i < keepLength; i++ {
		offsetInfo = &bnk.offsets[index+uint16(i)]
		offsetInfo.ClearType(program.CodeOffset)
		offsetInfo.SetType(program.CodeAsData | program.DataOffset)
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

// checkBranchingParam checks whether the branching instruction should do a variable check for the parameter
// and forces variable usage.
func checkBranchingParam(address uint16, opcode cpu.Opcode) (bool, bool) {
	switch {
	case opcode.Instruction.Name == m6502.Jmp.Name && opcode.Addressing == IndirectAddressing:
		return true, false
	case opcode.Instruction.Name == m6502.Jmp.Name || opcode.Instruction.Name == m6502.Jsr.Name:
		if opcode.Addressing == AbsoluteAddressing && address < nes.CodeBaseAddress {
			return true, true
		}
	}
	return false, false
}
