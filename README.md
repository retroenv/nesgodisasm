# retrodisasm - a tracing disassembler for retro systems

[![Build status](https://github.com/retroenv/retrodisasm/actions/workflows/go.yaml/badge.svg?branch=main)](https://github.com/retroenv/retrodisasm/actions)
[![go.dev reference](https://img.shields.io/badge/go.dev-reference-007d9c?logo=go&logoColor=white&style=flat-square)](https://pkg.go.dev/github.com/retroenv/retrodisasm)
[![Go Report Card](https://goreportcard.com/badge/github.com/retroenv/retrodisasm)](https://goreportcard.com/report/github.com/retroenv/retrodisasm)
[![codecov](https://codecov.io/gh/retroenv/retrodisasm/branch/main/graph/badge.svg?token=NS5UY28V3A)](https://codecov.io/gh/retroenv/retrodisasm)

> **Note:** This project was renamed from `nesgodisasm` to `retrodisasm` to reflect its expanded support for multiple retro systems beyond just NES.

retrodisasm is a tracing disassembler for retro console and computer systems.

## Supported Systems

| System | Architecture | Assemblers |
|--------|-------------|------------|
| **NES** | 6502 | asm6, ca65, nesasm, retroasm |
| **CHIP-8** | CHIP-8 VM | retroasm |

The tool automatically detects the system from file extensions (`.nes`, `.ch8`, `.rom`) or you can specify it manually with `-s`.

## Features

### Core Features
* **Bit-Perfect Reassembly** - Generated assembly reassembles to produce the exact same ROM binary
* **Multi-Architecture Support** - Modular architecture supporting NES and CHIP-8 systems
* **Execution Flow Tracing** - Differentiates between code and data through program flow analysis
* **Multiple Assembler Outputs** - Generates assembly compatible with various assemblers
* **Batch Processing** - Process multiple ROMs at once with automatic naming
* **Smart Output** - Does not output trailing zero bytes by default
* **Flexible & Extensible** - Easy to add support for new systems and assemblers

### NES-Specific Features
* Outputs [asm6](https://github.com/freem/asm6f)*/[ca65](https://github.com/cc65/cc65)/[nesasm](https://github.com/ClusterM/nesasm)/[retroasm](https://github.com/retroenv/retroasm) compatible assembly files
* Translates known RAM addresses to aliases
* Supports undocumented 6502 CPU opcodes
* Handles branching into opcode parts of instructions
* Experimental support for mappers with banking

### CHIP-8-Specific Features
* Outputs [retroasm](https://github.com/retroenv/retroasm) compatible assembly files
* Handles all standard CHIP-8 instructions (35 opcodes)

## Installation

The tool uses a modern software stack that does not have any system dependencies beside requiring a somewhat modern
operating system to run:

* Linux: 2.6.32+
* Windows: 10+
* macOS: 10.15 Catalina+

There are 2 options to install retrodisasm:

1. Download and unpack a binary release from [Releases](https://github.com/retroenv/retrodisasm/releases)

or

2. Compile the latest release from source:

```
go install github.com/retroenv/retrodisasm@latest
```

Compiling the tool from source code needs to have a recent version of [Golang](https://go.dev/) installed.

To use the `-verify` option, the chosen assembler needs to be installed.

## Usage

Basic usage (system auto-detected from file extension):

```bash
retrodisasm -o output.asm input.nes      # NES ROM
retrodisasm -o output.asm input.ch8      # CHIP-8 ROM
```

Manual system specification:

```bash
retrodisasm -s nes -o game.asm game.bin
retrodisasm -s chip8 -o program.asm program.rom
```

Example output (NES):

```asm
Reset:
  sei                            ; $8000 78
  cld                            ; $8001 D8
  lda #$10                       ; $8002 A9 10
  sta PPU_CTRL                   ; $8004 8D 00 20
...
```

Reassemble:

```bash
ca65 output.asm -o output.o && ld65 output.o -t nes -o output.nes
```

## Options

```
usage: retrodisasm [options] <file to disassemble>

  -a string
    	Assembler compatibility of the generated .asm file (asm6/ca65/nesasm/retroasm) (default "ca65")
  -batch string
    	process a batch of given path and file mask and automatically .asm file naming, for example *.nes
  -binary
    	read input file as raw binary file without any header
  -c string
    	Config file name to write output to for ca65 assembler
  -cdl string
    	name of the .cdl Code/Data log file to load
  -debug
    	enable debugging options for extended logging
  -i string
    	name of the input ROM file
  -o string
    	name of the output .asm file, printed on console if no name given
  -q	perform operations quietly
  -s string
    	system to disassemble for (nes, chip8) - if not auto-detected from file extension
  -verify
    	verify the generated output by assembling with ca65 and check if it matches the input
```

### System-Specific Options

**NES:**
- All assemblers supported: `-a asm6`, `-a ca65` (default), `-a nesasm`, `-a retroasm`
- CDL (Code/Data Log) support: `-cdl <file.cdl>`
- Verification: `-verify` (requires ca65 installed)

**CHIP-8:**
- Only retroasm supported: `-a retroasm`
- Auto-detection from `.ch8` or `.rom` extensions
- Manual specification: `-s chip8`

## Notes

\* `asm6f v1.6 (modifications v03)` or later is required
