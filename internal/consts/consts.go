// Package consts manages constants in the disassembled program.
package consts

import (
	"fmt"
	"sort"
	"strings"

	"github.com/retroenv/nesgodisasm/internal/arch"
	"github.com/retroenv/nesgodisasm/internal/program"
)

var _ arch.ConstantManager = &Consts{}

// Consts manages constants in the disassembled program.
type Consts struct {
	banks []*bank

	constants     map[uint16]arch.Constant
	usedConstants map[uint16]arch.Constant
}

type bank struct {
	constants     map[uint16]arch.Constant
	usedConstants map[uint16]arch.Constant
}

type architecture interface {
	Constants() (map[uint16]arch.Constant, error)
}

// New creates a new constants manager.
func New(ar architecture) (*Consts, error) {
	constants, err := ar.Constants()
	if err != nil {
		return nil, fmt.Errorf("getting constants: %w", err)
	}

	return &Consts{
		constants:     constants,
		usedConstants: make(map[uint16]arch.Constant),
	}, nil
}

// AddBank adds a new bank to the constants manager.
func (c *Consts) AddBank() {
	c.banks = append(c.banks, &bank{
		constants:     make(map[uint16]arch.Constant),
		usedConstants: make(map[uint16]arch.Constant),
	})
}

// ReplaceParameter replaces the parameter of an instruction by a constant name
// if the address of the instruction is found in the constants map.
func (c *Consts) ReplaceParameter(address uint16, opcode arch.Opcode, paramAsString string) (string, bool) {
	constantInfo, ok := c.constants[address]
	if !ok {
		return "", false
	}

	// split parameter string in case of x/y indexing, only the first part will be replaced by a const name
	paramParts := strings.Split(paramAsString, ",")

	if constantInfo.Read != "" && opcode.ReadsMemory() {
		c.usedConstants[address] = constantInfo
		paramParts[0] = constantInfo.Read
		return strings.Join(paramParts, ","), true
	}
	if constantInfo.Write != "" && opcode.WritesMemory() {
		c.usedConstants[address] = constantInfo
		paramParts[0] = constantInfo.Write
		return strings.Join(paramParts, ","), true
	}

	return paramAsString, true
}

// ProcessConstants processes all constants and updates all banks with the used ones. There is currently no tracking
// for in which bank a constant is used, it will be added to all banks for now.
// TODO fix constants to only output in used banks
func (c *Consts) ProcessConstants() {
	constants := make([]arch.Constant, 0, len(c.constants))
	for _, translation := range c.constants {
		constants = append(constants, translation)
	}
	sort.Slice(constants, func(i, j int) bool {
		return constants[i].Address < constants[j].Address
	})

	for _, constInfo := range constants {
		_, used := c.usedConstants[constInfo.Address]
		if !used {
			continue
		}

		for _, bnk := range c.banks {
			bnk.constants[constInfo.Address] = constInfo
			bnk.usedConstants[constInfo.Address] = constInfo
		}
	}
}

// SetProgramConstants sets the used constants in the program for outputting.
func (c *Consts) SetProgramConstants(app *program.Program) {
	for address := range c.usedConstants {
		constantInfo := c.constants[address]
		if constantInfo.Read != "" {
			app.Constants[constantInfo.Read] = address
		}
		if constantInfo.Write != "" {
			app.Constants[constantInfo.Write] = address
		}
	}
}

// SetBankConstants sets the used constants in the bank for outputting.
func (c *Consts) SetBankConstants(bankID int, prgBank *program.PRGBank) {
	bank := c.banks[bankID]
	for address := range bank.usedConstants {
		constantInfo := bank.constants[address]
		if constantInfo.Read != "" {
			prgBank.Constants[constantInfo.Read] = address
		}
		if constantInfo.Write != "" {
			prgBank.Constants[constantInfo.Write] = address
		}
	}
}
