// Package vars manages variables in the disassembled program.
package vars

import (
	"fmt"
	"sort"

	"github.com/retroenv/nesgodisasm/internal/arch"
	"github.com/retroenv/nesgodisasm/internal/program"
	"github.com/retroenv/retrogolib/arch/system/nes"
)

const (
	dataNaming            = "_data_%04x"
	dataNamingIndexed     = "_data_%04x_indexed"
	jumpTableNaming       = "_jump_table_%04x"
	variableNaming        = "_var_%04x"
	variableNamingIndexed = "_var_%04x_indexed"
)

// Vars manages variables in the disassembled program.
type Vars struct {
	arch arch.Architecture

	banks []*bank

	variables     map[uint16]*variable
	usedVariables map[uint16]struct{}
}

type bank struct {
	variables     map[uint16]*variable
	usedVariables map[uint16]struct{}
}

type variable struct {
	reads  bool
	writes bool

	address      uint16
	name         string
	indexedUsage bool                 // access with X/Y registers indicates table
	usageAt      []arch.BankReference // list of all indexes that use this offset
}

// New creates a new variables manager.
func New(arch arch.Architecture) *Vars {
	return &Vars{
		arch:          arch,
		variables:     make(map[uint16]*variable),
		usedVariables: make(map[uint16]struct{}),
	}
}

// AddReference adds a variable reference if the opcode is accessing
// the given address directly by reading or writing. In a special case like
// branching into a zeropage address the variable usage can be forced.
func (v *Vars) AddReference(dis arch.Disasm, addressReference,
	usageAddress uint16, opcode arch.Opcode, forceVariableUsage bool) {

	var reads, writes bool
	if opcode.ReadWritesMemory() {
		reads = true
		writes = true
	} else {
		reads = opcode.ReadsMemory()
		writes = opcode.WritesMemory()
	}
	if !reads && !writes && !forceVariableUsage {
		return
	}

	varInfo := v.variables[addressReference]
	if varInfo == nil {
		varInfo = &variable{
			address: addressReference,
		}
		v.variables[addressReference] = varInfo
	}

	mapper := dis.Mapper()
	bankRef := arch.BankReference{
		Mapped:  mapper.GetMappedBank(usageAddress),
		Address: usageAddress,
		Index:   mapper.GetMappedBankIndex(usageAddress),
	}
	bankRef.ID = bankRef.Mapped.ID()
	varInfo.usageAt = append(varInfo.usageAt, bankRef)

	if reads {
		varInfo.reads = true
	}
	if writes {
		varInfo.writes = true
	}

	if v.arch.IsAddressingIndexed(opcode) {
		varInfo.indexedUsage = true
	}
}

// Process processes all variables and updates the instructions that use them
// with a generated alias name.
func (v *Vars) Process(dis arch.Disasm) error {
	variables := make([]*variable, 0, len(v.variables))
	for _, varInfo := range v.variables {
		variables = append(variables, varInfo)
	}
	sort.Slice(variables, func(i, j int) bool {
		return variables[i].address < variables[j].address
	})

	for _, varInfo := range variables {
		if len(varInfo.usageAt) == 1 && !varInfo.indexedUsage && varInfo.address < nes.CodeBaseAddress {
			if !varInfo.reads || !varInfo.writes {
				continue // ignore only once usages or ones that are not read and write
			}
		}

		var dataOffsetInfo *arch.Offset
		var addressAdjustment uint16
		codeBaseAddress := dis.CodeBaseAddress()
		if varInfo.address >= codeBaseAddress {
			// if the referenced address is inside the code, a label will be created for it
			dataOffsetInfo, varInfo.address, addressAdjustment = v.getOpcodeStart(dis, varInfo.address)
		} else {
			// if the address is outside the code bank, a variable will be created
			v.usedVariables[varInfo.address] = struct{}{}

			for _, bankRef := range varInfo.usageAt {
				v.AddUsage(bankRef.ID, varInfo)
			}
		}

		var reference string
		varInfo.name, reference = v.dataName(dataOffsetInfo, varInfo.indexedUsage, varInfo.address, addressAdjustment)

		for _, bankRef := range varInfo.usageAt {
			offsetInfo := bankRef.Mapped.OffsetInfo(bankRef.Index)

			if err := v.arch.ProcessVariableUsage(offsetInfo, reference); err != nil {
				return fmt.Errorf("processing variable usage: %w", err)
			}
		}
	}
	return nil
}

// AddBank adds a new bank to the variables manager.
func (v *Vars) AddBank() {
	v.banks = append(v.banks, &bank{
		variables:     make(map[uint16]*variable),
		usedVariables: make(map[uint16]struct{}),
	})
}

// AddUsage adds a usage info of a variable to a bank.
func (v *Vars) AddUsage(bankIndex int, varInfo *variable) {
	bank := v.banks[bankIndex]
	bank.variables[varInfo.address] = varInfo
	bank.usedVariables[varInfo.address] = struct{}{}
}

// getOpcodeStart returns a reference to the opcode start of the given address.
// In case it's in the first or second byte of an instruction, referencing the middle of an instruction will be
// converted to a reference to the beginning of the instruction and optional address adjustment like +1 or +2.
func (v *Vars) getOpcodeStart(dis arch.Disasm, address uint16) (*arch.Offset, uint16, uint16) {
	var addressAdjustment uint16

	for {
		offsetInfo := dis.Mapper().OffsetInfo(address)
		if len(offsetInfo.Data) == 0 {
			address--
			addressAdjustment++
			continue
		}
		return offsetInfo, address, addressAdjustment
	}
}

// dataName calculates the name of a variable based on its address and optional address adjustment.
// It returns the name of the variable and a string to reference it, it is possible that the reference
// is using an adjuster like +1 or +2.
func (v *Vars) dataName(offsetInfo *arch.Offset, indexedUsage bool, address, addressAdjustment uint16) (string, string) {
	var name string

	if offsetInfo != nil && offsetInfo.Label != "" {
		// if destination has an existing label, reuse it
		name = offsetInfo.Label
	} else {
		prgAccess := offsetInfo != nil
		var jumpTable bool
		if prgAccess {
			jumpTable = offsetInfo.IsType(program.JumpTable)
		}

		switch {
		case jumpTable:
			name = fmt.Sprintf(jumpTableNaming, address)
		case prgAccess && indexedUsage:
			name = fmt.Sprintf(dataNamingIndexed, address)
		case prgAccess && !indexedUsage:
			name = fmt.Sprintf(dataNaming, address)
		case !prgAccess && indexedUsage:
			name = fmt.Sprintf(variableNamingIndexed, address)
		default:
			name = fmt.Sprintf(variableNaming, address)
		}
	}

	reference := name
	if addressAdjustment > 0 {
		reference = fmt.Sprintf("%s+%d", reference, addressAdjustment)
	}
	if offsetInfo != nil && offsetInfo.Label == "" {
		offsetInfo.Label = name
	}
	return name, reference
}

// SetBankVariables sets the used variables in the bank for outputting.
func (v *Vars) SetBankVariables(bankID int, prgBank *program.PRGBank) {
	bank := v.banks[bankID]
	for address := range bank.usedVariables {
		varInfo := bank.variables[address]
		prgBank.Variables[varInfo.name] = address
	}
}

// SetToProgram sets the used variables in the program for outputting.
func (v *Vars) SetToProgram(app *program.Program) {
	for address := range v.usedVariables {
		varInfo := v.variables[address]
		app.Variables[varInfo.name] = address
	}
}
