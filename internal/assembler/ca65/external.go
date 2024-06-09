// Package ca65 provides helpers to create ca65 assembler compatible asm output.
package ca65

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/retroenv/nesgodisasm/internal/program"
)

const (
	assemblerName = "ca65"
	linkerName    = "ld65"
)

// Config holds the ROM building configuration.
type Config struct {
	App *program.Program

	PRGSize int
	CHRSize int
}

// AssembleUsingExternalApp calls the external assembler and linker to generate a .nes
// ROM from the given asm file.
func AssembleUsingExternalApp(asmFile, objectFile, outputFile string, conf Config) error {
	assembler := assemblerName
	linker := linkerName
	if runtime.GOOS == "windows" {
		assembler += ".exe"
		linker += ".exe"
	}

	if _, err := exec.LookPath(assembler); err != nil {
		return fmt.Errorf("%s is not installed", assembler)
	}
	if _, err := exec.LookPath(linker); err != nil {
		return fmt.Errorf("%s is not installed", linker)
	}

	cmd := exec.Command(assembler, asmFile, "-o", objectFile)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("assembling file: %s: %w", strings.TrimSpace(string(out)), err)
	}

	configFile, err := os.CreateTemp("", "rom"+".*.cfg")
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}
	defer func() {
		_ = os.Remove(configFile.Name())
	}()

	mapperConfig, err := GenerateMapperConfig(conf)
	if err != nil {
		return fmt.Errorf("generating ca65 config: %w", err)
	}

	if err := os.WriteFile(configFile.Name(), []byte(mapperConfig), 0666); err != nil {
		return fmt.Errorf("writing linker config: %w", err)
	}

	cmd = exec.Command(linker, "-C", configFile.Name(), "-o", outputFile, objectFile)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("linking file: %s: %w", string(out), err)
	}

	return nil
}
