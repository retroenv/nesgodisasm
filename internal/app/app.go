// Package app provides the main application helper for the disassembler.
package app

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	disasm "github.com/retroenv/nesgodisasm/internal"
	"github.com/retroenv/nesgodisasm/internal/arch"
	"github.com/retroenv/nesgodisasm/internal/arch/chip8"
	"github.com/retroenv/nesgodisasm/internal/arch/m6502"
	"github.com/retroenv/nesgodisasm/internal/assembler"
	"github.com/retroenv/nesgodisasm/internal/assembler/asm6"
	"github.com/retroenv/nesgodisasm/internal/assembler/ca65"
	"github.com/retroenv/nesgodisasm/internal/assembler/nesasm"
	"github.com/retroenv/nesgodisasm/internal/options"
	"github.com/retroenv/retrogolib/arch/nes/cartridge"
	"github.com/retroenv/retrogolib/arch/nes/parameter"
	"github.com/retroenv/retrogolib/log"
)

// GetFiles returns the list of files to process, either a single file or the matched files for
// batch processing.
func GetFiles(options *options.Program) ([]string, error) {
	if options.Batch == "" {
		return []string{options.Input}, nil
	}

	files, err := filepath.Glob(options.Batch)
	if err != nil {
		return nil, fmt.Errorf("finding batch files failed: %w", err)
	}

	if len(files) == 0 {
		return nil, errors.New("no input files matched")
	}

	options.Output = ""
	return files, nil
}

// PrintInfo prints the information about the input file and the cartridge.
func PrintInfo(logger *log.Logger, opts options.Program, cart *cartridge.Cartridge) {
	if opts.Quiet {
		return
	}

	switch opts.System {
	case arch.SystemNES:
		logger.Info("Processing NES ROM",
			log.String("file", opts.Input),
			log.Uint8("mapper", cart.Mapper),
			log.String("assembler", opts.Assembler),
		)
		if cart.Mapper != 0 && cart.Mapper != 3 {
			logger.Warn("Support for this mapper is experimental, multi bank mapper support is still in development")
		}
	case arch.SystemChip8:
		logger.Info("Processing Chip-8 ROM",
			log.String("file", opts.Input),
		)
	}
}

// SystemArchitecture returns the architecture specific implementation for the given system.
func SystemArchitecture(paramConverter parameter.Converter, system string) (arch.Architecture, bool, error) {
	switch system {
	case arch.SystemNES:
		return m6502.New(paramConverter), false, nil
	case arch.SystemChip8:
		return chip8.New(paramConverter), true, nil
	default:
		return nil, false, errors.New("unsupported system or missing parameter")
	}
}

// InitializeAssemblerCompatibleMode sets the chosen assembler specific instances
// to be used to output compatible code.
func InitializeAssemblerCompatibleMode(assemblerName string) (disasm.FileWriterConstructor, parameter.Converter, error) {
	var fileWriterConstructor disasm.FileWriterConstructor
	var paramCfg parameter.Config

	switch strings.ToLower(assemblerName) {
	case assembler.Asm6:
		fileWriterConstructor = asm6.New
		paramCfg = asm6.ParamConfig

	case assembler.Ca65:
		fileWriterConstructor = ca65.New
		paramCfg = ca65.ParamConfig

	case assembler.Nesasm:
		fileWriterConstructor = nesasm.New
		paramCfg = nesasm.ParamConfig

	default:
		return nil, parameter.Converter{}, fmt.Errorf("unsupported assembler '%s'", assemblerName)
	}

	return fileWriterConstructor, parameter.New(paramCfg), nil
}