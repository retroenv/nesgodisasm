package nesasm

import (
	"fmt"
	"io"

	"github.com/retroenv/nesgodisasm/internal/program"
)

// addBankSelectors adds bank selectors to every 0x2000 byte offsets, as required
// by nesasm to avoid the error "Bank overflow, offset > $1FFF".
func addBankSelectors(prg []*program.PRGBank) {
	counter := 0
	bankNumber := 1
	bankSize := 0x2000
	var bankSwitch bool

	for _, bank := range prg {
		index := 0

		for {
			if bankSwitch { // if switch was carried over after last bank was filled
				bank.PRG[index].WriteCallback = writeBankSelector(bankNumber)
				bankNumber++
				bankSwitch = false
			}

			bankSpaceLeft := counter % bankSize
			if bankSpaceLeft == 0 {
				bankSpaceLeft = bankSize
			}

			bankBytesLeft := len(bank.PRG[index:])
			if bankSpaceLeft > bankBytesLeft {
				counter += bankBytesLeft
				break
			}
			if bankSpaceLeft == bankBytesLeft {
				counter += bankBytesLeft
				bankSwitch = true
				break
			}

			bank.PRG[index+bankSpaceLeft].WriteCallback = writeBankSelector(bankNumber)

			index += bankSpaceLeft
			counter += bankSpaceLeft
			bankNumber++
		}
	}
}

func writeBankSelector(bankNumber int) func(writer io.Writer) error {
	return func(writer io.Writer) error {
		if _, err := fmt.Fprintf(writer, "\n .bank %d\n\n", bankNumber); err != nil {
			return fmt.Errorf("writing bank switch: %w", err)
		}
		return nil
	}
}
