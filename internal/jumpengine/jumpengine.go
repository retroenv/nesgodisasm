// Package jumpengine provides jump engine detection and processing.
package jumpengine

import (
	"fmt"
	"slices"

	"github.com/retroenv/nesgodisasm/internal/arch"
	"github.com/retroenv/nesgodisasm/internal/program"
	"github.com/retroenv/retrogolib/log"
)

var _ arch.JumpEngine = &JumpEngine{}

const jumpEngineLastInstructionsCheck = 16

// jumpEngineCaller stores info about a caller of a jump engine, which is followed by a list of function addresses
type jumpEngineCaller struct {
	entries           int  // count of referenced functions in the table
	terminated        bool // marks whether the end of the table has been found
	tableStartAddress uint16
}

type JumpEngine struct {
	arch arch.Architecture

	jumpEngines            map[uint16]struct{} // set of all jump engine functions addresses
	jumpEngineCallers      []*jumpEngineCaller // jump engine caller tables to process
	jumpEngineCallersAdded map[uint16]*jumpEngineCaller
}

func New(ar arch.Architecture) *JumpEngine {
	return &JumpEngine{
		arch:                   ar,
		jumpEngines:            map[uint16]struct{}{},
		jumpEngineCallers:      []*jumpEngineCaller{},
		jumpEngineCallersAdded: map[uint16]*jumpEngineCaller{},
	}
}

// AddJumpEngine adds a jump engine function address to the list of jump engines.
func (j *JumpEngine) AddJumpEngine(address uint16) {
	j.jumpEngines[address] = struct{}{}
}

// GetFunctionTableReference detects a jump engine function context and its function table.
// TODO use jump address as key to be able to handle large function
// contexts containing multiple jump engines
func (j *JumpEngine) GetFunctionTableReference(context uint16, dataReferences []uint16) {
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
	j.jumpEngineCallersAdded[context] = jumpEngine
	j.jumpEngineCallers = append(j.jumpEngineCallers, jumpEngine)

	j.jumpEngineCallersAdded[context].tableStartAddress = smallestReference
}

// GetContextDataReferences parse all instructions of the function context until the jump
// and returns data references that could point to the function table.
func (j *JumpEngine) GetContextDataReferences(dis arch.Disasm, offsets []*arch.Offset,
	addresses []uint16) ([]uint16, error) {

	codeBaseAddress := dis.CodeBaseAddress()
	var dataReferences []uint16

	for i, offsetInfoInstruction := range offsets {
		address := addresses[i]
		opcode := offsetInfoInstruction.Opcode

		// look for an instructions that loads data from an address in the code or data
		// address range. this should be the table containing the function addresses.
		if opcode.Instruction().IsNil() || !opcode.ReadsMemory() {
			continue
		}

		param, _, err := j.arch.ReadOpParam(dis, opcode.Addressing(), address)
		if err != nil {
			return nil, fmt.Errorf("reading opcode parameters: %w", err)
		}

		reference, ok := j.arch.GetAddressingParam(param)
		if ok && reference >= codeBaseAddress && reference < j.arch.LastCodeAddress() {
			dataReferences = append(dataReferences, reference)
		}
	}
	return dataReferences, nil
}

// JumpContextInfo builds the list of instructions of the current function context.
// in some ROMs the jump engine can be part of a label inside a larger function,
// the jump engine detection will use the last instructions before the jmp.
func (j *JumpEngine) JumpContextInfo(dis arch.Disasm, jumpAddress uint16, offsetInfo *arch.Offset) ([]*arch.Offset, []uint16) {
	var offsets []*arch.Offset
	var addresses []uint16

	for address := offsetInfo.Context; address != 0 && address < jumpAddress; {
		offsetInfoInstruction := dis.Mapper().OffsetInfo(address)

		// skip offsets that have not been processed yet
		if len(offsetInfoInstruction.Data) == 0 {
			address++
			continue
		}

		offsets = append(offsets, offsetInfoInstruction)
		addresses = append(addresses, address)

		address += uint16(len(offsetInfoInstruction.Data))
	}

	if len(offsets) > jumpEngineLastInstructionsCheck {
		offsets = offsets[len(offsets)-jumpEngineLastInstructionsCheck-1:]
		addresses = addresses[len(addresses)-jumpEngineLastInstructionsCheck-1:]
	}

	return offsets, addresses
}

// HandleJumpEngineDestination processes a newly detected jump engine destination.
func (j *JumpEngine) HandleJumpEngineDestination(dis arch.Disasm, caller, destination uint16) error {
	for addr := range j.jumpEngines {
		if addr == destination {
			return j.HandleJumpEngineCallers(dis, caller)
		}
	}
	return nil
}

// HandleJumpEngineCallers processes all callers of a newly detected jump engine function.
func (j *JumpEngine) HandleJumpEngineCallers(dis arch.Disasm, context uint16) error {
	offsetInfo := dis.Mapper().OffsetInfo(context)
	offsetInfo.LabelComment = "jump engine detected"
	offsetInfo.SetType(program.JumpEngine)

	for _, ref := range offsetInfo.BranchFrom {
		if err := j.handleJumpEngineCaller(dis, ref.Address); err != nil {
			return err
		}
	}
	return nil
}

// handleJumpEngineCaller processes a newly detected jump engine caller, the return address of the call is
// marked as function reference instead of code. The first entry of the function table is processed.
func (j *JumpEngine) handleJumpEngineCaller(dis arch.Disasm, caller uint16) error {
	jumpEngine, ok := j.jumpEngineCallersAdded[caller]
	if !ok {
		jumpEngine = &jumpEngineCaller{}
		j.jumpEngineCallersAdded[caller] = jumpEngine
		j.jumpEngineCallers = append(j.jumpEngineCallers, jumpEngine)
	}

	// get the address of the function pointers after the jump engine call
	offsetInfo := dis.Mapper().OffsetInfo(caller)
	address := caller + uint16(len(offsetInfo.Data))

	// remove from code that should be parsed
	dis.DeleteFunctionReturnToParse(address)
	jumpEngine.tableStartAddress = address

	_, err := j.processJumpEngineEntry(dis, address, jumpEngine)
	return err
}

// processJumpEngineEntry processes a potential function reference in a jump engine table.
// It returns whether the entry was a valid function reference address and has been added for processing.
func (j *JumpEngine) processJumpEngineEntry(dis arch.Disasm, address uint16, jumpEngine *jumpEngineCaller) (bool, error) {
	if jumpEngine.terminated {
		return false, nil
	}

	// verify that the destination is in valid code address range
	destination, err := dis.ReadMemoryWord(address)
	if err != nil {
		return false, fmt.Errorf("reading memory word: %w", err)
	}
	codeBaseAddress := dis.CodeBaseAddress()
	if destination < codeBaseAddress || destination >= j.arch.LastCodeAddress() {
		jumpEngine.terminated = true
		return false, nil
	}

	mapper := dis.Mapper()
	offsetInfo1 := mapper.OffsetInfo(address)
	offsetInfo2 := mapper.OffsetInfo(address + 1)

	// if the potential jump table entry is already marked as code, the table end is reached
	if offsetInfo1.Type == program.CodeOffset || offsetInfo2.Type == program.CodeOffset {
		jumpEngine.terminated = true
		return false, nil
	}

	if jumpEngine.entries == 0 {
		offsetInfo1.SetType(program.JumpTable)
	}

	offsetInfo1.SetType(program.FunctionReference)
	offsetInfo2.SetType(program.FunctionReference)

	b1, err := dis.ReadMemory(address)
	if err != nil {
		return false, fmt.Errorf("reading memory: %w", err)
	}
	b2, err := dis.ReadMemory(address + 1)
	if err != nil {
		return false, fmt.Errorf("reading memory: %w", err)
	}

	offsetInfo1.Data = []byte{b1, b2}
	offsetInfo2.Data = nil

	jumpEngine.entries++

	dis.AddAddressToParse(destination, destination, address, nil, true)
	return true, nil
}

// ScanForNewJumpEngineEntry scans all jump engine calls for an unprocessed entry in the function address table that
// follows the call. It returns whether a new address to parse was added.
func (j *JumpEngine) ScanForNewJumpEngineEntry(dis arch.Disasm) (bool, error) {
	logger := dis.Logger()

	for len(j.jumpEngineCallers) != 0 {
		// Remove all terminated entries
		j.jumpEngineCallers = slices.DeleteFunc(j.jumpEngineCallers, func(ec *jumpEngineCaller) bool {
			return ec.terminated
		})

		// Find the jump engine table with the smallest number of processed entries.
		// This conservative approach avoids interpreting code in the table area as function references.
		minEntries := -1
		for _, engineCaller := range j.jumpEngineCallers {
			if i := engineCaller.entries; i < minEntries || minEntries == -1 {
				minEntries = i
			}
		}
		if minEntries == -1 {
			return false, nil
		}

		for i := 0; i < len(j.jumpEngineCallers); i++ {
			engineCaller := j.jumpEngineCallers[i]
			if engineCaller.entries != minEntries {
				continue
			}

			// calculate next address in table to process
			address := engineCaller.tableStartAddress + uint16(2*engineCaller.entries)
			isEntry, err := j.processJumpEngineEntry(dis, address, engineCaller)
			if err != nil {
				return false, err
			}
			if isEntry {
				return true, nil
			}
			logger.Debug("Jump engine table",
				log.String("address", fmt.Sprintf("0x%04X", engineCaller.tableStartAddress)),
				log.Int("entries", engineCaller.entries),
			)

			// jump engine table is processed, remove it from list to process
			j.jumpEngineCallers = slices.Delete(j.jumpEngineCallers, i, i+1)
			i--
		}
	}
	return false, nil
}
