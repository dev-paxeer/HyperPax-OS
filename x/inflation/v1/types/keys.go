// Copyright PaxLabs Ltd.(Paxeer Network)
// Paxeer Network Non-Commercial License 1.0 (ENCL-1.0)(https://github.com/Paxeer-Network/hyperpaxeer-os/blob/main/LICENSE_FAQ.md)


package types

// constants
const (
	// module name
	ModuleName = "inflation"

	// StoreKey to be used when creating the KVStore
	StoreKey = ModuleName

	// RouterKey to be used for message routing
	RouterKey = ModuleName
)

// prefix bytes for the inflation persistent store
const (
	prefixPeriod = iota + 1
	prefixEpochMintProvision
	prefixEpochIdentifier
	prefixEpochsPerPeriod
	prefixSkippedEpochs
)

// KVStore key prefixes
var (
	KeyPrefixPeriod          = []byte{prefixPeriod}
	KeyPrefixEpochIdentifier = []byte{prefixEpochIdentifier}
	KeyPrefixEpochsPerPeriod = []byte{prefixEpochsPerPeriod}
	KeyPrefixSkippedEpochs   = []byte{prefixSkippedEpochs}
)
