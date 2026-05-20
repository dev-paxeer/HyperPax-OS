// Copyright PaxLabs Ltd.(Paxeer Network)
// Paxeer Network Non-Commercial License 1.0 (ENCL-1.0)(https://github.com/Paxeer-Network/hyperpaxeer-os/blob/main/LICENSE_FAQ.md)


package keeper

import (
	"strconv"

	errorsmod "cosmossdk.io/errors"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"

	"github.com/evmos/evmos/v18/x/attestor/types"
)

// Event types emitted by the attestor module on root updates.
const (
	EventTypeRootsUpdated = "attestor_roots_updated"
)

// MsgServer implements the Cosmos message handling for the attestor module.
//
// The module exposes exactly one message type — MsgUpdateTEERoots — and it
// is gov-only. The handler verifies the signer matches the module authority
// (governance), then replaces the trusted root set for the requested family.
type MsgServer struct {
	Keeper
}

// NewMsgServerImpl returns an implementation of the attestor MsgServer.
func NewMsgServerImpl(keeper Keeper) MsgServer {
	return MsgServer{Keeper: keeper}
}

// UpdateTEERoots handles MsgUpdateTEERoots — replaces the trust set for a
// single TEE family. Only the module authority (governance) may submit.
//
// Idempotent: passing an empty Roots clears the family's trust set
// (effectively disabling verification for that family).
func (ms MsgServer) UpdateTEERoots(ctx sdk.Context, msg *types.MsgUpdateTEERoots) error {
	if err := msg.ValidateBasic(); err != nil {
		return err
	}

	signer, err := sdk.AccAddressFromBech32(msg.Authority)
	if err != nil {
		return sdkerrors.ErrInvalidAddress.Wrapf("invalid authority: %s", err)
	}
	if !signer.Equals(ms.Keeper.Authority()) {
		return errorsmod.Wrapf(
			types.ErrUnauthorizedUpdate,
			"signer %s is not the module authority %s",
			signer.String(), ms.Keeper.Authority().String(),
		)
	}

	family := msg.FamilyByte()
	if err := ms.Keeper.SetFamilyRoots(ctx, family, msg.Roots); err != nil {
		return err
	}

	ctx.EventManager().EmitEvent(sdk.NewEvent(
		EventTypeRootsUpdated,
		sdk.NewAttribute("family", strconv.FormatUint(uint64(family), 10)),
		sdk.NewAttribute("family_name", types.FamilyName(family)),
		sdk.NewAttribute("count", strconv.Itoa(len(msg.Roots))),
		sdk.NewAttribute("authority", signer.String()),
	))
	ms.Keeper.Logger(ctx).Info(
		"TEE roots updated",
		"family", types.FamilyName(family),
		"count", len(msg.Roots),
	)
	return nil
}
