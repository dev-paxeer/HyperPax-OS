// Copyright PaxLabs Ltd.(Paxeer Network)
// Paxeer Network Non-Commercial License 1.0 (ENCL-1.0)(https://github.com/Paxeer-Network/hyperpaxeer-os/blob/main/LICENSE_FAQ.md)

package contracts

import (
	_ "embed" // embed compiled smart contract
	"encoding/json"

	evmtypes "github.com/evmos/evmos/v18/x/evm/types"
)

var (
	//go:embed compiled_contracts/ERC20Burnable.json
	erc20BurnableJSON []byte

	// ERC20BurnableContract is the compiled ERC20Burnable contract
	ERC20BurnableContract evmtypes.CompiledContract
)

func init() {
	err := json.Unmarshal(erc20BurnableJSON, &ERC20BurnableContract)
	if err != nil {
		panic(err)
	}
}
