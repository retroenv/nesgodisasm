package ca65

import (
	"fmt"
	"os"
	"os/exec"
)

const (
	assembler = "ca65"
	linker    = "ld65"
)

// Config holds the ROM building configuration.
type Config struct {
	PRGSize int
	CHRSize int
}

// AssembleUsingExternalApp calls the external assembler and linker to generate a .nes
// ROM from the given asm file.
func AssembleUsingExternalApp(asmFile, objectFile, outputFile string, conf Config) error {
	if _, err := exec.LookPath(assembler); err != nil {
		return fmt.Errorf("%s is not installed", assembler)
	}
	if _, err := exec.LookPath(linker); err != nil {
		return fmt.Errorf("%s is not installed", linker)
	}

	cmd := exec.Command(assembler, asmFile, "-o", objectFile)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("assembling file: %s: %w", string(out), err)
	}

	configFile, err := os.CreateTemp("", "rom"+".*.cfg")
	if err != nil {
		return err
	}
	defer func() {
		_ = os.Remove(configFile.Name())
	}()

	mapperConfig := generateMapperConfig(conf)

	if err := os.WriteFile(configFile.Name(), []byte(mapperConfig), 0444); err != nil {
		return fmt.Errorf("writing linker config: %w", err)
	}

	cmd = exec.Command(linker, "-C", configFile.Name(), "-o", outputFile, objectFile)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("linking file: %s: %w", string(out), err)
	}

	return nil
}
