// Copyright PaxLabs Ltd.(Paxeer Network)
// Paxeer Network Non-Commercial License 1.0 (ENCL-1.0)(https://github.com/Paxeer-Network/hyperpaxeer-os/blob/main/LICENSE_FAQ.md)

package v20agent

import (
	"slices"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	upgradetypes "github.com/cosmos/cosmos-sdk/x/upgrade/types"

	evmkeeper "github.com/evmos/evmos/v18/x/evm/keeper"
)

// CreateUpgradeHandler creates an SDK upgrade handler for v20-agent-foundations.
//
// On execution this handler:
//
//  1. Appends `NewlyActivePrecompiles` (Scheduler 0x0905) to the EVM params'
//     `ActivePrecompiles` list.
//  2. Writes `ctx.BlockHeight() + EIP7702ActivationDelta` to the EVM keeper's
//     EIP-7702 activation slot. The actual gating in `state_transition.go`
//     lands alongside the geth fork backport (see
//     app/upgrades/v20agent/geth_7702_backport.md); until that backport ships,
//     this value has no behavioural effect — it just establishes the
//     parameter surface so a later upgrade does not need a fresh handler.
//  3. Asserts `FeeMarketParams.AgentLaneParams.Enabled = false`. The lane is
//     declared as default-disabled by `feemarketkeeper.SetDefaultLaneParams`
//     during module migrations; a follow-up gov proposal flips it on once
//     AgentWallet/ServiceRegistry contracts are deployed.
//  4. Runs module migrations — initializes the new x/scheduler module's
//     default params and genesis.
//
// Reference: app/upgrades/v19_paxspot/upgrades.go for the migration pattern.
func CreateUpgradeHandler(
	mm *module.Manager,
	configurator module.Configurator,
	ek *evmkeeper.Keeper,
	fmk FeeMarketLaneSetter,
) upgradetypes.UpgradeHandler {
	return func(ctx sdk.Context, _ upgradetypes.Plan, vm module.VersionMap) (module.VersionMap, error) {
		logger := ctx.Logger().With("upgrade", UpgradeName)

		// Step 1: activate the new Scheduler precompile.
		logger.Info("activating new precompiles", "addresses", NewlyActivePrecompiles)
		if err := activatePrecompiles(ctx, ek, NewlyActivePrecompiles); err != nil {
			return nil, err
		}

		// Step 2: set EIP-7702 activation block. We write to the EVM keeper's
		// dedicated KV slot rather than a protobuf Params field so this can
		// land before the geth fork backport — see x/evm/keeper/eip7702.go
		// and x/evm/types/key.go::prefixEIP7702BlockNumber.
		activation := ctx.BlockHeight() + EIP7702ActivationDelta
		logger.Info("setting EIP-7702 activation block", "height", activation)
		if err := ek.SetEIP7702BlockNumber(ctx, activation); err != nil {
			return nil, err
		}

		// Step 3: ensure the agent fee lane is disabled at activation. We
		// explicitly write the default (disabled) params so the lane state is
		// definitively present in store after the upgrade — defends against
		// a partial migration leaving the lane in an undefined state.
		logger.Info("locking agent fee lane to disabled")
		if err := fmk.SetDefaultLaneParams(ctx); err != nil {
			return nil, err
		}
		if fmk.IsAgentLaneEnabled(ctx) {
			return nil, errAgentLaneNotDisabled
		}

		// Step 4: run module migrations (initializes x/scheduler store + params).
		logger.Info("running module migrations")
		return mm.RunMigrations(ctx, configurator, vm)
	}
}

// FeeMarketLaneSetter is the subset of x/feemarket/keeper.Keeper that the
// upgrade handler needs. Defined here as an interface so the handler can be
// unit-tested against a fake without dragging the full feemarket keeper into
// the test fixture.
type FeeMarketLaneSetter interface {
	SetDefaultLaneParams(ctx sdk.Context) error
	IsAgentLaneEnabled(ctx sdk.Context) bool
}

// errAgentLaneNotDisabled is returned when step 3 completes the default-lane
// write but the lane still reports enabled — should never happen on a clean
// upgrade and indicates either a malformed prior LaneParams record or a bug.
var errAgentLaneNotDisabled = errAgentLane("agent fee lane MUST be disabled at v20-agent-foundations activation")

type errAgentLane string

func (e errAgentLane) Error() string { return string(e) }

// activatePrecompiles appends the given addresses (in lowercase 0x-hex form) to
// EvmParams.ActivePrecompiles, deduping. Mirrors the helper used in
// app/upgrades/v19_paxspot/upgrades.go.
func activatePrecompiles(ctx sdk.Context, ek *evmkeeper.Keeper, addrs []string) error {
	params := ek.GetParams(ctx)

	for _, addr := range addrs {
		if !slices.Contains(params.ActivePrecompiles, addr) {
			params.ActivePrecompiles = append(params.ActivePrecompiles, addr)
		}
	}

	return ek.SetParams(ctx, params)
}
