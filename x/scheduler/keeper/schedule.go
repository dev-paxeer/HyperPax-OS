// Copyright PaxLabs Ltd.(Paxeer Network)
// Paxeer Network Non-Commercial License 1.0 (ENCL-1.0)(https://github.com/Paxeer-Network/hyperpaxeer-os/blob/main/LICENSE_FAQ.md)

package keeper

import (
	"fmt"
	"math/big"
	"strconv"

	errorsmod "cosmossdk.io/errors"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/ethereum/go-ethereum/common"

	"github.com/evmos/evmos/v18/x/scheduler/types"
)

// Event types emitted by the scheduler module. Names use snake_case per
// AGENTS.md §3.5.
const (
	EventTypeJobScheduled   = "scheduler_job_scheduled"
	EventTypeJobCancelled   = "scheduler_job_cancelled"
	EventTypeJobRescheduled = "scheduler_job_rescheduled"
	EventTypeJobExecuted    = "scheduler_job_executed"
	EventTypeJobFailed      = "scheduler_job_failed"
)

// Schedule registers a new Job. Validates limits + deposit minimum, allocates a
// monotonic ID, persists the Job to all three indexes, and emits an event.
//
// The caller is responsible for ensuring `deposit` reflects funds actually
// committed to the scheduler module account. The actual value-transfer bridge
// from EVM `msg.value` into the bank module account is handled by the
// precompile dispatcher — see precompiles/scheduler/scheduler.go::handleSchedule
// for the pre-call escrow logic. The deposit is refunded (in full on Cancel,
// partially after dispatch in EndBlocker) via Keeper.RefundDeposit.
func (k Keeper) Schedule(
	ctx sdk.Context,
	creator common.Address,
	target common.Address,
	callData []byte,
	executeAtBlock uint64,
	gasLimit uint64,
	deposit *big.Int,
) (uint64, error) {
	if (target == common.Address{}) {
		return 0, types.ErrInvalidTarget
	}
	if gasLimit == 0 {
		return 0, types.ErrInvalidGasLimit
	}
	if deposit == nil || deposit.Sign() < 0 {
		return 0, errorsmod.Wrap(types.ErrDepositTooLow, "deposit must be non-negative")
	}

	currentHeight := uint64(ctx.BlockHeight())
	if executeAtBlock <= currentHeight {
		return 0, errorsmod.Wrapf(
			types.ErrPastBlock,
			"executeAtBlock %d <= current height %d",
			executeAtBlock, currentHeight,
		)
	}

	params := k.GetParams(ctx)

	if executeAtBlock-currentHeight > params.MaxScheduleHorizon {
		return 0, errorsmod.Wrapf(
			types.ErrHorizonExceeded,
			"executeAtBlock %d exceeds horizon: current=%d, max=%d",
			executeAtBlock, currentHeight, params.MaxScheduleHorizon,
		)
	}

	creatorBytes := creator.Bytes()

	if k.CountJobsByCreator(ctx, creatorBytes) >= params.MaxJobsPerCreator {
		return 0, errorsmod.Wrapf(
			types.ErrMaxJobsExceeded,
			"creator %s has reached max %d pending jobs",
			creator.Hex(), params.MaxJobsPerCreator,
		)
	}

	// Min deposit = gasLimit * baseFee * MinDepositFactor.
	// If feemarket has no base fee yet (fresh chain / disabled), allow zero
	// deposit — the EndBlocker will still bail if the call OOGs.
	baseFee := k.feeMarketKeeper.GetBaseFee(ctx)
	if baseFee == nil {
		baseFee = big.NewInt(0)
	}
	minDeposit := new(big.Int).Mul(big.NewInt(0).SetUint64(gasLimit), baseFee)
	minDeposit.Mul(minDeposit, big.NewInt(0).SetUint64(params.MinDepositFactor))

	if deposit.Cmp(minDeposit) < 0 {
		return 0, errorsmod.Wrapf(
			types.ErrDepositTooLow,
			"required >= %s, got %s",
			minDeposit.String(), deposit.String(),
		)
	}

	jobID := k.NextJobID(ctx)
	job := types.Job{
		ID:             jobID,
		Creator:        creatorBytes,
		Target:         target.Bytes(),
		CallData:       callData,
		ExecuteAtBlock: executeAtBlock,
		GasLimit:       gasLimit,
		Deposit:        deposit.String(),
		Active:         true,
	}
	if err := k.SetJob(ctx, job); err != nil {
		return 0, err
	}

	ctx.EventManager().EmitEvent(sdk.NewEvent(
		EventTypeJobScheduled,
		sdk.NewAttribute("job_id", strconv.FormatUint(jobID, 10)),
		sdk.NewAttribute("creator", creator.Hex()),
		sdk.NewAttribute("target", target.Hex()),
		sdk.NewAttribute("execute_at_block", strconv.FormatUint(executeAtBlock, 10)),
		sdk.NewAttribute("gas_limit", strconv.FormatUint(gasLimit, 10)),
		sdk.NewAttribute("deposit", deposit.String()),
	))

	k.Logger(ctx).Debug(
		"job scheduled",
		"job_id", jobID,
		"creator", creator.Hex(),
		"target", target.Hex(),
		"execute_at_block", executeAtBlock,
	)

	return jobID, nil
}

// Cancel removes a pending Job and refunds the recorded deposit back to the
// creator. Only the original creator may cancel. The deposit is returned in
// full via SendCoinsFromModuleToAccount.
func (k Keeper) Cancel(ctx sdk.Context, caller common.Address, jobID uint64) error {
	job, found := k.GetJob(ctx, jobID)
	if !found {
		return errorsmod.Wrapf(types.ErrJobNotFound, "job_id=%d", jobID)
	}
	if !job.Active {
		return errorsmod.Wrapf(types.ErrJobInactive, "job_id=%d", jobID)
	}
	if !equalAddress(job.Creator, caller.Bytes()) {
		return errorsmod.Wrapf(
			types.ErrUnauthorized,
			"caller %s is not the creator of job %d",
			caller.Hex(), jobID,
		)
	}

	// Refund the recorded deposit BEFORE deleting the job. If the bank send
	// fails (e.g. module account drained by an upstream bug), the cancel
	// aborts and the job stays pending so the operator can investigate.
	deposit := parseDeposit(job.Deposit)
	if err := k.RefundDeposit(ctx, caller, deposit); err != nil {
		return errorsmod.Wrapf(err, "failed to refund deposit for job_id=%d", jobID)
	}

	k.DeleteJob(ctx, job)

	ctx.EventManager().EmitEvent(sdk.NewEvent(
		EventTypeJobCancelled,
		sdk.NewAttribute("job_id", strconv.FormatUint(jobID, 10)),
		sdk.NewAttribute("creator", caller.Hex()),
		sdk.NewAttribute("refund", job.Deposit),
	))

	return nil
}

// Reschedule moves a Job to a new execution block. Atomically deletes the old
// due-block index entry and writes a new one. Only the creator may reschedule.
func (k Keeper) Reschedule(
	ctx sdk.Context,
	caller common.Address,
	jobID uint64,
	newExecuteAtBlock uint64,
) error {
	job, found := k.GetJob(ctx, jobID)
	if !found {
		return errorsmod.Wrapf(types.ErrJobNotFound, "job_id=%d", jobID)
	}
	if !job.Active {
		return errorsmod.Wrapf(types.ErrJobInactive, "job_id=%d", jobID)
	}
	if !equalAddress(job.Creator, caller.Bytes()) {
		return errorsmod.Wrapf(
			types.ErrUnauthorized,
			"caller %s is not the creator of job %d",
			caller.Hex(), jobID,
		)
	}

	currentHeight := uint64(ctx.BlockHeight())
	if newExecuteAtBlock <= currentHeight {
		return errorsmod.Wrapf(
			types.ErrPastBlock,
			"newExecuteAtBlock %d <= current height %d",
			newExecuteAtBlock, currentHeight,
		)
	}

	params := k.GetParams(ctx)
	if newExecuteAtBlock-currentHeight > params.MaxScheduleHorizon {
		return errorsmod.Wrapf(
			types.ErrHorizonExceeded,
			"newExecuteAtBlock %d exceeds horizon: current=%d, max=%d",
			newExecuteAtBlock, currentHeight, params.MaxScheduleHorizon,
		)
	}

	// Atomically: drop old indexes, then re-insert with the new due-block.
	oldBlock := job.ExecuteAtBlock
	k.DeleteJob(ctx, job)
	job.ExecuteAtBlock = newExecuteAtBlock
	if err := k.SetJob(ctx, job); err != nil {
		return fmt.Errorf("failed to re-persist job after reschedule: %w", err)
	}

	ctx.EventManager().EmitEvent(sdk.NewEvent(
		EventTypeJobRescheduled,
		sdk.NewAttribute("job_id", strconv.FormatUint(jobID, 10)),
		sdk.NewAttribute("old_block", strconv.FormatUint(oldBlock, 10)),
		sdk.NewAttribute("new_block", strconv.FormatUint(newExecuteAtBlock, 10)),
	))

	return nil
}

// equalAddress is a constant-time-safe byte comparator. We don't need true
// constant-time semantics here (all addresses are public), but we want to avoid
// short-circuit length surprises with malformed creator stores.
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
