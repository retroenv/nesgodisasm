package disasm

import (
	"fmt"

	"github.com/retroenv/nesgodisasm/internal/arch"
	"github.com/retroenv/nesgodisasm/internal/program"
	"github.com/retroenv/retrogolib/log"
)

const jumpEngineLastInstructionsCheck = 16

// jumpEngineCaller stores info about a caller of a jump engine, which is followed by a list of function addresses
type jumpEngineCaller struct {
	entries           int  // count of referenced functions in the table
	terminated        bool // marks whether the end of the table has been found
	tableStartAddress uint16
}

// AddJumpEngine adds a jump engine function address to the list of jump engines.
func (dis *Disasm) AddJumpEngine(address uint16) {
	dis.jumpEngines[address] = struct{}{}
}

// GetFunctionTableReference detects a jump engine function context and its function table.
// TODO use jump address as key to be able to handle large function
// contexts containing multiple jump engines
func (dis *Disasm) GetFunctionTableReference(context uint16, dataReferences []uint16) {
	// if there are multiple data references just look at the last 2
	if len(dataReferences) > 2 {
		dataReferences = dataReferences[len(dataReferences)-2:]
	}

	// get the lowest data pointer, jmp and rts based jump engines have the address accesses reversed
	var referenceDistance, smallestReference uint16
	if dataReferences[0] < dataReferences[1] {
		referenceDistance = dataReferences[1] - dataReferences[0]
		smallestReference = dataReferences[0]
	} else {
		referenceDistance = dataReferences[0] - dataReferences[1]
		smallestReference = dataReferences[1]
	}

	// the access to the function tables can be done using the same address and an incremented x register
	// or to an incremented address and the same x register
	if referenceDistance != 0 && referenceDistance != 1 {
		return
	}

	jumpEngine := &jumpEngineCaller{}
	dis.jumpEngineCallersAdded[context] = jumpEngine
	dis.jumpEngineCallers = append(dis.jumpEngineCallers, jumpEngine)

	dis.jumpEngineCallersAdded[context].tableStartAddress = smallestReference
}

// GetContextDataReferences parse all instructions of the function context until the jump
// and returns data references that could point to the function table.
func (dis *Disasm) GetContextDataReferences(offsets []arch.Offset, addresses []uint16) ([]uint16, error) {
	var dataReferences []uint16

	for i, offsetInfoInstruction := range offsets {
		address := addresses[i]
		opcode := offsetInfoInstruction.Opcode()

		// look for an instructions that loads data from an address in the code or data
		// address range. this should be the table containing the function addresses.
		if opcode.Instruction().IsNil() || !opcode.ReadsMemory() {
			continue
		}

		param, _, err := dis.arch.ReadOpParam(dis, opcode.Addressing(), address)
		if err != nil {
			return nil, fmt.Errorf("reading opcode parameters: %w", err)
		}

		reference, ok := dis.arch.GetAddressingParam(param)
		if ok && reference >= dis.codeBaseAddress && reference < dis.arch.LastCodeAddress() {
			dataReferences = append(dataReferences, reference)
		}
	}
	return dataReferences, nil
}

// JumpContextInfo builds the list of instructions of the current function context.
// in some ROMs the jump engine can be part of a label inside a larger function,
// the jump engine detection will use the last instructions before the jmp.
func (dis *Disasm) JumpContextInfo(jumpAddress uint16, offsetInfo arch.Offset) ([]arch.Offset, []uint16) {
	var offsets []arch.Offset
	var addresses []uint16

	for address := offsetInfo.Context(); address != 0 && address < jumpAddress; {
		offsetInfoInstruction := dis.mapper.offsetInfo(address)

		// skip offsets that have not been processed yet
		if len(offsetInfoInstruction.Data()) == 0 {
			address++
			continue
		}

		offsets = append(offsets, offsetInfoInstruction)
		addresses = append(addresses, address)

		address += uint16(len(offsetInfoInstruction.Data()))
	}

	if len(offsets) > jumpEngineLastInstructionsCheck {
		offsets = offsets[len(offsets)-jumpEngineLastInstructionsCheck-1:]
		addresses = addresses[len(addresses)-jumpEngineLastInstructionsCheck-1:]
	}

	return offsets, addresses
}

// HandleJumpEngineDestination processes a newly detected jump engine destination.
func (dis *Disasm) HandleJumpEngineDestination(caller, destination uint16) error {
	for addr := range dis.jumpEngines {
		if addr == destination {
			return dis.HandleJumpEngineCallers(caller)
		}
	}
	return nil
}

// HandleJumpEngineCallers processes all callers of a newly detected jump engine function.
func (dis *Disasm) HandleJumpEngineCallers(context uint16) error {
	offsetInfo := dis.mapper.offsetInfo(context)
	offsetInfo.LabelComment = "jump engine detected"
	offsetInfo.SetType(program.JumpEngine)

	for _, bankRef := range offsetInfo.branchFrom {
		if err := dis.handleJumpEngineCaller(bankRef.address); err != nil {
			return err
		}
	}
	return nil
}

// handleJumpEngineCaller processes a newly detected jump engine caller, the return address of the call is
// marked as function reference instead of code. The first entry of the function table is processed.
func (dis *Disasm) handleJumpEngineCaller(caller uint16) error {
	jumpEngine, ok := dis.jumpEngineCallersAdded[caller]
	if !ok {
		jumpEngine = &jumpEngineCaller{}
		dis.jumpEngineCallersAdded[caller] = jumpEngine
		dis.jumpEngineCallers = append(dis.jumpEngineCallers, jumpEngine)
	}

	// get the address of the function pointers after the jump engine call
	offsetInfo := dis.mapper.offsetInfo(caller)
	address := caller + uint16(len(offsetInfo.Data()))

	// remove from code that should be parsed
	delete(dis.functionReturnsToParseAdded, address)
	jumpEngine.tableStartAddress = address

	_, err := dis.processJumpEngineEntry(address, jumpEngine)
	return err
}

// processJumpEngineEntry processes a potential function reference in a jump engine table.
// It returns whether the entry was a valid function reference address and has been added for processing.
func (dis *Disasm) processJumpEngineEntry(address uint16, jumpEngine *jumpEngineCaller) (bool, error) {
	if jumpEngine.terminated {
		return false, nil
	}

	// verify that the destination is in valid code address range
	destination, err := dis.ReadMemoryWord(address)
	if err != nil {
		return false, err
	}
	if destination < dis.codeBaseAddress || destination >= dis.arch.LastCodeAddress() {
		jumpEngine.terminated = true
		return false, nil
	}

	offsetInfo1 := dis.mapper.offsetInfo(address)
	offsetInfo2 := dis.mapper.offsetInfo(address + 1)

	// if the potential jump table entry is already marked as code, the table end is reached
	if offsetInfo1.Offset.Type == program.CodeOffset || offsetInfo2.Offset.Type == program.CodeOffset {
		jumpEngine.terminated = true
		return false, nil
	}

	if jumpEngine.entries == 0 {
		offsetInfo1.Offset.SetType(program.JumpTable)
	}

	offsetInfo1.Offset.SetType(program.FunctionReference)
	offsetInfo2.Offset.SetType(program.FunctionReference)

	b1, err := dis.ReadMemory(address)
	if err != nil {
		return false, err
	}
	b2, err := dis.ReadMemory(address + 1)
	if err != nil {
		return false, err
	}

	offsetInfo1.SetData([]byte{b1, b2})
	offsetInfo2.SetData(nil)

	jumpEngine.entries++

	dis.AddAddressToParse(destination, destination, address, nil, true)
	return true, nil
}

// scanForNewJumpEngineEntry scans all jump engine calls for an unprocessed entry in the function address table that
// follows the call. It returns whether a new address to parse was added.
func (dis *Disasm) scanForNewJumpEngineEntry() (bool, error) {
	for len(dis.jumpEngineCallers) != 0 {
		minEntries := -1

		// find the jump engine table with the smallest number of processed entries,
		// this conservative approach avoids interpreting code in the table area as function references
		for i := 0; i < len(dis.jumpEngineCallers); i++ {
			engineCaller := dis.jumpEngineCallers[i]
			if engineCaller.terminated {
				// jump engine table is processed, remove it from list to process
				dis.jumpEngineCallers = append(dis.jumpEngineCallers[:i], dis.jumpEngineCallers[i+1:]...)
			}

			if i := engineCaller.entries; !engineCaller.terminated && (i < minEntries || minEntries == -1) {
				minEntries = i
			}
		}
		if minEntries == -1 {
			return false, nil
		}

		for i := 0; i < len(dis.jumpEngineCallers); i++ {
			engineCaller := dis.jumpEngineCallers[i]
			if engineCaller.entries != minEntries {
				continue
			}

			// calculate next address in table to process
			address := engineCaller.tableStartAddress + uint16(2*engineCaller.entries)
			isEntry, err := dis.processJumpEngineEntry(address, engineCaller)
			if err != nil {
				return false, err
			}
			if isEntry {
				return true, nil
			}
			dis.logger.Debug("Jump engine table",
				log.String("address", fmt.Sprintf("0x%04X", engineCaller.tableStartAddress)),
				log.Int("entries", engineCaller.entries),
			)

			// jump engine table is processed, remove it from list to process
			dis.jumpEngineCallers = append(dis.jumpEngineCallers[:i], dis.jumpEngineCallers[i+1:]...)
			i--
		}
	}
	return false, nil
}
