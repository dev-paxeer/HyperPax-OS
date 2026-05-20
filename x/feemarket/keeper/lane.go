// Copyright PaxLabs Ltd.(Paxeer Network)
// Paxeer Network Non-Commercial License 1.0 (ENCL-1.0)(https://github.com/Paxeer-Network/hyperpaxeer-os/blob/main/LICENSE_FAQ.md)

package keeper

import (
	"encoding/json"
	"math/big"

	errorsmod "cosmossdk.io/errors"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/ethereum/go-ethereum/common"

	"github.com/evmos/evmos/v18/x/feemarket/types"
)

// Agent fee lane state.
//
// Two pieces of state, both keyed under prefixes added by the v20-agent
// foundations upgrade:
//
//	KeyLaneParams         (single value)  -> JSON LaneParams
//	KeyPrefixLaneMember | addr (20 bytes) -> 1 byte marker
//
// LaneParams is a small struct (Enabled / GasPrice / Registry). Membership is
// mirrored from on-chain `LaneRegistration` events by Hooks.PostTxProcessing
// in keeper/lane_hook.go.

// GetLaneParams returns the stored lane params or the default (disabled) when
// no value has been written. Never returns a nil math.Int — the default uses
// math.ZeroInt() so callers can compare without an extra IsNil check.
func (k Keeper) GetLaneParams(ctx sdk.Context) types.LaneParams {
	store := ctx.KVStore(k.storeKey)
	bz := store.Get(types.KeyLaneParams)
	if len(bz) == 0 {
		return types.DefaultLaneParams()
	}
	var lp types.LaneParams
	if err := json.Unmarshal(bz, &lp); err != nil {
		// Malformed record — return default rather than panicking. The
		// upgrade handler will rewrite the default below.
		return types.DefaultLaneParams()
	}
	if lp.GasPrice.IsNil() {
		lp.GasPrice = types.DefaultLaneParams().GasPrice
	}
	return lp
}

// SetLaneParams persists the given LaneParams after validating them.
func (k Keeper) SetLaneParams(ctx sdk.Context, lp types.LaneParams) error {
	if err := lp.Validate(); err != nil {
		return errorsmod.Wrap(err, "lane params validation failed")
	}
	store := ctx.KVStore(k.storeKey)
	bz, err := json.Marshal(lp)
	if err != nil {
		return err
	}
	store.Set(types.KeyLaneParams, bz)
	return nil
}

// SetDefaultLaneParams writes the default (disabled) lane configuration.
// Used by the v20-agent-foundations upgrade handler to definitively place the
// lane in the disabled state at activation.
func (k Keeper) SetDefaultLaneParams(ctx sdk.Context) error {
	return k.SetLaneParams(ctx, types.DefaultLaneParams())
}

// IsAgentLaneEnabled returns whether the master switch is on. Cheap — single
// store hit + JSON decode.
func (k Keeper) IsAgentLaneEnabled(ctx sdk.Context) bool {
	return k.GetLaneParams(ctx).Enabled
}

// AddLaneMember registers an EVM address as a lane member. Idempotent: a
// second call for the same address is a no-op. Used by:
//   - keeper/lane_hook.go in response to mirrored LaneRegistration events
//   - gov-driven keeper helpers (test fixtures, ad-hoc allowlisting)
func (k Keeper) AddLaneMember(ctx sdk.Context, addr common.Address) {
	store := ctx.KVStore(k.storeKey)
	store.Set(types.LaneMemberKey(addr.Bytes()), []byte{1})
}

// RemoveLaneMember unregisters an EVM address from the lane. Idempotent.
func (k Keeper) RemoveLaneMember(ctx sdk.Context, addr common.Address) {
	store := ctx.KVStore(k.storeKey)
	store.Delete(types.LaneMemberKey(addr.Bytes()))
}

// IsLaneMember returns true when addr is in the lane membership set,
// regardless of the master Enabled switch. Callers that want behavioural
// gating should use IsAgentLaneCaller below.
func (k Keeper) IsLaneMember(ctx sdk.Context, addr common.Address) bool {
	store := ctx.KVStore(k.storeKey)
	return store.Has(types.LaneMemberKey(addr.Bytes()))
}

// IsAgentLaneCaller returns true when:
//   - the master switch is on, AND
//   - either the signer or target address is in the lane membership set.
//
// This is the canonical predicate the ante decorators consume. When the
// switch is off the function short-circuits at the LaneParams read.
func (k Keeper) IsAgentLaneCaller(ctx sdk.Context, signer, target common.Address) bool {
	if !k.IsAgentLaneEnabled(ctx) {
		return false
	}
	if k.IsLaneMember(ctx, signer) {
		return true
	}
	// Zero target = create-tx. Only the signer-side check applies.
	if (target == common.Address{}) {
		return false
	}
	return k.IsLaneMember(ctx, target)
}

// LaneGasPrice returns the lane-substituted gas price for a (signer, target)
// pair. Returns nil when the caller is not in the lane — callers should
// treat nil as "no substitution, fall through to standard min-gas-price".
func (k Keeper) LaneGasPrice(ctx sdk.Context, signer, target common.Address) *big.Int {
	if !k.IsAgentLaneCaller(ctx, signer, target) {
		return nil
	}
	lp := k.GetLaneParams(ctx)
	if lp.GasPrice.IsNil() || lp.GasPrice.IsZero() {
		return nil
	}
	return lp.GasPrice.BigInt()
}
