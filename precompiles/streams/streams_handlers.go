// Copyright PaxLabs Ltd.(Paxeer Network)
// Paxeer Network Non-Commercial License 1.0 (ENCL-1.0)(https://github.com/Paxeer-Network/hyperpaxeer-os/blob/main/LICENSE_FAQ.md)


package streams

import (
	"fmt"
	"math/big"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"

	"github.com/evmos/evmos/v18/x/streams/types"
)

// abiStream is the wire representation of a Stream, ABI-packed for the
// `getStream(uint256)` view method. Field names + types must match the tuple
// declared in IPaymentStreams.sol exactly.
type abiStream struct {
	Id            *big.Int       `abi:"id"`
	Payer         common.Address `abi:"payer"`
	Payee         common.Address `abi:"payee"`
	Token         common.Address `abi:"token"`
	RatePerSecond *big.Int       `abi:"ratePerSecond"`
	Cap           *big.Int       `abi:"cap"`
	StartTime     uint64         `abi:"startTime"`
	StopTime      uint64         `abi:"stopTime"`
	Settled       *big.Int       `abi:"settled"`
	Active        bool           `abi:"active"`
}

// handleOpen escrows `cap` worth of `token` from the caller into the streams
// module account and registers a new Stream.
//
// For native PAX (token == 0x0): bankKeeper handles the transfer directly via
// SendCoinsFromAccountToModule. The ABI is non-payable so msg.value is always
// zero here — payers must have a sufficient bank balance separately.
//
// For ERC-20 tokens: the payer must have called approve(streamsPrecompile,
// cap) on the token contract before invoking open(). The keeper then calls
// transferFrom(payer, moduleAddr, cap) via erc20Keeper.CallEVM.
func (p Precompile) handleOpen(
	ctx sdk.Context,
	contract *vm.Contract,
	_ vm.StateDB,
	method *abi.Method,
	args []interface{},
) ([]byte, error) {
	if len(args) != 6 {
		return nil, fmt.Errorf("open: expected 6 args, got %d", len(args))
	}
	payee, ok := args[0].(common.Address)
	if !ok {
		return nil, fmt.Errorf("open: invalid payee type: %T", args[0])
	}
	token, ok := args[1].(common.Address)
	if !ok {
		return nil, fmt.Errorf("open: invalid token type: %T", args[1])
	}
	rate, ok := args[2].(*big.Int)
	if !ok {
		return nil, fmt.Errorf("open: invalid ratePerSecond type: %T", args[2])
	}
	startTime, ok := args[3].(uint64)
	if !ok {
		return nil, fmt.Errorf("open: invalid startTime type: %T", args[3])
	}
	stopTime, ok := args[4].(uint64)
	if !ok {
		return nil, fmt.Errorf("open: invalid stopTime type: %T", args[4])
	}
	cap, ok := args[5].(*big.Int)
	if !ok {
		return nil, fmt.Errorf("open: invalid cap type: %T", args[5])
	}

	payer := contract.CallerAddress
	streamID, err := p.streamsKeeper.Open(ctx, payer, payee, token, rate, startTime, stopTime, cap)
	if err != nil {
		return nil, err
	}
	return method.Outputs.Pack(new(big.Int).SetUint64(streamID))
}

// handleSettle pulls accrued funds from the module account out to Payee.
// Anyone may call this; the destination is always the on-chain Payee.
func (p Precompile) handleSettle(
	ctx sdk.Context,
	method *abi.Method,
	args []interface{},
) ([]byte, error) {
	streamID, err := decodeStreamID("settle", args)
	if err != nil {
		return nil, err
	}
	paid, err := p.streamsKeeper.Settle(ctx, streamID)
	if err != nil {
		return nil, err
	}
	return method.Outputs.Pack(paid)
}

// handleClose finalizes a stream — settles outstanding accrual to Payee,
// refunds the unspent cap to Payer, deletes the stream. Caller must be Payer
// or Payee.
func (p Precompile) handleClose(
	ctx sdk.Context,
	contract *vm.Contract,
	method *abi.Method,
	args []interface{},
) ([]byte, error) {
	streamID, err := decodeStreamID("close", args)
	if err != nil {
		return nil, err
	}
	caller := contract.CallerAddress.Bytes()
	finalPaid, err := p.streamsKeeper.Close(ctx, streamID, caller)
	if err != nil {
		return nil, err
	}
	return method.Outputs.Pack(finalPaid)
}

// handleUpdateRate is the payer-only rate change. Implicitly settles at the
// old rate before applying the new one.
func (p Precompile) handleUpdateRate(
	ctx sdk.Context,
	contract *vm.Contract,
	method *abi.Method,
	args []interface{},
) ([]byte, error) {
	if len(args) != 2 {
		return nil, fmt.Errorf("updateRate: expected 2 args, got %d", len(args))
	}
	streamIDBig, ok := args[0].(*big.Int)
	if !ok || !streamIDBig.IsUint64() {
		return nil, fmt.Errorf("updateRate: invalid streamId type: %T", args[0])
	}
	newRate, ok := args[1].(*big.Int)
	if !ok {
		return nil, fmt.Errorf("updateRate: invalid newRate type: %T", args[1])
	}
	caller := contract.CallerAddress.Bytes()
	if err := p.streamsKeeper.UpdateRate(ctx, streamIDBig.Uint64(), caller, newRate); err != nil {
		return nil, err
	}
	return method.Outputs.Pack()
}

// handleAccrued returns the currently-accrued amount for a stream without
// settling it. Pure read.
func (p Precompile) handleAccrued(
	ctx sdk.Context,
	method *abi.Method,
	args []interface{},
) ([]byte, error) {
	streamID, err := decodeStreamID("accrued", args)
	if err != nil {
		return nil, err
	}
	s, found := p.streamsKeeper.GetStream(ctx, streamID)
	if !found {
		return method.Outputs.Pack(new(big.Int))
	}
	now := uint64(ctx.BlockTime().Unix())
	amount := p.streamsKeeper.Accrued(ctx, s, now)
	return method.Outputs.Pack(amount)
}

// handleGetStream returns the full Stream tuple. Returns the zero-tuple when
// the stream is missing, so view callers can disambiguate "missing" from
// "RPC error".
func (p Precompile) handleGetStream(
	ctx sdk.Context,
	method *abi.Method,
	args []interface{},
) ([]byte, error) {
	streamID, err := decodeStreamID("getStream", args)
	if err != nil {
		return nil, err
	}
	s, found := p.streamsKeeper.GetStream(ctx, streamID)
	if !found {
		return method.Outputs.Pack(abiStream{
			Id:            new(big.Int),
			RatePerSecond: new(big.Int),
			Cap:           new(big.Int),
			Settled:       new(big.Int),
		})
	}
	rate, _ := new(big.Int).SetString(s.RatePerSecond, 10)
	if rate == nil {
		rate = new(big.Int)
	}
	cap, _ := new(big.Int).SetString(s.Cap, 10)
	if cap == nil {
		cap = new(big.Int)
	}
	settled, _ := new(big.Int).SetString(s.Settled, 10)
	if settled == nil {
		settled = new(big.Int)
	}
	return method.Outputs.Pack(abiStream{
		Id:            new(big.Int).SetUint64(s.ID),
		Payer:         common.BytesToAddress(s.Payer),
		Payee:         common.BytesToAddress(s.Payee),
		Token:         common.BytesToAddress(s.Token),
		RatePerSecond: rate,
		Cap:           cap,
		StartTime:     s.StartTime,
		StopTime:      s.StopTime,
		Settled:       settled,
		Active:        s.Active,
	})
}

func decodeStreamID(method string, args []interface{}) (uint64, error) {
	if len(args) != 1 {
		return 0, fmt.Errorf("%s: expected 1 arg, got %d", method, len(args))
	}
	streamIDBig, ok := args[0].(*big.Int)
	if !ok {
		return 0, fmt.Errorf("%s: invalid streamId type: %T", method, args[0])
	}
	if !streamIDBig.IsUint64() {
		return 0, fmt.Errorf("%s: streamId out of uint64 range", method)
	}
	return streamIDBig.Uint64(), nil
}

// silence the unused-import linter when types is no longer referenced after
// future edits.
var _ = types.ModuleName
