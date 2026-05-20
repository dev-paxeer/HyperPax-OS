// Copyright PaxLabs Ltd.(Paxeer Network)
// Paxeer Network Non-Commercial License 1.0 (ENCL-1.0)(https://github.com/Paxeer-Network/hyperpaxeer-os/blob/main/LICENSE_FAQ.md)


package testdata

import (
	_ "embed" // embed compiled smart contract
	"encoding/json"

	"github.com/ethereum/go-ethereum/common"
	evmtypes "github.com/evmos/evmos/v18/x/evm/types"

	"github.com/evmos/evmos/v18/x/erc20/types"
)

var (
	//go:embed ERC20Minter_OpenZeppelinV5.json
	ERC20MinterV5JSON []byte //nolint: golint

	// ERC20MinterV5Contract is the compiled erc20 contract
	ERC20MinterV5Contract evmtypes.CompiledContract

	// ERC20MinterV5Address is the erc20 module address
	ERC20MinterV5Address common.Address
)

func init() {
	ERC20MinterV5Address = types.ModuleAddress

	err := json.Unmarshal(ERC20MinterV5JSON, &ERC20MinterV5Contract)
	if err != nil {
		panic(err)
	}

	if len(ERC20MinterV5Contract.Bin) == 0 {
		panic("load contract failed")
	}
}
