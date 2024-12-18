package disasm

import (
	"github.com/retroenv/retrogolib/arch/cpu/m6502"
)

type paramReaderFunc func(dis *Disasm, address uint16) (any, []byte, error)

var paramReader = map[m6502.AddressingMode]paramReaderFunc{
	m6502.ImpliedAddressing:     paramReaderImplied,
	m6502.ImmediateAddressing:   paramReaderImmediate,
	m6502.AccumulatorAddressing: paramReaderAccumulator,
	m6502.AbsoluteAddressing:    paramReaderAbsolute,
	m6502.AbsoluteXAddressing:   paramReaderAbsoluteX,
	m6502.AbsoluteYAddressing:   paramReaderAbsoluteY,
	m6502.ZeroPageAddressing:    paramReaderZeroPage,
	m6502.ZeroPageXAddressing:   paramReaderZeroPageX,
	m6502.ZeroPageYAddressing:   paramReaderZeroPageY,
	m6502.RelativeAddressing:    paramReaderRelative,
	m6502.IndirectAddressing:    paramReaderIndirect,
	m6502.IndirectXAddressing:   paramReaderIndirectX,
	m6502.IndirectYAddressing:   paramReaderIndirectY,
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
	return m6502.Accumulator(0), nil, nil
}

func paramReaderAbsolute(dis *Disasm, address uint16) (any, []byte, error) {
	w, opcodes, err := paramReadWord(dis, address)
	if err != nil {
		return nil, nil, err
	}

	return m6502.Absolute(w), opcodes, nil
}

func paramReaderAbsoluteX(dis *Disasm, address uint16) (any, []byte, error) {
	w, opcodes, err := paramReadWord(dis, address)
	if err != nil {
		return nil, nil, err
	}
	return m6502.AbsoluteX(w), opcodes, nil
}

func paramReaderAbsoluteY(dis *Disasm, address uint16) (any, []byte, error) {
	w, opcodes, err := paramReadWord(dis, address)
	if err != nil {
		return nil, nil, err
	}
	return m6502.AbsoluteY(w), opcodes, nil
}

func paramReaderZeroPage(dis *Disasm, address uint16) (any, []byte, error) {
	b, err := dis.readMemory(address + 1)
	if err != nil {
		return nil, nil, err
	}
	opcodes := []byte{b}
	return m6502.ZeroPage(b), opcodes, nil
}

func paramReaderZeroPageX(dis *Disasm, address uint16) (any, []byte, error) {
	b, err := dis.readMemory(address + 1)
	if err != nil {
		return nil, nil, err
	}
	opcodes := []byte{b}
	return m6502.ZeroPageX(b), opcodes, nil
}

func paramReaderZeroPageY(dis *Disasm, address uint16) (any, []byte, error) {
	b, err := dis.readMemory(address + 1)
	if err != nil {
		return nil, nil, err
	}
	opcodes := []byte{b}
	return m6502.ZeroPageY(b), opcodes, nil
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
	return m6502.Absolute(address), opcodes, nil
}

func paramReaderIndirect(dis *Disasm, address uint16) (any, []byte, error) {
	// do not read actual address in disassembler mode
	w, opcodes, err := paramReadWord(dis, address)
	if err != nil {
		return nil, nil, err
	}
	return m6502.Indirect(w), opcodes, nil
}

func paramReaderIndirectX(dis *Disasm, address uint16) (any, []byte, error) {
	b, err := dis.readMemory(address + 1)
	if err != nil {
		return nil, nil, err
	}
	opcodes := []byte{b}
	return m6502.IndirectX(b), opcodes, nil
}

func paramReaderIndirectY(dis *Disasm, address uint16) (any, []byte, error) {
	b, err := dis.readMemory(address + 1)
	if err != nil {
		return nil, nil, err
	}
	opcodes := []byte{b}
	return m6502.IndirectY(b), opcodes, nil
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
