package disasm

import (
	"strings"

	"github.com/retroenv/nesgodisasm/internal/arch"
)

// ReplaceParamByConstant replaces the parameter of an instruction by a constant name
// if the address of the instruction is found in the constants map.
func (dis *Disasm) ReplaceParamByConstant(address uint16, opcode arch.Opcode,
	paramAsString string) (string, bool) {

	constantInfo, ok := dis.constants[address]
	if !ok {
		return "", false
	}

	// split parameter string in case of x/y indexing, only the first part will be replaced by a const name
	paramParts := strings.Split(paramAsString, ",")

	if constantInfo.Read != "" && opcode.ReadsMemory() {
		dis.usedConstants[address] = constantInfo
		paramParts[0] = constantInfo.Read
		return strings.Join(paramParts, ","), true
	}
	if constantInfo.Write != "" && opcode.WritesMemory() {
		dis.usedConstants[address] = constantInfo
		paramParts[0] = constantInfo.Write
		return strings.Join(paramParts, ","), true
	}

	return paramAsString, true
}
