// Copyright PaxLabs Ltd.(Paxeer Network)
// Paxeer Network Non-Commercial License 1.0 (ENCL-1.0)(https://github.com/Paxeer-Network/hyperpaxeer-os/blob/main/LICENSE_FAQ.md)

package scheduler

import (
	"embed"
	"fmt"
	"math/big"

	storetypes "github.com/cosmos/cosmos-sdk/store/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"

	cmn "github.com/evmos/evmos/v18/precompiles/common"
	schedulerkeeper "github.com/evmos/evmos/v18/x/scheduler/keeper"
)

// Compile-time interface assertion.
var _ vm.PrecompiledContract = &Precompile{}

// PrecompileAddress is the EVM address that maps to this precompile.
const PrecompileAddress = "0x0000000000000000000000000000000000000905"

// ABI method names. Keep in sync with abi.json AND
// contracts/paxspot/src/interfaces/precompiles/IScheduler.sol.
const (
	ScheduleMethod   = "schedule"
	CancelMethod     = "cancel"
	RescheduleMethod = "reschedule"
	GetJobMethod     = "getJob"
	PendingMethod    = "pending"
)

//go:embed abi.json
var f embed.FS

// Precompile implements the Scheduler precompile (0x0905). State lives in
// `x/scheduler`; this struct is just the EVM-facing entrypoint that decodes
// arguments, charges gas, and dispatches to the keeper.
//
// Mirrors the shape of `precompiles/paxspot/oracle/oracle.go` (the canonical
// stateful + keeper-backed precompile template).
type Precompile struct {
	cmn.Precompile
	schedulerKeeper schedulerkeeper.Keeper
}

// abiJob is the wire representation of a scheduled Job, packed/unpacked through
// the go-ethereum ABI encoder. Field names + types must match the IScheduler.sol
// `Job` struct exactly.
type abiJob struct {
	Id             *big.Int       `abi:"id"`
	Creator        common.Address `abi:"creator"`
	Target         common.Address `abi:"target"`
	CallData       []byte         `abi:"callData"`
	ExecuteAtBlock uint64         `abi:"executeAtBlock"`
	GasLimit       uint64         `abi:"gasLimit"`
	Deposit        *big.Int       `abi:"deposit"`
	Active         bool           `abi:"active"`
}

// NewPrecompile constructs the Scheduler precompile. Call this from
// `x/evm/keeper/precompiles.go::AvailablePrecompiles`.
func NewPrecompile(schedulerKeeper schedulerkeeper.Keeper) (*Precompile, error) {
	newABI, err := cmn.LoadABI(f, "abi.json")
	if err != nil {
		return nil, err
	}

	return &Precompile{
		Precompile: cmn.Precompile{
			ABI:                  newABI,
			KvGasConfig:          storetypes.KVGasConfig(),
			TransientKVGasConfig: storetypes.TransientGasConfig(),
		},
		schedulerKeeper: schedulerKeeper,
	}, nil
}

// Address returns the precompile's EVM address.
func (Precompile) Address() common.Address {
	return common.HexToAddress(PrecompileAddress)
}

// IsTransaction returns true for state-mutating methods. Used by RunSetup to
// decide whether to reject the call when invoked under STATICCALL.
func (Precompile) IsTransaction(method string) bool {
	switch method {
	case ScheduleMethod, CancelMethod, RescheduleMethod:
		return true
	default:
		return false
	}
}

// RequiredGas returns the precompile's flat gas cost for the requested method.
// Default values match the spec in Paxeer_Chain_Upgrades.md §2.4. The values
// are baked into x/scheduler module Params (gov-tunable per AGENTS.md §3.4)
// but RequiredGas runs before ctx is available, so we use a static estimate
// here and let Run() refine via the SDK gas meter.
func (p Precompile) RequiredGas(input []byte) uint64 {
	if len(input) < 4 {
		return 0
	}
	method, err := p.MethodById(input[:4])
	if err != nil {
		return 0
	}

	switch method.Name {
	case ScheduleMethod:
		// 5_000 base + 50/byte over the full input (selector + ABI-encoded args).
		// Conservative upper bound; Run() does not under-charge.
		return 5_000 + 50*uint64(len(input))
	case CancelMethod:
		return 3_000
	case RescheduleMethod:
		return 4_000
	case GetJobMethod, PendingMethod:
		return 200
	default:
		return 0
	}
}

// Run is the EVM entrypoint. Decodes calldata, dispatches to keeper methods,
// ABI-encodes the response. Cosmos events emitted by the keeper carry the
// authoritative state-change record; for Solidity-event parity (see
// IScheduler.sol JobScheduled / JobCancelled / JobRescheduled) we additionally
// emit ethtypes.Log entries via the StateDB returned from RunSetup.
func (p Precompile) Run(evm *vm.EVM, contract *vm.Contract, readOnly bool) (bz []byte, err error) {
	ctx, stateDB, method, initialGas, args, err := p.RunSetup(evm, contract, readOnly, p.IsTransaction)
	if err != nil {
		return nil, err
	}
	defer cmn.HandleGasError(ctx, contract, initialGas, &err)()

	switch method.Name {
	case ScheduleMethod:
		bz, err = p.handleSchedule(ctx, evm, contract, stateDB, method, args)
	case CancelMethod:
		bz, err = p.handleCancel(ctx, contract, method, args)
	case RescheduleMethod:
		bz, err = p.handleReschedule(ctx, contract, method, args)
	case GetJobMethod:
		bz, err = p.handleGetJob(ctx, method, args)
	case PendingMethod:
		bz, err = p.handlePending(ctx, method, args)
	default:
		return nil, fmt.Errorf("unknown method: %s", method.Name)
	}
	if err != nil {
		return nil, err
	}
	return bz, nil
}

// ─── Handlers ────────────────────────────────────────────────────────────────

// handleSchedule registers a new Job. `contract.Value()` (msg.value) funds the
// deposit. Escrow flow (see x/scheduler/keeper/escrow.go for the bank-side):
//
//  1. The EVM CALL with value V already credited the precompile's stateDB
//     balance with V (and debited the caller). To redirect V to the scheduler
//     module account we cancel the precompile-side credit via stateDB.SubBalance,
//     then call keeper.EscrowDeposit which performs a real bank-level
//     SendCoinsFromAccountToModule from the caller into the scheduler module.
//  2. After commit, SetBalance reconciles: caller's bank balance now matches
//     the EVM journal (both reduced by V), and the precompile address holds 0
//     (stateDB credit reversed).
//  3. If keeper.Schedule rejects the job, the EVM frame reverts, which rolls
//     back the bank transfer — the deposit returns to the caller automatically.
func (p Precompile) handleSchedule(
	ctx sdk.Context,
	_ *vm.EVM,
	contract *vm.Contract,
	stateDB vm.StateDB,
	method *abi.Method,
	args []interface{},
) ([]byte, error) {
	decoded, err := decodeScheduleArgs(args)
	if err != nil {
		return nil, err
	}

	// `contract.Value()` is set by the EVM when a payable call carries
	// msg.value > 0. Treat nil as zero — the keeper will reject below if a
	// non-zero minimum deposit is required by params + base fee.
	deposit := contract.Value()
	if deposit == nil {
		deposit = new(big.Int)
	}

	caller := contract.CallerAddress

	// Escrow the deposit BEFORE registering the job. Doing it in this order
	// means a job-validation failure reverts the bank transfer via the EVM
	// frame revert, returning funds to the caller.
	if deposit.Sign() > 0 {
		// Cancel the implicit msg.value credit to the precompile address so
		// SetBalance(precompile, 0) is a no-op at commit time. Without this,
		// the EVM journal would think the precompile holds V at commit, and
		// SetBalance would mint V into a non-module address — silent value
		// inflation.
		stateDB.SubBalance(p.Address(), deposit)
		if err := p.schedulerKeeper.EscrowDeposit(ctx, caller, deposit); err != nil {
			return nil, fmt.Errorf("escrow deposit failed: %w", err)
		}
	}

	jobID, err := p.schedulerKeeper.Schedule(
		ctx,
		caller,
		decoded.Target,
		decoded.CallData,
		decoded.ExecuteAtBlock,
		decoded.GasLimit,
		deposit,
	)
	if err != nil {
		return nil, err
	}

	return method.Outputs.Pack(new(big.Int).SetUint64(jobID))
}

// scheduleArgs is the decoded argument set for the `schedule` method.
type scheduleArgs struct {
	Target         common.Address
	CallData       []byte
	ExecuteAtBlock uint64
	GasLimit       uint64
}

// decodeScheduleArgs reads the four schedule() arguments from the unpacked ABI
// args slice. Returns a typed struct with descriptive errors per arg.
func decodeScheduleArgs(args []interface{}) (scheduleArgs, error) {
	if len(args) != 4 {
		return scheduleArgs{}, fmt.Errorf("schedule: expected 4 args, got %d", len(args))
	}
	target, ok := args[0].(common.Address)
	if !ok {
		return scheduleArgs{}, fmt.Errorf("schedule: invalid target type: %T", args[0])
	}
	callData, ok := args[1].([]byte)
	if !ok {
		return scheduleArgs{}, fmt.Errorf("schedule: invalid callData type: %T", args[1])
	}
	executeAtBlock, ok := args[2].(uint64)
	if !ok {
		return scheduleArgs{}, fmt.Errorf("schedule: invalid executeAtBlock type: %T", args[2])
	}
	gasLimit, ok := args[3].(uint64)
	if !ok {
		return scheduleArgs{}, fmt.Errorf("schedule: invalid gasLimit type: %T", args[3])
	}
	return scheduleArgs{
		Target:         target,
		CallData:       callData,
		ExecuteAtBlock: executeAtBlock,
		GasLimit:       gasLimit,
	}, nil
}

// handleCancel decodes the jobId argument and dispatches to keeper.Cancel.
func (p Precompile) handleCancel(
	ctx sdk.Context,
	contract *vm.Contract,
	method *abi.Method,
	args []interface{},
) ([]byte, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("cancel: expected 1 arg, got %d", len(args))
	}
	jobIdBig, ok := args[0].(*big.Int)
	if !ok {
		return nil, fmt.Errorf("cancel: invalid jobId type: %T", args[0])
	}
	if !jobIdBig.IsUint64() {
		return nil, fmt.Errorf("cancel: jobId out of uint64 range")
	}
	jobID := jobIdBig.Uint64()

	caller := contract.CallerAddress
	if err := p.schedulerKeeper.Cancel(ctx, caller, jobID); err != nil {
		return nil, err
	}

	return method.Outputs.Pack()
}

// handleReschedule decodes (jobId, newBlock) and dispatches to keeper.Reschedule.
func (p Precompile) handleReschedule(
	ctx sdk.Context,
	contract *vm.Contract,
	method *abi.Method,
	args []interface{},
) ([]byte, error) {
	if len(args) != 2 {
		return nil, fmt.Errorf("reschedule: expected 2 args, got %d", len(args))
	}
	jobIdBig, ok := args[0].(*big.Int)
	if !ok {
		return nil, fmt.Errorf("reschedule: invalid jobId type: %T", args[0])
	}
	if !jobIdBig.IsUint64() {
		return nil, fmt.Errorf("reschedule: jobId out of uint64 range")
	}
	newBlock, ok := args[1].(uint64)
	if !ok {
		return nil, fmt.Errorf("reschedule: invalid newBlock type: %T", args[1])
	}

	caller := contract.CallerAddress
	if err := p.schedulerKeeper.Reschedule(ctx, caller, jobIdBig.Uint64(), newBlock); err != nil {
		return nil, err
	}

	return method.Outputs.Pack()
}

// handleGetJob looks up a single Job by id and ABI-packs it as a tuple.
func (p Precompile) handleGetJob(
	ctx sdk.Context,
	method *abi.Method,
	args []interface{},
) ([]byte, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("getJob: expected 1 arg, got %d", len(args))
	}
	jobIdBig, ok := args[0].(*big.Int)
	if !ok {
		return nil, fmt.Errorf("getJob: invalid jobId type: %T", args[0])
	}
	if !jobIdBig.IsUint64() {
		return nil, fmt.Errorf("getJob: jobId out of uint64 range")
	}

	job, found := p.schedulerKeeper.GetJob(ctx, jobIdBig.Uint64())
	if !found {
		// Return zero-valued Job rather than error so view callers can
		// distinguish "no such job" from "RPC failure".
		return method.Outputs.Pack(abiJob{
			Id:      new(big.Int),
			Deposit: new(big.Int),
		})
	}

	deposit, ok := new(big.Int).SetString(job.Deposit, 10)
	if !ok {
		deposit = new(big.Int)
	}

	return method.Outputs.Pack(abiJob{
		Id:             new(big.Int).SetUint64(job.ID),
		Creator:        common.BytesToAddress(job.Creator),
		Target:         common.BytesToAddress(job.Target),
		CallData:       job.CallData,
		ExecuteAtBlock: job.ExecuteAtBlock,
		GasLimit:       job.GasLimit,
		Deposit:        deposit,
		Active:         job.Active,
	})
}

// handlePending returns the list of pending jobIds owned by `creator`.
func (p Precompile) handlePending(
	ctx sdk.Context,
	method *abi.Method,
	args []interface{},
) ([]byte, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("pending: expected 1 arg, got %d", len(args))
	}
	creator, ok := args[0].(common.Address)
	if !ok {
		return nil, fmt.Errorf("pending: invalid creator type: %T", args[0])
	}

	idsU64 := p.schedulerKeeper.PendingJobIDsByCreator(ctx, creator.Bytes())
	ids := make([]*big.Int, len(idsU64))
	for i, id := range idsU64 {
		ids[i] = new(big.Int).SetUint64(id)
	}
	return method.Outputs.Pack(ids)
}
