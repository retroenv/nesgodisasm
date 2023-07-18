// Package options contains the program options.
package options

import (
	"io"
	"strings"
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
	Assembler      string        // what assembler to use
	CodeDataLog    io.ReadCloser // Code/Data log file to parse
	ZeroPagePrefix string

	CodeOnly       bool
	HexComments    bool
	OffsetComments bool
	ZeroBytes      bool
}

// NewDisassembler returns a new options instance with default options.
func NewDisassembler(assembler string) Disassembler {
	opts := Disassembler{
		Assembler:      strings.ToLower(assembler),
		HexComments:    true,
		OffsetComments: true,
	}

	if opts.Assembler == "ca65" {
		opts.ZeroPagePrefix = "z:"
	}

	return opts
}
