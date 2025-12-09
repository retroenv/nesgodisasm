// Package options contains the program options.
package options

import (
	"io"
	"strings"

	"github.com/retroenv/retrogolib/arch"
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
}

// Disassembler defines options to control the disassembler.
type Disassembler struct {
	Assembler   string        // what assembler to use
	CodeDataLog io.ReadCloser // Code/Data log file to parse
	System      arch.System   // system type (e.g., nes, chip8)

	AssemblerSupportsUnofficial bool // assembler can output unofficial opcodes (false for nesasm)
	Binary                      bool
	CodeOnly                    bool
	HexComments                 bool
	OffsetComments              bool
	StopAtUnofficial            bool // stop tracing at unofficial opcodes unless explicitly branched to
	ZeroBytes                   bool
}

// NewDisassembler returns a new options instance with default options.
func NewDisassembler(assemblerName, system string) Disassembler {
	return Disassembler{
		Assembler: strings.ToLower(assemblerName),
		System:    arch.System(system),

		HexComments:    true,
		OffsetComments: true,
	}
}
