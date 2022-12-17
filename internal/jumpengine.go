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
	for address := offsetInfo.context; address != 0 && address < jumpAddress; {
		index := dis.addressToIndex(address)
		offsetInfoInstruction := &dis.offsets[index]
		opcode := offsetInfoInstruction.opcode

		// if current function has a branching instruction beside the final jump, it's probably not one
		// of the common jump engines
		if _, ok := cpu.BranchingInstructions[opcode.Instruction.Name]; ok {
			return
		}

		// look for an instructions that loads data from an address in the code or data
		// address range. this should be the table containing the function addresses.
		if opcode.ReadsMemory() {
			param, _ := dis.readOpParam(opcode.Addressing, address)
			reference, ok := getAddressingParam(param)
			if ok && reference > dis.codeBaseAddress {
				jumpEngine := &jumpEngineCaller{
					tableStartAddress: reference,
				}
				dis.jumpEngineCallersAdded[offsetInfo.context] = jumpEngine
				dis.jumpEngineCallers = append(dis.jumpEngineCallers, jumpEngine)
				break
			}
		}

		address += uint16(len(offsetInfoInstruction.OpcodeBytes))
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

	destination := dis.readMemoryWord(address)
	if destination < dis.codeBaseAddress {
		jumpEngine.terminated = true
		return false
	}

	indexByte1 := dis.addressToIndex(address)
	offsetInfo1 := &dis.offsets[indexByte1]
	indexByte2 := dis.addressToIndex(address + 1)
	offsetInfo2 := &dis.offsets[indexByte2]

	if offsetInfo1.Offset.Type == program.CodeOffset || offsetInfo2.Offset.Type == program.CodeOffset {
		jumpEngine.terminated = true
		return false
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

			// jump engine table is processed, remove it from list to process
			dis.jumpEngineCallers = append(dis.jumpEngineCallers[:i], dis.jumpEngineCallers[i+1:]...)
			i--
		}
	}
	return false
}
