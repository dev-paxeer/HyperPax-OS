// Copyright PaxLabs Ltd.(Paxeer Network)
// Paxeer Network Non-Commercial License 1.0 (ENCL-1.0)(https://github.com/Paxeer-Network/hyperpaxeer-os/blob/main/LICENSE_FAQ.md)

package keeper

import (
	"fmt"

	"github.com/cometbft/cometbft/libs/log"
	"github.com/cosmos/cosmos-sdk/codec"
	storetypes "github.com/cosmos/cosmos-sdk/store/types"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/evmos/evmos/v18/x/streams/types"
)

// Keeper of the streams store.
type Keeper struct {
	storeKey  storetypes.StoreKey
	cdc       codec.BinaryCodec
	authority sdk.AccAddress

	accountKeeper types.AccountKeeper
	bankKeeper    types.BankKeeper
	erc20Keeper   types.ERC20Keeper
}

// NewKeeper creates a new streams Keeper instance.
func NewKeeper(
	storeKey storetypes.StoreKey,
	cdc codec.BinaryCodec,
	authority sdk.AccAddress,
	accountKeeper types.AccountKeeper,
	bankKeeper types.BankKeeper,
	erc20Keeper types.ERC20Keeper,
) Keeper {
	if err := sdk.VerifyAddressFormat(authority); err != nil {
		panic(err)
	}

	return Keeper{
		storeKey:      storeKey,
		cdc:           cdc,
		authority:     authority,
		accountKeeper: accountKeeper,
		bankKeeper:    bankKeeper,
		erc20Keeper:   erc20Keeper,
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

// Authority returns the address with permission to update module params.
func (k Keeper) Authority() sdk.AccAddress {
	return k.authority
}

// ModuleAccountAddress is implemented in keeper/escrow.go alongside the
// escrow / payout helpers it serves. Keeping it here would duplicate the
// receiver method; the canonical definition lives next to its consumers.
