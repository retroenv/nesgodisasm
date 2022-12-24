package disasm

import (
	"fmt"
	"sort"

	. "github.com/retroenv/retrogolib/nes/addressing"
	"github.com/retroenv/retrogolib/nes/cpu"
	"github.com/retroenv/retrogolib/nes/parameter"
)

const (
	dataNaming            = "_data_%04x"
	dataNamingIndexed     = "_data_%04x_indexed"
	variableNaming        = "_var_%04x"
	variableNamingIndexed = "_var_%04x_indexed"
)

type variable struct {
	reads  bool
	writes bool

	address      uint16
	name         string
	indexedUsage bool     // access with X/Y registers indicates table
	usageAt      []uint16 // list of all indexes that use this offset
}

// addVariableReference adds a variable reference if the opcode is accessing the given address directly by
// reading or writing. In a special case like branching into a zeropage address the variable usage can be forced.
func (dis *Disasm) addVariableReference(address, index uint16, opcode cpu.Opcode, forceVariableUsage bool) bool {
	var reads, writes bool
	if opcode.ReadWritesMemory() {
		reads = true
		writes = true
	} else {
		reads = opcode.ReadsMemory()
		writes = opcode.WritesMemory()
	}
	if !reads && !writes && !forceVariableUsage {
		return false
	}

	varInfo := dis.variables[address]
	if varInfo == nil {
		varInfo = &variable{
			address: address,
		}
		dis.variables[address] = varInfo
	}
	varInfo.usageAt = append(varInfo.usageAt, index)

	if reads {
		varInfo.reads = true
	}
	if writes {
		varInfo.writes = true
	}

	switch opcode.Addressing {
	case ZeroPageXAddressing, ZeroPageYAddressing,
		AbsoluteXAddressing, AbsoluteYAddressing,
		IndirectXAddressing, IndirectYAddressing:
		varInfo.indexedUsage = true
	}

	return true
}

// processVariables processes all variables and updates the instructions that use them
// with a generated alias name.
func (dis *Disasm) processVariables() error {
	variables := make([]*variable, 0, len(dis.variables))
	for _, varInfo := range dis.variables {
		variables = append(variables, varInfo)
	}
	sort.Slice(variables, func(i, j int) bool {
		return variables[i].address < variables[j].address
	})

	for _, varInfo := range variables {
		if len(varInfo.usageAt) == 1 && !varInfo.indexedUsage && varInfo.address < CodeBaseAddress {
			if !varInfo.reads || !varInfo.writes {
				continue // ignore only once usages or ones that are not read and write
			}
		}

		var dataOffsetInfo *offset
		var addressAdjustment uint16
		if varInfo.address >= dis.codeBaseAddress {
			dataOffsetInfo, varInfo.address, addressAdjustment = dis.getOpcodeStart(varInfo.address)
		} else {
			dis.usedVariables[varInfo.address] = struct{}{}
		}

		var reference string
		varInfo.name, reference = dis.dataName(dataOffsetInfo, varInfo.indexedUsage, varInfo.address, addressAdjustment)

		for _, usedAddress := range varInfo.usageAt {
			index := dis.addressToIndex(usedAddress)
			offsetInfo := &dis.offsets[index]

			converted, err := parameter.String(dis.converter, offsetInfo.opcode.Addressing, reference)
			if err != nil {
				return fmt.Errorf("getting parameter as string: %w", err)
			}

			switch offsetInfo.opcode.Addressing {
			case ZeroPageAddressing, ZeroPageXAddressing, ZeroPageYAddressing:
				offsetInfo.Code = fmt.Sprintf("%s z:%s", offsetInfo.opcode.Instruction.Name, converted)
			case AbsoluteAddressing, AbsoluteXAddressing, AbsoluteYAddressing:
				offsetInfo.Code = fmt.Sprintf("%s a:%s", offsetInfo.opcode.Instruction.Name, converted)
			case IndirectAddressing, IndirectXAddressing, IndirectYAddressing:
				offsetInfo.Code = fmt.Sprintf("%s %s", offsetInfo.opcode.Instruction.Name, converted)
			}
		}
	}
	return nil
}

// getOpcodeStart returns a reference to the opcode start of the given address.
// In case it's in the first or second byte of an instruction, referencing the middle of an instruction will be
// converted to a reference to the beginning of the instruction and optional address adjustment like +1 or +2.
func (dis *Disasm) getOpcodeStart(address uint16) (*offset, uint16, uint16) {
	var addressAdjustment uint16
	for {
		index := dis.addressToIndex(address)
		offsetInfo := &dis.offsets[index]

		if len(offsetInfo.OpcodeBytes) == 0 {
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
func (dis *Disasm) dataName(offsetInfo *offset, indexedUsage bool, address, addressAdjustment uint16) (string, string) {
	var name string

	if offsetInfo != nil && offsetInfo.Label != "" {
		// if destination has an existing label, reuse it
		name = offsetInfo.Label
	} else {
		prgAccess := offsetInfo != nil

		switch {
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
