// Copyright PaxLabs Ltd.(Paxeer Network)
// Paxeer Network Non-Commercial License 1.0 (ENCL-1.0)(https://github.com/Paxeer-Network/hyperpaxeer-os/blob/main/LICENSE_FAQ.md)


package types

// Attestation is the verified, normalized result of a TEE attestation quote.
// This is the Go-side mirror of the Solidity `Attestation` struct in
// `ITEEAttestor.sol`. The precompile ABI-encodes this back to Solidity.
//
// All fields are extracted from the (vendor-specific) quote during verification
// — they're NOT module state.
type Attestation struct {
	// Family is the TEE family identifier (see types.FamilyIntelTDX etc).
	Family uint8 `json:"family"`
	// MRTD is the measurement (TDX) / launch digest (SNP) / image hash (NVIDIA).
	MRTD [32]byte `json:"mrtd"`
	// ReportData is 32 bytes of caller-provided data committed inside the quote
	// (typically the keccak256 of inputs/outputs the TEE attests to).
	ReportData [32]byte `json:"report_data"`
	// Timestamp is the unix-second timestamp of the quote (per the TEE
	// platform's clock).
	Timestamp uint64 `json:"timestamp"`
	// Debug is true if the TEE reported itself in debug mode (untrusted).
	Debug bool `json:"debug"`
}
