package disasm

import (
	. "github.com/retroenv/retrogolib/nes/addressing"
)

type paramReaderFunc func(*Disasm) (any, []byte)

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

func paramReaderImplied(*Disasm) (any, []byte) {
	return nil, nil
}

func paramReaderImmediate(dis *Disasm) (any, []byte) {
	b := dis.readMemory(dis.pc + 1)
	opcodes := []byte{b}
	return int(b), opcodes
}

func paramReaderAccumulator(*Disasm) (any, []byte) {
	return Accumulator(0), nil
}

func paramReaderAbsolute(dis *Disasm) (any, []byte) {
	b1 := uint16(dis.readMemory(dis.pc + 1))
	b2 := uint16(dis.readMemory(dis.pc + 2))
	w := b2<<8 | b1

	opcodes := []byte{byte(b1), byte(b2)}
	return Absolute(w), opcodes
}

func paramReaderAbsoluteX(dis *Disasm) (any, []byte) {
	b1 := uint16(dis.readMemory(dis.pc + 1))
	b2 := uint16(dis.readMemory(dis.pc + 2))
	w := b2<<8 | b1

	opcodes := []byte{byte(b1), byte(b2)}
	return AbsoluteX(w), opcodes
}

func paramReaderAbsoluteY(dis *Disasm) (any, []byte) {
	b1 := uint16(dis.readMemory(dis.pc + 1))
	b2 := uint16(dis.readMemory(dis.pc + 2))
	w := b2<<8 | b1

	opcodes := []byte{byte(b1), byte(b2)}
	return AbsoluteY(w), opcodes
}

func paramReaderZeroPage(dis *Disasm) (any, []byte) {
	b := dis.readMemory(dis.pc + 1)
	opcodes := []byte{b}
	return ZeroPage(b), opcodes
}

func paramReaderZeroPageX(dis *Disasm) (any, []byte) {
	b := dis.readMemory(dis.pc + 1)
	opcodes := []byte{b}
	return ZeroPageX(b), opcodes
}

func paramReaderZeroPageY(dis *Disasm) (any, []byte) {
	b := dis.readMemory(dis.pc + 1)
	opcodes := []byte{b}
	return ZeroPageY(b), opcodes
}

func paramReaderRelative(dis *Disasm) (any, []byte) {
	offset := uint16(dis.readMemory(dis.pc + 1))

	var address uint16
	if offset < 0x80 {
		address = dis.pc + 2 + offset
	} else {
		address = dis.pc + 2 + offset - 0x100
	}

	opcodes := []byte{byte(offset)}
	return Absolute(address), opcodes
}

func paramReaderIndirect(dis *Disasm) (any, []byte) {
	address := dis.readMemoryWordBug(dis.pc + 1)
	b1 := dis.readMemory(dis.pc + 1)
	b2 := dis.readMemory(dis.pc + 2)

	opcodes := []byte{b1, b2}
	return Indirect(address), opcodes
}

func paramReaderIndirectX(dis *Disasm) (any, []byte) {
	b := dis.readMemory(dis.pc + 1)
	opcodes := []byte{b}
	return IndirectX(b), opcodes
}

func paramReaderIndirectY(dis *Disasm) (any, []byte) {
	b := dis.readMemory(dis.pc + 1)
	opcodes := []byte{b}
	return IndirectY(b), opcodes
}
