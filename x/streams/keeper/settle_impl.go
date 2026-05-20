// Copyright PaxLabs Ltd.(Paxeer Network)
// Paxeer Network Non-Commercial License 1.0 (ENCL-1.0)(https://github.com/Paxeer-Network/hyperpaxeer-os/blob/main/LICENSE_FAQ.md)


package keeper

import (
	"math/big"
	"strconv"

	errorsmod "cosmossdk.io/errors"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/ethereum/go-ethereum/common"

	"github.com/evmos/evmos/v18/x/streams/types"
)

// Event types emitted by the streams module. Names use snake_case per
// AGENTS.md §3.5. Solidity-event parity (StreamOpened, StreamSettled, etc.)
// is the precompile's responsibility — the keeper-level events here are the
// authoritative state-change record for off-chain indexers.
const (
	EventTypeStreamOpened      = "stream_opened"
	EventTypeStreamSettled     = "stream_settled"
	EventTypeStreamClosed      = "stream_closed"
	EventTypeStreamRateUpdated = "stream_rate_updated"
)

// parseAmount decodes a decimal-string amount into a non-negative *big.Int.
// Empty / malformed strings yield zero.
func parseAmount(s string) *big.Int {
	if s == "" {
		return new(big.Int)
	}
	v, ok := new(big.Int).SetString(s, 10)
	if !ok || v.Sign() < 0 {
		return new(big.Int)
	}
	return v
}

// Open registers a new Stream and escrows `cap` of `token` from payer to the
// module account. Validates limits + caller authorization, allocates a
// monotonic ID, persists the Stream, and emits an event.
//
// Custody model is "escrow at open" (D3 locked): the full cap moves up-front,
// not pulled per-block as it accrues. Close returns the unsettled remainder.
func (k Keeper) Open(
	ctx sdk.Context,
	payer common.Address,
	payee common.Address,
	token common.Address,
	ratePerSecond *big.Int,
	startTime uint64,
	stopTime uint64,
	cap *big.Int,
) (uint64, error) {
	if (payee == common.Address{}) {
		return 0, types.ErrInvalidPayee
	}
	if payer == payee {
		return 0, types.ErrSelfPayment
	}
	if ratePerSecond == nil || ratePerSecond.Sign() <= 0 {
		return 0, types.ErrInvalidRate
	}
	if cap == nil || cap.Sign() <= 0 {
		return 0, types.ErrInvalidCap
	}

	now := uint64(ctx.BlockTime().Unix())
	if startTime == 0 {
		startTime = now
	}

	params := k.GetParams(ctx)

	// stopTime == 0 means open-ended. When set, must be sufficiently far
	// past start to avoid sub-MinDuration noise streams.
	if stopTime != 0 {
		if stopTime < startTime+params.MinDuration {
			return 0, errorsmod.Wrapf(
				types.ErrInvalidTime,
				"stop_time %d < start_time + min_duration %d",
				stopTime, startTime+params.MinDuration,
			)
		}
	}

	payerBytes := payer.Bytes()
	if k.CountStreamsByPayer(ctx, payerBytes) >= params.MaxStreamsPerPayer {
		return 0, errorsmod.Wrapf(
			types.ErrMaxStreamsExceeded,
			"payer %s has reached max %d streams",
			payer.Hex(), params.MaxStreamsPerPayer,
		)
	}

	// Escrow the cap up-front. Failure here aborts before persisting the
	// stream — the EVM precompile frame will revert any earlier stateDB
	// changes (see precompiles/streams/streams.go::handleOpen).
	if err := k.escrowFromPayer(ctx, payer, token.Bytes(), cap); err != nil {
		return 0, errorsmod.Wrap(err, "failed to escrow cap from payer")
	}

	streamID := k.NextStreamID(ctx)
	stream := types.Stream{
		ID:            streamID,
		Payer:         payerBytes,
		Payee:         payee.Bytes(),
		Token:         token.Bytes(),
		RatePerSecond: ratePerSecond.String(),
		Cap:           cap.String(),
		StartTime:     startTime,
		StopTime:      stopTime,
		Settled:       "0",
		Active:        true,
	}
	if err := k.SetStream(ctx, stream); err != nil {
		return 0, err
	}

	ctx.EventManager().EmitEvent(sdk.NewEvent(
		EventTypeStreamOpened,
		sdk.NewAttribute("stream_id", strconv.FormatUint(streamID, 10)),
		sdk.NewAttribute("payer", payer.Hex()),
		sdk.NewAttribute("payee", payee.Hex()),
		sdk.NewAttribute("token", token.Hex()),
		sdk.NewAttribute("rate_per_second", ratePerSecond.String()),
		sdk.NewAttribute("start_time", strconv.FormatUint(startTime, 10)),
		sdk.NewAttribute("stop_time", strconv.FormatUint(stopTime, 10)),
		sdk.NewAttribute("cap", cap.String()),
	))

	return streamID, nil
}

// streamAccrued computes `min(rate * elapsed, cap) - settled` for a stream at
// the given unix-second timestamp. Pure math — no state writes.
func streamAccrued(s types.Stream, now uint64) *big.Int {
	rate := parseAmount(s.RatePerSecond)
	cap := parseAmount(s.Cap)
	settled := parseAmount(s.Settled)

	// effectiveNow = min(now, stopTime) when stopTime > 0
	eff := now
	if s.StopTime != 0 && eff > s.StopTime {
		eff = s.StopTime
	}
	if eff <= s.StartTime {
		return new(big.Int)
	}
	elapsed := eff - s.StartTime

	earned := new(big.Int).Mul(rate, new(big.Int).SetUint64(elapsed))
	if earned.Cmp(cap) > 0 {
		earned = cap
	}

	out := new(big.Int).Sub(earned, settled)
	if out.Sign() < 0 {
		return new(big.Int)
	}
	return out
}

// Accrued returns the amount currently withdrawable to Payee for the given
// stream, computed as `min(rate * elapsed, cap) - settled`. Read-only.
func (k Keeper) Accrued(ctx sdk.Context, s types.Stream, now uint64) *big.Int {
	return streamAccrued(s, now)
}

// Settle pays out the currently accrued amount to Payee, updating Settled.
// Caller is responsible for authorization checks; this is a payee-pull
// operation but anyone can trigger it (the funds always go to Payee).
//
// Returns the amount paid (zero if nothing has accrued since last settle).
func (k Keeper) Settle(ctx sdk.Context, streamID uint64) (*big.Int, error) {
	s, found := k.GetStream(ctx, streamID)
	if !found {
		return nil, errorsmod.Wrapf(types.ErrStreamNotFound, "stream_id=%d", streamID)
	}
	if !s.Active {
		return nil, errorsmod.Wrapf(types.ErrStreamInactive, "stream_id=%d", streamID)
	}

	now := uint64(ctx.BlockTime().Unix())
	payout := streamAccrued(s, now)
	if payout.Sign() == 0 {
		return new(big.Int), nil
	}

	payee := common.BytesToAddress(s.Payee)
	if err := k.payOutToPayee(ctx, payee, s.Token, payout); err != nil {
		return nil, err
	}

	settled := new(big.Int).Add(parseAmount(s.Settled), payout)
	s.Settled = settled.String()
	if err := k.SetStream(ctx, s); err != nil {
		return nil, err
	}

	// Committed = rate * elapsed (capped at cap), Delivered = paid this call.
	// These shape an event the generalized ReputationOracle can score.
	committed := new(big.Int).Set(payout)
	ctx.EventManager().EmitEvent(sdk.NewEvent(
		EventTypeStreamSettled,
		sdk.NewAttribute("stream_id", strconv.FormatUint(streamID, 10)),
		sdk.NewAttribute("payee", payee.Hex()),
		sdk.NewAttribute("paid", payout.String()),
		sdk.NewAttribute("committed", committed.String()),
	))

	return payout, nil
}

// Close finalizes the stream: settles outstanding accrual, refunds the unspent
// portion of Cap to Payer, and deletes the stream from state.
//
// Authorization: only Payer or Payee may close.
//
// Returns the final amount paid out to Payee on the close.
func (k Keeper) Close(ctx sdk.Context, streamID uint64, caller []byte) (*big.Int, error) {
	s, found := k.GetStream(ctx, streamID)
	if !found {
		return nil, errorsmod.Wrapf(types.ErrStreamNotFound, "stream_id=%d", streamID)
	}
	if !s.Active {
		return nil, errorsmod.Wrapf(types.ErrStreamInactive, "stream_id=%d", streamID)
	}
	if !equalAddress(caller, s.Payer) && !equalAddress(caller, s.Payee) {
		return nil, errorsmod.Wrapf(
			types.ErrUnauthorized,
			"caller %x is not the payer or payee of stream %d",
			caller, streamID,
		)
	}

	now := uint64(ctx.BlockTime().Unix())
	payout := streamAccrued(s, now)
	payer := common.BytesToAddress(s.Payer)
	payee := common.BytesToAddress(s.Payee)

	// Final settle.
	if payout.Sign() > 0 {
		if err := k.payOutToPayee(ctx, payee, s.Token, payout); err != nil {
			return nil, err
		}
		settled := new(big.Int).Add(parseAmount(s.Settled), payout)
		s.Settled = settled.String()
	}

	// Refund the unspent portion of cap to payer.
	cap := parseAmount(s.Cap)
	settled := parseAmount(s.Settled)
	refund := new(big.Int).Sub(cap, settled)
	if refund.Sign() < 0 {
		refund = new(big.Int)
	}
	if refund.Sign() > 0 {
		if err := k.refundToPayer(ctx, payer, s.Token, refund); err != nil {
			return nil, err
		}
	}

	// Delete from all three indexes.
	k.DeleteStream(ctx, s)

	ctx.EventManager().EmitEvent(sdk.NewEvent(
		EventTypeStreamClosed,
		sdk.NewAttribute("stream_id", strconv.FormatUint(streamID, 10)),
		sdk.NewAttribute("final_paid", payout.String()),
		sdk.NewAttribute("refund", refund.String()),
	))

	return payout, nil
}

// UpdateRate is the payer-only rate change. It implicitly settles current
// accrual at the OLD rate, then applies the new rate from `ctx.BlockTime()`
// onward. StartTime is bumped to "now" so subsequent accrual uses the new rate
// over the new interval.
func (k Keeper) UpdateRate(ctx sdk.Context, streamID uint64, caller []byte, newRate *big.Int) error {
	s, found := k.GetStream(ctx, streamID)
	if !found {
		return errorsmod.Wrapf(types.ErrStreamNotFound, "stream_id=%d", streamID)
	}
	if !s.Active {
		return errorsmod.Wrapf(types.ErrStreamInactive, "stream_id=%d", streamID)
	}
	if !equalAddress(caller, s.Payer) {
		return errorsmod.Wrapf(
			types.ErrUnauthorized,
			"caller %x is not the payer of stream %d",
			caller, streamID,
		)
	}
	if newRate == nil || newRate.Sign() <= 0 {
		return types.ErrInvalidRate
	}

	now := uint64(ctx.BlockTime().Unix())

	// Settle at the OLD rate first.
	payout := streamAccrued(s, now)
	if payout.Sign() > 0 {
		payee := common.BytesToAddress(s.Payee)
		if err := k.payOutToPayee(ctx, payee, s.Token, payout); err != nil {
			return err
		}
		settled := new(big.Int).Add(parseAmount(s.Settled), payout)
		s.Settled = settled.String()
	}

	oldRate := s.RatePerSecond
	s.RatePerSecond = newRate.String()
	s.StartTime = now
	if err := k.SetStream(ctx, s); err != nil {
		return err
	}

	ctx.EventManager().EmitEvent(sdk.NewEvent(
		EventTypeStreamRateUpdated,
		sdk.NewAttribute("stream_id", strconv.FormatUint(streamID, 10)),
		sdk.NewAttribute("old_rate", oldRate),
		sdk.NewAttribute("new_rate", newRate.String()),
	))
	return nil
}

// equalAddress is a constant-time-friendly byte comparator for stream
// authorization checks. Mirrors the helper in x/scheduler/keeper/schedule.go.
func equalAddress(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	var diff byte
	for i := range a {
		diff |= a[i] ^ b[i]
	}
	return diff == 0
}
