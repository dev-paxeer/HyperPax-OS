// Copyright PaxLabs Ltd.(Paxeer Network)
// Paxeer Network Non-Commercial License 1.0 (ENCL-1.0)(https://github.com/Paxeer-Network/hyperpaxeer-os/blob/main/LICENSE_FAQ.md)

package keeper

import (
	"fmt"
	"math/big"
	"runtime/debug"
	"strconv"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/ethereum/go-ethereum/common"

	"github.com/evmos/evmos/v18/x/scheduler/types"
)

// EndBlocker dispatches every Job whose ExecuteAtBlock == ctx.BlockHeight().
//
// Invariants (see AGENTS.md §3.6):
//   - A single failing dispatch MUST NOT halt block production.
//   - State writes from a failed call are rolled back via CacheContext.
//   - Successful calls commit their state changes via cacheCtx.write().
//   - Each job is removed from every index (DeleteJob) regardless of outcome,
//     so the same job cannot be re-dispatched in a subsequent block.
//   - Deposit accounting:
//   - On success: refund (deposit - gasUsed * baseFee) to the creator.
//   - On failure: refund the FULL deposit (the EVM rolled back, no value
//     was consumed by the scheduled call itself).
//     The keeper.RefundDeposit call uses the OUTER ctx so refunds survive a
//     dispatch panic just like DeleteJob does.
func (k Keeper) EndBlocker(ctx sdk.Context) {
	height := uint64(ctx.BlockHeight())

	// Step 1: collect all due jobs into a slice up-front. Iterating + deleting
	// the same prefix is fragile (the cosmos-sdk iterator semantics around
	// concurrent deletion are implementation-defined), so we always snapshot
	// first, then mutate.
	due := make([]types.Job, 0)
	k.IterateJobsDueAt(ctx, height, func(j types.Job) bool {
		if !j.Active {
			return false
		}
		due = append(due, j)
		return false
	})
	if len(due) == 0 {
		return
	}

	logger := k.Logger(ctx)
	logger.Debug("scheduler EndBlocker dispatching jobs", "count", len(due), "height", height)

	for _, j := range due {
		k.dispatchJob(ctx, j)
	}
}

// dispatchJob runs a single Job within a CacheContext + recover() guard. A
// panic or error is captured and converted to a `scheduler_job_failed` event;
// the outer block always continues.
func (k Keeper) dispatchJob(ctx sdk.Context, j types.Job) {
	jobID := j.ID
	target := common.BytesToAddress(j.Target)
	creator := common.BytesToAddress(j.Creator)
	deposit := parseDeposit(j.Deposit)

	// Always remove the job — successful or failed — so the same record is not
	// re-dispatched in subsequent blocks. Deletion is performed on the OUTER
	// ctx so it survives even if the cached EVM call rolls back.
	defer k.DeleteJob(ctx, j)

	// Cache-context isolation. Any panic or error from the EVM call rolls back
	// changes that happened inside this scope.
	cacheCtx, writeCache := ctx.CacheContext()

	// Recover guard: a panic inside ApplyMessage/EstimateGasInternal or any
	// downstream contract MUST NOT propagate up into ABCI EndBlock.
	var (
		callErr      error
		gasUsed      uint64
		recoveredMsg string
	)

	func() {
		defer func() {
			if r := recover(); r != nil {
				recoveredMsg = fmt.Sprintf("panic: %v\n%s", r, debug.Stack())
			}
		}()

		resp, err := k.evmKeeper.CallEVMWithData(cacheCtx, creator, &target, j.CallData, true)
		if err != nil {
			callErr = err
			return
		}
		if resp != nil {
			gasUsed = resp.GasUsed
		}
	}()

	// Failed dispatch — refund full deposit, emit failure event, discard cache.
	if recoveredMsg != "" || callErr != nil {
		errStr := recoveredMsg
		if errStr == "" {
			errStr = callErr.Error()
		}
		// Truncate huge error/panic strings to keep events bounded.
		if len(errStr) > 512 {
			errStr = errStr[:512]
		}
		refund := new(big.Int).Set(deposit)
		if refundErr := k.RefundDeposit(ctx, creator, refund); refundErr != nil {
			// Don't halt the block — log and continue. The deposit stays in
			// the module account; an operator can recover it via gov.
			k.Logger(ctx).Error(
				"scheduler job refund failed",
				"job_id", jobID,
				"creator", creator.Hex(),
				"refund", refund.String(),
				"error", refundErr.Error(),
			)
			refund = new(big.Int)
		}
		ctx.EventManager().EmitEvent(sdk.NewEvent(
			EventTypeJobFailed,
			sdk.NewAttribute("job_id", strconv.FormatUint(jobID, 10)),
			sdk.NewAttribute("creator", creator.Hex()),
			sdk.NewAttribute("target", target.Hex()),
			sdk.NewAttribute("error", errStr),
			sdk.NewAttribute("refund", refund.String()),
		))
		k.Logger(ctx).Info(
			"scheduler job failed",
			"job_id", jobID,
			"creator", creator.Hex(),
			"target", target.Hex(),
			"error", errStr,
		)
		return
	}

	// Successful dispatch — commit cached state changes to the outer ctx, then
	// refund (deposit - gasUsed * baseFee) to the creator.
	writeCache()

	baseFee := k.feeMarketKeeper.GetBaseFee(ctx)
	if baseFee == nil {
		baseFee = big.NewInt(0)
	}
	consumed := new(big.Int).Mul(big.NewInt(0).SetUint64(gasUsed), baseFee)
	refund := new(big.Int).Sub(deposit, consumed)
	if refund.Sign() < 0 {
		refund = new(big.Int)
	}
	if refundErr := k.RefundDeposit(ctx, creator, refund); refundErr != nil {
		k.Logger(ctx).Error(
			"scheduler job refund failed",
			"job_id", jobID,
			"creator", creator.Hex(),
			"refund", refund.String(),
			"error", refundErr.Error(),
		)
		refund = new(big.Int)
	}

	ctx.EventManager().EmitEvent(sdk.NewEvent(
		EventTypeJobExecuted,
		sdk.NewAttribute("job_id", strconv.FormatUint(jobID, 10)),
		sdk.NewAttribute("creator", creator.Hex()),
		sdk.NewAttribute("target", target.Hex()),
		sdk.NewAttribute("gas_used", strconv.FormatUint(gasUsed, 10)),
		sdk.NewAttribute("consumed", consumed.String()),
		sdk.NewAttribute("refund", refund.String()),
	))
	k.Logger(ctx).Debug(
		"scheduler job executed",
		"job_id", jobID,
		"creator", creator.Hex(),
		"target", target.Hex(),
		"gas_used", gasUsed,
		"refund", refund.String(),
	)
}
