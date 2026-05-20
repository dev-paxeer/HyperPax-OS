// Copyright PaxLabs Ltd.(Paxeer Network)
// Paxeer Network Non-Commercial License 1.0 (ENCL-1.0)(https://github.com/Paxeer-Network/hyperpaxeer-os/blob/main/LICENSE_FAQ.md)


package types

import errorsmod "cosmossdk.io/errors"

var (
	ErrUnknownFamily       = errorsmod.Register(ModuleName, 2, "unknown TEE family")
	ErrNoRootsLoaded       = errorsmod.Register(ModuleName, 3, "no trusted roots loaded for family")
	ErrInvalidQuote        = errorsmod.Register(ModuleName, 4, "attestation quote failed parse or signature verification")
	ErrUntrustedChain      = errorsmod.Register(ModuleName, 5, "quote signing chain does not anchor to a trusted root")
	ErrAttestationStale    = errorsmod.Register(ModuleName, 6, "attestation timestamp older than max_attestation_age")
	ErrDebugRejected       = errorsmod.Register(ModuleName, 7, "TEE in debug mode rejected by policy")
	ErrReportDataMismatch  = errorsmod.Register(ModuleName, 8, "report_data does not match expected value")
	ErrUnauthorizedUpdate  = errorsmod.Register(ModuleName, 9, "only governance may update trusted roots")
)
