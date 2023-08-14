package disasm

type bank struct {
	prg []byte

	variables     map[uint16]*variable
	usedVariables map[uint16]struct{}

	jumpEngines            map[uint16]struct{} // set of all jump engine functions addresses
	jumpEngineCallers      []*jumpEngineCaller // jump engine caller tables to process
	jumpEngineCallersAdded map[uint16]*jumpEngineCaller
	branchDestinations     map[uint16]struct{} // set of all addresses that are branched to
	offsets                []offset

	offsetsToParse      []uint16
	offsetsToParseAdded map[uint16]struct{}
	offsetsParsed       map[uint16]struct{}

	functionReturnsToParse      []uint16
	functionReturnsToParseAdded map[uint16]struct{}
}

type bankReference struct {
	bank    *bank
	address uint16
	index   uint16
}

func newBank(prg []byte) *bank {
	return &bank{
		prg:                         prg,
		variables:                   map[uint16]*variable{},
		usedVariables:               map[uint16]struct{}{},
		offsets:                     make([]offset, len(prg)),
		jumpEngineCallersAdded:      map[uint16]*jumpEngineCaller{},
		jumpEngines:                 map[uint16]struct{}{},
		branchDestinations:          map[uint16]struct{}{},
		offsetsToParseAdded:         map[uint16]struct{}{},
		offsetsParsed:               map[uint16]struct{}{},
		functionReturnsToParseAdded: map[uint16]struct{}{},
	}
}

// TODO gets passed address
func (b *bank) offsetInfo(index uint16) *offset {
	offsetInfo := &b.offsets[index]
	return offsetInfo
}
