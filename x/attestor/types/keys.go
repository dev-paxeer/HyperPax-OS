// Copyright PaxLabs Ltd.(Paxeer Network)
// Paxeer Network Non-Commercial License 1.0 (ENCL-1.0)(https://github.com/Paxeer-Network/hyperpaxeer-os/blob/main/LICENSE_FAQ.md)


package types

import "encoding/binary"

const (
	// ModuleName defines the module name.
	ModuleName = "attestor"

	// StoreKey defines the primary module store key.
	StoreKey = ModuleName

	// RouterKey defines the module's message routing key.
	RouterKey = ModuleName
)

// TEE family identifiers — kept stable across the module + precompile +
// Solidity ABI. Order MUST match `enum Family` in `ITEEAttestor.sol`.
const (
	FamilyIntelTDX   uint8 = 0
	FamilyAMDSEVSNP  uint8 = 1
	FamilyNVIDIAH100 uint8 = 2
	FamilyIntelSGX   uint8 = 3

	FamilyMax = FamilyIntelSGX
)

// FamilyName returns a human-readable label for events / logs.
func FamilyName(f uint8) string {
	switch f {
	case FamilyIntelTDX:
		return "intel_tdx"
	case FamilyAMDSEVSNP:
		return "amd_sev_snp"
	case FamilyNVIDIAH100:
		return "nvidia_h100"
	case FamilyIntelSGX:
		return "intel_sgx"
	default:
		return "unknown"
	}
}

// KV store key prefixes.
//
// Layout:
//
//	0x01 | family(1) | u32-be(index)  ->  []byte (PEM cert OR DER pubkey)
//	0x02 | family(1)                  ->  u32-be(count)
//	0x03                              ->  Params (JSON)
var (
	RootKeyPrefix       = []byte{0x01}
	RootCountKeyPrefix  = []byte{0x02}
	ParamsKey           = []byte{0x03}
)

// RootKey returns the store key for a single trusted root cert/pubkey.
func RootKey(family uint8, index uint32) []byte {
	key := make([]byte, 0, 1+1+4)
	key = append(key, RootKeyPrefix...)
	key = append(key, family)
	var tmp [4]byte
	binary.BigEndian.PutUint32(tmp[:], index)
	return append(key, tmp[:]...)
}

// RootsByFamilyPrefix returns the iteration prefix for all roots of a family.
func RootsByFamilyPrefix(family uint8) []byte {
	return append(append([]byte{}, RootKeyPrefix...), family)
}

// RootCountKey returns the store key for the count of roots in a family.
func RootCountKey(family uint8) []byte {
	return append(append([]byte{}, RootCountKeyPrefix...), family)
}
