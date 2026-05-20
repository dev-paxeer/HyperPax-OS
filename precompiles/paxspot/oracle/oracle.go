package oracle

import (
	"embed"
	"fmt"
	"math/big"
	"sort"

	storetypes "github.com/cosmos/cosmos-sdk/store/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	cmn "github.com/evmos/evmos/v18/precompiles/common"
	paxoraclekeeper "github.com/evmos/evmos/v18/x/paxoracle/keeper"
	paxoracletypes "github.com/evmos/evmos/v18/x/paxoracle/types"
)

var _ vm.PrecompiledContract = &Precompile{}

const (
	PrecompileAddress = "0x0000000000000000000000000000000000000903"

	// Gas costs reduced by upgrade v20-agent-foundations.
	// Pre-v20 values were aggregate=100, perFeed=50, getValidatorPrice=500, submitPrice=1000.
	GasAggregate         uint64 = 20
	GasAggregatePerFeed  uint64 = 5
	GasGetValidatorPrice uint64 = 100
	GasSubmitPrice       uint64 = 300

	AggregateMethod         = "aggregate"
	GetValidatorPriceMethod = "getValidatorPrice"
	SubmitPriceMethod       = "submitPrice"
)

//go:embed abi.json
var f embed.FS

// Precompile implements the oracle aggregator precompile.
// aggregate() is stateless (pure math on input feeds).
// getValidatorPrice() is stateful — reads from x/paxoracle via keeper.
type Precompile struct {
	cmn.Precompile
	oracleKeeper paxoraclekeeper.Keeper
}

func NewPrecompile(oracleKeeper paxoraclekeeper.Keeper) (*Precompile, error) {
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
		oracleKeeper: oracleKeeper,
	}, nil
}

func (Precompile) Address() common.Address {
	return common.HexToAddress(PrecompileAddress)
}

// IsTransaction returns true if the method modifies state.
func (Precompile) IsTransaction(method string) bool {
	return method == SubmitPriceMethod
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
	case AggregateMethod:
		return GasAggregate + GasAggregatePerFeed*5
	case GetValidatorPriceMethod:
		return GasGetValidatorPrice
	case SubmitPriceMethod:
		return GasSubmitPrice
	default:
		return 0
	}
}

func (p Precompile) Run(evm *vm.EVM, contract *vm.Contract, readOnly bool) (bz []byte, err error) {
	ctx, _, method, initialGas, args, err := p.RunSetup(evm, contract, readOnly, p.IsTransaction)
	if err != nil {
		return nil, err
	}

	defer cmn.HandleGasError(ctx, contract, initialGas, &err)()

	switch method.Name {
	case AggregateMethod:
		bz, err = p.aggregate(method, args)
	case GetValidatorPriceMethod:
		bz, err = p.getValidatorPrice(ctx, method, args)
	case SubmitPriceMethod:
		bz, err = p.submitPrice(ctx, evm, contract, method, args)
	default:
		return nil, fmt.Errorf("unknown method: %s", method.Name)
	}

	if err != nil {
		return nil, err
	}

	return bz, nil
}

// getValidatorPrice reads the consensus median price from the x/paxoracle module state.
func (p Precompile) getValidatorPrice(ctx sdk.Context, method *abi.Method, args []interface{}) ([]byte, error) {
	marketId, ok := args[0].([32]byte)
	if !ok {
		return nil, fmt.Errorf("getValidatorPrice: invalid marketId type: %T", args[0])
	}

	price, quorum, timestamp, err := p.oracleKeeper.GetMedianPrice(ctx, marketId)
	if err != nil {
		return nil, fmt.Errorf("getValidatorPrice: %w", err)
	}

	return method.Outputs.Pack(price, quorum, timestamp)
}

// priceFeed represents a decoded PriceFeed struct from the ABI.
type priceFeed struct {
	Price      *big.Int
	Confidence *big.Int
	Timestamp  *big.Int
}

// aggregate computes a confidence-weighted median price from multiple feeds.
//
// Algorithm:
//  1. Filter out feeds with zero confidence
//  2. Sort feeds by price ascending
//  3. Walk through sorted feeds accumulating confidence weight
//  4. Median = price where cumulative confidence crosses 50% of total confidence
//  5. Output confidence = minimum confidence across all feeds (conservative)
func (p Precompile) aggregate(method *abi.Method, args []interface{}) ([]byte, error) {
	rawFeeds, ok := args[0].([]struct {
		Price      *big.Int `json:"price"`
		Confidence *big.Int `json:"confidence"`
		Timestamp  *big.Int `json:"timestamp"`
	})
	if !ok {
		return nil, fmt.Errorf("invalid feeds type: %T", args[0])
	}

	if len(rawFeeds) == 0 {
		return nil, fmt.Errorf("no feeds provided")
	}

	feeds := make([]priceFeed, 0, len(rawFeeds))
	for _, rf := range rawFeeds {
		if rf.Confidence.Sign() > 0 {
			feeds = append(feeds, priceFeed{
				Price:      rf.Price,
				Confidence: rf.Confidence,
				Timestamp:  rf.Timestamp,
			})
		}
	}

	if len(feeds) == 0 {
		return nil, fmt.Errorf("no feeds with non-zero confidence")
	}

	if len(feeds) == 1 {
		return method.Outputs.Pack(feeds[0].Price, feeds[0].Confidence)
	}

	sort.Slice(feeds, func(i, j int) bool {
		return feeds[i].Price.Cmp(feeds[j].Price) < 0
	})

	totalConfidence := new(big.Int)
	minConfidence := new(big.Int).Set(feeds[0].Confidence)
	for _, feed := range feeds {
		totalConfidence.Add(totalConfidence, feed.Confidence)
		if feed.Confidence.Cmp(minConfidence) < 0 {
			minConfidence.Set(feed.Confidence)
		}
	}

	halfWeight := new(big.Int).Div(totalConfidence, big.NewInt(2))
	cumWeight := new(big.Int)
	medianPrice := feeds[0].Price

	for _, feed := range feeds {
		cumWeight.Add(cumWeight, feed.Confidence)
		if cumWeight.Cmp(halfWeight) >= 0 {
			medianPrice = feed.Price
			break
		}
	}

	return method.Outputs.Pack(medianPrice, minConfidence)
}

// submitPrice allows validators to submit price attestations directly via EVM tx,
// bypassing the Cosmos SDK tx path entirely. The caller must be an active validator.
func (p Precompile) submitPrice(
	ctx sdk.Context,
	evm *vm.EVM,
	contract *vm.Contract,
	method *abi.Method,
	args []interface{},
) ([]byte, error) {
	if len(args) != 3 {
		return nil, fmt.Errorf("submitPrice: expected 3 args, got %d", len(args))
	}

	marketId, ok := args[0].([32]byte)
	if !ok {
		return nil, fmt.Errorf("submitPrice: invalid marketId type: %T", args[0])
	}

	price, ok := args[1].(*big.Int)
	if !ok {
		return nil, fmt.Errorf("submitPrice: invalid price type: %T", args[1])
	}

	confidence, ok := args[2].(*big.Int)
	if !ok {
		return nil, fmt.Errorf("submitPrice: invalid confidence type: %T", args[2])
	}

	// Convert EVM caller address to Cosmos account address
	callerEvmAddr := contract.CallerAddress
	callerAccAddr := sdk.AccAddress(callerEvmAddr.Bytes())

	// Validate caller is an active validator
	if !p.oracleKeeper.IsValidator(ctx, callerAccAddr) {
		return nil, fmt.Errorf("submitPrice: caller %s is not an active validator", callerAccAddr.String())
	}

	// Store the price submission
	sub := paxoracletypes.PriceSubmission{
		ValidatorAddr: callerAccAddr.String(),
		MarketId:      marketId,
		Price:         price,
		Confidence:    confidence,
		BlockHeight:   ctx.BlockHeight(),
		Timestamp:     ctx.BlockTime().Unix(),
	}

	p.oracleKeeper.SetPriceSubmission(ctx, sub)

	ctx.EventManager().EmitEvent(
		sdk.NewEvent(
			"paxoracle_price_submitted",
			sdk.NewAttribute("validator", callerAccAddr.String()),
			sdk.NewAttribute("market_id", fmt.Sprintf("%x", marketId)),
			sdk.NewAttribute("price", price.String()),
			sdk.NewAttribute("block_height", fmt.Sprintf("%d", ctx.BlockHeight())),
		),
	)

	return method.Outputs.Pack(true)
}
