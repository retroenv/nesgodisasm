// Package assembler defines the available assembler output formats.
package assembler

import (
	"io"
)

const (
	Asm6   = "asm6"
	Ca65   = "ca65"
	Nesasm = "nesasm"
)

// NewBankWriter is a callback that creates a new file for a bank of ROMs
// that have multiple PRG banks.
type NewBankWriter func(baseName string) (io.WriteCloser, error)
