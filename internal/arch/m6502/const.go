package m6502

import (
	"fmt"

	"github.com/retroenv/nesgodisasm/internal/arch"
	"github.com/retroenv/retrogolib/arch/cpu/m6502"
	"github.com/retroenv/retrogolib/arch/nes/register"
)

// Constants builds the map of all known NES constants from all
// modules that maps an address to a constant name.
func (ar *Arch6502) Constants() (map[uint16]arch.Constant, error) {
	m := map[uint16]arch.Constant{}
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

func mergeConstantsMaps(destination map[uint16]arch.Constant, source map[uint16]m6502.AccessModeConstant) error {
	for address, constantInfo := range source {
		translation := destination[address]
		translation.Address = address

		if constantInfo.Mode&m6502.ReadAccess != 0 {
			if translation.Read != "" {
				return fmt.Errorf("constant with address 0x%04X and read mode is defined twice", address)
			}
			translation.Read = constantInfo.Constant
		}

		if constantInfo.Mode&m6502.WriteAccess != 0 {
			if translation.Write != "" {
				return fmt.Errorf("constant with address 0x%04X and write mode is defined twice", address)
			}
			translation.Write = constantInfo.Constant
		}

		destination[address] = translation
	}
	return nil
}
