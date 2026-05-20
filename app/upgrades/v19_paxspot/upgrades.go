package v19paxspot


import (
	"slices"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	upgradetypes "github.com/cosmos/cosmos-sdk/x/upgrade/types"
	evmkeeper "github.com/evmos/evmos/v18/x/evm/keeper"
)

// PaxSpot precompile addresses
var paxSpotPrecompiles = []string{
	"0x0000000000000000000000000000000000000901", // OROB resolver
	"0x0000000000000000000000000000000000000902", // BatchClearing
	"0x0000000000000000000000000000000000000903", // OracleAggregator
	"0x0000000000000000000000000000000000000904", // PoFQ scorer
}

// Removed outpost addresses that previously occupied the 0x900-0x901 range
var removedOutposts = []string{
	"0x0000000000000000000000000000000000000900", // was Stride outpost
	"0x0000000000000000000000000000000000000901", // was Osmosis outpost
}

// CreateUpgradeHandler creates an SDK upgrade handler for v19-paxspot.
//
// This upgrade:
//  1. Removes Stride and Osmosis outpost precompiles (0x900, 0x901) from ActivePrecompiles.
//  2. Adds PaxSpot precompiles (0x901-0x904) to ActivePrecompiles.
//  3. Runs module migrations — this initializes the x/paxoracle module store and default params.
func CreateUpgradeHandler(
	mm *module.Manager,
	configurator module.Configurator,
	ek *evmkeeper.Keeper,
) upgradetypes.UpgradeHandler {
	return func(ctx sdk.Context, _ upgradetypes.Plan, vm module.VersionMap) (module.VersionMap, error) {
		logger := ctx.Logger().With("upgrade", UpgradeName)

		// Step 1: Update EVM ActivePrecompiles — remove old outposts, add PaxSpot
		logger.Info("updating EVM active precompiles: removing outposts, adding PaxSpot")
		if err := migratePaxSpotPrecompiles(ctx, ek); err != nil {
			return nil, err
		}

		// Step 2: Run module migrations (initializes x/paxoracle store + default params)
		logger.Info("running module migrations (x/paxoracle init)")
		return mm.RunMigrations(ctx, configurator, vm)
	}
}

// migratePaxSpotPrecompiles removes old Stride/Osmosis outpost addresses from
// the EVM ActivePrecompiles list and adds the PaxSpot precompile addresses.
func migratePaxSpotPrecompiles(ctx sdk.Context, ek *evmkeeper.Keeper) error {
	params := ek.GetParams(ctx)

	// Remove old outpost addresses
	filtered := make([]string, 0, len(params.ActivePrecompiles))
	for _, addr := range params.ActivePrecompiles {
		if !slices.Contains(removedOutposts, addr) {
			filtered = append(filtered, addr)
		}
	}

	// Add PaxSpot precompile addresses (skip if already present)
	for _, addr := range paxSpotPrecompiles {
		if !slices.Contains(filtered, addr) {
			filtered = append(filtered, addr)
		}
	}

	params.ActivePrecompiles = filtered

	return ek.SetParams(ctx, params)
}
