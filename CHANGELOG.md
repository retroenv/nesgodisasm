# Changelog for nesgodisasm

All notable changes to this project will be documented in this file.

## [v0.2.2] - 2024-01-06

Added:

* add nesasm output compatibility support
* show warning of mapper is not nrom
* start of banking support
* support asm6f as a assembler parameter alias for asm6
* output data lines with address in comments

Fixed:

* fix assembler binary name for verification on windows
* fix crash when disassembling invalid irq handler addresses
* improve code base address handling for large PRG
* improve instruction parameter converter prefix and suffix handling


## [v0.2.1] - 2023-07-19

Added:

* added asm6 output compatibility support

Fixed:

* fix crashes when disassembling nesfab generated roms


## [v0.2.0] - 2023-01-05

Added:

* jump engine detection
* write CRC32 checksums of segments in the output header as comments
* batch processing of multiple input files
* new logger output with different verbosity levels

Changed:

* the project was moved into its own git repository

Fixed:

* data references into unofficial instruction opcodes
* data references before code base start
* instruction overlap with IRQ handler start address
* support different code base addresses
* variable detection for zero page access
* variable usage detection for indirect jumps


## [v0.1.1] - 2022-08-02

Added:

* add var aliases for zeropage accesses
* support code/data logs
* support more mappers
* unofficial instruction opcodes are bundled

Fixed:

* fix wrong address in comments for non standard rom base addresses
* support data references into instruction opcodes


## [v0.1.0] - 2022-06-26

First version of nesgodisasm released.
