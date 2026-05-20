package keeper

import (
	"encoding/json"
	"math/big"
	"sort"

	"github.com/cosmos/cosmos-sdk/store/prefix"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/evmos/evmos/v18/x/paxoracle/types"
)

// priceSubmissionJSON is an internal serialization struct for KV store persistence.
type priceSubmissionJSON struct {
	ValidatorAddr string `json:"v"`
	Price         string `json:"p"`
	Confidence    string `json:"c"`
	BlockHeight   int64  `json:"h"`
	Timestamp     int64  `json:"t"`
}

// SetPriceSubmission stores a validator's price attestation for a market.
func (k Keeper) SetPriceSubmission(ctx sdk.Context, sub types.PriceSubmission) {
	store := ctx.KVStore(k.storeKey)
	valAddr := []byte(sub.ValidatorAddr)
	key := types.PriceSubmissionKey(sub.MarketId, valAddr)

	data := priceSubmissionJSON{
		ValidatorAddr: sub.ValidatorAddr,
		Price:         sub.Price.String(),
		Confidence:    sub.Confidence.String(),
		BlockHeight:   sub.BlockHeight,
		Timestamp:     sub.Timestamp,
	}

	bz, err := json.Marshal(data)
	if err != nil {
		panic(err)
	}
	store.Set(key, bz)
}

// GetPriceSubmission retrieves a single validator's price submission for a market.
func (k Keeper) GetPriceSubmission(ctx sdk.Context, marketId [32]byte, valAddr string) (types.PriceSubmission, bool) {
	store := ctx.KVStore(k.storeKey)
	key := types.PriceSubmissionKey(marketId, []byte(valAddr))
	bz := store.Get(key)
	if bz == nil {
		return types.PriceSubmission{}, false
	}

	var data priceSubmissionJSON
	if err := json.Unmarshal(bz, &data); err != nil {
		return types.PriceSubmission{}, false
	}

	price, ok := new(big.Int).SetString(data.Price, 10)
	if !ok {
		return types.PriceSubmission{}, false
	}
	confidence, ok := new(big.Int).SetString(data.Confidence, 10)
	if !ok {
		return types.PriceSubmission{}, false
	}

	return types.PriceSubmission{
		ValidatorAddr: data.ValidatorAddr,
		MarketId:      marketId,
		Price:         price,
		Confidence:    confidence,
		BlockHeight:   data.BlockHeight,
		Timestamp:     data.Timestamp,
	}, true
}

// GetAllSubmissionsForMarket returns all price submissions for a given market.
func (k Keeper) GetAllSubmissionsForMarket(ctx sdk.Context, marketId [32]byte) []types.PriceSubmission {
	store := ctx.KVStore(k.storeKey)
	prefixStore := prefix.NewStore(store, types.PriceSubmissionsByMarketPrefix(marketId))

	var submissions []types.PriceSubmission
	iter := prefixStore.Iterator(nil, nil)
	defer iter.Close()

	for ; iter.Valid(); iter.Next() {
		var data priceSubmissionJSON
		if err := json.Unmarshal(iter.Value(), &data); err != nil {
			continue
		}

		price, ok := new(big.Int).SetString(data.Price, 10)
		if !ok {
			continue
		}
		confidence, ok := new(big.Int).SetString(data.Confidence, 10)
		if !ok {
			continue
		}

		submissions = append(submissions, types.PriceSubmission{
			ValidatorAddr: data.ValidatorAddr,
			MarketId:      marketId,
			Price:         price,
			Confidence:    confidence,
			BlockHeight:   data.BlockHeight,
			Timestamp:     data.Timestamp,
		})
	}

	return submissions
}

// GetAllSubmissions returns every price submission in the store (used for genesis export).
func (k Keeper) GetAllSubmissions(ctx sdk.Context) []types.PriceSubmission {
	store := ctx.KVStore(k.storeKey)
	prefixStore := prefix.NewStore(store, types.PriceSubmissionKeyPrefix)

	var submissions []types.PriceSubmission
	iter := prefixStore.Iterator(nil, nil)
	defer iter.Close()

	for ; iter.Valid(); iter.Next() {
		var data priceSubmissionJSON
		if err := json.Unmarshal(iter.Value(), &data); err != nil {
			continue
		}

		price, ok := new(big.Int).SetString(data.Price, 10)
		if !ok {
			continue
		}
		confidence, ok := new(big.Int).SetString(data.Confidence, 10)
		if !ok {
			continue
		}

		// Extract marketId from key: the iterator key is relative to the prefix,
		// so it's marketId (32 bytes) + valAddr.
		keyBytes := iter.Key()
		if len(keyBytes) < 32 {
			continue
		}
		var marketId [32]byte
		copy(marketId[:], keyBytes[:32])

		submissions = append(submissions, types.PriceSubmission{
			ValidatorAddr: data.ValidatorAddr,
			MarketId:      marketId,
			Price:         price,
			Confidence:    confidence,
			BlockHeight:   data.BlockHeight,
			Timestamp:     data.Timestamp,
		})
	}

	return submissions
}

// GetMedianPrice computes the confidence-weighted median price from non-stale validator submissions.
// Returns (price, quorum, oldestTimestamp, error).
// This is the method the oracle precompile calls.
func (k Keeper) GetMedianPrice(ctx sdk.Context, marketId [32]byte) (*big.Int, *big.Int, *big.Int, error) {
	params := k.GetParams(ctx)
	currentHeight := ctx.BlockHeight()
	stalenessThreshold := params.StalenessThreshold

	submissions := k.GetAllSubmissionsForMarket(ctx, marketId)

	// Filter out stale submissions
	var fresh []types.PriceSubmission
	for _, sub := range submissions {
		if currentHeight-sub.BlockHeight <= stalenessThreshold {
			fresh = append(fresh, sub)
		}
	}

	if len(fresh) == 0 {
		return nil, nil, nil, types.ErrStaleSubmissions.Wrapf(
			"no non-stale submissions for market %x (threshold=%d blocks)",
			marketId, stalenessThreshold,
		)
	}

	if uint64(len(fresh)) < params.MinQuorum {
		return nil, nil, nil, types.ErrInsufficientQuorum.Wrapf(
			"got %d submissions, need %d", len(fresh), params.MinQuorum,
		)
	}

	// Sort by price ascending
	sort.Slice(fresh, func(i, j int) bool {
		return fresh[i].Price.Cmp(fresh[j].Price) < 0
	})

	// Compute confidence-weighted median
	totalConfidence := new(big.Int)
	for _, sub := range fresh {
		totalConfidence.Add(totalConfidence, sub.Confidence)
	}

	halfWeight := new(big.Int).Div(totalConfidence, big.NewInt(2))
	cumWeight := new(big.Int)
	medianPrice := fresh[0].Price

	for _, sub := range fresh {
		cumWeight.Add(cumWeight, sub.Confidence)
		if cumWeight.Cmp(halfWeight) >= 0 {
			medianPrice = sub.Price
			break
		}
	}

	// Find oldest timestamp in the quorum
	oldestTimestamp := fresh[0].Timestamp
	for _, sub := range fresh[1:] {
		if sub.Timestamp < oldestTimestamp {
			oldestTimestamp = sub.Timestamp
		}
	}

	quorum := new(big.Int).SetInt64(int64(len(fresh)))
	ts := new(big.Int).SetInt64(oldestTimestamp)

	return medianPrice, quorum, ts, nil
}

// SetSupportedMarket adds a market to the supported markets set.
func (k Keeper) SetSupportedMarket(ctx sdk.Context, market types.SupportedMarket) {
	store := ctx.KVStore(k.storeKey)
	key := types.SupportedMarketKey(market.MarketId)

	bz, err := json.Marshal(market)
	if err != nil {
		panic(err)
	}
	store.Set(key, bz)
}

// IsSupportedMarket checks if a market is in the supported set.
func (k Keeper) IsSupportedMarket(ctx sdk.Context, marketId [32]byte) bool {
	store := ctx.KVStore(k.storeKey)
	return store.Has(types.SupportedMarketKey(marketId))
}

// GetAllSupportedMarkets returns all supported markets.
func (k Keeper) GetAllSupportedMarkets(ctx sdk.Context) []types.SupportedMarket {
	store := ctx.KVStore(k.storeKey)
	prefixStore := prefix.NewStore(store, types.SupportedMarketKeyPrefix)

	var markets []types.SupportedMarket
	iter := prefixStore.Iterator(nil, nil)
	defer iter.Close()

	for ; iter.Valid(); iter.Next() {
		var m types.SupportedMarket
		if err := json.Unmarshal(iter.Value(), &m); err != nil {
			continue
		}
		markets = append(markets, m)
	}

	return markets
}
