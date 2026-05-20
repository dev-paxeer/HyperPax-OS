package types

const (
	// ModuleName defines the module name
	ModuleName = "paxoracle"

	// StoreKey defines the primary module store key
	StoreKey = ModuleName

	// RouterKey defines the module's message routing key
	RouterKey = ModuleName
)

// KV store key prefixes
var (
	// PriceSubmissionKeyPrefix is the prefix for storing individual validator price submissions.
	// Key format: PriceSubmissionKeyPrefix | marketId (32 bytes) | valAddr (20 bytes)
	PriceSubmissionKeyPrefix = []byte{0x01}

	// ParamsKey is the key for module parameters.
	ParamsKey = []byte{0x02}

	// SupportedMarketKeyPrefix is the prefix for supported market IDs.
	// Key format: SupportedMarketKeyPrefix | marketId (32 bytes)
	SupportedMarketKeyPrefix = []byte{0x03}
)

// PriceSubmissionKey returns the store key for a validator's price submission on a market.
func PriceSubmissionKey(marketId [32]byte, valAddr []byte) []byte {
	key := make([]byte, 0, 1+32+len(valAddr))
	key = append(key, PriceSubmissionKeyPrefix...)
	key = append(key, marketId[:]...)
	key = append(key, valAddr...)
	return key
}

// PriceSubmissionsByMarketPrefix returns the prefix for all submissions on a given market.
func PriceSubmissionsByMarketPrefix(marketId [32]byte) []byte {
	key := make([]byte, 0, 1+32)
	key = append(key, PriceSubmissionKeyPrefix...)
	key = append(key, marketId[:]...)
	return key
}

// SupportedMarketKey returns the store key for a supported market.
func SupportedMarketKey(marketId [32]byte) []byte {
	key := make([]byte, 0, 1+32)
	key = append(key, SupportedMarketKeyPrefix...)
	key = append(key, marketId[:]...)
	return key
}
