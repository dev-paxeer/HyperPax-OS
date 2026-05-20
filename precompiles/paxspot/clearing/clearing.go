package clearing

import (
	"embed"
	"fmt"
	"math/big"
	"sort"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	cmn "github.com/evmos/evmos/v18/precompiles/common"
)

var _ vm.PrecompiledContract = &Precompile{}

const (
	PrecompileAddress = "0x0000000000000000000000000000000000000902"

	// Gas costs reduced by upgrade v20-agent-foundations.
	// Pre-v20 values were base=200, per-order=30.
	GasComputeClearingBase     uint64 = 50
	GasComputeClearingPerOrder uint64 = 3

	ComputeClearingMethod = "computeClearing"
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
	case ComputeClearingMethod:
		args, err := method.Inputs.Unpack(input[4:])
		if err != nil {
			return GasComputeClearingBase + GasComputeClearingPerOrder*20
		}
		buyOffsets, ok := args[1].([]int16)
		if !ok {
			return GasComputeClearingBase + GasComputeClearingPerOrder*20
		}
		sellOffsets, ok := args[3].([]int16)
		if !ok {
			return GasComputeClearingBase + GasComputeClearingPerOrder*20
		}
		totalOrders := uint64(len(buyOffsets) + len(sellOffsets))
		return GasComputeClearingBase + GasComputeClearingPerOrder*totalOrders
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
	case ComputeClearingMethod:
		bz, err = p.computeClearing(method, args)
	default:
		return nil, fmt.Errorf("unknown method: %s", method.Name)
	}

	if err != nil {
		return nil, err
	}

	return bz, nil
}

// orderEntry represents a single order at a given offset with a size.
type orderEntry struct {
	offsetBps int16
	size      *big.Int
}

// computeClearing finds the uniform clearing price for a batch auction.
//
// Algorithm:
//  1. Build cumulative demand curve: walk buys from most aggressive (highest offset)
//     to least aggressive. At each level, cumDemand = sum of all buys at this offset or higher.
//  2. Build cumulative supply curve: walk sells from cheapest (lowest offset) to most expensive.
//     At each level, cumSupply = sum of all sells at this offset or lower.
//  3. Clearing offset = the offset where cumDemand and cumSupply cross.
//     Matched volume = min(cumDemand, cumSupply) at that crossing point.
func (p Precompile) computeClearing(method *abi.Method, args []interface{}) ([]byte, error) {
	oraclePrice, ok := args[0].(*big.Int)
	if !ok {
		return nil, fmt.Errorf("invalid oraclePrice type")
	}

	buyOffsets, ok := args[1].([]int16)
	if !ok {
		return nil, fmt.Errorf("invalid buyOffsets type")
	}

	buySizes, ok := args[2].([]*big.Int)
	if !ok {
		return nil, fmt.Errorf("invalid buySizes type")
	}

	sellOffsets, ok := args[3].([]int16)
	if !ok {
		return nil, fmt.Errorf("invalid sellOffsets type")
	}

	sellSizes, ok := args[4].([]*big.Int)
	if !ok {
		return nil, fmt.Errorf("invalid sellSizes type")
	}

	if len(buyOffsets) != len(buySizes) {
		return nil, fmt.Errorf("buy arrays length mismatch")
	}

	if len(sellOffsets) != len(sellSizes) {
		return nil, fmt.Errorf("sell arrays length mismatch")
	}

	// Handle empty sides
	if len(buyOffsets) == 0 || len(sellOffsets) == 0 {
		return method.Outputs.Pack(int16(0), oraclePrice, big.NewInt(0))
	}

	// Build buy orders sorted by offset descending (most aggressive first)
	buys := make([]orderEntry, len(buyOffsets))
	for i := range buyOffsets {
		buys[i] = orderEntry{offsetBps: buyOffsets[i], size: buySizes[i]}
	}
	sort.Slice(buys, func(i, j int) bool {
		return buys[i].offsetBps > buys[j].offsetBps
	})

	// Build sell orders sorted by offset ascending (cheapest first)
	sells := make([]orderEntry, len(sellOffsets))
	for i := range sellOffsets {
		sells[i] = orderEntry{offsetBps: sellOffsets[i], size: sellSizes[i]}
	}
	sort.Slice(sells, func(i, j int) bool {
		return sells[i].offsetBps < sells[j].offsetBps
	})

	// Check if any crossing is possible: best buy must be >= cheapest sell
	if buys[0].offsetBps < sells[0].offsetBps {
		// No crossing — no trades possible
		return method.Outputs.Pack(int16(0), oraclePrice, big.NewInt(0))
	}

	// Collect all unique offset levels from both sides, sorted ascending
	levelSet := make(map[int16]bool)
	for _, b := range buys {
		levelSet[b.offsetBps] = true
	}
	for _, s := range sells {
		levelSet[s.offsetBps] = true
	}

	levels := make([]int16, 0, len(levelSet))
	for l := range levelSet {
		levels = append(levels, l)
	}
	sort.Slice(levels, func(i, j int) bool {
		return levels[i] < levels[j]
	})

	// At each price level, compute cumulative demand (buys willing to pay >= level)
	// and cumulative supply (sells willing to accept <= level).
	// Find the level where supply first meets or exceeds demand.
	var clearingOffset int16
	matchedVolume := new(big.Int)

	for _, level := range levels {
		// Cumulative demand at this level: sum of all buy sizes with offset >= level
		cumDemand := new(big.Int)
		for _, b := range buys {
			if b.offsetBps >= level {
				cumDemand.Add(cumDemand, b.size)
			}
		}

		// Cumulative supply at this level: sum of all sell sizes with offset <= level
		cumSupply := new(big.Int)
		for _, s := range sells {
			if s.offsetBps <= level {
				cumSupply.Add(cumSupply, s.size)
			}
		}

		// The clearing point is where max(min(demand, supply)) occurs
		vol := minBigInt(cumDemand, cumSupply)
		if vol.Cmp(matchedVolume) > 0 {
			matchedVolume.Set(vol)
			clearingOffset = level
		}
	}

	// Compute absolute clearing price from offset
	multiplier := new(big.Int).SetInt64(10000 + int64(clearingOffset))
	clearingPrice := new(big.Int).Mul(oraclePrice, multiplier)
	clearingPrice.Div(clearingPrice, bpsDivisor)

	return method.Outputs.Pack(clearingOffset, clearingPrice, matchedVolume)
}

func minBigInt(a, b *big.Int) *big.Int {
	if a.Cmp(b) < 0 {
		return new(big.Int).Set(a)
	}
	return new(big.Int).Set(b)
}
