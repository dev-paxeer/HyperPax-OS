// Copyright PaxLabs Ltd.(Paxeer Network)
// Paxeer Network Non-Commercial License 1.0 (ENCL-1.0)(https://github.com/Paxeer-Network/hyperpaxeer-os/blob/main/LICENSE_FAQ.md)


package types

import errorsmod "cosmossdk.io/errors"

// Sentinel errors for the scheduler module. Codes 2+ are user-facing; code 1 is
// reserved by convention for "internal error".
var (
	ErrJobNotFound          = errorsmod.Register(ModuleName, 2, "job not found")
	ErrUnauthorized         = errorsmod.Register(ModuleName, 3, "caller is not the job's creator")
	ErrHorizonExceeded      = errorsmod.Register(ModuleName, 4, "executeAtBlock exceeds max schedule horizon")
	ErrPastBlock            = errorsmod.Register(ModuleName, 5, "executeAtBlock is not in the future")
	ErrMaxJobsExceeded      = errorsmod.Register(ModuleName, 6, "creator has reached max jobs per creator")
	ErrDepositTooLow        = errorsmod.Register(ModuleName, 7, "deposit below required minimum")
	ErrInvalidGasLimit      = errorsmod.Register(ModuleName, 8, "gas limit must be > 0")
	ErrInvalidTarget        = errorsmod.Register(ModuleName, 9, "target address is invalid (zero)")
	ErrJobInactive          = errorsmod.Register(ModuleName, 10, "job is no longer active")
)
