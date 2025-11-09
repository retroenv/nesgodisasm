package fileprocessor

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/retroenv/retrodisasm/internal/options"
	"github.com/retroenv/retrogolib/arch"
	"github.com/retroenv/retrogolib/assert"
	"github.com/retroenv/retrogolib/log"
)

// TestBinaryOptionSetsCodeOnly verifies that when Binary option is true,
// CodeOnly is automatically set to true to prevent NES-specific segments
// from being included in the output (fixes issue #82).
func TestBinaryOptionSetsCodeOnly(t *testing.T) {
	tmpDir := t.TempDir()
	testCode := createTestCode()
	testBinFile := createTestBinaryFile(t, tmpDir, testCode)

	tests := []struct {
		name                  string
		binary                bool
		expectHeaderSegment   bool
		expectTilesSegment    bool
		expectVectorsSegment  bool
		expectCodeSegmentName bool
	}{
		{
			name:                  "binary mode excludes NES segments",
			binary:                true,
			expectHeaderSegment:   false,
			expectTilesSegment:    false,
			expectVectorsSegment:  false,
			expectCodeSegmentName: false, // CodeOnly mode doesn't write segment name
		},
		{
			name:                  "non-binary mode includes NES segments",
			binary:                false,
			expectHeaderSegment:   true,
			expectTilesSegment:    true,
			expectVectorsSegment:  true,
			expectCodeSegmentName: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runSegmentTest(t, tmpDir, testBinFile, testCode, tt.binary,
				tt.expectHeaderSegment, tt.expectTilesSegment,
				tt.expectVectorsSegment, tt.expectCodeSegmentName)
		})
	}
}

func createTestCode() []byte {
	// Simple 6502 program: LDA #$00, STA $0200, RTS
	return []byte{
		0xa9, 0x00, // LDA #$00
		0x8d, 0x00, 0x02, // STA $0200
		0x60, // RTS
	}
}

func createTestBinaryFile(t *testing.T, tmpDir string, code []byte) string {
	t.Helper()
	testBinFile := filepath.Join(tmpDir, "test.bin")
	err := os.WriteFile(testBinFile, code, 0644)
	assert.NoError(t, err)
	return testBinFile
}

func runSegmentTest(t *testing.T, tmpDir, testBinFile string, testCode []byte, binary,
	expectHeader, expectTiles, expectVectors, expectCodeSegment bool) {

	t.Helper()

	logger := log.NewTestLogger(t)

	// Use a simple filename based on binary mode to avoid nested path issues
	filename := "binary.asm"
	if !binary {
		filename = "nes.asm"
	}
	outputFile := filepath.Join(tmpDir, filename)

	programOpts := options.Program{
		Input:     testBinFile,
		Output:    outputFile,
		Assembler: "ca65",
		Binary:    binary,
		System:    arch.NES.String(),
		Quiet:     true,
	}

	// For non-binary mode, create a proper NES file
	if !binary {
		nesFile := filepath.Join(tmpDir, "test.nes")
		nesData := createMinimalNESROM(testCode)
		err := os.WriteFile(nesFile, nesData, 0644)
		assert.NoError(t, err)
		programOpts.Input = nesFile
	}

	disasmOpts := options.NewDisassembler(programOpts.Assembler, programOpts.System)

	ctx := context.Background()
	err := ProcessFile(ctx, logger, programOpts, disasmOpts)
	assert.NoError(t, err)

	verifyOutput(t, outputFile, expectHeader, expectTiles, expectVectors, expectCodeSegment)
}

func verifyOutput(t *testing.T, outputFile string, expectHeader, expectTiles, expectVectors, expectCodeSegment bool) {
	t.Helper()

	outputBytes, err := os.ReadFile(outputFile)
	assert.NoError(t, err)

	outputStr := string(outputBytes)
	assert.True(t, len(outputStr) > 0, "output should not be empty")

	// Verify segment presence/absence
	hasHeader := strings.Contains(outputStr, ".segment \"HEADER\"")
	hasTiles := strings.Contains(outputStr, ".segment \"TILES\"")
	hasVectors := strings.Contains(outputStr, ".segment \"VECTORS\"")
	hasCodeSegment := strings.Contains(outputStr, ".segment \"CODE\"")

	assert.Equal(t, expectHeader, hasHeader, "HEADER segment presence mismatch")
	assert.Equal(t, expectTiles, hasTiles, "TILES segment presence mismatch")
	assert.Equal(t, expectVectors, hasVectors, "VECTORS segment presence mismatch")
	assert.Equal(t, expectCodeSegment, hasCodeSegment, "CODE segment name presence mismatch")

	// Verify we have actual code output
	hasCode := strings.Contains(outputStr, "lda") || strings.Contains(outputStr, "sta") || strings.Contains(outputStr, "rts")
	assert.True(t, hasCode, "output should contain disassembled code")
}

// createMinimalNESROM creates a minimal valid NES ROM with the given code
func createMinimalNESROM(code []byte) []byte {
	rom := make([]byte, 0, 16+16384+8192) // Header + 16KB PRG + 8KB CHR

	// iNES header (16 bytes)
	header := []byte{
		0x4E, 0x45, 0x53, 0x1A, // "NES" + MS-DOS EOF
		0x01,       // 1x 16KB PRG-ROM
		0x01,       // 1x 8KB CHR-ROM
		0x00,       // Mapper 0, horizontal mirroring
		0x00,       // Mapper 0
		0x00,       // No PRG-RAM
		0x00,       // NTSC
		0x00, 0x00, // Unused
		0x00, 0x00, 0x00, 0x00, // Padding
	}
	rom = append(rom, header...)

	// PRG-ROM (16384 bytes)
	prgROM := make([]byte, 16384)
	copy(prgROM, code)

	// Set reset vector to point to start of code (0x8000)
	prgROM[0x3FFC] = 0x00
	prgROM[0x3FFD] = 0x80

	rom = append(rom, prgROM...)

	// CHR-ROM (8192 bytes)
	chrROM := make([]byte, 8192)
	rom = append(rom, chrROM...)

	return rom
}
