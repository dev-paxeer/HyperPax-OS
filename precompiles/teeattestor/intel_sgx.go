// Copyright PaxLabs Ltd.(Paxeer Network)
// Paxeer Network Non-Commercial License 1.0 (ENCL-1.0)(https://github.com/Paxeer-Network/hyperpaxeer-os/blob/main/LICENSE_FAQ.md)


package teeattestor

import (
	errorsmod "cosmossdk.io/errors"

	"github.com/evmos/evmos/v18/x/attestor/types"
)

// verifyIntelSGX verifies an Intel SGX (DCAP) attestation envelope against
// the trusted roots loaded for the SGX family. ECDSA-P256 over the
// envelope's signed prefix; the pubkey field carries the PCK leaf
// certificate or its SubjectPublicKeyInfo.
//
// Production extension: parse Intel's quote v3 format with the
// github.com/intel/dcap reference Go bindings (or go-tdx-guest's
// Verify which also handles SGX), and chain to the Intel SGX root CA
// (load via gov MsgUpdateTEERoots).
func verifyIntelSGX(roots [][]byte, quote []byte) (types.Attestation, error) {
	env, err := parseEnvelope(types.FamilyIntelSGX, quote)
	if err != nil {
		return types.Attestation{}, err
	}
	if !rootsContain(roots, env.pubkey) {
		return types.Attestation{}, errorsmod.Wrap(types.ErrUntrustedChain, "SGX: envelope pubkey not in trusted roots")
	}
	if err := verifyECDSAOverDER(env); err != nil {
		return types.Attestation{}, errorsmod.Wrap(types.ErrInvalidQuote, "SGX: "+err.Error())
	}
	return extractAttestation(env), nil
}
