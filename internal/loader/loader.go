// Package loader handles cartridge file loading operations.
package loader

import (
	"fmt"
	"io"
	"os"

	"github.com/retroenv/retrodisasm/internal/options"
	"github.com/retroenv/retrogolib/arch"
	"github.com/retroenv/retrogolib/arch/system/nes/cartridge"
)

// Loader handles loading cartridge files from disk.
type Loader struct{}

// New creates a new cartridge loader.
func New() *Loader {
	return &Loader{}
}

// Load loads and parses a cartridge file based on the system type and options.
// It supports both NES ROM format and raw binary mode for systems like CHIP-8.
// Returns the cartridge and an optional Code/Data Log reader if specified.
func (l *Loader) Load(opts options.Program, system arch.System) (*cartridge.Cartridge, io.ReadCloser, error) {
	file, err := os.Open(opts.Input)
	if err != nil {
		return nil, nil, fmt.Errorf("opening file %s: %w", opts.Input, err)
	}
	defer func() { _ = file.Close() }()

	var cart *cartridge.Cartridge

	// Handle CHIP-8 and binary files as raw buffer data
	switch {
	case opts.Binary, system == arch.CHIP8System:
		cart, err = cartridge.LoadBuffer(file)
	default:
		cart, err = cartridge.LoadFile(file)
	}
	if err != nil {
		return nil, nil, fmt.Errorf("loading cartridge: %w", err)
	}

	// Open Code/Data Log file if specified
	var cdlReader io.ReadCloser
	if opts.CodeDataLog != "" {
		cdlReader, err = os.Open(opts.CodeDataLog)
		if err != nil {
			return nil, nil, fmt.Errorf("opening CDL file %s: %w", opts.CodeDataLog, err)
		}
	}

	return cart, cdlReader, nil
}
