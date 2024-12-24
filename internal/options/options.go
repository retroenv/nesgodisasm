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
	CodeDataLog string
	Config      string
	Input       string
	Output      string
	System      string

	AssembleTest bool
	Binary       bool
	Debug        bool
	Quiet        bool

	NoHexComments bool
	NoOffsets     bool
}

// Disassembler defines options to control the disassembler.
type Disassembler struct {
	Assembler   string        // what assembler to use
	CodeDataLog io.ReadCloser // Code/Data log file to parse

	Binary                   bool
	CodeOnly                 bool
	HexComments              bool
	NoUnofficialInstructions bool
	OffsetComments           bool
	ZeroBytes                bool
}

// NewDisassembler returns a new options instance with default options.
func NewDisassembler(assemblerName string) Disassembler {
	return Disassembler{
		Assembler:      strings.ToLower(assemblerName),
		HexComments:    true,
		OffsetComments: true,
	}
}
