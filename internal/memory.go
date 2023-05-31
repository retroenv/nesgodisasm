package disasm

import (
	"fmt"

	. "github.com/retroenv/retrogolib/addressing"
	"github.com/retroenv/retrogolib/arch/nes"
)

func (dis *Disasm) readMemory(address uint16) byte {
	var value byte

	switch {
	case address < 0x2000:
		value = dis.cart.CHR[address]

	case address >= nes.CodeBaseAddress:
		index := dis.addressToIndex(address)
		value = dis.cart.PRG[index]

	default:
		panic(fmt.Sprintf("invalid read from address #%0000x", address))
	}
	return value
}

func (dis *Disasm) readMemoryWord(address uint16) uint16 {
	low := uint16(dis.readMemory(address))
	high := uint16(dis.readMemory(address + 1))
	w := (high << 8) | low
	return w
}

// readOpParam reads the opcode parameters after the first opcode byte
// and translates it into emulator specific types.
func (dis *Disasm) readOpParam(addressing Mode, address uint16) (any, []byte) {
	fun, ok := paramReader[addressing]
	if !ok {
		panic(fmt.Errorf("unsupported addressing mode %00x", addressing))
	}
	return fun(dis, address)
}
