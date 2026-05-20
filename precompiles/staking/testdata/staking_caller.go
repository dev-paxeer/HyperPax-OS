// Copyright PaxLabs Ltd.(Paxeer Network)
// Paxeer Network Non-Commercial License 1.0 (ENCL-1.0)(https://github.com/Paxeer-Network/hyperpaxeer-os/blob/main/LICENSE_FAQ.md)


package testdata

import (
	_ "embed" // embed compiled smart contract
	"encoding/json"

	evmtypes "github.com/evmos/evmos/v18/x/evm/types"
)

var (
	//go:embed StakingCaller.json
	StakingCallerJSON []byte

	// StakingCallerContract is the compiled contract calling the staking precompile
	StakingCallerContract evmtypes.CompiledContract
)

func init() {
	err := json.Unmarshal(StakingCallerJSON, &StakingCallerContract)
	if err != nil {
		panic(err)
	}

	if len(StakingCallerContract.Bin) == 0 {
		panic("failed to load smart contract that calls staking precompile")
	}
}
