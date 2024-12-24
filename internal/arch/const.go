package arch

import "github.com/retroenv/nesgodisasm/internal/program"

// ConstantManager manages constants in the disassembled program.
type ConstantManager interface {
	// AddBank adds a new bank to the constants manager.
	AddBank()
	// Process processes all constants and updates all banks with the used ones. There is currently no tracking
	// for in which bank a constant is used, it will be added to all banks for now.
	Process()
	// ReplaceParameter replaces the parameter of an instruction by a constant name
	// if the address of the instruction is found in the constants map.
	ReplaceParameter(address uint16, opcode Opcode, paramAsString string) (string, bool)
	// SetBankConstants sets the used constants in the bank for outputting.
	SetBankConstants(bankID int, prgBank *program.PRGBank)
	// SetToProgram sets the used constants in the program for outputting.
	SetToProgram(app *program.Program)
}
