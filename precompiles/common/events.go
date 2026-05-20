// Copyright PaxLabs Ltd.(Paxeer Network)
// Paxeer Network Non-Commercial License 1.0 (ENCL-1.0)(https://github.com/Paxeer-Network/hyperpaxeer-os/blob/main/LICENSE_FAQ.md)


package common

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
)

// EmitEventArgs are the arguments required to emit an authorization event.
//
// The event type can be:
//   - ApprovalEvent
//   - GenericApprovalEvent
//   - AllowanceChangeEvent
//   - ...
type EmitEventArgs struct {
	Ctx            sdk.Context
	StateDB        vm.StateDB
	ContractAddr   common.Address
	ContractEvents map[string]abi.Event
	EventData      interface{}
}
