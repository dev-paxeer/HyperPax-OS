// Copyright PaxLabs Ltd.(Paxeer Network)
// Paxeer Network Non-Commercial License 1.0 (ENCL-1.0)(https://github.com/Paxeer-Network/hyperpaxeer-os/blob/main/LICENSE_FAQ.md)

package streams

import (
	"embed"
	"fmt"

	storetypes "github.com/cosmos/cosmos-sdk/store/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"

	cmn "github.com/evmos/evmos/v18/precompiles/common"
	streamskeeper "github.com/evmos/evmos/v18/x/streams/keeper"
)

var _ vm.PrecompiledContract = &Precompile{}

const PrecompileAddress = "0x0000000000000000000000000000000000000906"

const (
	OpenMethod       = "open"
	SettleMethod     = "settle"
	CloseMethod      = "close"
	UpdateRateMethod = "updateRate"
	AccruedMethod    = "accrued"
	GetStreamMethod  = "getStream"
)

//go:embed abi.json
var f embed.FS

// Precompile implements the PaymentStreams precompile (0x0906). State lives
// in `x/streams`; this struct is just the EVM-facing dispatcher that decodes
// arguments, charges gas, and forwards to the keeper. Method bodies are in
// streams_handlers.go.
type Precompile struct {
	cmn.Precompile
	streamsKeeper streamskeeper.Keeper
}

// NewPrecompile constructs the PaymentStreams precompile.
func NewPrecompile(streamsKeeper streamskeeper.Keeper) (*Precompile, error) {
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
		streamsKeeper: streamsKeeper,
	}, nil
}

func (Precompile) Address() common.Address {
	return common.HexToAddress(PrecompileAddress)
}

func (Precompile) IsTransaction(method string) bool {
	switch method {
	case OpenMethod, SettleMethod, CloseMethod, UpdateRateMethod:
		return true
	default:
		return false
	}
}

// RequiredGas returns flat gas cost per method. Per AGENTS.md §3.4 these become
// gov-tunable via `x/streams.Params.Gas*` once we have ctx access in Run.
func (p Precompile) RequiredGas(input []byte) uint64 {
	if len(input) < 4 {
		return 0
	}
	method, err := p.MethodById(input[:4])
	if err != nil {
		return 0
	}

	switch method.Name {
	case OpenMethod:
		// Plus the underlying ERC-20 transferFrom cost — accounted on the
		// child call frame, not here.
		return 8_000
	case SettleMethod:
		return 4_000
	case CloseMethod:
		return 5_000
	case UpdateRateMethod:
		return 4_000
	case AccruedMethod, GetStreamMethod:
		return 200
	default:
		return 0
	}
}

// Run is the EVM entrypoint. Decodes calldata, dispatches to keeper methods,
// ABI-encodes the response. Cosmos events emitted by the keeper carry the
// authoritative state-change record.
func (p Precompile) Run(evm *vm.EVM, contract *vm.Contract, readOnly bool) (bz []byte, err error) {
	ctx, stateDB, method, initialGas, args, err := p.RunSetup(evm, contract, readOnly, p.IsTransaction)
	if err != nil {
		return nil, err
	}
	defer cmn.HandleGasError(ctx, contract, initialGas, &err)()

	switch method.Name {
	case OpenMethod:
		bz, err = p.handleOpen(ctx, contract, stateDB, method, args)
	case SettleMethod:
		bz, err = p.handleSettle(ctx, method, args)
	case CloseMethod:
		bz, err = p.handleClose(ctx, contract, method, args)
	case UpdateRateMethod:
		bz, err = p.handleUpdateRate(ctx, contract, method, args)
	case AccruedMethod:
		bz, err = p.handleAccrued(ctx, method, args)
	case GetStreamMethod:
		bz, err = p.handleGetStream(ctx, method, args)
	default:
		return nil, fmt.Errorf("unknown method: %s", method.Name)
	}
	if err != nil {
		return nil, err
	}
	return bz, nil
}
