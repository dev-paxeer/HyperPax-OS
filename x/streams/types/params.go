// Copyright PaxLabs Ltd.(Paxeer Network)
// Paxeer Network Non-Commercial License 1.0 (ENCL-1.0)(https://github.com/Paxeer-Network/hyperpaxeer-os/blob/main/LICENSE_FAQ.md)


package types

import "fmt"

// Default parameter values. Mirrors Paxeer_Chain_Upgrades.md §3.1.
const (
	DefaultGasOpen       uint64 = 8_000
	DefaultGasSettle     uint64 = 4_000
	DefaultGasClose      uint64 = 5_000
	DefaultGasUpdateRate uint64 = 4_000
	DefaultGasView       uint64 = 200

	// DefaultMinDuration is the minimum stream duration (StopTime - StartTime in
	// seconds) accepted at open-time. Prevents tiny noise streams.
	DefaultMinDuration uint64 = 60

	// DefaultMaxStreamsPerPayer caps per-payer usage to bound iteration cost.
	DefaultMaxStreamsPerPayer uint32 = 4096
)

// Params holds the gov-tunable configuration for the streams module.
type Params struct {
	GasOpen       uint64 `json:"gas_open"`
	GasSettle     uint64 `json:"gas_settle"`
	GasClose      uint64 `json:"gas_close"`
	GasUpdateRate uint64 `json:"gas_update_rate"`
	GasView       uint64 `json:"gas_view"`

	MinDuration         uint64 `json:"min_duration"`
	MaxStreamsPerPayer  uint32 `json:"max_streams_per_payer"`
}

// DefaultParams returns the default streams module parameters.
func DefaultParams() Params {
	return Params{
		GasOpen:            DefaultGasOpen,
		GasSettle:          DefaultGasSettle,
		GasClose:           DefaultGasClose,
		GasUpdateRate:      DefaultGasUpdateRate,
		GasView:            DefaultGasView,
		MinDuration:        DefaultMinDuration,
		MaxStreamsPerPayer: DefaultMaxStreamsPerPayer,
	}
}

// Validate performs basic validation on streams parameters.
func (p Params) Validate() error {
	if p.GasOpen == 0 {
		return fmt.Errorf("gas_open must be > 0")
	}
	if p.GasSettle == 0 {
		return fmt.Errorf("gas_settle must be > 0")
	}
	if p.GasClose == 0 {
		return fmt.Errorf("gas_close must be > 0")
	}
	if p.GasUpdateRate == 0 {
		return fmt.Errorf("gas_update_rate must be > 0")
	}
	if p.MinDuration == 0 {
		return fmt.Errorf("min_duration must be > 0")
	}
	if p.MaxStreamsPerPayer == 0 {
		return fmt.Errorf("max_streams_per_payer must be > 0")
	}
	return nil
}
