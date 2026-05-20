// Copyright PaxLabs Ltd.(Paxeer Network)
// Paxeer Network Non-Commercial License 1.0 (ENCL-1.0)(https://github.com/Paxeer-Network/hyperpaxeer-os/blob/main/LICENSE_FAQ.md)

package types

const (
	// ModuleName string name of module
	ModuleName = "feemarket"

	// StoreKey key for base fee and block gas used.
	// The Fee Market module should use a prefix store.
	StoreKey = ModuleName

	// RouterKey uses module name for routing
	RouterKey = ModuleName

	// TransientKey is the key to access the FeeMarket transient store, that is reset
	// during the Commit phase.
	TransientKey = "transient_" + ModuleName
)

// prefix bytes for the feemarket persistent store
const (
	prefixBlockGasWanted    = iota + 1
	deprecatedPrefixBaseFee // unused
	// prefixLaneParams stores the JSON-encoded LaneParams record (single
	// value, no sub-key). Held outside the protobuf-generated `Params`
	// struct to avoid triggering proto regen for the v20-agent upgrade.
	prefixLaneParams
	// prefixLaneMember = 0x10. Each entry under this prefix is keyed by an
	// EVM address (20 bytes) and signals "this address is in the lane". The
	// value byte is just a marker. Membership is mirrored from on-chain
	// `LaneRegistration` events by the EVM hook in keeper/lane_hook.go.
	prefixLaneMember = 0x10
)

const (
	prefixTransientBlockGasUsed = iota + 1
)

// KVStore key prefixes
var (
	KeyPrefixBlockGasWanted = []byte{prefixBlockGasWanted}
	KeyLaneParams           = []byte{prefixLaneParams}
	KeyPrefixLaneMember     = []byte{prefixLaneMember}
)

// LaneMemberKey returns the store key for a single (addr -> bool) lane
// membership entry.
func LaneMemberKey(addr []byte) []byte {
	key := make([]byte, 0, 1+len(addr))
	key = append(key, KeyPrefixLaneMember...)
	key = append(key, addr...)
	return key
}

// Transient Store key prefixes
var (
	KeyPrefixTransientBlockGasWanted = []byte{prefixTransientBlockGasUsed}
)
