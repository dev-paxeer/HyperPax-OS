package types

import errorsmod "cosmossdk.io/errors"

var (
	ErrInvalidMarketId       = errorsmod.Register(ModuleName, 2, "invalid market id")
	ErrInvalidPrice          = errorsmod.Register(ModuleName, 3, "invalid price: must be positive")
	ErrInvalidConfidence     = errorsmod.Register(ModuleName, 4, "invalid confidence: must be 0 < c <= 1e18")
	ErrNotValidator          = errorsmod.Register(ModuleName, 5, "signer is not an active validator")
	ErrMarketNotSupported    = errorsmod.Register(ModuleName, 6, "market not supported")
	ErrInsufficientQuorum    = errorsmod.Register(ModuleName, 7, "insufficient validator quorum for price")
	ErrStaleSubmissions      = errorsmod.Register(ModuleName, 8, "all submissions are stale")
	ErrInvalidSigner         = errorsmod.Register(ModuleName, 9, "invalid signer address")
)
