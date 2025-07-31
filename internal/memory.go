package disasm

import (
	"fmt"

	"github.com/retroenv/retrogolib/arch/system/nes"
)

func (dis *Disasm) ReadMemory(address uint16) (byte, error) {
	var value byte

	switch {
	case address < 0x2000:
		value = dis.cart.CHR[address]

	case address >= nes.CodeBaseAddress:
		value = dis.mapper.ReadMemory(address)

	default:
		return 0, fmt.Errorf("invalid read from address #%0000x", address)
	}
	return value, nil
}

func (dis *Disasm) ReadMemoryWord(address uint16) (uint16, error) {
	b, err := dis.ReadMemory(address)
	if err != nil {
		return 0, err
	}
	low := uint16(b)

	b, err = dis.ReadMemory(address + 1)
	if err != nil {
		return 0, err
	}

	high := uint16(b)
	w := (high << 8) | low
	return w, nil
}
