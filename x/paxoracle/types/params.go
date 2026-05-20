package types

import (
	"fmt"
)

// Default parameter values
const (
	// DefaultStalenessThreshold is the maximum number of blocks before a price submission is considered stale.
	DefaultStalenessThreshold int64 = 15

	// DefaultMinQuorum is the minimum number of non-stale validator submissions required for a valid median.
	DefaultMinQuorum uint64 = 1

	// DefaultMaxConfidence is 1e18, representing 100% confidence.
	DefaultMaxConfidence int64 = 1e18
)

// Params defines the parameters for the paxoracle module.
type Params struct {
	// StalenessThreshold is the max blocks before a submission is stale.
	StalenessThreshold int64 `json:"staleness_threshold"`
	// MinQuorum is the minimum number of validators needed for a valid median price.
	MinQuorum uint64 `json:"min_quorum"`
}

// DefaultParams returns the default paxoracle module parameters.
func DefaultParams() Params {
	return Params{
		StalenessThreshold: DefaultStalenessThreshold,
		MinQuorum:          DefaultMinQuorum,
	}
}

// Validate performs basic validation of paxoracle parameters.
func (p Params) Validate() error {
	if p.StalenessThreshold <= 0 {
		return fmt.Errorf("staleness threshold must be positive: %d", p.StalenessThreshold)
	}
	if p.MinQuorum == 0 {
		return fmt.Errorf("min quorum must be at least 1")
	}
	return nil
}
