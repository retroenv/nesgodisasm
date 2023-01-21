package disasm

import (
	"encoding/binary"
	"fmt"

	"github.com/retroenv/nesgodisasm/internal/program"
	"github.com/retroenv/retrogolib/log"
	. "github.com/retroenv/retrogolib/nes/addressing"
	"github.com/retroenv/retrogolib/nes/cpu"
)

const (
	jumpEngineLastInstructionsCheck = 16
	jumpEngineMaxContextSize        = 0x25
)

// jumpEngineCaller stores info about a caller of a jump engine, which is followed by a list of function addresses
type jumpEngineCaller struct {
	entries           int  // count of referenced functions in the table
	terminated        bool // marks whether the end of the table has been found
	tableStartAddress uint16
}

// checkForJumpEngineJmp checks if the current instruction is the jump instruction inside a jump engine function.
// The function offsets after the call to the jump engine will be used as destinations to disassemble as code.
// This can be found in some official games like Super Mario Bros.
func (dis *Disasm) checkForJumpEngineJmp(offsetInfo *offset, jumpAddress uint16) {
	instruction := offsetInfo.opcode.Instruction
	if instruction.Name != cpu.Jmp.Name || offsetInfo.opcode.Addressing != IndirectAddressing {
		return
	}

	contextOffsets, contextAddresses := dis.jumpContextInfo(offsetInfo, jumpAddress)
	contextSize := jumpAddress - offsetInfo.context + 3
	dataReferences := dis.getContextDataReferences(contextOffsets, contextAddresses)

	if len(dataReferences) > 1 {
		dis.getFunctionTableReference(offsetInfo.context, dataReferences)
	}

	dis.options.Logger.Debug("Jump engine detected",
		log.String("address", fmt.Sprintf("0x%04X", jumpAddress)),
		log.Uint16("code_size", contextSize),
	)

	// if code reaches this point, no branching instructions beside the final indirect jmp have been found
	// in the function, this makes it likely a jump engine
	dis.jumpEngines[offsetInfo.context] = struct{}{}

	if contextSize < jumpEngineMaxContextSize {
		dis.handleJumpEngineCallers(offsetInfo.context)
	} else {
		offsetInfo.Comment = "jump engine detected"
	}
}

// TODO use jump address as key to be able to handle large function
// contexts containing multiple jump engines
func (dis *Disasm) getFunctionTableReference(context uint16, dataReferences []uint16) {
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

// getContextDataReferences parse all instructions of the function context until the jump
// and returns data references that could point to the function table.
func (dis *Disasm) getContextDataReferences(offsets []*offset, addresses []uint16) []uint16 {
	var dataReferences []uint16

	for i, offsetInfoInstruction := range offsets {
		address := addresses[i]
		opcode := offsetInfoInstruction.opcode

		// look for an instructions that loads data from an address in the code or data
		// address range. this should be the table containing the function addresses.
		if !opcode.ReadsMemory() {
			continue
		}

		param, _ := dis.readOpParam(opcode.Addressing, address)
		reference, ok := getAddressingParam(param)
		if ok && reference >= dis.codeBaseAddress && reference < irqStartAddress {
			dataReferences = append(dataReferences, reference)
		}
	}
	return dataReferences
}

// jumpContextInfo builds the list of instructions of the current function context.
// in some ROMs the jump engine can be part of a label inside a larger function,
// the jump engine detection will use the last instructions before the jmp.
func (dis *Disasm) jumpContextInfo(offsetInfo *offset, jumpAddress uint16) ([]*offset, []uint16) {
	var offsets []*offset
	var addresses []uint16

	for address := offsetInfo.context; address != 0 && address < jumpAddress; {
		index := dis.addressToIndex(address)
		offsetInfoInstruction := &dis.offsets[index]

		// skip offsets that have not been processed yet
		if len(offsetInfoInstruction.OpcodeBytes) == 0 {
			address++
			continue
		}

		offsets = append(offsets, offsetInfoInstruction)
		addresses = append(addresses, address)

		address += uint16(len(offsetInfoInstruction.OpcodeBytes))
	}

	if len(offsets) > jumpEngineLastInstructionsCheck {
		offsets = offsets[len(offsets)-jumpEngineLastInstructionsCheck-1:]
		addresses = addresses[len(addresses)-jumpEngineLastInstructionsCheck-1:]
	}

	return offsets, addresses
}

// checkForJumpEngineCall checks if the current instruction is a call into a jump engine function.
func (dis *Disasm) checkForJumpEngineCall(offsetInfo *offset, address uint16) {
	instruction := offsetInfo.opcode.Instruction
	if instruction.Name != cpu.Jsr.Name || offsetInfo.opcode.Addressing != AbsoluteAddressing {
		return
	}

	_, opcodes := dis.readOpParam(offsetInfo.opcode.Addressing, dis.pc)
	destination := binary.LittleEndian.Uint16(opcodes)
	for addr := range dis.jumpEngines {
		if addr == destination {
			dis.handleJumpEngineCaller(address)
			return
		}
	}
}

// handleJumpEngineCallers processes all callers of a newly detected jump engine function.
func (dis *Disasm) handleJumpEngineCallers(context uint16) {
	index := dis.addressToIndex(context)
	offsetInfo := &dis.offsets[index]
	offsetInfo.LabelComment = "jump engine detected"
	offsetInfo.SetType(program.JumpEngine)

	for _, caller := range offsetInfo.branchFrom {
		dis.handleJumpEngineCaller(caller)
	}
}

// handleJumpEngineCaller processes a newly detected jump engine caller, the return address of the call is
// marked as function reference instead of code. The first entry of the function table is processed.
func (dis *Disasm) handleJumpEngineCaller(caller uint16) {
	jumpEngine, ok := dis.jumpEngineCallersAdded[caller]
	if !ok {
		jumpEngine = &jumpEngineCaller{}
		dis.jumpEngineCallersAdded[caller] = jumpEngine
		dis.jumpEngineCallers = append(dis.jumpEngineCallers, jumpEngine)
	}

	// get the address of the function pointers after the jump engine call
	index := dis.addressToIndex(caller)
	offsetInfo := &dis.offsets[index]
	address := caller + uint16(len(offsetInfo.OpcodeBytes))

	// remove from code that should be parsed
	delete(dis.functionReturnsToParseAdded, address)
	jumpEngine.tableStartAddress = address

	dis.processJumpEngineEntry(jumpEngine, address)
}

// processJumpEngineEntry processes a potential function reference in a jump engine table.
// It returns whether the entry was a valid function reference address and has been added for processing.
func (dis *Disasm) processJumpEngineEntry(jumpEngine *jumpEngineCaller, address uint16) bool {
	if jumpEngine.terminated {
		return false
	}

	// verify that the destination is in valid code address range
	destination := dis.readMemoryWord(address)
	if destination < dis.codeBaseAddress || destination >= irqStartAddress {
		jumpEngine.terminated = true
		return false
	}

	indexByte1 := dis.addressToIndex(address)
	offsetInfo1 := &dis.offsets[indexByte1]
	indexByte2 := dis.addressToIndex(address + 1)
	offsetInfo2 := &dis.offsets[indexByte2]

	// if the potential jump table entry is already marked as code, the table end is reached
	if offsetInfo1.Offset.Type == program.CodeOffset || offsetInfo2.Offset.Type == program.CodeOffset {
		jumpEngine.terminated = true
		return false
	}

	if jumpEngine.entries == 0 {
		offsetInfo1.Offset.SetType(program.JumpTable)
	}

	offsetInfo1.Offset.SetType(program.FunctionReference)
	offsetInfo2.Offset.SetType(program.FunctionReference)

	offsetInfo1.OpcodeBytes = []byte{dis.readMemory(address), dis.readMemory(address + 1)}
	offsetInfo2.OpcodeBytes = nil

	jumpEngine.entries++

	dis.addAddressToParse(destination, destination, address, nil, true)
	return true
}

// scanForNewJumpEngineEntry scans all jump engine calls for an unprocessed entry in the function address table that
// follows the call. It returns whether a new address to parse was added.
func (dis *Disasm) scanForNewJumpEngineEntry() bool {
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
			return false
		}

		for i := 0; i < len(dis.jumpEngineCallers); i++ {
			engineCaller := dis.jumpEngineCallers[i]
			if engineCaller.entries != minEntries {
				continue
			}

			// calculate next address in table to process
			address := engineCaller.tableStartAddress + uint16(2*engineCaller.entries)
			if dis.processJumpEngineEntry(engineCaller, address) {
				return true
			}
			dis.options.Logger.Debug("Jump engine table",
				log.String("address", fmt.Sprintf("0x%04X", engineCaller.tableStartAddress)),
				log.Int("entries", engineCaller.entries),
			)

			// jump engine table is processed, remove it from list to process
			dis.jumpEngineCallers = append(dis.jumpEngineCallers[:i], dis.jumpEngineCallers[i+1:]...)
			i--
		}
	}
	return false
}
