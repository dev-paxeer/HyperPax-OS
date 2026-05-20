// Copyright PaxLabs Ltd.(Paxeer Network)
// Paxeer Network Non-Commercial License 1.0 (ENCL-1.0)(https://github.com/Paxeer-Network/hyperpaxeer-os/blob/main/LICENSE_FAQ.md)


package types

import (
	errorsmod "cosmossdk.io/errors"
)

// errors
var (
	ErrInsufficientVestedCoins   = errorsmod.Register(ModuleName, 2, "insufficient vested coins error")
	ErrVestingLockup             = errorsmod.Register(ModuleName, 3, "vesting lockup error")
	ErrInsufficientUnlockedCoins = errorsmod.Register(ModuleName, 4, "insufficient unlocked coins error")
	ErrNothingToClawback         = errorsmod.Register(ModuleName, 5, "nothing to clawback from the account")
	ErrNotSubjectToClawback      = errorsmod.Register(ModuleName, 6, "account is not subject to clawback vesting")
	ErrNotSubjectToGovClawback   = errorsmod.Register(ModuleName, 7, "account does not have governance clawback enabled")
)
