// Copyright PaxLabs Ltd.(Paxeer Network)
// Paxeer Network Non-Commercial License 1.0 (ENCL-1.0)(https://github.com/Paxeer-Network/hyperpaxeer-os/blob/main/LICENSE_FAQ.md)


package teeattestor

import (
	"bytes"
	"crypto/ed25519"
	"crypto/x509"
	"encoding/binary"
	"errors"

	"github.com/evmos/evmos/v18/x/attestor/types"
)

// Envelope format
//
// All four per-family verifiers consume a single Paxeer-defined envelope
// rather than each family's vendor-specific quote format. The envelope is a
// deterministic byte layout:
//
//	[ 0:  5]  magic = "PXVE\x01"           (5 bytes, ASCII + version)
//	[ 5:  6]  family                       (1 byte, must match the dispatch arg)
//	[ 6:  7]  debug                        (1 byte, 0 = production, 1 = debug)
//	[ 7: 39]  mrtd                         (32 bytes)
//	[39: 71]  report_data                  (32 bytes)
//	[71: 79]  timestamp_unix               (8 bytes, big-endian uint64)
//	[79: 95]  nonce                        (16 bytes, opaque)
//	[95: 97]  sig_len                      (2 bytes, big-endian uint16)
//	[97: 97+sig_len]                       signature
//	[97+sig_len: 99+sig_len]  pubkey_len   (2 bytes, big-endian uint16)
//	[99+sig_len: 99+sig_len+pubkey_len]    pubkey/cert (DER or PEM, family-dependent)
//
// The signed bytes are everything from index 0 up to and including the nonce
// (i.e. data[0:95]).
//
// Production deployments will swap each family's verify_* function for one
// that consumes the vendor's native quote format (Intel TDX QEv4, AMD
// SEV-SNP attestation report, NVIDIA H100 EAT JWT, Intel SGX-DCAP quote v3)
// and is anchored against the vendor's published trust root. The envelope
// here provides a stable interface for the precompile dispatcher and a
// production-ready code path for fully self-signed development chains.

const (
	envelopeMagicLen     = 5
	envelopeMRTDOffset   = 7
	envelopeMRTDLen      = 32
	envelopeReportOffset = envelopeMRTDOffset + envelopeMRTDLen
	envelopeReportLen    = 32
	envelopeTSOffset     = envelopeReportOffset + envelopeReportLen
	envelopeTSLen        = 8
	envelopeNonceOffset  = envelopeTSOffset + envelopeTSLen
	envelopeNonceLen     = 16
	envelopeSigLenOffset = envelopeNonceOffset + envelopeNonceLen
	envelopeMinLen       = envelopeSigLenOffset + 2 + 2 // sig_len + pubkey_len header bytes
)

var envelopeMagic = []byte{'P', 'X', 'V', 'E', 0x01}

// envelope is the parsed view of a Paxeer attestation envelope. All slices
// reference the underlying input bytes — the caller must not retain them
// past the verify call.
type envelope struct {
	family       uint8
	debug        bool
	mrtd         [32]byte
	reportData   [32]byte
	timestamp    uint64
	nonce        [16]byte
	signature    []byte
	pubkey       []byte
	signedPrefix []byte // data[0:envelopeSigLenOffset]
}

// parseEnvelope decodes the byte layout above. Returns ErrInvalidQuote on
// any structural mismatch — callers should always wrap with errorsmod.Wrap.
func parseEnvelope(family uint8, data []byte) (*envelope, error) {
	if len(data) < envelopeMinLen {
		return nil, types.ErrInvalidQuote.Wrap("envelope too short")
	}
	if !bytes.Equal(data[:envelopeMagicLen], envelopeMagic) {
		return nil, types.ErrInvalidQuote.Wrap("magic mismatch")
	}
	famByte := data[envelopeMagicLen]
	if famByte != family {
		return nil, types.ErrInvalidQuote.Wrapf("family mismatch: envelope=%d, dispatch=%d", famByte, family)
	}
	debugByte := data[envelopeMagicLen+1]
	if debugByte > 1 {
		return nil, types.ErrInvalidQuote.Wrap("debug byte must be 0 or 1")
	}

	env := &envelope{
		family:    famByte,
		debug:     debugByte == 1,
		timestamp: binary.BigEndian.Uint64(data[envelopeTSOffset : envelopeTSOffset+envelopeTSLen]),
	}
	copy(env.mrtd[:], data[envelopeMRTDOffset:envelopeMRTDOffset+envelopeMRTDLen])
	copy(env.reportData[:], data[envelopeReportOffset:envelopeReportOffset+envelopeReportLen])
	copy(env.nonce[:], data[envelopeNonceOffset:envelopeNonceOffset+envelopeNonceLen])

	sigLen := int(binary.BigEndian.Uint16(data[envelopeSigLenOffset : envelopeSigLenOffset+2]))
	pkLenOffset := envelopeSigLenOffset + 2 + sigLen
	if pkLenOffset+2 > len(data) {
		return nil, types.ErrInvalidQuote.Wrap("signature exceeds envelope length")
	}
	env.signature = data[envelopeSigLenOffset+2 : pkLenOffset]

	pkLen := int(binary.BigEndian.Uint16(data[pkLenOffset : pkLenOffset+2]))
	pkEnd := pkLenOffset + 2 + pkLen
	if pkEnd > len(data) {
		return nil, types.ErrInvalidQuote.Wrap("pubkey exceeds envelope length")
	}
	env.pubkey = data[pkLenOffset+2 : pkEnd]

	env.signedPrefix = data[:envelopeSigLenOffset]
	return env, nil
}

// rootsContain reports whether the envelope's pubkey appears verbatim in the
// trusted roots set (DER or PEM). Used as the trust anchor for self-signed
// envelopes when no vendor PKI chain is wired.
func rootsContain(roots [][]byte, pubkey []byte) bool {
	for _, r := range roots {
		if bytes.Equal(r, pubkey) {
			return true
		}
	}
	return false
}

// verifyEd25519 verifies an Ed25519 signature over signedPrefix using the
// envelope's pubkey. The pubkey must be a 32-byte Ed25519 public key.
func verifyEd25519(env *envelope) error {
	if len(env.pubkey) != ed25519.PublicKeySize {
		return errors.New("pubkey is not 32 bytes (Ed25519)")
	}
	if len(env.signature) != ed25519.SignatureSize {
		return errors.New("signature is not 64 bytes (Ed25519)")
	}
	if !ed25519.Verify(ed25519.PublicKey(env.pubkey), env.signedPrefix, env.signature) {
		return errors.New("Ed25519 signature verification failed")
	}
	return nil
}

// verifyECDSAOverDER verifies an ECDSA signature over signedPrefix using the
// envelope's DER-encoded SubjectPublicKeyInfo / certificate. Used by TDX,
// SEV-SNP, and SGX families.
func verifyECDSAOverDER(env *envelope) error {
	cert, err := x509.ParseCertificate(env.pubkey)
	if err == nil {
		// Certificate path: verify against the cert's public key.
		if err := cert.CheckSignature(cert.SignatureAlgorithm, env.signedPrefix, env.signature); err == nil {
			return nil
		}
		// Fall through to raw key check if CheckSignature fails — the cert
		// may be a chain leaf with a different signature algorithm.
	}
	pub, err := x509.ParsePKIXPublicKey(env.pubkey)
	if err != nil {
		return errors.New("pubkey is neither a parseable certificate nor a PKIX SubjectPublicKeyInfo")
	}
	// We pin the curve at the keeper level by trusting which roots we accept.
	// Here we just re-use x509's signature algorithm dispatch.
	checker := &x509.Certificate{PublicKey: pub, SignatureAlgorithm: x509.ECDSAWithSHA256}
	return checker.CheckSignature(x509.ECDSAWithSHA256, env.signedPrefix, env.signature)
}

// extractAttestation builds the Attestation result struct from a verified
// envelope. The caller is responsible for trust anchoring before calling.
func extractAttestation(env *envelope) types.Attestation {
	return types.Attestation{
		Family:     env.family,
		MRTD:       env.mrtd,
		ReportData: env.reportData,
		Timestamp:  env.timestamp,
		Debug:      env.debug,
	}
}
