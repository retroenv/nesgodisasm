package ca65

import (
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/retroenv/nesgodisasm/internal/disasmoptions"
	"github.com/retroenv/nesgodisasm/internal/program"
	"github.com/retroenv/retrogolib/nes/cartridge"
)

var cpuSelector = `.setcpu "6502x"` // allow unofficial opcodes

var iNESHeader = `.byte "NES", $1a                 ; Magic string that always begins an iNES header`

var headerByte = ".byte $%02x %-22s ; %s\n"

var vectors = ".addr %s, %s, %s\n"

// FileWriter writes the assembly file content.
type FileWriter struct {
}

type headerByteWrite struct {
	value   byte
	comment string
}

type segmentWrite struct {
	name string
}

type customWrite func(options *disasmoptions.Options, app *program.Program, writer io.Writer) error

type lineWrite string

const dataBytesPerLine = 16

// Write writes the assembly file content including header, footer, code and data.
func (f FileWriter) Write(options *disasmoptions.Options, app *program.Program, writer io.Writer) error {
	control1, control2 := cartridge.ControlBytes(app.Battery, byte(app.Mirror), app.Mapper, len(app.Trainer) > 0)

	var writes []any

	if !options.CodeOnly {
		writes = []any{
			customWrite(f.writeChecksums),
			customWrite(f.writeCodeBaseAddress),
			lineWrite(cpuSelector),
			segmentWrite{name: "HEADER"},
			lineWrite(iNESHeader),
			headerByteWrite{value: byte(len(app.PRG) / 16384), comment: "Number of 16KB PRG-ROM banks"},
			headerByteWrite{value: byte(len(app.CHR) / 8192), comment: "Number of 8KB CHR-ROM banks"},
			headerByteWrite{value: control1, comment: "Control bits 1"},
			headerByteWrite{value: control2, comment: "Control bits 2"},
			headerByteWrite{value: app.RAM, comment: "Number of 8KB PRG-RAM banks"},
			headerByteWrite{value: app.VideoFormat, comment: "Video format NTSC/PAL"},
		}
	}

	writes = append(writes,
		customWrite(f.writeConstants),
		customWrite(f.writeVariables),
		customWrite(f.writeCode),
	)

	if !options.CodeOnly {
		writes = append(writes, customWrite(f.writeCHR), segmentWrite{name: "VECTORS"})
	}

	for _, write := range writes {
		switch t := write.(type) {
		case headerByteWrite:
			if _, err := fmt.Fprintf(writer, headerByte, t.value, "", t.comment); err != nil {
				return fmt.Errorf("writing header: %w", err)
			}

		case segmentWrite:
			if err := f.writeSegment(writer, t.name); err != nil {
				return err
			}

		case lineWrite:
			if _, err := fmt.Fprintln(writer, t); err != nil {
				return fmt.Errorf("writing line: %w", err)
			}

		case customWrite:
			if err := t(options, app, writer); err != nil {
				return err
			}
		}
	}

	if !options.CodeOnly {
		if _, err := fmt.Fprintf(writer, vectors, app.Handlers.NMI, app.Handlers.Reset, app.Handlers.IRQ); err != nil {
			return fmt.Errorf("writing vectors: %w", err)
		}
	}
	return nil
}

// writeSegment writes a segment header to the output.
func (f FileWriter) writeSegment(writer io.Writer, name string) error {
	if name != "HEADER" {
		if _, err := fmt.Fprintln(writer); err != nil {
			return fmt.Errorf("writing segment: %w", err)
		}
	}

	_, err := fmt.Fprintf(writer, ".segment \"%s\"\n\n", name)
	if err != nil {
		return fmt.Errorf("writing segment footer: %w", err)
	}
	return nil
}

// writeConstants writes constant aliases to the output.
func (f FileWriter) writeConstants(_ *disasmoptions.Options, app *program.Program, writer io.Writer) error {
	if len(app.Constants) == 0 {
		return nil
	}

	return outputAliasMap(app.Constants, writer)
}

// writeVariables writes variable aliases to the output.
func (f FileWriter) writeVariables(_ *disasmoptions.Options, app *program.Program, writer io.Writer) error {
	if len(app.Variables) == 0 {
		return nil
	}

	return outputAliasMap(app.Variables, writer)
}

// writeChecksums writes the CRC32 checksums as comments to the output.
func (f FileWriter) writeChecksums(_ *disasmoptions.Options, app *program.Program, writer io.Writer) error {
	if _, err := fmt.Fprintf(writer, "; PRG CRC32 checksum: %08x\n", app.Checksums.PRG); err != nil {
		return fmt.Errorf("writing prg checksum: %w", err)
	}
	if _, err := fmt.Fprintf(writer, "; CHR CRC32 checksum: %08x\n", app.Checksums.CHR); err != nil {
		return fmt.Errorf("writing chr checksum: %w", err)
	}
	if _, err := fmt.Fprintf(writer, "; Overall CRC32 checksum: %08x\n", app.Checksums.Overall); err != nil {
		return fmt.Errorf("writing overall checksum: %w", err)
	}
	return nil
}

func (f FileWriter) writeCodeBaseAddress(_ *disasmoptions.Options, app *program.Program, writer io.Writer) error {
	if _, err := fmt.Fprintf(writer, "; Code base address: $%04x\n\n", app.CodeBaseAddress); err != nil {
		return fmt.Errorf("writing code base address: %w", err)
	}
	return nil
}

func outputAliasMap(aliases map[string]uint16, writer io.Writer) error {
	if _, err := fmt.Fprintln(writer); err != nil {
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
		if _, err := fmt.Fprintf(writer, "%s = $%04X\n", constant, address); err != nil {
			return fmt.Errorf("writing alias: %w", err)
		}
	}

	if _, err := fmt.Fprintln(writer); err != nil {
		return fmt.Errorf("writing line: %w", err)
	}
	return nil
}

// writeCHR writes the CHR content to the output.
func (f FileWriter) writeCHR(options *disasmoptions.Options, app *program.Program, writer io.Writer) error {
	if err := f.writeSegment(writer, "TILES"); err != nil {
		return err
	}

	if options.ZeroBytes {
		_, err := bundleDataWrites(writer, app.CHR, false)
		return err
	}

	lastNonZeroByte := getLastNonZeroCHRByte(app)
	_, err := bundleDataWrites(writer, app.CHR[:lastNonZeroByte], false)
	return err
}

// writeCode writes the code to the output.
func (f FileWriter) writeCode(options *disasmoptions.Options, app *program.Program, writer io.Writer) error {
	if !options.CodeOnly {
		if err := f.writeSegment(writer, "CODE"); err != nil {
			return err
		}
	}

	endIndex, err := getLastNonZeroPRGByte(options, app)
	if err != nil {
		return err
	}

	return processPRG(app, writer, endIndex)
}

func processPRG(app *program.Program, writer io.Writer, endIndex int) error {
	var previousLineWasCode bool

	for i := 0; i < endIndex; i++ {
		offset := app.PRG[i]

		if err := writeLabel(writer, i, offset); err != nil {
			return err
		}

		// print an empty line in case of data after code and vice versa
		if i > 0 && offset.Label == "" && offset.IsType(program.CodeOffset|program.CodeAsData) != previousLineWasCode {
			if _, err := fmt.Fprintln(writer); err != nil {
				return fmt.Errorf("writing line: %w", err)
			}
		}
		previousLineWasCode = offset.IsType(program.CodeOffset | program.CodeAsData)

		adjustment, err := writeOffset(app, writer, i, endIndex, offset)
		if err != nil {
			return err
		}
		i += adjustment
	}
	return nil
}

func writeOffset(app *program.Program, writer io.Writer, index, endIndex int, offset program.Offset) (int, error) {
	if offset.IsType(program.CodeOffset) && len(offset.OpcodeBytes) == 0 {
		return 0, nil
	}
	if offset.IsType(program.FunctionReference) {
		if err := writeCodeLine(writer, offset); err != nil {
			return 0, fmt.Errorf("writing function reference: %w", err)
		}
		return 1, nil
	}

	if offset.IsType(program.DataOffset) {
		count, err := bundlePRGDataWrites(app, writer, index, endIndex)
		if err != nil {
			return 0, err
		}
		if count > 0 {
			return count - 1, nil
		}
		return 0, err
	}

	if err := writeCodeLine(writer, offset); err != nil {
		return 0, fmt.Errorf("writing code line: %w", err)
	}
	return len(offset.OpcodeBytes) - 1, nil
}

func writeLabel(writer io.Writer, index int, offset program.Offset) error {
	if offset.Label == "" {
		return nil
	}

	if index > 0 {
		if _, err := fmt.Fprintln(writer); err != nil {
			return fmt.Errorf("writing line: %w", err)
		}
	}

	if offset.LabelComment == "" {
		if _, err := fmt.Fprintf(writer, "%s:\n", offset.Label); err != nil {
			return fmt.Errorf("writing label: %w", err)
		}
	} else {
		if _, err := fmt.Fprintf(writer, "%-32s ; %s\n", offset.Label+":", offset.LabelComment); err != nil {
			return fmt.Errorf("writing label: %w", err)
		}
	}
	return nil
}

func writeCodeLine(writer io.Writer, offset program.Offset) error {
	if offset.Comment == "" {
		if _, err := fmt.Fprintf(writer, "  %s\n", offset.Code); err != nil {
			return fmt.Errorf("writing line: %w", err)
		}
	} else {
		if _, err := fmt.Fprintf(writer, "  %-30s ; %s\n", offset.Code, offset.Comment); err != nil {
			return fmt.Errorf("writing line: %w", err)
		}
	}
	return nil
}

// bundlePRGDataWrites parses PRG to create bundled writes of data bytes per line.
func bundlePRGDataWrites(app *program.Program, writer io.Writer, startIndex, endIndex int) (int, error) {
	var data []byte

	for i := startIndex; i < endIndex; i++ {
		offset := app.PRG[i]
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
	offset := app.PRG[startIndex]

	switch len(data) {
	case 0:
		return 0, nil

	case 1:
		line = fmt.Sprintf(".byte $%02x", data[0])

	default:
		line, err = bundleDataWrites(writer, data, offset.Comment != "")
		if err != nil {
			return 0, err
		}
	}

	if line != "" {
		if offset.Comment == "" {
			_, err = fmt.Fprintf(writer, "%s\n", line)
		} else {
			_, err = fmt.Fprintf(writer, "%-32s ; %s\n", line, offset.Comment)
		}
		if err != nil {
			return 0, fmt.Errorf("writing prg line: %w", err)
		}
	}
	return len(data), nil
}

// bundleDataWrites bundles writes of data bytes to print dataBytesPerLine bytes per line.
func bundleDataWrites(writer io.Writer, data []byte, returnLine bool) (string, error) {
	remaining := len(data)
	for i := 0; remaining > 0; {
		toWrite := remaining
		if toWrite > dataBytesPerLine {
			toWrite = dataBytesPerLine
		}

		buf := &strings.Builder{}
		if _, err := buf.WriteString(".byte "); err != nil {
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

		if _, err := fmt.Fprintf(writer, "%s\n", line); err != nil {
			return "", fmt.Errorf("writing data line: %w", err)
		}

		i += toWrite
		remaining -= toWrite
	}

	return "", nil
}

// getLastNonZeroPRGByte searches for the last byte in PRG that is not zero.
func getLastNonZeroPRGByte(options *disasmoptions.Options, app *program.Program) (int, error) {
	endIndex := len(app.PRG) - 6 // leave space for vectors
	if options.ZeroBytes {
		return endIndex, nil
	}

	start := len(app.PRG) - 1 - 6 // skip irq pointers

	for i := start; i >= 0; i-- {
		offset := app.PRG[i]
		if (len(offset.OpcodeBytes) == 0 || offset.OpcodeBytes[0] == 0) && offset.Label == "" {
			continue
		}
		return i + 1, nil
	}
	return 0, errors.New("could not find last zero byte")
}

// getLastNonZeroCHRByte searches for the last byte in CHR that is not zero.
func getLastNonZeroCHRByte(app *program.Program) int {
	for i := len(app.CHR) - 1; i >= 0; i-- {
		b := app.CHR[i]
		if b == 0 {
			continue
		}
		return i + 1
	}
	return 0
}
