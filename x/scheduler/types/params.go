// Copyright PaxLabs Ltd.(Paxeer Network)
// Paxeer Network Non-Commercial License 1.0 (ENCL-1.0)(https://github.com/Paxeer-Network/hyperpaxeer-os/blob/main/LICENSE_FAQ.md)


package types

import "fmt"

// Default parameter values. These reflect the spec in
// `Paxeer_Chain_Upgrades.md` §2.4 — keep in sync with that doc.
const (
	DefaultGasScheduleBase     uint64 = 5_000
	DefaultGasSchedulePerByte  uint64 = 50
	DefaultGasCancel           uint64 = 3_000
	DefaultGasReschedule       uint64 = 4_000
	DefaultGasView             uint64 = 200
	DefaultMaxJobsPerCreator   uint32 = 1024
	DefaultMaxScheduleHorizon  uint64 = 2_592_000 // ~60 days at 2s blocks
	DefaultMinDepositFactor    uint64 = 2        // deposit >= gasLimit * baseFee * factor
)

// Params holds the gov-tunable configuration for the scheduler module.
//
// Per AGENTS.md §3.4, all gas constants for the new precompile are baked into
// this struct so they can be changed via gov param-change without a hard fork.
type Params struct {
	// Gas — used by the precompile RequiredGas dispatch.
	GasScheduleBase    uint64 `json:"gas_schedule_base"`
	GasSchedulePerByte uint64 `json:"gas_schedule_per_byte"`
	GasCancel          uint64 `json:"gas_cancel"`
	GasReschedule      uint64 `json:"gas_reschedule"`
	GasView            uint64 `json:"gas_view"`

	// Limits — enforced at schedule-time.
	MaxJobsPerCreator  uint32 `json:"max_jobs_per_creator"`
	MaxScheduleHorizon uint64 `json:"max_schedule_horizon"`

	// MinDepositFactor — minimum deposit acceptable at schedule time, expressed
	// as a multiplier over `gasLimit * baseFee`.
	MinDepositFactor uint64 `json:"min_deposit_factor"`
}

// DefaultParams returns the default scheduler module parameters.
func DefaultParams() Params {
	return Params{
		GasScheduleBase:    DefaultGasScheduleBase,
		GasSchedulePerByte: DefaultGasSchedulePerByte,
		GasCancel:          DefaultGasCancel,
		GasReschedule:      DefaultGasReschedule,
		GasView:            DefaultGasView,
		MaxJobsPerCreator:  DefaultMaxJobsPerCreator,
		MaxScheduleHorizon: DefaultMaxScheduleHorizon,
		MinDepositFactor:   DefaultMinDepositFactor,
	}
}

// Validate performs basic validation on scheduler parameters.
func (p Params) Validate() error {
	if p.GasScheduleBase == 0 {
		return fmt.Errorf("gas_schedule_base must be > 0")
	}
	if p.GasCancel == 0 {
		return fmt.Errorf("gas_cancel must be > 0")
	}
	if p.GasReschedule == 0 {
		return fmt.Errorf("gas_reschedule must be > 0")
	}
	if p.MaxJobsPerCreator == 0 {
		return fmt.Errorf("max_jobs_per_creator must be > 0")
	}
	if p.MaxScheduleHorizon == 0 {
		return fmt.Errorf("max_schedule_horizon must be > 0")
	}
	if p.MinDepositFactor == 0 {
		return fmt.Errorf("min_deposit_factor must be >= 1")
	}
	return nil
}
