// Copyright PaxLabs Ltd.(Paxeer Network)
// Paxeer Network Non-Commercial License 1.0 (ENCL-1.0)(https://github.com/Paxeer-Network/hyperpaxeer-os/blob/main/LICENSE_FAQ.md)

package keeper

import (
	"encoding/binary"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/evmos/evmos/v18/x/evm/types"
)

// EIP-7702 (SetCodeTx, type 0x04) activation height storage.
//
// The activation height lives outside the protobuf-generated `Params` struct
// — adding a field there would require regenerating evm.pb.go from .proto,
// which AGENTS.md §4.3 calls out as something we avoid in the v20 upgrade
// pass. Instead we store the height under a dedicated KV prefix
// (`KeyPrefixEIP7702BlockNumber`, 0x04 byte). Absence of the key means
// EIP-7702 is disabled — that's the default for pre-v20 chains and for
// freshly initialized chains.
//
// The v20-agent-foundations upgrade handler writes a future activation height
// to this key. The actual gating in `state_transition.go::ApplyMessage` is
// landed alongside the geth fork backport (see
// app/upgrades/v20agent/geth_7702_backport.md); until that backport ships,
// this value is read-only and has no behavioural effect on the EVM frame.

// GetEIP7702BlockNumber returns the activation block height for EIP-7702.
// Returns 0 when the key is unset (EIP-7702 disabled).
func (k Keeper) GetEIP7702BlockNumber(ctx sdk.Context) int64 {
	store := ctx.KVStore(k.storeKey)
	bz := store.Get(types.KeyPrefixEIP7702BlockNumber)
	if len(bz) != 8 {
		return 0
	}
	return int64(binary.BigEndian.Uint64(bz))
}

// SetEIP7702BlockNumber persists the activation block height for EIP-7702.
// Negative values are rejected. A height of 0 acts as "disabled" (matches the
// unset default).
func (k Keeper) SetEIP7702BlockNumber(ctx sdk.Context, height int64) error {
	if height < 0 {
		return types.ErrInvalidEIP7702Height
	}
	store := ctx.KVStore(k.storeKey)
	if height == 0 {
		store.Delete(types.KeyPrefixEIP7702BlockNumber)
		return nil
	}
	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], uint64(height))
	store.Set(types.KeyPrefixEIP7702BlockNumber, buf[:])
	return nil
}

// IsEIP7702Enabled reports whether the chain has reached the EIP-7702
// activation height. Returns false when the height is unset or the chain is
// still below it.
func (k Keeper) IsEIP7702Enabled(ctx sdk.Context) bool {
	height := k.GetEIP7702BlockNumber(ctx)
	if height == 0 {
		return false
	}
	return ctx.BlockHeight() >= height
}
