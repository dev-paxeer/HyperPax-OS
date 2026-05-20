// Copyright PaxLabs Ltd.(Paxeer Network)
// Paxeer Network Non-Commercial License 1.0 (ENCL-1.0)(https://github.com/Paxeer-Network/hyperpaxeer-os/blob/main/LICENSE_FAQ.md)


package testdata

import (
	_ "embed" // embed compiled smart contract
	"encoding/json"

	evmtypes "github.com/evmos/evmos/v18/x/evm/types"
)

var (
	//go:embed WEVMOS.json
	WevmosJSON []byte

	// WEVMOSContract is the compiled contract of WEVMOS
	WEVMOSContract evmtypes.CompiledContract
)

func init() {
	err := json.Unmarshal(WevmosJSON, &WEVMOSContract)
	if err != nil {
		panic(err)
	}

	if len(WEVMOSContract.Bin) == 0 {
		panic("failed to load WEVMOS smart contract")
	}
}
