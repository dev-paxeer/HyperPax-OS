// Copyright PaxLabs Ltd.(Paxeer Network)
// Paxeer Network Non-Commercial License 1.0 (ENCL-1.0)(https://github.com/Paxeer-Network/hyperpaxeer-os/blob/main/LICENSE_FAQ.md)

package keeper

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"

	evmtypes "github.com/evmos/evmos/v18/x/evm/types"
)

// Compile-time interface assertion: feemarket Hooks implements EvmHooks.
var _ evmtypes.EvmHooks = Hooks{}

// Hooks wraps the feemarket Keeper to satisfy the EVM PostTxProcessing hook
// interface. Wired into the multi-EVM-hooks chain in app/app.go alongside the
// erc20 + revenue hooks (see Evmos.SetHooks call site).
type Hooks struct {
	k Keeper
}

// Hooks returns a Hooks wrapper for use in multi-hook composition.
func (k Keeper) Hooks() Hooks { return Hooks{k} }

// LaneRegistration / LaneDeregistration are the on-chain events the
// AgentWalletFactory / ServiceRegistry contracts emit when an address is
// added to or removed from the lane allow-list.
//
// Topic schema (matching IServiceRegistry.sol):
//
//	LaneRegistration(address indexed addr)
//	LaneDeregistration(address indexed addr)
//
// The hook only mirrors events whose log.Address matches the configured
// LaneParams.Registry. This prevents random contracts from polluting the
// in-keeper LaneMember set.
var (
	laneRegistrationTopic   = crypto.Keccak256Hash([]byte("LaneRegistration(address)"))
	laneDeregistrationTopic = crypto.Keccak256Hash([]byte("LaneDeregistration(address)"))
)

// PostTxProcessing implements evmtypes.EvmHooks.PostTxProcessing.
//
// On every committed EVM tx we scan the receipt for LaneRegistration /
// LaneDeregistration events emitted by the configured registry contract and
// mirror them into the in-keeper LaneMember set. This is a single-pass
// O(numLogs) scan with no external state reads beyond the LaneParams record.
//
// Returning a non-nil error here would revert the entire EVM tx. We never do
// that — if the lane registry is misconfigured (zero address, bogus events)
// the worst case is we silently miss a registration, not a tx revert.
func (h Hooks) PostTxProcessing(ctx sdk.Context, _ core.Message, receipt *ethtypes.Receipt) error {
	if receipt == nil || len(receipt.Logs) == 0 {
		return nil
	}
	lp := h.k.GetLaneParams(ctx)
	registry := lp.RegistryAddress()
	if (registry == common.Address{}) {
		// No registry configured — nothing to mirror.
		return nil
	}
	for _, log := range receipt.Logs {
		if log == nil || log.Address != registry {
			continue
		}
		if len(log.Topics) < 2 {
			continue
		}
		addr := common.BytesToAddress(log.Topics[1].Bytes())
		switch log.Topics[0] {
		case laneRegistrationTopic:
			h.k.AddLaneMember(ctx, addr)
			h.k.Logger(ctx).Debug("lane member registered", "addr", addr.Hex())
		case laneDeregistrationTopic:
			h.k.RemoveLaneMember(ctx, addr)
			h.k.Logger(ctx).Debug("lane member deregistered", "addr", addr.Hex())
		}
	}
	return nil
}
