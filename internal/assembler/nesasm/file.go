package nesasm

import (
	"fmt"
	"io"

	"github.com/retroenv/nesgodisasm/internal/options"
	"github.com/retroenv/nesgodisasm/internal/program"
	"github.com/retroenv/nesgodisasm/internal/writer"
)

var headerByte = " .%s %d %-22s ; %s\n"

var vectors = " .dw %s, %s, %s\n\n"

// FileWriter writes the assembly file content.
type FileWriter struct {
	app      *program.Program
	options  *options.Disassembler
	ioWriter io.Writer
	writer   *writer.Writer
}

type segmentWrite struct {
	bank    int
	address string
}

type customWrite func() error

type lineWrite string

// New creates a new file writer.
// nolint: ireturn
func New(app *program.Program, options *options.Disassembler, ioWriter io.Writer) writer.AssemblerWriter {
	opts := writer.Options{
		DirectivePrefix: " ",
	}
	return FileWriter{
		app:      app,
		options:  options,
		ioWriter: ioWriter,
		writer:   writer.New(app, ioWriter, opts),
	}
}

// Write writes the assembly file content including header, footer, code and data.
func (f FileWriter) Write() error {
	var writes []any

	if !f.options.CodeOnly {
		writes = []any{
			customWrite(f.writer.WriteCommentHeader),
			customWrite(f.writeROMHeader),
			segmentWrite{address: fmt.Sprintf("$%04x", f.app.CodeBaseAddress)},
		}
	}

	writes = append(writes,
		customWrite(f.writeConstants),
		customWrite(f.writeVariables),
		customWrite(f.writeCode),
		customWrite(f.writeVectors),
		customWrite(f.writeCHR),
	)

	for _, write := range writes {
		switch t := write.(type) {
		case segmentWrite:
			if err := f.writeSegment(t.address, t.bank); err != nil {
				return err
			}

		case lineWrite:
			if _, err := fmt.Fprintln(f.ioWriter, t); err != nil {
				return fmt.Errorf("writing line: %w", err)
			}

		case customWrite:
			if err := t(); err != nil {
				return err
			}
		}
	}

	return nil
}

// writeROMHeader writes the ROM header configuration to the output.
func (f FileWriter) writeROMHeader() error {
	if _, err := fmt.Fprintf(f.ioWriter, headerByte, "inesprg", len(f.app.PRG)/16384, " ", "Number of 16KB PRG-ROM banks"); err != nil {
		return fmt.Errorf("writing header: %w", err)
	}
	if _, err := fmt.Fprintf(f.ioWriter, headerByte, "ineschr", len(f.app.CHR)/8192, " ", "Number of 8KB CHR-ROM banks"); err != nil {
		return fmt.Errorf("writing header: %w", err)
	}
	if _, err := fmt.Fprintf(f.ioWriter, headerByte, "inesmap", f.app.Mapper, " ", "Mapper"); err != nil {
		return fmt.Errorf("writing header: %w", err)
	}
	if _, err := fmt.Fprintf(f.ioWriter, headerByte, "inesmir", f.app.Mirror, " ", "Mirror mode"); err != nil {
		return fmt.Errorf("writing header: %w", err)
	}

	return nil
}

// writeSegment writes a segment header to the output.
func (f FileWriter) writeSegment(address string, bank int) error {
	if bank >= 0 {
		if _, err := fmt.Fprintf(f.ioWriter, "\n .bank %d\n", bank); err != nil {
			return fmt.Errorf("writing segment: %w", err)
		}
	}
	if _, err := fmt.Fprintf(f.ioWriter, " .org %s\n\n", address); err != nil {
		return fmt.Errorf("writing segment: %w", err)
	}
	return nil
}

// writeConstants writes constant aliases to the output.
func (f FileWriter) writeConstants() error {
	if err := f.writer.OutputAliasMap(f.app.Constants); err != nil {
		return fmt.Errorf("writing constants output alias map: %w", err)
	}
	return nil
}

// writeVariables writes variable aliases to the output.
func (f FileWriter) writeVariables() error {
	if err := f.writer.OutputAliasMap(f.app.Variables); err != nil {
		return fmt.Errorf("writing variables output alias map: %w", err)
	}
	return nil
}

// writeCHR writes the CHR content to the output.
func (f FileWriter) writeCHR() error {
	if _, err := fmt.Fprint(f.ioWriter, "\n .DATA"); err != nil {
		return fmt.Errorf("writing CHR bank: %w", err)
	}
	if _, err := fmt.Fprintf(f.ioWriter, "\n .bank %d\n", 4); err != nil {
		return fmt.Errorf("writing CHR bank: %w", err)
	}

	if f.options.ZeroBytes {
		if _, err := f.writer.BundleDataWrites(f.app.CHR, false); err != nil {
			return fmt.Errorf("writing CHR data: %w", err)
		}
		return nil
	}

	lastNonZeroByte := f.app.GetLastNonZeroCHRByte()
	_, err := f.writer.BundleDataWrites(f.app.CHR[:lastNonZeroByte], false)
	if err != nil {
		return fmt.Errorf("writing CHR data: %w", err)
	}

	remaining := len(f.app.CHR) - lastNonZeroByte
	if remaining > 0 {
		if _, err := fmt.Fprintf(f.ioWriter, "\n .ds %d\n", remaining); err != nil {
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

	vectorStart := int(f.app.CodeBaseAddress) + len(f.app.PRG) - 6
	addr := fmt.Sprintf("$%04X", vectorStart)

	if err := f.writeSegment(addr, 3); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(f.ioWriter, vectors, f.app.Handlers.NMI, f.app.Handlers.Reset, f.app.Handlers.IRQ); err != nil {
		return fmt.Errorf("writing vectors: %w", err)
	}
	return nil
}

// writeCode writes the code to the output.
func (f FileWriter) writeCode() error {
	endIndex, err := f.app.GetLastNonZeroPRGByte(f.options)
	if err != nil {
		return fmt.Errorf("getting last non zero PRG byte: %w", err)
	}

	if err := f.writer.ProcessPRG(endIndex); err != nil {
		return fmt.Errorf("writing PRG: %w", err)
	}
	return nil
}
