// Copyright PaxLabs Ltd.(Paxeer Network)
// Paxeer Network Non-Commercial License 1.0 (ENCL-1.0)(https://github.com/Paxeer-Network/hyperpaxeer-os/blob/main/LICENSE_FAQ.md)


package attestor

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"

	"github.com/evmos/evmos/v18/x/attestor/keeper"
	"github.com/evmos/evmos/v18/x/attestor/types"
)

// NewHandler returns an sdk.Handler for attestor messages. The module exposes
// exactly one message — MsgUpdateTEERoots — and it is gov-only.
func NewHandler(k keeper.Keeper) sdk.Handler {
	msgServer := keeper.NewMsgServerImpl(k)

	return func(ctx sdk.Context, msg sdk.Msg) (*sdk.Result, error) {
		switch msg := msg.(type) {
		case *types.MsgUpdateTEERoots:
			if err := msgServer.UpdateTEERoots(ctx, msg); err != nil {
				return nil, err
			}
			return &sdk.Result{Events: ctx.EventManager().ABCIEvents()}, nil
		default:
			return nil, sdkerrors.ErrUnknownRequest.Wrapf("unrecognized %s message type: %T", types.ModuleName, msg)
		}
	}
}
