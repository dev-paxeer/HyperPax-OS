package types

import (
	"fmt"
	"math/big"
)

// PriceSubmission represents a single validator's price attestation stored in state.
type PriceSubmission struct {
	// ValidatorAddr is the validator's operator address (bech32).
	ValidatorAddr string `json:"validator_addr"`
	// MarketId is the 32-byte market identifier.
	MarketId [32]byte `json:"market_id"`
	// Price is the attested price (18 decimals, positive).
	Price *big.Int `json:"price"`
	// Confidence is the confidence level (0, 1e18].
	Confidence *big.Int `json:"confidence"`
	// BlockHeight is the block at which this submission was made.
	BlockHeight int64 `json:"block_height"`
	// Timestamp is the unix timestamp of the block.
	Timestamp int64 `json:"timestamp"`
}

// SupportedMarket represents a market that the oracle module accepts prices for.
type SupportedMarket struct {
	// MarketId is the 32-byte market identifier (e.g. keccak256("ETH/USDC")).
	MarketId [32]byte `json:"market_id"`
	// Ticker is a human-readable label (e.g. "ETH/USDC").
	Ticker string `json:"ticker"`
}

// GenesisState defines the paxoracle module's genesis state.
type GenesisState struct {
	Params           Params            `json:"params"`
	SupportedMarkets []SupportedMarket `json:"supported_markets"`
	Submissions      []PriceSubmission `json:"submissions"`
}

// DefaultGenesisState returns the default genesis state.
func DefaultGenesisState() *GenesisState {
	return &GenesisState{
		Params:           DefaultParams(),
		SupportedMarkets: []SupportedMarket{},
		Submissions:      []PriceSubmission{},
	}
}

// Validate performs basic genesis state validation.
func (gs GenesisState) Validate() error {
	if err := gs.Params.Validate(); err != nil {
		return fmt.Errorf("invalid params: %w", err)
	}

	seen := make(map[[32]byte]bool)
	for _, m := range gs.SupportedMarkets {
		empty := [32]byte{}
		if m.MarketId == empty {
			return fmt.Errorf("supported market has zero market id")
		}
		if seen[m.MarketId] {
			return fmt.Errorf("duplicate supported market: %x", m.MarketId)
		}
		seen[m.MarketId] = true
	}

	return nil
}
