// Package ca65 provides helpers to create ca65 assembler compatible asm output.
package ca65

import (
	"fmt"

	"github.com/retroenv/retrogolib/nes/addressing"
)

const Name = "ca65"

// ParamConverter converts the opcode parameters to ca65
// compatible output.
type ParamConverter struct {
}

// Immediate converts the parameters to the assembler implementation compatible string.
func (c ParamConverter) Immediate(param any) string {
	return fmt.Sprintf("#$%02X", param)
}

// Accumulator converts the parameters to the assembler implementation compatible string.
func (c ParamConverter) Accumulator() string {
	return "a"
}

// Absolute converts the parameters to the assembler implementation compatible string.
func (c ParamConverter) Absolute(param any) string {
	switch val := param.(type) {
	case int, addressing.Absolute:
		return fmt.Sprintf("$%04X", val)
	case string:
		return val
	default:
		panic(fmt.Sprintf("unsupported param type %T", val))
	}
}

// AbsoluteX converts the parameters to the assembler implementation compatible string.
func (c ParamConverter) AbsoluteX(param any) string {
	switch val := param.(type) {
	case int, addressing.Absolute, addressing.AbsoluteX:
		return fmt.Sprintf("$%04X,X", val)
	case string:
		return fmt.Sprintf("%s,X", val)
	default:
		panic(fmt.Sprintf("unsupported param type %T", val))
	}
}

// AbsoluteY converts the parameters to the assembler implementation compatible string.
func (c ParamConverter) AbsoluteY(param any) string {
	switch val := param.(type) {
	case int, addressing.Absolute, addressing.AbsoluteY:
		return fmt.Sprintf("$%04X,Y", val)
	case string:
		return fmt.Sprintf("%s,Y", val)
	default:
		panic(fmt.Sprintf("unsupported param type %T", val))
	}
}

// ZeroPage converts the parameters to the assembler implementation compatible string.
func (c ParamConverter) ZeroPage(param any) string {
	switch val := param.(type) {
	case int, addressing.Absolute, addressing.ZeroPage:
		return fmt.Sprintf("$%02X", val)
	case string:
		return val
	default:
		panic(fmt.Sprintf("unsupported param type %T", val))
	}
}

// ZeroPageX converts the parameters to the assembler implementation compatible string.
func (c ParamConverter) ZeroPageX(param any) string {
	switch val := param.(type) {
	case int, addressing.Absolute, addressing.ZeroPage, addressing.ZeroPageX:
		return fmt.Sprintf("$%02X,X", val)
	case string:
		return fmt.Sprintf("%s,X", val)
	default:
		panic(fmt.Sprintf("unsupported param type %T", val))
	}
}

// ZeroPageY converts the parameters to the assembler implementation compatible string.
func (c ParamConverter) ZeroPageY(param any) string {
	switch val := param.(type) {
	case int, addressing.Absolute, addressing.ZeroPage, addressing.ZeroPageY:
		return fmt.Sprintf("$%02X,Y", val)
	case string:
		return fmt.Sprintf("%s,Y", val)
	default:
		panic(fmt.Sprintf("unsupported param type %T", val))
	}
}

// Relative converts the parameters to the assembler implementation compatible string.
func (c ParamConverter) Relative(param any) string {
	if param == nil {
		return ""
	}
	return fmt.Sprintf("$%04X", param)
}

// Indirect converts the parameters to the assembler implementation compatible string.
func (c ParamConverter) Indirect(param any) string {
	address, ok := param.(addressing.Indirect)
	if ok {
		return fmt.Sprintf("($%04X)", address)
	}
	alias := param.(string)
	return fmt.Sprintf("(%s)", alias)
}

// IndirectX converts the parameters to the assembler implementation compatible string.
func (c ParamConverter) IndirectX(param any) string {
	switch val := param.(type) {
	case addressing.Indirect, addressing.IndirectX:
		return fmt.Sprintf("($%04X,X)", val)
	case string:
		return fmt.Sprintf("(%s,X)", val)
	default:
		panic(fmt.Sprintf("unsupported param type %T", val))
	}
}

// IndirectY converts the parameters to the assembler implementation compatible string.
func (c ParamConverter) IndirectY(param any) string {
	switch val := param.(type) {
	case addressing.Indirect, addressing.IndirectY:
		return fmt.Sprintf("($%04X),Y", val)
	case string:
		return fmt.Sprintf("(%s),Y", val)
	default:
		panic(fmt.Sprintf("unsupported param type %T", val))
	}
}
