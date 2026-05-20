package keeper

import (
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/evmos/evmos/v18/x/paxoracle/types"
)

// MsgServer implements the message handling for the paxoracle module.
type MsgServer struct {
	Keeper
}

// NewMsgServerImpl returns an implementation of the paxoracle MsgServer interface.
func NewMsgServerImpl(keeper Keeper) MsgServer {
	return MsgServer{Keeper: keeper}
}

// SubmitPrice handles MsgSubmitPrice — stores a validator's price attestation.
func (ms MsgServer) SubmitPrice(ctx sdk.Context, msg *types.MsgSubmitPrice) error {
	// Validate that the signer is an active validator
	signerAddr, err := sdk.AccAddressFromBech32(msg.Signer)
	if err != nil {
		return types.ErrInvalidSigner.Wrapf("invalid signer: %s", err)
	}

	valAddr := sdk.ValAddress(signerAddr)
	_, found := ms.stakingKeeper.GetValidator(ctx, valAddr)
	if !found {
		return types.ErrNotValidator.Wrapf("address %s is not an active validator", msg.Signer)
	}

	marketId := msg.GetMarketIdArray()

	// Verify market is supported (if we have any supported markets configured)
	markets := ms.GetAllSupportedMarkets(ctx)
	if len(markets) > 0 && !ms.IsSupportedMarket(ctx, marketId) {
		return types.ErrMarketNotSupported.Wrapf("market %x is not in the supported set", marketId)
	}

	price, ok := msg.GetPriceBigInt()
	if !ok {
		return types.ErrInvalidPrice.Wrap("invalid price string")
	}
	confidence, ok := msg.GetConfidenceBigInt()
	if !ok {
		return types.ErrInvalidConfidence.Wrap("invalid confidence string")
	}

	// Store the submission
	sub := types.PriceSubmission{
		ValidatorAddr: msg.Signer,
		MarketId:      marketId,
		Price:         price,
		Confidence:    confidence,
		BlockHeight:   ctx.BlockHeight(),
		Timestamp:     ctx.BlockTime().Unix(),
	}

	ms.SetPriceSubmission(ctx, sub)

	ms.Logger(ctx).Debug(
		"price submitted",
		"validator", msg.Signer,
		"market", marketId,
		"price", msg.Price,
		"block", ctx.BlockHeight(),
	)

	ctx.EventManager().EmitEvent(
		sdk.NewEvent(
			"paxoracle_price_submitted",
			sdk.NewAttribute("validator", msg.Signer),
			sdk.NewAttribute("price", msg.Price),
			sdk.NewAttribute("block_height", sdk.NewInt(ctx.BlockHeight()).String()),
		),
	)

	return nil
}
