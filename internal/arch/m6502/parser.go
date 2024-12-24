package m6502

import (
	"errors"
	"fmt"

	"github.com/retroenv/nesgodisasm/internal/arch"
	"github.com/retroenv/nesgodisasm/internal/program"
	"github.com/retroenv/retrogolib/arch/cpu/m6502"
	"github.com/retroenv/retrogolib/arch/nes"
	"github.com/retroenv/retrogolib/arch/nes/parameter"
)

var errInstructionOverlapsIRQHandlers = errors.New("instruction overlaps IRQ handler start")

// initializeOffsetInfo initializes the offset info and returns
// whether the offset should process inspection for code parameters.
func initializeOffsetInfo(dis arch.Disasm, offsetInfo *arch.Offset) (bool, error) {
	if offsetInfo.IsType(program.CodeOffset) {
		return false, nil // was set by CDL
	}

	pc := dis.ProgramCounter()
	b, err := dis.ReadMemory(pc)
	if err != nil {
		return false, fmt.Errorf("reading memory at address %04x: %w", pc, err)
	}
	offsetInfo.Data = make([]byte, 1, m6502.MaxOpcodeSize)
	offsetInfo.Data[0] = b

	if offsetInfo.IsType(program.DataOffset) {
		return false, nil // was set by CDL
	}

	opcode := m6502.Opcodes[b]
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

// processParamInstruction processes an instruction with parameters.
// Special handling is required as this instruction could branch to a different location.
func (ar *Arch6502) processParamInstruction(dis arch.Disasm, address uint16, offsetInfo *arch.Offset) (string, error) {
	opcode := offsetInfo.Opcode
	pc := dis.ProgramCounter()
	param, opcodes, err := ar.ReadOpParam(dis, opcode.Addressing(), pc)
	if err != nil {
		return "", fmt.Errorf("reading opcode parameters: %w", err)
	}
	offsetInfo.Data = append(offsetInfo.Data, opcodes...)

	if address+uint16(len(offsetInfo.Data)) > m6502.InterruptVectorStartAddress {
		return "", errInstructionOverlapsIRQHandlers
	}

	paramAsString, err := parameter.String(ar.converter, m6502.AddressingMode(opcode.Addressing()), param)
	if err != nil {
		return "", fmt.Errorf("getting parameter as string: %w", err)
	}

	paramAsString = ar.replaceParamByAlias(dis, address, opcode, param, paramAsString)

	if _, ok := m6502.BranchingInstructions[opcode.Instruction().Name()]; ok {
		addr, ok := param.(m6502.Absolute)
		if ok {
			dis.AddAddressToParse(uint16(addr), offsetInfo.Context, pc, opcode.Instruction(), true)
		}
	}

	return paramAsString, nil
}

// handleInstructionIRQOverlap handles an instruction overlapping with the start of the IRQ handlers.
// The opcodes are cut until the start of the IRQ handlers and the offset is converted to type data.
func (ar *Arch6502) handleInstructionIRQOverlap(dis arch.Disasm, address uint16, offsetInfo *arch.Offset) {
	if address > m6502.InterruptVectorStartAddress {
		return
	}

	keepLength := int(m6502.InterruptVectorStartAddress - address)
	offsetInfo.Data = offsetInfo.Data[:keepLength]

	for i := range keepLength {
		offsetInfo = dis.Mapper().OffsetInfo(address + uint16(i))
		offsetInfo.ClearType(program.CodeOffset)
		offsetInfo.SetType(program.CodeAsData | program.DataOffset)
	}
}

// replaceParamByAlias replaces the absolute address with an alias name if it can match it to
// a constant, zero page variable or a code reference.
func (ar *Arch6502) replaceParamByAlias(dis arch.Disasm, address uint16, opcode arch.Opcode, param any, paramAsString string) string {
	forceVariableUsage := false
	addressReference, addressValid := ar.GetAddressingParam(param)
	if !addressValid || addressReference >= m6502.InterruptVectorStartAddress {
		return paramAsString
	}

	if _, ok := m6502.BranchingInstructions[opcode.Instruction().Name()]; ok {
		var handleParam bool
		handleParam, forceVariableUsage = checkBranchingParam(addressReference, opcode)
		if !handleParam {
			return paramAsString
		}
	}

	consts := dis.Constants()
	changedParamAsString, ok := consts.ReplaceParameter(addressReference, opcode, paramAsString)
	if ok {
		return changedParamAsString
	}

	dis.Variables().AddReference(dis, addressReference, address, opcode, forceVariableUsage)
	return paramAsString
}

// checkBranchingParam checks whether the branching instruction should do a variable check for the parameter
// and forces variable usage.
func checkBranchingParam(address uint16, opcode arch.Opcode) (bool, bool) {
	name := opcode.Instruction().Name()
	addressing := m6502.AddressingMode(opcode.Addressing())

	switch {
	case name == m6502.Jmp.Name && addressing == m6502.IndirectAddressing:
		return true, false
	case name == m6502.Jmp.Name || name == m6502.Jsr.Name:
		if addressing == m6502.AbsoluteAddressing && address < nes.CodeBaseAddress {
			return true, true
		}
	}
	return false, false
}
