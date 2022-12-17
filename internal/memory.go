package disasm

import (
	"fmt"

	. "github.com/retroenv/retrogolib/nes/addressing"
)

func (dis *Disasm) readMemory(address uint16) byte {
	var value byte

	switch {
	case address < 0x2000:
		value = dis.cart.CHR[address]

	case address >= CodeBaseAddress:
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

// readMemoryWordBug reads a word from a memory address and emulates a 6502 bug that caused
// the low byte to wrap without incrementing the high byte.
func (dis *Disasm) readMemoryWordBug(address uint16) uint16 {
	low := uint16(dis.readMemory(address))
	address = (address & 0xFF00) | uint16(byte(address)+1)
	high := uint16(dis.readMemory(address))
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
