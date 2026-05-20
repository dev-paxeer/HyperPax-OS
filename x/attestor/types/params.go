// Copyright PaxLabs Ltd.(Paxeer Network)
// Paxeer Network Non-Commercial License 1.0 (ENCL-1.0)(https://github.com/Paxeer-Network/hyperpaxeer-os/blob/main/LICENSE_FAQ.md)


package types

import "fmt"

// Default parameter values. Mirrors Paxeer_Chain_Upgrades.md §3.2.
const (
	DefaultGasVerifyBase   uint64 = 30_000
	DefaultGasFamilyTDX    uint64 = 10_000
	DefaultGasFamilySEV    uint64 = 12_000
	DefaultGasFamilyNVIDIA uint64 = 15_000
	DefaultGasFamilySGX    uint64 = 8_000
	DefaultGasView         uint64 = 200

	// DefaultMaxAttestationAge is the maximum age (in seconds) accepted between
	// a quote's claimed timestamp and ctx.BlockTime() at verification time.
	DefaultMaxAttestationAge int64 = 600
)

// Params holds the gov-tunable configuration for the attestor module.
type Params struct {
	GasVerifyBase   uint64 `json:"gas_verify_base"`
	GasFamilyTDX    uint64 `json:"gas_family_tdx"`
	GasFamilySEV    uint64 `json:"gas_family_sev"`
	GasFamilyNVIDIA uint64 `json:"gas_family_nvidia"`
	GasFamilySGX    uint64 `json:"gas_family_sgx"`
	GasView         uint64 `json:"gas_view"`

	// MaxAttestationAge in seconds. Quote timestamps farther in the past than
	// this are rejected even if cryptographically valid.
	MaxAttestationAge int64 `json:"max_attestation_age"`

	// DebugAllowed: when false (default), any quote with `debug = true` is
	// rejected during verification. Production setting is `false`. Test/dev
	// chains may flip this for fixture work.
	DebugAllowed bool `json:"debug_allowed"`
}

// DefaultParams returns the default attestor module parameters.
func DefaultParams() Params {
	return Params{
		GasVerifyBase:     DefaultGasVerifyBase,
		GasFamilyTDX:      DefaultGasFamilyTDX,
		GasFamilySEV:      DefaultGasFamilySEV,
		GasFamilyNVIDIA:   DefaultGasFamilyNVIDIA,
		GasFamilySGX:      DefaultGasFamilySGX,
		GasView:           DefaultGasView,
		MaxAttestationAge: DefaultMaxAttestationAge,
		DebugAllowed:      false,
	}
}

// Validate performs basic validation on attestor parameters.
func (p Params) Validate() error {
	if p.GasVerifyBase == 0 {
		return fmt.Errorf("gas_verify_base must be > 0")
	}
	if p.MaxAttestationAge <= 0 {
		return fmt.Errorf("max_attestation_age must be > 0")
	}
	return nil
}
