package disasm

import (
	"fmt"

	"github.com/retroenv/nesgodisasm/internal/arch"
	"github.com/retroenv/nesgodisasm/internal/program"
)

// followExecutionFlow parses opcodes and follows the execution flow to parse all code.
// nolint: funlen
func (dis *Disasm) followExecutionFlow() error {
	for {
		addr, err := dis.addressToDisassemble()
		if err != nil {
			return err
		}
		if addr == -1 {
			break
		}
		address := uint16(addr)

		if _, ok := dis.offsetsParsed[address]; ok {
			continue
		}
		dis.offsetsParsed[address] = struct{}{}

		dis.pc = address
		offsetInfo := dis.mapper.OffsetInfo(dis.pc)

		inspectCode, err := dis.arch.ProcessOffset(dis, address, offsetInfo)
		if err != nil {
			return fmt.Errorf("error processing offset at address %04x: %w", address, err)
		}
		if !inspectCode {
			continue
		}

		dis.checkInstructionOverlap(address, offsetInfo)

		if dis.arch.HandleDisambiguousInstructions(dis, address, offsetInfo) {
			continue
		}

		dis.changeAddressRangeToCode(address, offsetInfo.Data)
	}
	return nil
}

// in case the current instruction overlaps with an already existing instruction,
// cut the current one short.
func (dis *Disasm) checkInstructionOverlap(address uint16, offsetInfo *arch.Offset) {
	for i := 1; i < len(offsetInfo.Data) && int(address)+i < int(dis.arch.LastCodeAddress()); i++ {
		followingAddress := address + uint16(i)
		offsetInfoFollowing := dis.mapper.OffsetInfo(followingAddress)

		// Check for regular code overlap or CodeAsData that's a branch destination
		// (CodeAsData that's NOT a branch destination can be consumed by this instruction)
		isOverlap := offsetInfoFollowing.IsType(program.CodeOffset) ||
			(offsetInfoFollowing.IsType(program.CodeAsData) && dis.isBranchDestination(followingAddress))

		if !isOverlap {
			continue
		}

		offsetInfoFollowing.Comment = "branch into instruction detected"
		offsetInfo.Comment = offsetInfo.Code
		offsetInfo.Data = offsetInfo.Data[:i]
		offsetInfo.Code = ""
		offsetInfo.ClearType(program.CodeOffset)
		offsetInfo.SetType(program.CodeAsData | program.DataOffset)
		return
	}
}

// isBranchDestination checks if an address is a branch destination.
func (dis *Disasm) isBranchDestination(address uint16) bool {
	_, ok := dis.branchDestinations[address]
	return ok
}

// addressToDisassemble returns the next address to disassemble, if there are no more addresses to parse,
// -1 will be returned. Return address from function addresses have the lowest priority, to be able to
// handle jump table functions correctly.
func (dis *Disasm) addressToDisassemble() (int, error) {
	for {
		if len(dis.offsetsToParse) > 0 {
			address := dis.offsetsToParse[0]
			dis.offsetsToParse = dis.offsetsToParse[1:]
			return int(address), nil
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
			return int(address), nil
		}

		isEntry, err := dis.jumpEngine.ScanForNewJumpEngineEntry(dis)
		if err != nil {
			return 0, fmt.Errorf("scanning for new jump engine entry: %w", err)
		}
		if !isEntry {
			return -1, nil
		}
	}
}

// AddAddressToParse adds an address to the list to be processed if the address has not been processed yet.
func (dis *Disasm) AddAddressToParse(address, context, from uint16,
	currentInstruction arch.Instruction, isABranchDestination bool) {

	// ignore branching into addresses before the code base address, for example when generating code in
	// zeropage and branching into it to execute it.
	if address < dis.codeBaseAddress {
		return
	}

	offsetInfo := dis.mapper.OffsetInfo(address)
	if isABranchDestination && currentInstruction != nil && currentInstruction.IsCall() {
		offsetInfo.SetType(program.CallDestination)
		if offsetInfo.Context == 0 {
			offsetInfo.Context = address // begin a new context
		}
	} else if offsetInfo.Context == 0 {
		offsetInfo.Context = context // continue current context
	}

	if isABranchDestination {
		if from > 0 {
			bankRef := arch.BankReference{
				Mapped:  dis.mapper.GetMappedBank(from),
				Address: from,
				Index:   dis.mapper.GetMappedBankIndex(from),
			}
			bankRef.ID = bankRef.Mapped.ID()
			offsetInfo.BranchFrom = append(offsetInfo.BranchFrom, bankRef)
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
	if currentInstruction != nil && currentInstruction.IsCall() {
		dis.functionReturnsToParse = append(dis.functionReturnsToParse, address)
		dis.functionReturnsToParseAdded[address] = struct{}{}
	} else {
		dis.offsetsToParse = append(dis.offsetsToParse, address)
	}
}

// DeleteFunctionReturnToParse deletes a function return address from the list of addresses to parse.
func (dis *Disasm) DeleteFunctionReturnToParse(address uint16) {
	delete(dis.functionReturnsToParseAdded, address)
}
