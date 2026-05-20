package keeper

import (
	"fmt"

	"github.com/cometbft/cometbft/libs/log"
	"github.com/cosmos/cosmos-sdk/codec"
	storetypes "github.com/cosmos/cosmos-sdk/store/types"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/evmos/evmos/v18/x/paxoracle/types"
)

// Keeper of the paxoracle store.
type Keeper struct {
	storeKey      storetypes.StoreKey
	cdc           codec.BinaryCodec
	authority     sdk.AccAddress
	stakingKeeper types.StakingKeeper
}

// NewKeeper creates a new paxoracle Keeper instance.
func NewKeeper(
	storeKey storetypes.StoreKey,
	cdc codec.BinaryCodec,
	authority sdk.AccAddress,
	stakingKeeper types.StakingKeeper,
) Keeper {
	if err := sdk.VerifyAddressFormat(authority); err != nil {
		panic(err)
	}

	return Keeper{
		storeKey:      storeKey,
		cdc:           cdc,
		authority:     authority,
		stakingKeeper: stakingKeeper,
	}
}

// Logger returns a module-specific logger.
func (k Keeper) Logger(ctx sdk.Context) log.Logger {
	return ctx.Logger().With("module", fmt.Sprintf("x/%s", types.ModuleName))
}

// GetStoreKey returns the module's store key.
func (k Keeper) GetStoreKey() storetypes.StoreKey {
	return k.storeKey
}

// IsValidator returns true if the given bech32 account address corresponds to an active validator.
func (k Keeper) IsValidator(ctx sdk.Context, accAddr sdk.AccAddress) bool {
	valAddr := sdk.ValAddress(accAddr)
	_, found := k.stakingKeeper.GetValidator(ctx, valAddr)
	return found
}
