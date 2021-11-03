package env

import (
	"fmt"
	"testing"

	"github.com/celo-org/celo-blockchain/common"
)

func TestUniqueContractAddresses(t *testing.T) {
	addresses := make(map[common.Address]bool)

	for name, addr := range genesisAddresses {
		if addresses[addr] {
			t.Errorf("Duplicated use of address. %s - %s", addr.Hex(), name)
		}
		addresses[addr] = true
	}

	for name, addr := range libraryAddresses {
		if addresses[addr] {
			t.Errorf("Duplicated use of address. %s - %s", addr.Hex(), name)
		}
		addresses[addr] = true
	}
}
func TestNamed(t *testing.T) {
	fmt.Println(common.ZeroAddress.String())
	//0x  000 000 000 000 000 000 000 000 000 000 000 000 000 0   //  0x40个0
}
