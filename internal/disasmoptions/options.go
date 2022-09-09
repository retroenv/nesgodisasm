// Package disasmoptions implements disassembler options that are passed down to the actual
// assembly writer.
package disasmoptions

import "io"

// Options defines options to control the disassembler.
type Options struct {
	Assembler   string        // what assembler to use
	CodeDataLog io.ReadCloser // Code/Data log file to parse

	CodeOnly       bool
	HexComments    bool
	OffsetComments bool
	ZeroBytes      bool
}

// New returns a new options instance with default options.
func New() Options {
	return Options{
		HexComments:    true,
		OffsetComments: true,
	}
}
