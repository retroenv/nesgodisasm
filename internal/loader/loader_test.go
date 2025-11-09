package loader

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/retroenv/retrodisasm/internal/options"
	"github.com/retroenv/retrogolib/arch"
	"github.com/retroenv/retrogolib/assert"
)

//nolint:funlen,cyclop // test functions can be long and complex
func TestLoad(t *testing.T) {
	t.Run("load binary file", func(t *testing.T) {
		tmpFile := createTempFile(t, []byte{0x01, 0x02, 0x03, 0x04})
		defer os.Remove(tmpFile) //nolint:errcheck // test cleanup

		loader := New()
		opts := options.Program{
			Input:  tmpFile,
			Binary: true,
		}

		cart, cdlReader, err := loader.Load(opts, arch.NES)
		assert.NoError(t, err)
		assert.NotNil(t, cart)
		assert.Nil(t, cdlReader)
	})

	t.Run("load CHIP8 file", func(t *testing.T) {
		tmpFile := createTempFile(t, []byte{0x12, 0x34, 0x56, 0x78})
		defer os.Remove(tmpFile) //nolint:errcheck // test cleanup

		loader := New()
		opts := options.Program{
			Input: tmpFile,
		}

		cart, cdlReader, err := loader.Load(opts, arch.CHIP8System)
		assert.NoError(t, err)
		assert.NotNil(t, cart)
		assert.Nil(t, cdlReader)
	})

	t.Run("load NES file with valid header", func(t *testing.T) {
		// Create minimal valid NES ROM with iNES header
		nesData := make([]byte, 16+16384) // Header + 16KB PRG
		copy(nesData[0:4], []byte{'N', 'E', 'S', 0x1A})
		nesData[4] = 1 // 1 PRG bank

		tmpFile := createTempFile(t, nesData)
		defer os.Remove(tmpFile) //nolint:errcheck // test cleanup

		loader := New()
		opts := options.Program{
			Input: tmpFile,
		}

		cart, cdlReader, err := loader.Load(opts, arch.NES)
		assert.NoError(t, err)
		assert.NotNil(t, cart)
		assert.Nil(t, cdlReader)
	})

	t.Run("error on non-existent file", func(t *testing.T) {
		loader := New()
		opts := options.Program{
			Input: "/nonexistent/file.nes",
		}

		_, _, err := loader.Load(opts, arch.NES)
		assert.Error(t, err)
	})

	t.Run("load with CDL file", func(t *testing.T) {
		tmpFile := createTempFile(t, []byte{0x01, 0x02, 0x03, 0x04})
		defer os.Remove(tmpFile) //nolint:errcheck // test cleanup

		tmpCDL := createTempFile(t, []byte{0xC0, 0xDE})
		defer os.Remove(tmpCDL) //nolint:errcheck // test cleanup

		loader := New()
		opts := options.Program{
			Input:       tmpFile,
			Binary:      true,
			CodeDataLog: tmpCDL,
		}

		cart, cdlReader, err := loader.Load(opts, arch.NES)
		assert.NoError(t, err)
		assert.NotNil(t, cart)
		assert.NotNil(t, cdlReader)
		_ = cdlReader.Close()
	})

	t.Run("error on non-existent CDL file", func(t *testing.T) {
		tmpFile := createTempFile(t, []byte{0x01, 0x02, 0x03, 0x04})
		defer os.Remove(tmpFile) //nolint:errcheck // test cleanup

		loader := New()
		opts := options.Program{
			Input:       tmpFile,
			Binary:      true,
			CodeDataLog: "/nonexistent/cdl.log",
		}

		_, _, err := loader.Load(opts, arch.NES)
		assert.Error(t, err)
	})
}

func createTempFile(t *testing.T, data []byte) string {
	t.Helper()
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.bin")
	if err := os.WriteFile(tmpFile, data, 0600); err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	return tmpFile
}
