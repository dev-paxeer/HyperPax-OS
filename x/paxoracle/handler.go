package paxoracle

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"

	"github.com/evmos/evmos/v18/x/paxoracle/keeper"
	"github.com/evmos/evmos/v18/x/paxoracle/types"
)

// NewHandler returns a handler for paxoracle messages.
func NewHandler(k keeper.Keeper) sdk.Handler {
	msgServer := keeper.NewMsgServerImpl(k)

	return func(ctx sdk.Context, msg sdk.Msg) (*sdk.Result, error) {
		switch msg := msg.(type) {
		case *types.MsgSubmitPrice:
			err := msgServer.SubmitPrice(ctx, msg)
			if err != nil {
				return nil, err
			}
			return &sdk.Result{Events: ctx.EventManager().ABCIEvents()}, nil
		default:
			return nil, sdkerrors.ErrUnknownRequest.Wrapf("unrecognized %s message type: %T", types.ModuleName, msg)
		}
	}
}
