// Package writer implements common assembly file writing functionality.
package writer

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/retroenv/nesgodisasm/internal/program"
)

const dataBytesPerLine = 16

// AssemblerWriter defines a shared interface used by the different assembler compatibility packages.
// Their constructors need to return this shared interface, having them return the actual type instead of
// the interface results in compiler errors for the constructor variable that they are assigned to.
type AssemblerWriter interface {
	Write() error
}

// Writer implements common assembly file writing functionality.
type Writer struct {
	app     *program.Program
	options Options
	writer  io.Writer
}

// Options of the writer.
type Options struct {
	DirectivePrefix string // nesasm requires a space before a directive
}

// New creates a new writer.
func New(app *program.Program, writer io.Writer, options Options) *Writer {
	return &Writer{
		app:     app,
		options: options,
		writer:  writer,
	}
}

// ProcessPRG processes the PRG segment and writes all code offsets, labels and their comments.
func (w Writer) ProcessPRG(endIndex int) error {
	var previousLineWasCode bool

	for i := 0; i < endIndex; i++ {
		offset := w.app.PRG[i]

		if err := w.writeLabel(i, offset); err != nil {
			return err
		}

		// print an empty line in case of data after code and vice versa
		if i > 0 && offset.Label == "" && offset.IsType(program.CodeOffset|program.CodeAsData) != previousLineWasCode {
			if _, err := fmt.Fprintln(w.writer); err != nil {
				return fmt.Errorf("writing line: %w", err)
			}
		}
		previousLineWasCode = offset.IsType(program.CodeOffset | program.CodeAsData)

		adjustment, err := w.writeOffset(i, endIndex, offset)
		if err != nil {
			return err
		}
		i += adjustment
	}
	return nil
}

// BundleDataWrites bundles writes of data bytes to print dataBytesPerLine bytes per line.
func (w Writer) BundleDataWrites(data []byte, returnLine bool) (string, error) {
	remaining := len(data)
	for i := 0; remaining > 0; {
		toWrite := remaining
		if toWrite > dataBytesPerLine {
			toWrite = dataBytesPerLine
		}

		buf := &strings.Builder{}
		if _, err := fmt.Fprintf(buf, "%s.byte ", w.options.DirectivePrefix); err != nil {
			return "", fmt.Errorf("writing data prefix: %w", err)
		}

		for j := 0; j < toWrite; j++ {
			if _, err := fmt.Fprintf(buf, "$%02x, ", data[i+j]); err != nil {
				return "", fmt.Errorf("writing data byte: %w", err)
			}
		}

		line := strings.TrimRight(buf.String(), ", ")
		if returnLine {
			return line, nil
		}

		if _, err := fmt.Fprintf(w.writer, "%s\n", line); err != nil {
			return "", fmt.Errorf("writing data line: %w", err)
		}

		i += toWrite
		remaining -= toWrite
	}

	return "", nil
}

// OutputAliasMap outputs an alias map, for constants or variables.
func (w Writer) OutputAliasMap(aliases map[string]uint16) error {
	if len(aliases) == 0 {
		return nil
	}

	if _, err := fmt.Fprintln(w.writer); err != nil {
		return fmt.Errorf("writing line: %w", err)
	}

	// sort the aliases by name before outputting to avoid random map order
	names := make([]string, 0, len(aliases))
	for constant := range aliases {
		names = append(names, constant)
	}
	sort.Strings(names)

	for _, constant := range names {
		address := aliases[constant]
		if _, err := fmt.Fprintf(w.writer, "%s = $%04X\n", constant, address); err != nil {
			return fmt.Errorf("writing alias: %w", err)
		}
	}

	if _, err := fmt.Fprintln(w.writer); err != nil {
		return fmt.Errorf("writing line: %w", err)
	}
	return nil
}

// WriteCommentHeader writes the CRC32 checksums and code base address as comments to the output.
func (w Writer) WriteCommentHeader() error {
	if _, err := fmt.Fprintf(w.writer, "; PRG CRC32 checksum: %08x\n", w.app.Checksums.PRG); err != nil {
		return fmt.Errorf("writing prg checksum: %w", err)
	}
	if _, err := fmt.Fprintf(w.writer, "; CHR CRC32 checksum: %08x\n", w.app.Checksums.CHR); err != nil {
		return fmt.Errorf("writing chr checksum: %w", err)
	}
	if _, err := fmt.Fprintf(w.writer, "; Overall CRC32 checksum: %08x\n", w.app.Checksums.Overall); err != nil {
		return fmt.Errorf("writing overall checksum: %w", err)
	}
	if _, err := fmt.Fprintf(w.writer, "; Code base address: $%04x\n\n", w.app.CodeBaseAddress); err != nil {
		return fmt.Errorf("writing code base address: %w", err)
	}
	return nil
}

func (w Writer) writeOffset(index, endIndex int, offset program.Offset) (int, error) {
	if offset.IsType(program.CodeOffset) && len(offset.OpcodeBytes) == 0 {
		return 0, nil
	}
	if offset.IsType(program.FunctionReference) {
		if err := w.writeCodeLine(offset); err != nil {
			return 0, fmt.Errorf("writing function reference: %w", err)
		}
		return 1, nil
	}

	if offset.IsType(program.DataOffset) {
		count, err := w.bundlePRGDataWrites(index, endIndex)
		if err != nil {
			return 0, err
		}
		if count > 0 {
			return count - 1, nil
		}
		return 0, err
	}

	if err := w.writeCodeLine(offset); err != nil {
		return 0, fmt.Errorf("writing code line: %w", err)
	}
	return len(offset.OpcodeBytes) - 1, nil
}

func (w Writer) writeLabel(index int, offset program.Offset) error {
	if offset.Label == "" {
		return nil
	}

	if index > 0 {
		if _, err := fmt.Fprintln(w.writer); err != nil {
			return fmt.Errorf("writing line: %w", err)
		}
	}

	if offset.LabelComment == "" {
		if _, err := fmt.Fprintf(w.writer, "%s:\n", offset.Label); err != nil {
			return fmt.Errorf("writing label: %w", err)
		}
	} else {
		if _, err := fmt.Fprintf(w.writer, "%-32s ; %s\n", offset.Label+":", offset.LabelComment); err != nil {
			return fmt.Errorf("writing label: %w", err)
		}
	}
	return nil
}

func (w Writer) writeCodeLine(offset program.Offset) error {
	if offset.Comment == "" {
		if _, err := fmt.Fprintf(w.writer, "  %s\n", offset.Code); err != nil {
			return fmt.Errorf("writing line: %w", err)
		}
	} else {
		if _, err := fmt.Fprintf(w.writer, "  %-30s ; %s\n", offset.Code, offset.Comment); err != nil {
			return fmt.Errorf("writing line: %w", err)
		}
	}
	return nil
}

// bundlePRGDataWrites parses PRG to create bundled writes of data bytes per line.
func (w Writer) bundlePRGDataWrites(startIndex, endIndex int) (int, error) {
	var data []byte

	for i := startIndex; i < endIndex; i++ {
		offset := w.app.PRG[i]
		// opcode bytes can be nil if data bytes have been combined for an unofficial nop
		if !offset.IsType(program.DataOffset) || len(offset.OpcodeBytes) == 0 {
			break
		}
		// stop at first label or code after start index
		if i > startIndex && (offset.IsType(program.CodeOffset|program.CodeAsData) || offset.Label != "") {
			break
		}
		data = append(data, offset.OpcodeBytes...)
	}

	var err error
	var line string
	offset := w.app.PRG[startIndex]

	switch len(data) {
	case 0:
		return 0, nil

	case 1:
		line = fmt.Sprintf("%s.byte $%02x", w.options.DirectivePrefix, data[0])

	default:
		line, err = w.BundleDataWrites(data, offset.Comment != "")
		if err != nil {
			return 0, err
		}
	}

	if line != "" {
		if offset.Comment == "" {
			_, err = fmt.Fprintf(w.writer, "%s\n", line)
		} else {
			_, err = fmt.Fprintf(w.writer, "%-32s ; %s\n", line, offset.Comment)
		}
		if err != nil {
			return 0, fmt.Errorf("writing prg line: %w", err)
		}
	}
	return len(data), nil
}
