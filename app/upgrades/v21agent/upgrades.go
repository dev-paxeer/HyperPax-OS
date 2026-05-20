// Copyright PaxLabs Ltd.(Paxeer Network)
// Paxeer Network Non-Commercial License 1.0 (ENCL-1.0)(https://github.com/Paxeer-Network/hyperpaxeer-os/blob/main/LICENSE_FAQ.md)


package v21agent

import (
	"slices"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	upgradetypes "github.com/cosmos/cosmos-sdk/x/upgrade/types"

	evmkeeper "github.com/evmos/evmos/v18/x/evm/keeper"
)

// CreateUpgradeHandler creates an SDK upgrade handler for v21-agent-payments.
//
// On execution this handler MUST:
//
//  1. Append `NewlyActivePrecompiles` (PaymentStreams 0x906, TEEAttestor 0x907,
//     EIP712Helper 0x908) to the EVM params' `ActivePrecompiles` list.
//  2. Run module migrations — initializes x/streams and x/attestor default params
//     and genesis. (EIP712Helper is stateless, no module to migrate.)
//
// TEE root certificates are NOT loaded by this handler. Roots are added by a
// follow-up gov proposal (MsgUpdateTEERoots) signed by governance — keeps the
// upgrade itself deterministic and reviewable.
//
// Reference: app/upgrades/v19_paxspot/upgrades.go for the migration pattern.
func CreateUpgradeHandler(
	mm *module.Manager,
	configurator module.Configurator,
	ek *evmkeeper.Keeper,
) upgradetypes.UpgradeHandler {
	return func(ctx sdk.Context, _ upgradetypes.Plan, vm module.VersionMap) (module.VersionMap, error) {
		logger := ctx.Logger().With("upgrade", UpgradeName)

		// Step 1: activate the new precompiles.
		logger.Info("activating new precompiles", "addresses", NewlyActivePrecompiles)
		if err := activatePrecompiles(ctx, ek, NewlyActivePrecompiles); err != nil {
			return nil, err
		}

		// Step 2: run module migrations.
		logger.Info("running module migrations")
		return mm.RunMigrations(ctx, configurator, vm)
	}
}

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
