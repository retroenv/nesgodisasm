package disasm

import (
	"fmt"
	"strings"

	. "github.com/retroenv/retrogolib/nes/addressing"
	"github.com/retroenv/retrogolib/nes/cpu"
	"github.com/retroenv/retrogolib/nes/register"
)

type constTranslation struct {
	Read  string
	Write string
}

func (dis *Disasm) replaceParamByConstant(opcode cpu.Opcode, paramAsString string, address uint16, constantInfo constTranslation) string {
	// split parameter string in case of x/y indexing, only the first part will be replaced by a const name
	paramParts := strings.Split(paramAsString, ",")

	if constantInfo.Read != "" && opcode.ReadsMemory() {
		dis.usedConstants[address] = constantInfo
		paramParts[0] = constantInfo.Read
		return strings.Join(paramParts, ",")
	}
	if constantInfo.Write != "" && opcode.WritesMemory() {
		dis.usedConstants[address] = constantInfo
		paramParts[0] = constantInfo.Write
		return strings.Join(paramParts, ",")
	}

	return paramAsString
}

// buildConstMap builds the map of all known NES constants from all
// modules that maps an address to a constant name.
func buildConstMap() (map[uint16]constTranslation, error) {
	m := map[uint16]constTranslation{}
	if err := mergeConstantsMaps(m, register.APUAddressToName); err != nil {
		return nil, fmt.Errorf("processing apu constants: %w", err)
	}
	if err := mergeConstantsMaps(m, register.ControllerAddressToName); err != nil {
		return nil, fmt.Errorf("processing controller constants: %w", err)
	}
	if err := mergeConstantsMaps(m, register.PPUAddressToName); err != nil {
		return nil, fmt.Errorf("processing ppu constants: %w", err)
	}
	return m, nil
}

func mergeConstantsMaps(destination map[uint16]constTranslation, source map[uint16]AccessModeConstant) error {
	for address, constantInfo := range source {
		translation := destination[address]

		if constantInfo.Mode&ReadAccess != 0 {
			if translation.Read != "" {
				return fmt.Errorf("constant with address 0x%04X and read mode is defined twice", address)
			}
			translation.Read = constantInfo.Constant
		}

		if constantInfo.Mode&WriteAccess != 0 {
			if translation.Write != "" {
				return fmt.Errorf("constant with address 0x%04X and write mode is defined twice", address)
			}
			translation.Write = constantInfo.Constant
		}

		destination[address] = translation
	}
	return nil
}
