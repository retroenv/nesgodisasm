# nesgodisasm - a tracing disassembler for NES ROMs

[![Build status](https://github.com/retroenv/nesgodisasm/actions/workflows/go.yaml/badge.svg?branch=main)](https://github.com/retroenv/nesgodisasm/actions)
[![go.dev reference](https://img.shields.io/badge/go.dev-reference-007d9c?logo=go&logoColor=white&style=flat-square)](https://pkg.go.dev/github.com/retroenv/nesgodisasm)
[![Go Report Card](https://goreportcard.com/badge/github.com/retroenv/nesgodisasm)](https://goreportcard.com/report/github.com/retroenv/nesgodisasm)
[![codecov](https://codecov.io/gh/retroenv/nesgodisasm/branch/main/graph/badge.svg?token=NS5UY28V3A)](https://codecov.io/gh/retroenv/nesgodisasm)


nesgodisasm allows you to disassemble programs for the Nintendo Entertainment System (NES).

## Features

* Outputs ca65 compatible .asm files that can be used to reproduce the same original NES ROM
* Translates known RAM addresses to aliases
* Traces the program execution flow to differentiate between code and data
* Supports undocumented 6502 CPU opcodes
* Supports branching into opcode parts of an instruction
* Does not output trailing zero bytes of banks by default
* Batch processing mode to disassembling multiple ROMs at once
* Flexible architecture that allows it to create output modules for other assemblers 

Currently, only ROMs that use mapper 0 are supported.

## Installation

The tool uses a modern software stack that does not have any system dependencies beside requiring a somewhat modern
operating system to run:

* Linux: 2.6.32+
* Windows: 7+
* macOS: High Sierra 10.13+

There are 2 options to install nesgodisasm:

1. Download and unpack a binary release from [Releases](https://github.com/retroenv/nesgo/releases)

or

2. Compile the latest release from source: 

```
go install github.com/retroenv/nesgodisasm@latest
```

Compiling the tool from source code needs to have a recent version of [Golang](https://go.dev/) installed.

To use the `-verify` option, [cc65](https://github.com/cc65/cc65) needs to be installed.

## Usage

Disassemble a ROM:

```
nesgodisasm -o example.asm example.nes
```

The generated assembly file content will look like:

```
...
Reset:
  sei                            ; $8000 78
  cld                            ; $8001 D8
  lda #$10                       ; $8002 A9 10
  sta PPU_CTRL                   ; $8004 8D 00 20
  ldx #$FF                       ; $8007 A2 FF
  txs                            ; $8009 9A

_label_800a:
  lda PPU_STATUS                 ; $800A AD 02 20
  bpl _label_800a                ; $800D 10 FB

...
.segment "VECTORS"

.addr NMI, Reset, IRQ
```

Assemble an .asm file back to a ROM:

```
ca65 example.asm -o example.o
ld65 example.o -t nes -o example.nes 
```

## Options

```
usage: nesgodisasm [options] <file to disassemble>

  -batch string
    	process a batch of given path and file mask and automatically .asm file naming, for example *.nes
  -cdl string
    	name of the .cdl Code/Data log file to load
  -debug
    	enable debugging options (more logging and printing of ca65 config for the ROM)
  -nohexcomments
    	do not output opcode bytes as hex values in comments
  -nooffsets
    	do not output offsets in comments
  -o string
    	name of the output .asm file, printed on console if no name given
  -q	perform operations quietly
  -verify
    	verify the generated output by assembling with ca65 and check if it matches the input
  -z	output the trailing zero bytes of banks
```
