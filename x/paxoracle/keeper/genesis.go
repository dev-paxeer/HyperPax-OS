package keeper

import (
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/evmos/evmos/v18/x/paxoracle/types"
)

// InitGenesis initializes the paxoracle module's state from a provided genesis state.
func (k Keeper) InitGenesis(ctx sdk.Context, genState types.GenesisState) {
	if err := k.SetParams(ctx, genState.Params); err != nil {
		panic(err)
	}

	for _, market := range genState.SupportedMarkets {
		k.SetSupportedMarket(ctx, market)
	}

	for _, sub := range genState.Submissions {
		k.SetPriceSubmission(ctx, sub)
	}
}

// ExportGenesis returns a GenesisState for a given context and keeper.
func (k Keeper) ExportGenesis(ctx sdk.Context) *types.GenesisState {
	return &types.GenesisState{
		Params:           k.GetParams(ctx),
		SupportedMarkets: k.GetAllSupportedMarkets(ctx),
		Submissions:      k.GetAllSubmissions(ctx),
	}
}
