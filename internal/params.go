package disasm

import (
	. "github.com/retroenv/retrogolib/addressing"
)

type paramReaderFunc func(dis *Disasm, address uint16) (any, []byte, error)

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

func paramReaderImplied(*Disasm, uint16) (any, []byte, error) {
	return nil, nil, nil
}

func paramReaderImmediate(dis *Disasm, address uint16) (any, []byte, error) {
	b, err := dis.readMemory(address + 1)
	if err != nil {
		return nil, nil, err
	}
	opcodes := []byte{b}
	return int(b), opcodes, nil
}

func paramReaderAccumulator(*Disasm, uint16) (any, []byte, error) {
	return Accumulator(0), nil, nil
}

func paramReaderAbsolute(dis *Disasm, address uint16) (any, []byte, error) {
	w, opcodes, err := paramReadWord(dis, address)
	if err != nil {
		return nil, nil, err
	}

	return Absolute(w), opcodes, nil
}

func paramReaderAbsoluteX(dis *Disasm, address uint16) (any, []byte, error) {
	w, opcodes, err := paramReadWord(dis, address)
	if err != nil {
		return nil, nil, err
	}
	return AbsoluteX(w), opcodes, nil
}

func paramReaderAbsoluteY(dis *Disasm, address uint16) (any, []byte, error) {
	w, opcodes, err := paramReadWord(dis, address)
	if err != nil {
		return nil, nil, err
	}
	return AbsoluteY(w), opcodes, nil
}

func paramReaderZeroPage(dis *Disasm, address uint16) (any, []byte, error) {
	b, err := dis.readMemory(address + 1)
	if err != nil {
		return nil, nil, err
	}
	opcodes := []byte{b}
	return ZeroPage(b), opcodes, nil
}

func paramReaderZeroPageX(dis *Disasm, address uint16) (any, []byte, error) {
	b, err := dis.readMemory(address + 1)
	if err != nil {
		return nil, nil, err
	}
	opcodes := []byte{b}
	return ZeroPageX(b), opcodes, nil
}

func paramReaderZeroPageY(dis *Disasm, address uint16) (any, []byte, error) {
	b, err := dis.readMemory(address + 1)
	if err != nil {
		return nil, nil, err
	}
	opcodes := []byte{b}
	return ZeroPageY(b), opcodes, nil
}

func paramReaderRelative(dis *Disasm, address uint16) (any, []byte, error) {
	b, err := dis.readMemory(address + 1)
	if err != nil {
		return nil, nil, err
	}

	offset := uint16(b)
	if offset < 0x80 {
		address += 2 + offset
	} else {
		address += 2 + offset - 0x100
	}

	opcodes := []byte{byte(offset)}
	return Absolute(address), opcodes, nil
}

func paramReaderIndirect(dis *Disasm, address uint16) (any, []byte, error) {
	// do not read actual address in disassembler mode
	w, opcodes, err := paramReadWord(dis, address)
	if err != nil {
		return nil, nil, err
	}
	return Indirect(w), opcodes, nil
}

func paramReaderIndirectX(dis *Disasm, address uint16) (any, []byte, error) {
	b, err := dis.readMemory(address + 1)
	if err != nil {
		return nil, nil, err
	}
	opcodes := []byte{b}
	return IndirectX(b), opcodes, nil
}

func paramReaderIndirectY(dis *Disasm, address uint16) (any, []byte, error) {
	b, err := dis.readMemory(address + 1)
	if err != nil {
		return nil, nil, err
	}
	opcodes := []byte{b}
	return IndirectY(b), opcodes, nil
}

func paramReadWord(dis *Disasm, address uint16) (uint16, []byte, error) {
	b1, err := dis.readMemory(address + 1)
	if err != nil {
		return 0, nil, err
	}
	b2, err := dis.readMemory(address + 2)
	if err != nil {
		return 0, nil, err
	}
	w := uint16(b2)<<uint16(8) | uint16(b1)
	opcodes := []byte{b1, b2}
	return w, opcodes, nil
}
