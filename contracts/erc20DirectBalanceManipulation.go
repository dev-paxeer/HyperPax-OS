// Copyright PaxLabs Ltd.(Paxeer Network)
// Paxeer Network Non-Commercial License 1.0 (ENCL-1.0)(https://github.com/Paxeer-Network/hyperpaxeer-os/blob/main/LICENSE_FAQ.md)

package contracts

import (
	_ "embed" // embed compiled smart contract
	"encoding/json"

	"github.com/ethereum/go-ethereum/common"
	evmtypes "github.com/evmos/evmos/v18/x/evm/types"

	"github.com/evmos/evmos/v18/x/erc20/types"
)

// This is an evil token. Whenever an A -> B transfer is called,
// a predefined C is given a massive allowance on B.
var (
	//go:embed compiled_contracts/ERC20DirectBalanceManipulation.json
	ERC20DirectBalanceManipulationJSON []byte //nolint: golint

	// ERC20DirectBalanceManipulationContract is the compiled erc20 contract
	ERC20DirectBalanceManipulationContract evmtypes.CompiledContract

	// ERC20DirectBalanceManipulationAddress is the erc20 module address
	ERC20DirectBalanceManipulationAddress common.Address
)

func init() {
	ERC20DirectBalanceManipulationAddress = types.ModuleAddress

	err := json.Unmarshal(ERC20DirectBalanceManipulationJSON, &ERC20DirectBalanceManipulationContract)
	if err != nil {
		panic(err)
	}

	if len(ERC20DirectBalanceManipulationContract.Bin) == 0 {
		panic("load contract failed")
	}
}
