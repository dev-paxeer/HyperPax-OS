// Copyright PaxLabs Ltd.(Paxeer Network)
// Paxeer Network Non-Commercial License 1.0 (ENCL-1.0)(https://github.com/Paxeer-Network/hyperpaxeer-os/blob/main/LICENSE_FAQ.md)


package teeattestor

import (
	errorsmod "cosmossdk.io/errors"

	"github.com/evmos/evmos/v18/x/attestor/types"
)

// verifyAMDSEVSNP verifies an AMD SEV-SNP attestation envelope against the
// trusted roots loaded for the SEV-SNP family. Same envelope + ECDSA-P384
// pattern as TDX. Production extension: swap to github.com/google/go-sev-guest
// Verify which parses the AMD attestation report format and validates the
// VLEK/VCEK chain against AMD's published root.
//
// Trust model — see verifyIntelTDX for the rationale; SEV-SNP uses the same
// DER-anchored envelope flow at the v21 development tier.
func verifyAMDSEVSNP(roots [][]byte, quote []byte) (types.Attestation, error) {
	env, err := parseEnvelope(types.FamilyAMDSEVSNP, quote)
	if err != nil {
		return types.Attestation{}, err
	}
	if !rootsContain(roots, env.pubkey) {
		return types.Attestation{}, errorsmod.Wrap(types.ErrUntrustedChain, "SEV-SNP: envelope pubkey not in trusted roots")
	}
	if err := verifyECDSAOverDER(env); err != nil {
		return types.Attestation{}, errorsmod.Wrap(types.ErrInvalidQuote, "SEV-SNP: "+err.Error())
	}
	return extractAttestation(env), nil
}
