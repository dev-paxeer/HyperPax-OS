// Copyright PaxLabs Ltd.(Paxeer Network)
// Paxeer Network Non-Commercial License 1.0 (ENCL-1.0)(https://github.com/Paxeer-Network/hyperpaxeer-os/blob/main/LICENSE_FAQ.md)


package teeattestor

import (
	errorsmod "cosmossdk.io/errors"

	"github.com/evmos/evmos/v18/x/attestor/types"
)

// verifyNVIDIA verifies an NVIDIA H100 confidential-compute attestation
// envelope against the trusted roots loaded for the NVIDIA family.
//
// D5 = pure-Go: NVIDIA's H100 attestation format is a JWT-style EAT
// signed with Ed25519. Rather than depend on a vendor SDK, the v21
// development tier accepts a Paxeer envelope carrying:
//   - 32-byte Ed25519 public key (the GPU's attestation key)
//   - 64-byte Ed25519 signature over signedPrefix
//
// Production extension: parse NVIDIA's JWT EAT format directly, validate the
// embedded VBIOS measurements + CC mode bit, and chain against NVIDIA's
// published GPU attestation root key. The envelope-format flow here is the
// v21 development scaffolding.
func verifyNVIDIA(roots [][]byte, quote []byte) (types.Attestation, error) {
	env, err := parseEnvelope(types.FamilyNVIDIAH100, quote)
	if err != nil {
		return types.Attestation{}, err
	}
	if !rootsContain(roots, env.pubkey) {
		return types.Attestation{}, errorsmod.Wrap(types.ErrUntrustedChain, "NVIDIA: envelope pubkey not in trusted roots")
	}
	if err := verifyEd25519(env); err != nil {
		return types.Attestation{}, errorsmod.Wrap(types.ErrInvalidQuote, "NVIDIA: "+err.Error())
	}
	return extractAttestation(env), nil
}
