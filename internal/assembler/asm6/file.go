package asm6

import (
	"fmt"
	"io"

	"github.com/retroenv/nesgodisasm/internal/assembler"
	"github.com/retroenv/nesgodisasm/internal/options"
	"github.com/retroenv/nesgodisasm/internal/program"
	"github.com/retroenv/nesgodisasm/internal/writer"
	"github.com/retroenv/retrogolib/arch/nes/cartridge"
)

var iNESHeader = `.db "NES", $1a                 ; Magic string that always begins an iNES header`

var headerByte = ".db $%02x %-22s ; %s\n"

var vectors = ".dw %s, %s, %s\n\n"

// FileWriter writes the assembly file content.
type FileWriter struct {
	app           *program.Program
	options       *options.Disassembler
	mainWriter    io.Writer
	newBankWriter assembler.NewBankWriter
	writer        *writer.Writer
}

type headerByteWrite struct {
	value   byte
	comment string
}

type segmentWrite struct {
	address string
}

type prgBankWrite struct {
	bank *program.PRGBank
}

type customWrite func() error

type lineWrite string

// New creates a new file writer.
// nolint: ireturn
func New(app *program.Program, options *options.Disassembler, mainWriter io.Writer, newBankWriter assembler.NewBankWriter) writer.AssemblerWriter {
	opts := writer.Options{}
	return FileWriter{
		app:           app,
		options:       options,
		mainWriter:    mainWriter,
		newBankWriter: newBankWriter,
		writer:        writer.New(app, mainWriter, opts),
	}
}

// Write writes the assembly file content including header, footer, code and data.
// nolint:funlen, cyclop
func (f FileWriter) Write() error {
	control1, control2 := cartridge.ControlBytes(f.app.Battery, byte(f.app.Mirror), f.app.Mapper, len(f.app.Trainer) > 0)

	var writes []any // nolint:prealloc

	if !f.options.CodeOnly {
		writes = []any{
			customWrite(f.writer.WriteCommentHeader),
			lineWrite(iNESHeader),
			headerByteWrite{value: byte(f.app.PrgSize() / 16384), comment: "Number of 16KB PRG-ROM banks"},
			headerByteWrite{value: byte(len(f.app.CHR) / 8192), comment: "Number of 8KB CHR-ROM banks"},
			headerByteWrite{value: control1, comment: "Control bits 1"},
			headerByteWrite{value: control2, comment: "Control bits 2"},
			headerByteWrite{value: f.app.RAM, comment: "Number of 8KB PRG-RAM banks"},
			headerByteWrite{value: f.app.VideoFormat, comment: "Video format NTSC/PAL"},
			lineWrite(".dsb 6"),
			segmentWrite{address: fmt.Sprintf("$%04x", f.app.CodeBaseAddress)},
		}
	}

	for _, bank := range f.app.PRG {
		writes = append(writes,
			prgBankWrite{bank: bank},
		)
	}

	writes = append(writes,
		customWrite(f.writeVectors),
		customWrite(f.writeCHR),
	)

	for _, write := range writes {
		switch t := write.(type) {
		case headerByteWrite:
			if _, err := fmt.Fprintf(f.mainWriter, headerByte, t.value, "", t.comment); err != nil {
				return fmt.Errorf("writing header: %w", err)
			}

		case segmentWrite:
			if err := f.writeSegment(t.address); err != nil {
				return err
			}

		case lineWrite:
			if _, err := fmt.Fprintln(f.mainWriter, t); err != nil {
				return fmt.Errorf("writing line: %w", err)
			}

		case customWrite:
			if err := t(); err != nil {
				return err
			}

		case prgBankWrite:
			if err := f.writeConstants(t.bank); err != nil {
				return err
			}
			if err := f.writeVariables(t.bank); err != nil {
				return err
			}
			if err := f.writeCode(t.bank); err != nil {
				return err
			}
		}
	}

	return nil
}

// writeSegment writes a segment header to the output.
func (f FileWriter) writeSegment(address string) error {
	_, err := fmt.Fprintf(f.mainWriter, "\n.org %s\n\n", address)
	if err != nil {
		return fmt.Errorf("writing segment: %w", err)
	}
	return nil
}

// writeConstants writes constant aliases to the output.
func (f FileWriter) writeConstants(bank *program.PRGBank) error {
	if err := f.writer.OutputAliasMap(bank.Constants); err != nil {
		return fmt.Errorf("writing constants output alias map: %w", err)
	}
	return nil
}

// writeVariables writes variable aliases to the output.
func (f FileWriter) writeVariables(bank *program.PRGBank) error {
	if err := f.writer.OutputAliasMap(bank.Variables); err != nil {
		return fmt.Errorf("writing variables output alias map: %w", err)
	}
	return nil
}

// writeCHR writes the CHR content to the output.
func (f FileWriter) writeCHR() error {
	if f.options.ZeroBytes {
		if _, err := f.writer.BundleDataWrites(f.app.CHR, false); err != nil {
			return fmt.Errorf("writing CHR data: %w", err)
		}
		return nil
	}

	lastNonZeroByte := f.app.CHR.GetLastNonZeroByte()
	_, err := f.writer.BundleDataWrites(f.app.CHR[:lastNonZeroByte], false)
	if err != nil {
		return fmt.Errorf("writing CHR data: %w", err)
	}

	remaining := len(f.app.CHR) - lastNonZeroByte
	if remaining > 0 {
		if _, err := fmt.Fprintf(f.mainWriter, "\n.dsb %d\n", remaining); err != nil {
			return fmt.Errorf("writing CHR remainder: %w", err)
		}
	}

	return nil
}

// writeVectors writes the IRQ vectors.
func (f FileWriter) writeVectors() error {
	if f.options.CodeOnly {
		return nil
	}

	vectorStart := int(f.app.CodeBaseAddress) + f.app.PrgSize() - 6
	addr := fmt.Sprintf("$%04X", vectorStart)

	if err := f.writeSegment(addr); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(f.mainWriter, vectors, f.app.Handlers.NMI, f.app.Handlers.Reset, f.app.Handlers.IRQ); err != nil {
		return fmt.Errorf("writing vectors: %w", err)
	}
	return nil
}

// writeCode writes the code to the output.
func (f FileWriter) writeCode(bank *program.PRGBank) error {
	endIndex, err := bank.GetLastNonZeroByte(f.options)
	if err != nil {
		return fmt.Errorf("getting last non zero PRG byte: %w", err)
	}

	if err := f.writer.ProcessPRG(bank, endIndex); err != nil {
		return fmt.Errorf("writing PRG: %w", err)
	}
	return nil
}
