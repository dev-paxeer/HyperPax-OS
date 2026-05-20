// Copyright PaxLabs Ltd.(Paxeer Network)
// Paxeer Network Non-Commercial License 1.0 (ENCL-1.0)(https://github.com/Paxeer-Network/hyperpaxeer-os/blob/main/LICENSE_FAQ.md)

package types

import (
	"fmt"

	"cosmossdk.io/math"
	"github.com/ethereum/go-ethereum/common"
)

// LaneParams holds the agent fee lane configuration. Stored separately from
// the protobuf-generated `Params` struct (under KeyLaneParams) so the v20
// upgrade does not require regenerating feemarket.pb.go from .proto sources.
//
// Field semantics:
//
//   - Enabled — master switch. While false, all per-tx lookups
//     short-circuit and the lane has zero observable effect on fee
//     calculation. Default is false; gov flips it true after AgentWallet
//     contracts are deployed.
//   - GasPrice — the substituted gas price (in EVM wei units) applied to
//     transactions whose signer or target is in the lane. Stored as
//     `math.Int` so values can exceed uint64 if needed.
//   - Registry — EVM address of the on-chain `ServiceRegistry` /
//     `AgentWalletFactory` contract whose `LaneRegistration` events are
//     mirrored into the in-keeper LaneMember set by the EVM hook
//     (keeper/lane_hook.go). Zero address means no registry is wired —
//     lane membership only changes via gov-driven keeper helpers.
//
// Reference: Paxeer_Chain_Upgrades_Integration_Plan.md §2.3.1.
type LaneParams struct {
	Enabled  bool     `json:"enabled"`
	GasPrice math.Int `json:"gas_price"`
	Registry [20]byte `json:"registry"`
}

// DefaultLaneParams returns the v20-activation default: lane disabled, zero
// gas price, zero registry address. Gov flips Enabled + Registry post-deploy.
func DefaultLaneParams() LaneParams {
	return LaneParams{
		Enabled:  false,
		GasPrice: math.ZeroInt(),
		Registry: [20]byte{},
	}
}

// Validate performs basic structural validation on lane parameters.
func (lp LaneParams) Validate() error {
	if lp.GasPrice.IsNil() {
		return fmt.Errorf("lane gas_price must be non-nil (use math.ZeroInt() for unset)")
	}
	if lp.GasPrice.IsNegative() {
		return fmt.Errorf("lane gas_price must be non-negative: %s", lp.GasPrice.String())
	}
	if lp.Enabled && lp.GasPrice.IsZero() {
		return fmt.Errorf("lane is enabled but gas_price is zero — refusing to install a free fee lane")
	}
	return nil
}

// RegistryAddress returns the registry address as a common.Address (helpful
// at the keeper / hook boundary).
func (lp LaneParams) RegistryAddress() common.Address {
	return common.BytesToAddress(lp.Registry[:])
}
