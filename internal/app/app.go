// Package app provides the main application helper for the disassembler.
package app

import (
	"fmt"
	"strings"

	"github.com/retroenv/retrodisasm/internal/assembler"
	"github.com/retroenv/retrodisasm/internal/assembler/asm6"
	"github.com/retroenv/retrodisasm/internal/assembler/ca65"
	"github.com/retroenv/retrodisasm/internal/assembler/nesasm"
	"github.com/retroenv/retrodisasm/internal/assembler/retroasm"
	"github.com/retroenv/retrodisasm/internal/disasm"
	"github.com/retroenv/retrodisasm/internal/options"
	archsys "github.com/retroenv/retrogolib/arch"
	"github.com/retroenv/retrogolib/arch/system/nes/cartridge"
	"github.com/retroenv/retrogolib/arch/system/nes/parameter"
	"github.com/retroenv/retrogolib/log"
)

// PrintInfo prints the information about the input file and the cartridge.
func PrintInfo(logger *log.Logger, opts options.Program, cart *cartridge.Cartridge, system archsys.System) {
	if opts.Quiet {
		return
	}

	switch system {
	case archsys.NES:
		logger.Info("Processing NES ROM",
			log.String("file", opts.Input),
			log.Uint8("mapper", cart.Mapper),
			log.String("assembler", opts.Assembler),
		)
		if cart.Mapper != 0 && cart.Mapper != 3 {
			logger.Warn("Support for this mapper is experimental, multi bank mapper support is still in development")
		}

	case archsys.CHIP8System:
		logger.Info("Processing Chip-8 ROM",
			log.String("file", opts.Input),
			log.String("assembler", opts.Assembler),
		)
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

	case assembler.Retroasm:
		fileWriterConstructor = retroasm.New
		paramCfg = retroasm.ParamConfig

	default:
		return nil, parameter.Converter{}, fmt.Errorf("unsupported assembler '%s'", assemblerName)
	}

	return fileWriterConstructor, parameter.New(paramCfg), nil
}
