package arch

import "github.com/retroenv/nesgodisasm/internal/program"

// VariableManager manages variables in the disassembled program.
type VariableManager interface {
	// AddBank adds a new bank to the variable manager.
	AddBank()
	// AddReference adds a variable reference if the opcode is accessing
	// the given address directly by reading or writing.
	AddReference(dis Disasm, addressReference, usageAddress uint16, opcode Opcode, forceVariableUsage bool)
	// Process processes all variables and updates the instructions that use them
	// with a generated alias name.
	Process(dis Disasm) error
	// SetBankVariables sets the used constants in the bank for outputting.
	SetBankVariables(bankID int, prgBank *program.PRGBank)
	// SetToProgram sets the used constants in the program for outputting.
	SetToProgram(app *program.Program)
}
