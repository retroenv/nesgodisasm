// Package config handles application configuration and setup
package config

import (
	disasm "github.com/retroenv/nesgodisasm/internal"
	"github.com/retroenv/nesgodisasm/internal/arch/m6502"
	"github.com/retroenv/nesgodisasm/internal/assembler/asm6"
	"github.com/retroenv/nesgodisasm/internal/assembler/ca65"
	"github.com/retroenv/nesgodisasm/internal/assembler/nesasm"
	"github.com/retroenv/retrogolib/arch/nes/parameter"
	"github.com/retroenv/retrogolib/log"
)

// CreateLogger creates a logger with appropriate settings
func CreateLogger(debug, quiet bool) *log.Logger {
	cfg := log.DefaultConfig()
	if debug {
		cfg.Level = log.DebugLevel
	} else if quiet {
		cfg.Level = log.ErrorLevel
	}
	return log.NewWithConfig(cfg)
}

// CreateFileWriterConstructor creates the appropriate file writer constructor based on assembler type
func CreateFileWriterConstructor(assemblerType string) (disasm.FileWriterConstructor, error) {
	switch assemblerType {
	case "asm6":
		return asm6.New, nil
	case "ca65":
		return ca65.New, nil
	case "nesasm":
		return nesasm.New, nil
	default:
		return nil, &UnsupportedAssemblerError{Assembler: assemblerType}
	}
}

// CreateArchitecture creates the CPU architecture implementation
func CreateArchitecture() *m6502.Arch6502 {
	converter := parameter.New(parameter.Config{})
	return m6502.New(converter)
}

// UnsupportedAssemblerError represents an error for unsupported assembler types
type UnsupportedAssemblerError struct {
	Assembler string
}

func (e *UnsupportedAssemblerError) Error() string {
	return "unsupported assembler: " + e.Assembler
}
