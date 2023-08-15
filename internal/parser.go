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
func (dis *Disasm) followExecutionFlow() error {
	for address := dis.addressToDisassemble(); address != 0; address = dis.addressToDisassemble() {
		if _, ok := dis.offsetsParsed[address]; ok {
			continue
		}
		dis.offsetsParsed[address] = struct{}{}

		dis.pc = address
		offsetInfo := dis.mapper.offsetInfo(dis.pc)
		inspectCode := dis.initializeOffsetInfo(offsetInfo)
		if !inspectCode {
			continue
		}

		instruction := offsetInfo.opcode.Instruction

		if offsetInfo.opcode.Addressing == ImpliedAddressing {
			offsetInfo.Code = instruction.Name
		} else {
			params, err := dis.processParamInstruction(dis.pc, offsetInfo)
			if err != nil {
				if errors.Is(err, errInstructionOverlapsIRQHandlers) {
					dis.handleInstructionIRQOverlap(address, offsetInfo)
					continue
				}
				return err
			}
			offsetInfo.Code = fmt.Sprintf("%s %s", instruction.Name, params)
		}

		if _, ok := m6502.NotExecutingFollowingOpcodeInstructions[instruction.Name]; ok {
			dis.checkForJumpEngineJmp(dis.pc, offsetInfo)
		} else {
			opcodeLength := uint16(len(offsetInfo.OpcodeBytes))
			followingOpcodeAddress := dis.pc + opcodeLength
			dis.addAddressToParse(followingOpcodeAddress, offsetInfo.context, address, instruction, false)
			dis.checkForJumpEngineCall(dis.pc, offsetInfo)
		}

		dis.checkInstructionOverlap(address, offsetInfo)

		if dis.handleDisambiguousInstructions(address, offsetInfo) {
			continue
		}

		dis.changeAddressRangeToCode(address, offsetInfo.OpcodeBytes)
	}
	return nil
}

// in case the current instruction overlaps with an already existing instruction,
// cut the current one short.
func (dis *Disasm) checkInstructionOverlap(address uint16, offsetInfo *offset) {
	for i := 1; i < len(offsetInfo.OpcodeBytes) && int(address)+i < irqStartAddress; i++ {
		offsetInfoFollowing := dis.mapper.offsetInfo(address + uint16(i))
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

// initializeOffsetInfo initializes the offset info and returns
// whether the offset should process inspection for code parameters.
func (dis *Disasm) initializeOffsetInfo(offsetInfo *offset) bool {
	if offsetInfo.IsType(program.CodeOffset) {
		return false // was set by CDL
	}

	b := dis.readMemory(dis.pc)
	offsetInfo.OpcodeBytes = make([]byte, 1, 3)
	offsetInfo.OpcodeBytes[0] = b

	if offsetInfo.IsType(program.DataOffset) {
		return false // was set by CDL
	}

	opcode := m6502.Opcodes[b]
	if opcode.Instruction == nil {
		// consider an unknown instruction as start of data
		offsetInfo.SetType(program.DataOffset)
		return false
	}

	offsetInfo.opcode = opcode
	return true
}

// processParamInstruction processes an instruction with parameters.
// Special handling is required as this instruction could branch to a different location.
func (dis *Disasm) processParamInstruction(address uint16, offsetInfo *offset) (string, error) {
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

	paramAsString = dis.replaceParamByAlias(address, opcode, param, paramAsString)

	if _, ok := m6502.BranchingInstructions[opcode.Instruction.Name]; ok {
		addr, ok := param.(Absolute)
		if ok {
			dis.addAddressToParse(uint16(addr), offsetInfo.context, dis.pc, opcode.Instruction, true)
		}
	}

	return paramAsString, nil
}

// replaceParamByAlias replaces the absolute address with an alias name if it can match it to
// a constant, zero page variable or a code reference.
func (dis *Disasm) replaceParamByAlias(address uint16, opcode cpu.Opcode, param any, paramAsString string) string {
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
		return dis.replaceParamByConstant(addressReference, opcode, paramAsString, constantInfo)
	}

	if !dis.addVariableReference(addressReference, address, opcode, forceVariableUsage) {
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
func (dis *Disasm) addressToDisassemble() uint16 {
	for {
		if len(dis.offsetsToParse) > 0 {
			address := dis.offsetsToParse[0]
			dis.offsetsToParse = dis.offsetsToParse[1:]
			return address
		}

		for len(dis.functionReturnsToParse) > 0 {
			address := dis.functionReturnsToParse[0]
			dis.functionReturnsToParse = dis.functionReturnsToParse[1:]

			_, ok := dis.functionReturnsToParseAdded[address]
			// if the address was removed from the set it marks the address as not being parsed anymore,
			// this way is more efficient than iterating the slice to delete the element
			if !ok {
				continue
			}
			delete(dis.functionReturnsToParseAdded, address)
			return address
		}

		if !dis.scanForNewJumpEngineEntry() {
			return 0
		}
	}
}

// addAddressToParse adds an address to the list to be processed if the address has not been processed yet.
func (dis *Disasm) addAddressToParse(address, context, from uint16, currentInstruction *cpu.Instruction,
	isABranchDestination bool) {

	// ignore branching into addresses before the code base address, for example when generating code in
	// zeropage and branching into it to execute it.
	if address < dis.codeBaseAddress {
		return
	}

	offsetInfo := dis.mapper.offsetInfo(address)
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
			bankRef := bankReference{
				bank:    dis.mapper.getMappedBank(from),
				address: from,
				index:   dis.mapper.getMappedBankIndex(from),
			}
			offsetInfo.branchFrom = append(offsetInfo.branchFrom, bankRef)
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
	if currentInstruction != nil && currentInstruction.Name == m6502.Jsr.Name {
		dis.functionReturnsToParse = append(dis.functionReturnsToParse, address)
		dis.functionReturnsToParseAdded[address] = struct{}{}
	} else {
		dis.offsetsToParse = append(dis.offsetsToParse, address)
	}
}

// handleInstructionIRQOverlap handles an instruction overlapping with the start of the IRQ handlers.
// The opcodes are cut until the start of the IRQ handlers and the offset is converted to type data.
func (dis *Disasm) handleInstructionIRQOverlap(address uint16, offsetInfo *offset) {
	if address > irqStartAddress {
		return
	}

	keepLength := int(irqStartAddress - address)
	offsetInfo.OpcodeBytes = offsetInfo.OpcodeBytes[:keepLength]

	for i := 0; i < keepLength; i++ {
		offsetInfo = dis.mapper.offsetInfo(address + uint16(i))
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
