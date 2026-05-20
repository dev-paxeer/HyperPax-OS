// Copyright PaxLabs Ltd.(Paxeer Network)
// Paxeer Network Non-Commercial License 1.0 (ENCL-1.0)(https://github.com/Paxeer-Network/hyperpaxeer-os/blob/main/LICENSE_FAQ.md)


package types

import errorsmod "cosmossdk.io/errors"

var (
	ErrStreamNotFound        = errorsmod.Register(ModuleName, 2, "stream not found")
	ErrUnauthorized          = errorsmod.Register(ModuleName, 3, "caller is not authorized for this stream")
	ErrInvalidPayee          = errorsmod.Register(ModuleName, 4, "invalid payee address")
	ErrInvalidRate           = errorsmod.Register(ModuleName, 5, "rate must be > 0")
	ErrInvalidCap            = errorsmod.Register(ModuleName, 6, "cap must be > 0 (uncapped streams not supported in v21)")
	ErrInvalidTime           = errorsmod.Register(ModuleName, 7, "stop_time must be >= start_time + min_duration, or 0")
	ErrStreamInactive        = errorsmod.Register(ModuleName, 8, "stream is no longer active")
	ErrInsufficientEscrow    = errorsmod.Register(ModuleName, 9, "module account holds insufficient escrow for payout")
	ErrMaxStreamsExceeded    = errorsmod.Register(ModuleName, 10, "payer has reached max streams per payer")
	ErrSelfPayment           = errorsmod.Register(ModuleName, 11, "payer and payee must differ")
)
