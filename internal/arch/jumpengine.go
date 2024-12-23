package arch

// JumpEngine contains jump engine related helper.
type JumpEngine interface {
	// AddJumpEngine adds a jump engine function address to the list of jump engines.
	AddJumpEngine(address uint16)
	// GetContextDataReferences parse all instructions of the function context until the jump
	// and returns data references that could point to the function table.
	GetContextDataReferences(dis Disasm, offsets []Offset, addresses []uint16) ([]uint16, error)
	// GetFunctionTableReference detects a jump engine function context and its function table.
	GetFunctionTableReference(context uint16, dataReferences []uint16)
	// HandleJumpEngineDestination processes a newly detected jump engine destination.
	HandleJumpEngineDestination(dis Disasm, caller, destination uint16) error
	// HandleJumpEngineCallers processes all callers of a newly detected jump engine function.
	HandleJumpEngineCallers(dis Disasm, context uint16) error
	// JumpContextInfo builds the list of instructions of the current function context.
	JumpContextInfo(dis Disasm, jumpAddress uint16, offsetInfo Offset) ([]Offset, []uint16)
	// ScanForNewJumpEngineEntry scans all jump engine calls for an unprocessed entry in the function address table that
	// follows the call. It returns whether a new address to parse was added.
	ScanForNewJumpEngineEntry(dis Disasm) (bool, error)
}
