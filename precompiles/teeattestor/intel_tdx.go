// Copyright PaxLabs Ltd.(Paxeer Network)
// Paxeer Network Non-Commercial License 1.0 (ENCL-1.0)(https://github.com/Paxeer-Network/hyperpaxeer-os/blob/main/LICENSE_FAQ.md)


package teeattestor

import (
	errorsmod "cosmossdk.io/errors"

	"github.com/evmos/evmos/v18/x/attestor/types"
)

// verifyIntelTDX verifies an Intel TDX attestation envelope against the
// trusted roots loaded for the TDX family. Trust model:
//
//  1. The envelope's pubkey must appear in the roots set OR be signed by a
//     root via x509 chain validation. The naive case (self-anchored DER
//     pubkey present in roots) is the v21 development path; production
//     deployments should load the Intel SGX/TDX PCK CA chain from
//     pccs.intel.com via gov MsgUpdateTEERoots.
//  2. ECDSA-P256 signature over signedPrefix.
//
// Returns the parsed Attestation on success.
//
// Production extension: replace this body with the github.com/google/go-tdx-guest
// Verify path (parses the native QEv4 quote format and walks the PCK chain).
// The envelope-format scaffolding here is what the precompile dispatcher
// consumes; the verifier function signature must remain stable across the
// swap.
func verifyIntelTDX(roots [][]byte, quote []byte) (types.Attestation, error) {
	env, err := parseEnvelope(types.FamilyIntelTDX, quote)
	if err != nil {
		return types.Attestation{}, err
	}
	if !rootsContain(roots, env.pubkey) {
		return types.Attestation{}, errorsmod.Wrap(types.ErrUntrustedChain, "TDX: envelope pubkey not in trusted roots")
	}
	if err := verifyECDSAOverDER(env); err != nil {
		return types.Attestation{}, errorsmod.Wrap(types.ErrInvalidQuote, "TDX: "+err.Error())
	}
	return extractAttestation(env), nil
}
