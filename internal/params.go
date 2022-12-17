package disasm

import (
	. "github.com/retroenv/retrogolib/nes/addressing"
)

type paramReaderFunc func(dis *Disasm, address uint16) (any, []byte)

var paramReader = map[Mode]paramReaderFunc{
	ImpliedAddressing:     paramReaderImplied,
	ImmediateAddressing:   paramReaderImmediate,
	AccumulatorAddressing: paramReaderAccumulator,
	AbsoluteAddressing:    paramReaderAbsolute,
	AbsoluteXAddressing:   paramReaderAbsoluteX,
	AbsoluteYAddressing:   paramReaderAbsoluteY,
	ZeroPageAddressing:    paramReaderZeroPage,
	ZeroPageXAddressing:   paramReaderZeroPageX,
	ZeroPageYAddressing:   paramReaderZeroPageY,
	RelativeAddressing:    paramReaderRelative,
	IndirectAddressing:    paramReaderIndirect,
	IndirectXAddressing:   paramReaderIndirectX,
	IndirectYAddressing:   paramReaderIndirectY,
}

func paramReaderImplied(*Disasm, uint16) (any, []byte) {
	return nil, nil
}

func paramReaderImmediate(dis *Disasm, address uint16) (any, []byte) {
	b := dis.readMemory(address + 1)
	opcodes := []byte{b}
	return int(b), opcodes
}

func paramReaderAccumulator(*Disasm, uint16) (any, []byte) {
	return Accumulator(0), nil
}

func paramReaderAbsolute(dis *Disasm, address uint16) (any, []byte) {
	b1 := uint16(dis.readMemory(address + 1))
	b2 := uint16(dis.readMemory(address + 2))
	w := b2<<8 | b1

	opcodes := []byte{byte(b1), byte(b2)}
	return Absolute(w), opcodes
}

func paramReaderAbsoluteX(dis *Disasm, address uint16) (any, []byte) {
	b1 := uint16(dis.readMemory(address + 1))
	b2 := uint16(dis.readMemory(address + 2))
	w := b2<<8 | b1

	opcodes := []byte{byte(b1), byte(b2)}
	return AbsoluteX(w), opcodes
}

func paramReaderAbsoluteY(dis *Disasm, address uint16) (any, []byte) {
	b1 := uint16(dis.readMemory(address + 1))
	b2 := uint16(dis.readMemory(address + 2))
	w := b2<<8 | b1

	opcodes := []byte{byte(b1), byte(b2)}
	return AbsoluteY(w), opcodes
}

func paramReaderZeroPage(dis *Disasm, address uint16) (any, []byte) {
	b := dis.readMemory(address + 1)
	opcodes := []byte{b}
	return ZeroPage(b), opcodes
}

func paramReaderZeroPageX(dis *Disasm, address uint16) (any, []byte) {
	b := dis.readMemory(address + 1)
	opcodes := []byte{b}
	return ZeroPageX(b), opcodes
}

func paramReaderZeroPageY(dis *Disasm, address uint16) (any, []byte) {
	b := dis.readMemory(address + 1)
	opcodes := []byte{b}
	return ZeroPageY(b), opcodes
}

func paramReaderRelative(dis *Disasm, address uint16) (any, []byte) {
	offset := uint16(dis.readMemory(address + 1))

	if offset < 0x80 {
		address += 2 + offset
	} else {
		address += 2 + offset - 0x100
	}

	opcodes := []byte{byte(offset)}
	return Absolute(address), opcodes
}

func paramReaderIndirect(dis *Disasm, address uint16) (any, []byte) {
	address = dis.readMemoryWordBug(address + 1)
	b1 := dis.readMemory(address + 1)
	b2 := dis.readMemory(address + 2)

	opcodes := []byte{b1, b2}
	return Indirect(address), opcodes
}

func paramReaderIndirectX(dis *Disasm, address uint16) (any, []byte) {
	b := dis.readMemory(address + 1)
	opcodes := []byte{b}
	return IndirectX(b), opcodes
}

func paramReaderIndirectY(dis *Disasm, address uint16) (any, []byte) {
	b := dis.readMemory(address + 1)
	opcodes := []byte{b}
	return IndirectY(b), opcodes
}
