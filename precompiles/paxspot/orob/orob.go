package orob

import (
	"embed"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	cmn "github.com/evmos/evmos/v18/precompiles/common"
)

var _ vm.PrecompiledContract = &Precompile{}

const (
	PrecompileAddress = "0x0000000000000000000000000000000000000901"

	// Gas costs reduced by upgrade v20-agent-foundations.
	// Reflect actual native execution cost on validators we control.
	// Pre-v20 values were resolveOffset=50, resolveOffsetBatch=30, toOffset=80.
	GasResolveOffset      uint64 = 5
	GasResolveOffsetBatch uint64 = 3 // per element
	GasToOffset           uint64 = 5

	ResolveOffsetMethod      = "resolveOffset"
	ResolveOffsetBatchMethod = "resolveOffsetBatch"
	ToOffsetMethod           = "toOffset"
)

var (
	bpsDivisor = big.NewInt(10000)
)

//go:embed abi.json
var f embed.FS

type Precompile struct {
	abi.ABI
}

func NewPrecompile() (*Precompile, error) {
	newABI, err := cmn.LoadABI(f, "abi.json")
	if err != nil {
		return nil, err
	}

	return &Precompile{
		ABI: newABI,
	}, nil
}

func (Precompile) Address() common.Address {
	return common.HexToAddress(PrecompileAddress)
}

func (p Precompile) RequiredGas(input []byte) uint64 {
	if len(input) < 4 {
		return 0
	}

	method, err := p.MethodById(input[:4])
	if err != nil {
		return 0
	}

	switch method.Name {
	case ResolveOffsetMethod:
		return GasResolveOffset
	case ResolveOffsetBatchMethod:
		// Estimate gas from input length: each int16 is 32 bytes ABI-encoded
		// base overhead + per-element cost
		args, err := method.Inputs.Unpack(input[4:])
		if err != nil {
			return GasResolveOffsetBatch * 10 // fallback estimate
		}
		offsets, ok := args[1].([]int16)
		if !ok {
			return GasResolveOffsetBatch * 10
		}
		return GasResolveOffsetBatch * uint64(len(offsets))
	case ToOffsetMethod:
		return GasToOffset
	default:
		return 0
	}
}

func (p Precompile) Run(_ *vm.EVM, contract *vm.Contract, _ bool) (bz []byte, err error) {
	if len(contract.Input) < 4 {
		return nil, fmt.Errorf("input too short")
	}

	method, err := p.MethodById(contract.Input[:4])
	if err != nil {
		return nil, err
	}

	args, err := method.Inputs.Unpack(contract.Input[4:])
	if err != nil {
		return nil, err
	}

	switch method.Name {
	case ResolveOffsetMethod:
		bz, err = p.resolveOffset(method, args)
	case ResolveOffsetBatchMethod:
		bz, err = p.resolveOffsetBatch(method, args)
	case ToOffsetMethod:
		bz, err = p.toOffset(method, args)
	default:
		return nil, fmt.Errorf("unknown method: %s", method.Name)
	}

	if err != nil {
		return nil, err
	}

	return bz, nil
}

// resolveOffset computes: absolutePrice = oraclePrice * (10000 + offsetBps) / 10000
func (p Precompile) resolveOffset(method *abi.Method, args []interface{}) ([]byte, error) {
	oraclePrice, ok := args[0].(*big.Int)
	if !ok {
		return nil, fmt.Errorf("invalid oraclePrice type")
	}

	offsetBps, ok := args[1].(int16)
	if !ok {
		return nil, fmt.Errorf("invalid offsetBps type")
	}

	result := computeAbsolutePrice(oraclePrice, offsetBps)
	return method.Outputs.Pack(result)
}

// resolveOffsetBatch resolves multiple offsets against a single oracle price.
func (p Precompile) resolveOffsetBatch(method *abi.Method, args []interface{}) ([]byte, error) {
	oraclePrice, ok := args[0].(*big.Int)
	if !ok {
		return nil, fmt.Errorf("invalid oraclePrice type")
	}

	offsets, ok := args[1].([]int16)
	if !ok {
		return nil, fmt.Errorf("invalid offsetsBps type")
	}

	results := make([]*big.Int, len(offsets))
	for i, offset := range offsets {
		results[i] = computeAbsolutePrice(oraclePrice, offset)
	}

	return method.Outputs.Pack(results)
}

// toOffset computes: offsetBps = ((absolutePrice - oraclePrice) * 10000) / oraclePrice
func (p Precompile) toOffset(method *abi.Method, args []interface{}) ([]byte, error) {
	oraclePrice, ok := args[0].(*big.Int)
	if !ok {
		return nil, fmt.Errorf("invalid oraclePrice type")
	}

	absolutePrice, ok := args[1].(*big.Int)
	if !ok {
		return nil, fmt.Errorf("invalid absolutePrice type")
	}

	if oraclePrice.Sign() == 0 {
		return nil, fmt.Errorf("oraclePrice cannot be zero")
	}

	// offsetBps = ((absolutePrice - oraclePrice) * 10000) / oraclePrice
	diff := new(big.Int).Sub(absolutePrice, oraclePrice)
	diff.Mul(diff, bpsDivisor)
	diff.Div(diff, oraclePrice)

	return method.Outputs.Pack(int16(diff.Int64()))
}

// computeAbsolutePrice: oraclePrice * (10000 + offsetBps) / 10000
func computeAbsolutePrice(oraclePrice *big.Int, offsetBps int16) *big.Int {
	multiplier := new(big.Int).SetInt64(10000 + int64(offsetBps))
	result := new(big.Int).Mul(oraclePrice, multiplier)
	result.Div(result, bpsDivisor)
	return result
}
