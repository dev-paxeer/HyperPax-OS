package pofq

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
	PrecompileAddress = "0x0000000000000000000000000000000000000904"

	// Gas costs reduced by upgrade v20-agent-foundations.
	// Pre-v20 values were scoreFill=50, scoreBatchPerFill=40, updateRollingScore=80.
	GasScoreFill          uint64 = 5
	GasScoreBatchPerFill  uint64 = 3
	GasUpdateRollingScore uint64 = 8

	ScoreFillMethod          = "scoreFill"
	ScoreBatchMethod         = "scoreBatch"
	UpdateRollingScoreMethod = "updateRollingScore"
)

var (
	oneE18     = new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil) // 1e18
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
	case ScoreFillMethod:
		return GasScoreFill
	case ScoreBatchMethod:
		args, err := method.Inputs.Unpack(input[4:])
		if err != nil {
			return GasScoreBatchPerFill * 10
		}
		fillPrices, ok := args[0].([]*big.Int)
		if !ok {
			return GasScoreBatchPerFill * 10
		}
		return GasScoreBatchPerFill * uint64(len(fillPrices))
	case UpdateRollingScoreMethod:
		return GasUpdateRollingScore
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
	case ScoreFillMethod:
		bz, err = p.scoreFill(method, args)
	case ScoreBatchMethod:
		bz, err = p.scoreBatch(method, args)
	case UpdateRollingScoreMethod:
		bz, err = p.updateRollingScore(method, args)
	default:
		return nil, fmt.Errorf("unknown method: %s", method.Name)
	}

	if err != nil {
		return nil, err
	}

	return bz, nil
}

// scoreFill computes: score = 1e18 - (|fillPrice - oraclePrice| * 1e18 / oraclePrice)
// Result is clamped to [0, 1e18]. 1e18 = perfect fill, 0 = worst.
func (p Precompile) scoreFill(method *abi.Method, args []interface{}) ([]byte, error) {
	fillPrice, ok := args[0].(*big.Int)
	if !ok {
		return nil, fmt.Errorf("invalid fillPrice type")
	}

	oraclePrice, ok := args[1].(*big.Int)
	if !ok {
		return nil, fmt.Errorf("invalid oraclePrice type")
	}

	score := computeFillScore(fillPrice, oraclePrice)
	return method.Outputs.Pack(score)
}

// scoreBatch computes volume-weighted average fill quality for a batch.
func (p Precompile) scoreBatch(method *abi.Method, args []interface{}) ([]byte, error) {
	fillPrices, ok := args[0].([]*big.Int)
	if !ok {
		return nil, fmt.Errorf("invalid fillPrices type")
	}

	oraclePrices, ok := args[1].([]*big.Int)
	if !ok {
		return nil, fmt.Errorf("invalid oraclePrices type")
	}

	sizes, ok := args[2].([]*big.Int)
	if !ok {
		return nil, fmt.Errorf("invalid sizes type")
	}

	if len(fillPrices) != len(oraclePrices) || len(fillPrices) != len(sizes) {
		return nil, fmt.Errorf("array length mismatch: fills=%d oracles=%d sizes=%d",
			len(fillPrices), len(oraclePrices), len(sizes))
	}

	if len(fillPrices) == 0 {
		return method.Outputs.Pack(big.NewInt(0), big.NewInt(0))
	}

	weightedSum := new(big.Int)
	totalVolume := new(big.Int)

	for i := range fillPrices {
		score := computeFillScore(fillPrices[i], oraclePrices[i])
		weighted := new(big.Int).Mul(score, sizes[i])
		weightedSum.Add(weightedSum, weighted)
		totalVolume.Add(totalVolume, sizes[i])
	}

	if totalVolume.Sign() == 0 {
		return method.Outputs.Pack(big.NewInt(0), big.NewInt(0))
	}

	avgScore := new(big.Int).Div(weightedSum, totalVolume)
	return method.Outputs.Pack(avgScore, totalVolume)
}

// updateRollingScore computes an exponentially-decayed rolling average:
//
//	updatedScore = (currentScore * currentWeight * (10000 - decayBps) + newScore * newWeight * 10000)
//	             / (currentWeight * (10000 - decayBps) + newWeight * 10000)
func (p Precompile) updateRollingScore(method *abi.Method, args []interface{}) ([]byte, error) {
	currentScore, ok := args[0].(*big.Int)
	if !ok {
		return nil, fmt.Errorf("invalid currentScore type")
	}

	currentWeight, ok := args[1].(*big.Int)
	if !ok {
		return nil, fmt.Errorf("invalid currentWeight type")
	}

	newScore, ok := args[2].(*big.Int)
	if !ok {
		return nil, fmt.Errorf("invalid newScore type")
	}

	newWeight, ok := args[3].(*big.Int)
	if !ok {
		return nil, fmt.Errorf("invalid newWeight type")
	}

	decayBps, ok := args[4].(uint16)
	if !ok {
		return nil, fmt.Errorf("invalid decayBps type")
	}

	if decayBps >= 10000 {
		return nil, fmt.Errorf("decayBps must be < 10000")
	}

	decayFactor := new(big.Int).SetInt64(10000 - int64(decayBps))
	fullFactor := big.NewInt(10000)

	// Decayed old contribution
	oldWeighted := new(big.Int).Mul(currentScore, currentWeight)
	oldWeighted.Mul(oldWeighted, decayFactor)

	// New contribution
	newWeighted := new(big.Int).Mul(newScore, newWeight)
	newWeighted.Mul(newWeighted, fullFactor)

	// Total weight
	oldW := new(big.Int).Mul(currentWeight, decayFactor)
	newW := new(big.Int).Mul(newWeight, fullFactor)
	totalWeight := new(big.Int).Add(oldW, newW)

	if totalWeight.Sign() == 0 {
		return method.Outputs.Pack(big.NewInt(0), big.NewInt(0))
	}

	// Weighted average
	numerator := new(big.Int).Add(oldWeighted, newWeighted)
	updatedScore := new(big.Int).Div(numerator, totalWeight)

	return method.Outputs.Pack(updatedScore, totalWeight)
}

// computeFillScore: score = 1e18 - (|fillPrice - oraclePrice| * 1e18 / oraclePrice)
// Clamped to [0, 1e18].
func computeFillScore(fillPrice, oraclePrice *big.Int) *big.Int {
	if oraclePrice.Sign() == 0 {
		return new(big.Int)
	}

	diff := new(big.Int).Sub(fillPrice, oraclePrice)
	diff.Abs(diff)

	deviation := new(big.Int).Mul(diff, oneE18)
	deviation.Div(deviation, new(big.Int).Abs(new(big.Int).Set(oraclePrice)))

	score := new(big.Int).Sub(new(big.Int).Set(oneE18), deviation)
	if score.Sign() < 0 {
		score.SetInt64(0)
	}

	return score
}
