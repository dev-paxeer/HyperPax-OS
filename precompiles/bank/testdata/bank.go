// Copyright PaxLabs Ltd.(Paxeer Network)
// Paxeer Network Non-Commercial License 1.0 (ENCL-1.0)(https://github.com/Paxeer-Network/hyperpaxeer-os/blob/main/LICENSE_FAQ.md)


package testdata

import (
	_ "embed" // embed compiled smart contract
	"encoding/json"

	evmtypes "github.com/evmos/evmos/v18/x/evm/types"
)

var (
	//go:embed BankCaller.json
	BankCallerJSON []byte

	// BankCallerContract is the compiled contract of BankCaller.sol
	BankCallerContract evmtypes.CompiledContract
)

func init() {
	err := json.Unmarshal(BankCallerJSON, &BankCallerContract)
	if err != nil {
		panic(err)
	}

	if len(BankCallerContract.Bin) == 0 {
		panic("failed to load BankCaller smart contract")
	}
}
