// Package m6502 provides a 6502 architecture specific disassembler code.
package m6502

import (
	"errors"
	"fmt"

	"github.com/retroenv/retrodisasm/internal/arch"
	"github.com/retroenv/retrogolib/arch/cpu/m6502"
	"github.com/retroenv/retrogolib/arch/system/nes/cartridge"
	"github.com/retroenv/retrogolib/arch/system/nes/parameter"
)

var _ arch.Architecture = &Arch6502{}

// New returns a new 6502 architecture configuration.
func New(converter parameter.Converter) *Arch6502 {
	return &Arch6502{
		converter: converter,
	}
}

type Arch6502 struct {
	converter parameter.Converter
}

// LastCodeAddress returns the last possible address of code.
// This is used in systems where the last address is reserved for
// the interrupt vector table.
func (ar *Arch6502) LastCodeAddress() uint16 {
	return m6502.InterruptVectorStartAddress
}

func (ar *Arch6502) ProcessOffset(dis arch.Disasm, address uint16, offsetInfo *arch.Offset) (bool, error) {
	inspectCode, err := initializeOffsetInfo(dis, offsetInfo)
	if err != nil {
		return false, err
	}
	if !inspectCode {
		return false, nil
	}

	op := offsetInfo.Opcode
	instruction := op.Instruction()
	name := instruction.Name()
	pc := dis.ProgramCounter()

	if op.Addressing() == int(m6502.ImpliedAddressing) {
		offsetInfo.Code = name
	} else {
		params, err := ar.processParamInstruction(dis, pc, offsetInfo)
		if err != nil {
			if errors.Is(err, errInstructionOverlapsIRQHandlers) {
				ar.handleInstructionIRQOverlap(dis, address, offsetInfo)
				return true, nil
			}
			return false, err
		}
		offsetInfo.Code = fmt.Sprintf("%s %s", name, params)
	}

	if _, ok := m6502.NotExecutingFollowingOpcodeInstructions[name]; ok {
		if err := ar.checkForJumpEngineJmp(dis, pc, offsetInfo); err != nil {
			return false, err
		}
	} else {
		opcodeLength := uint16(len(offsetInfo.Data))
		followingOpcodeAddress := pc + opcodeLength
		dis.AddAddressToParse(followingOpcodeAddress, offsetInfo.Context, address, instruction, false)
		if err := ar.checkForJumpEngineCall(dis, pc, offsetInfo); err != nil {
			return false, err
		}
	}

	return true, nil
}

// BankWindowSize returns the bank window size.
func (ar *Arch6502) BankWindowSize(_ *cartridge.Cartridge) int {
	return 0x2000 // TODO calculate dynamically
}
