package disasm

import (
	"encoding/binary"

	"github.com/retroenv/nesgodisasm/internal/program"
	. "github.com/retroenv/retrogolib/nes/addressing"
	"github.com/retroenv/retrogolib/nes/cpu"
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
	if instruction.Name != cpu.JmpInstruction || offsetInfo.opcode.Addressing != IndirectAddressing {
		return
	}

	// parse all instructions of the function context until the jump
	for address := offsetInfo.context; address != 0 && address > jumpAddress; {
		address = dis.addressToOffset(address)
		offsetInfo = &dis.offsets[address]

		// if current function has a branching instruction beside the final jump, it's probably not one
		// of the common jump engines
		if _, ok := cpu.BranchingInstructions[offsetInfo.opcode.Instruction.Name]; ok {
			return
		}

		address += uint16(len(offsetInfo.OpcodeBytes))
	}

	// if code reaches this point, no branching instructions beside the final indirect jmp have been found
	// in the function, this makes it a likely jump engine
	dis.jumpEngines[offsetInfo.context] = struct{}{}

	dis.handleJumpEngineCallers(offsetInfo.context)
}

// checkForJumpEngineCall checks if the current instruction is a call into a jump engine function.
func (dis *Disasm) checkForJumpEngineCall(offsetInfo *offset, address uint16) {
	instruction := offsetInfo.opcode.Instruction
	if instruction.Name != cpu.JsrInstruction || offsetInfo.opcode.Addressing != AbsoluteAddressing {
		return
	}

	_, opcodes := dis.readOpParam(offsetInfo.opcode.Addressing)
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
	context = dis.addressToOffset(context)
	jumpEngineOffset := &dis.offsets[context]
	jumpEngineOffset.LabelComment = "jump engine detected"

	for _, caller := range jumpEngineOffset.branchFrom {
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
	offset := dis.addressToOffset(caller)
	offsetInfo := &dis.offsets[offset]
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

	destination := dis.readMemoryWord(address)
	if destination < CodeBaseAddress {
		jumpEngine.terminated = true
		return false
	}

	offset1 := dis.addressToOffset(address)
	offsetInfo1 := &dis.offsets[offset1]
	offset2 := dis.addressToOffset(address + 1)
	offsetInfo2 := &dis.offsets[offset2]

	if offsetInfo1.Offset.Type == program.CodeOffset || offsetInfo2.Offset.Type == program.CodeOffset {
		jumpEngine.terminated = true
		return false
	}

	offsetInfo1.Offset.SetType(program.FunctionReference)
	offsetInfo2.Offset.SetType(program.FunctionReference)

	offsetInfo1.OpcodeBytes = []byte{dis.readMemory(address), dis.readMemory(address + 1)}

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

			// jump engine table is processed, remove it from list to process
			dis.jumpEngineCallers = append(dis.jumpEngineCallers[:i], dis.jumpEngineCallers[i+1:]...)
			i--
		}
	}
	return false
}
