// Package options contains the program options.
package options

import (
	"io"
	"strings"

	"github.com/retroenv/nesgodisasm/internal/assembler"
)

// Program options of the disassembler.
type Program struct {
	Assembler   string
	Batch       string
	Input       string
	Output      string
	CodeDataLog string

	AssembleTest bool
	Debug        bool
	Quiet        bool

	NoHexComments bool
	NoOffsets     bool
}

// Disassembler defines options to control the disassembler.
type Disassembler struct {
	Assembler   string        // what assembler to use
	CodeDataLog io.ReadCloser // Code/Data log file to parse

	AbsolutePrefix string
	ZeroPagePrefix string

	CodeOnly       bool
	HexComments    bool
	OffsetComments bool
	ZeroBytes      bool
}

// NewDisassembler returns a new options instance with default options.
func NewDisassembler(assemblerName string) Disassembler {
	opts := Disassembler{
		Assembler:      strings.ToLower(assemblerName),
		HexComments:    true,
		OffsetComments: true,
	}

	switch opts.Assembler {
	case assembler.Asm6:
		opts.AbsolutePrefix = "a:"
		opts.ZeroPagePrefix = ""

	case assembler.Ca65:
		opts.AbsolutePrefix = "a:"
		opts.ZeroPagePrefix = "z:"

	case assembler.Nesasm:
		opts.AbsolutePrefix = ""
		opts.ZeroPagePrefix = "<"
	}

	return opts
}
