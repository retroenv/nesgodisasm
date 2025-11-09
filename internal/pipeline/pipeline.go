// Package pipeline orchestrates the disassembly workflow stages.
package pipeline

import (
	"context"
	"fmt"
	"io"

	"github.com/retroenv/retrodisasm/internal/app"
	"github.com/retroenv/retrodisasm/internal/arch/chip8"
	"github.com/retroenv/retrodisasm/internal/arch/m6502"
	"github.com/retroenv/retrodisasm/internal/assembler"
	"github.com/retroenv/retrodisasm/internal/detector"
	"github.com/retroenv/retrodisasm/internal/disasm"
	"github.com/retroenv/retrodisasm/internal/loader"
	"github.com/retroenv/retrodisasm/internal/options"
	"github.com/retroenv/retrodisasm/internal/program"
	"github.com/retroenv/retrodisasm/internal/verification"
	"github.com/retroenv/retrogolib/arch"
	"github.com/retroenv/retrogolib/arch/system/nes/cartridge"
	"github.com/retroenv/retrogolib/arch/system/nes/parameter"
	"github.com/retroenv/retrogolib/log"
)

// Pipeline orchestrates the complete disassembly workflow.
type Pipeline struct {
	logger   *log.Logger
	detector *detector.Detector
	loader   *loader.Loader
}

// New creates a new disassembly pipeline.
func New(logger *log.Logger) *Pipeline {
	return &Pipeline{
		logger:   logger,
		detector: detector.New(logger),
		loader:   loader.New(),
	}
}

// Execute runs the complete disassembly pipeline.
func (p *Pipeline) Execute(ctx context.Context, opts options.Program, disasmOpts options.Disassembler, writer io.Writer) (*program.Program, error) {
	// Detect system architecture
	system := p.detector.Detect(opts)

	// Update disasm options with detected system
	disasmOpts.System = system
	disasmOpts.Binary = opts.Binary

	// When using binary mode, only output code without NES-specific segments
	if opts.Binary {
		disasmOpts.CodeOnly = true
	}

	// Validate assembler compatibility
	if err := assembler.ValidateSystemAssembler(system, opts.Assembler); err != nil {
		return nil, fmt.Errorf("incompatible assembler: %w", err)
	}

	// Load cartridge
	cart, cdlReader, err := p.loader.Load(opts, system)
	if err != nil {
		return nil, fmt.Errorf("loading cartridge: %w", err)
	}
	if cdlReader != nil {
		defer func() { _ = cdlReader.Close() }()
		disasmOpts.CodeDataLog = cdlReader
	}

	// Create disassembler
	dis, err := p.createDisassembler(opts, disasmOpts, cart, system)
	if err != nil {
		return nil, fmt.Errorf("creating disassembler: %w", err)
	}

	// Print info before processing
	app.PrintInfo(p.logger, opts, cart, system)

	// Run disassembly
	result, err := p.runDisassembly(dis, writer)
	if err != nil {
		return nil, fmt.Errorf("disassembling: %w", err)
	}

	// Verify output (if requested)
	if opts.AssembleTest {
		if err := verification.VerifyOutput(ctx, p.logger, opts, cart, result); err != nil {
			return nil, fmt.Errorf("verification failed: %w", err)
		}
		p.logger.Info("Verification successful")
	}

	return result, nil
}

// createDisassembler creates and configures the disassembler for the detected system.
func (p *Pipeline) createDisassembler(opts options.Program, disasmOpts options.Disassembler,
	cart *cartridge.Cartridge, system arch.System) (*disasm.Disasm, error) {

	fileWriterConstructor, paramConverter, err := app.InitializeAssemblerCompatibleMode(opts.Assembler)
	if err != nil {
		return nil, fmt.Errorf("initializing assembler compatible mode: %w", err)
	}

	dis, err := p.createDisassemblerForSystem(system, paramConverter, cart, disasmOpts, fileWriterConstructor)
	if err != nil {
		return nil, fmt.Errorf("creating disassembler: %w", err)
	}
	return dis, nil
}

// createDisassemblerForSystem creates a disassembler for the specified system architecture.
func (p *Pipeline) createDisassemblerForSystem(system arch.System, paramConverter parameter.Converter,
	cart *cartridge.Cartridge, disasmOpts options.Disassembler, fileWriterConstructor disasm.FileWriterConstructor) (*disasm.Disasm, error) {

	switch system {
	case arch.NES:
		archImpl := m6502.New(p.logger, paramConverter)
		dis, err := disasm.New(p.logger, archImpl, cart, disasmOpts, fileWriterConstructor)
		if err != nil {
			return nil, fmt.Errorf("creating m6502 disassembler: %w", err)
		}
		return dis, nil
	case arch.CHIP8System:
		archImpl := chip8.New(paramConverter)
		dis, err := disasm.New(p.logger, archImpl, cart, disasmOpts, fileWriterConstructor)
		if err != nil {
			return nil, fmt.Errorf("creating chip8 disassembler: %w", err)
		}
		return dis, nil
	default:
		return nil, fmt.Errorf("unsupported system '%s'", system)
	}
}

// runDisassembly executes the disassembly process.
func (p *Pipeline) runDisassembly(dis *disasm.Disasm, writer io.Writer) (*program.Program, error) {
	newBankWriter := func(bankName string) (io.WriteCloser, error) {
		return &nopCloser{writer}, nil
	}

	result, err := dis.Process(writer, newBankWriter)
	if err != nil {
		return nil, fmt.Errorf("processing disassembly: %w", err)
	}
	return result, nil
}

// nopCloser wraps an io.Writer to add a no-op Close method.
type nopCloser struct {
	io.Writer
}

func (nc *nopCloser) Close() error {
	return nil
}
