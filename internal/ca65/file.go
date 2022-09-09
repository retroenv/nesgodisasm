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
			lineWrite(cpuSelector),
			segmentWrite{name: "HEADER"},
			lineWrite(iNESHeader),
			headerByteWrite{value: byte(len(app.PRG) / 16384), comment: "Number of 16KB PRG-ROM banks"},
			headerByteWrite{value: byte(len(app.CHR) / 8192), comment: "Number of 8KB CHR-ROM banks"},
			headerByteWrite{value: control1, comment: "Control bits 1"},
			headerByteWrite{value: control2, comment: "Control bits 1"},
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
				return err
			}

		case segmentWrite:
			if err := f.writeSegment(writer, t.name); err != nil {
				return err
			}

		case lineWrite:
			if _, err := fmt.Fprintln(writer, t); err != nil {
				return err
			}

		case customWrite:
			if err := t(options, app, writer); err != nil {
				return err
			}
		}
	}

	if !options.CodeOnly {
		if _, err := fmt.Fprintf(writer, vectors, app.Handlers.NMI, app.Handlers.Reset, app.Handlers.IRQ); err != nil {
			return err
		}
	}
	return nil
}

// writeSegment writes a segment header to the output.
func (f FileWriter) writeSegment(writer io.Writer, name string) error {
	if name != "HEADER" {
		if _, err := fmt.Fprintln(writer); err != nil {
			return err
		}
	}

	_, err := fmt.Fprintf(writer, ".segment \"%s\"\n\n", name)
	return err
}

// writeConstants will write constant aliases to the output.
func (f FileWriter) writeConstants(_ *disasmoptions.Options, app *program.Program, writer io.Writer) error {
	if len(app.Constants) == 0 {
		return nil
	}

	return outputAliasMap(app.Constants, writer)
}

// writeVariables will write variable aliases to the output.
func (f FileWriter) writeVariables(_ *disasmoptions.Options, app *program.Program, writer io.Writer) error {
	if len(app.Variables) == 0 {
		return nil
	}

	return outputAliasMap(app.Variables, writer)
}

func outputAliasMap(aliases map[string]uint16, writer io.Writer) error {
	if _, err := fmt.Fprintln(writer); err != nil {
		return err
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
			return err
		}
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
		res := app.PRG[i]

		if err := writeLabel(writer, i, res.Label); err != nil {
			return err
		}

		// print an empty line in case of data after code and vice versa
		if i > 0 && res.Label == "" && res.IsType(program.CodeOffset|program.CodeAsData) != previousLineWasCode {
			if _, err := fmt.Fprintln(writer); err != nil {
				return err
			}
		}
		previousLineWasCode = res.IsType(program.CodeOffset | program.CodeAsData)

		if res.IsType(program.CodeOffset) && len(res.OpcodeBytes) == 0 {
			continue
		}

		if res.IsType(program.DataOffset) {
			count, err := bundlePRGDataWrites(app, writer, i, endIndex)
			if err != nil {
				return err
			}
			if count > 0 {
				i = i + count - 1
			}
			continue
		}

		if err := writeCodeLine(writer, res); err != nil {
			return err
		}
		i += len(res.OpcodeBytes) - 1
	}
	return nil
}

func writeLabel(writer io.Writer, offset int, label string) error {
	if label == "" {
		return nil
	}

	if offset > 0 {
		if _, err := fmt.Fprintln(writer); err != nil {
			return err
		}
	}

	if _, err := fmt.Fprintf(writer, "%s:\n", label); err != nil {
		return err
	}
	return nil
}

func writeCodeLine(writer io.Writer, res program.Offset) error {
	if res.Comment == "" {
		if _, err := fmt.Fprintf(writer, "  %s\n", res.Code); err != nil {
			return err
		}
	} else {
		if _, err := fmt.Fprintf(writer, "  %-30s ; %s\n", res.Code, res.Comment); err != nil {
			return err
		}
	}
	return nil
}

// bundlePRGDataWrites parses PRG to create bundled writes of data bytes per line.
func bundlePRGDataWrites(app *program.Program, writer io.Writer, startIndex, endIndex int) (int, error) {
	var data []byte

	for i := startIndex; i < endIndex; i++ {
		res := app.PRG[i]
		// opcode bytes can be nil if data bytes have been combined for an unofficial nop
		if !res.IsType(program.DataOffset) || len(res.OpcodeBytes) == 0 {
			break
		}
		// stop at first label or code after start index
		if i > startIndex && (res.IsType(program.CodeOffset|program.CodeAsData) || res.Label != "") {
			break
		}
		data = append(data, res.OpcodeBytes...)
	}

	var err error
	var line string
	res := app.PRG[startIndex]

	switch len(data) {
	case 0:
		return 0, nil

	case 1:
		line = fmt.Sprintf(".byte $%02x", data[0])

	default:
		line, err = bundleDataWrites(writer, data, res.Comment != "")
		if err != nil {
			return 0, err
		}
	}

	if line != "" {
		if res.Comment == "" {
			_, err = fmt.Fprintf(writer, "%s\n", line)
		} else {
			_, err = fmt.Fprintf(writer, "%-32s ; %s\n", line, res.Comment)
		}
		if err != nil {
			return 0, err
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
			return "", err
		}

		for j := 0; j < toWrite; j++ {
			if _, err := fmt.Fprintf(buf, "$%02x, ", data[i+j]); err != nil {
				return "", err
			}
		}

		line := strings.TrimRight(buf.String(), ", ")
		if returnLine {
			return line, nil
		}

		if _, err := fmt.Fprintf(writer, "%s\n", line); err != nil {
			return "", err
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
		res := app.PRG[i]
		if len(res.OpcodeBytes) == 0 || res.OpcodeBytes[0] == 0 {
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
