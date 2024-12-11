package disasm

import (
	"fmt"

	. "github.com/retroenv/retrogolib/addressing"
	"github.com/retroenv/retrogolib/arch/nes"
)

func (dis *Disasm) readMemory(address uint16) (byte, error) {
	var value byte

	switch {
	case address < 0x2000:
		value = dis.cart.CHR[address]

	case address >= nes.CodeBaseAddress:
		value = dis.mapper.readMemory(address)

	default:
		return 0, fmt.Errorf("invalid read from address #%0000x", address)
	}
	return value, nil
}

func (dis *Disasm) readMemoryWord(address uint16) (uint16, error) {
	b, err := dis.readMemory(address)
	if err != nil {
		return 0, err
	}
	low := uint16(b)

	b, err = dis.readMemory(address + 1)
	if err != nil {
		return 0, err
	}

	high := uint16(b)
	w := (high << 8) | low
	return w, nil
}

// readOpParam reads the opcode parameters after the first opcode byte
// and translates it into emulator specific types.
func (dis *Disasm) readOpParam(addressing Mode, address uint16) (any, []byte, error) {
	fun, ok := paramReader[addressing]
	if !ok {
		return nil, nil, fmt.Errorf("unsupported addressing mode %00x", addressing)
	}
	return fun(dis, address)
}
