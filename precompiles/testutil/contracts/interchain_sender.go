// Copyright PaxLabs Ltd.(Paxeer Network)
// Paxeer Network Non-Commercial License 1.0 (ENCL-1.0)(https://github.com/Paxeer-Network/hyperpaxeer-os/blob/main/LICENSE_FAQ.md)


package contracts

import (
	_ "embed" // embed compiled smart contract
	"encoding/json"

	evmtypes "github.com/evmos/evmos/v18/x/evm/types"
)

var (
	//go:embed InterchainSender.json
	InterchainSenderJSON []byte

	// InterchainSenderContract is the compiled contract calling the distribution precompile
	InterchainSenderContract evmtypes.CompiledContract
)

func init() {
	err := json.Unmarshal(InterchainSenderJSON, &InterchainSenderContract)
	if err != nil {
		panic(err)
	}

	if len(InterchainSenderContract.Bin) == 0 {
		panic("failed to load smart contract that calls distribution precompile")
	}
}
