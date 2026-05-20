// Copyright PaxLabs Ltd.(Paxeer Network)
// Paxeer Network Non-Commercial License 1.0 (ENCL-1.0)(https://github.com/Paxeer-Network/hyperpaxeer-os/blob/main/LICENSE_FAQ.md)

package types

import "encoding/binary"

const (
	// ModuleName defines the module name.
	ModuleName = "scheduler"

	// StoreKey defines the primary module store key.
	StoreKey = ModuleName

	// RouterKey defines the module's message routing key.
	RouterKey = ModuleName

	// ModuleAccountName is the account that holds escrowed scheduler deposits.
	// Resolved via authtypes.NewModuleAddress(ModuleAccountName). Registered
	// in app.maccPerms with no minter/burner permissions — acts as a plain
	// recipient for SendCoinsFromAccountToModule.
	ModuleAccountName = ModuleName
)

// KV store key prefixes.
//
// Layout:
//
//	0x01 | be8(executeAtBlock) | be8(jobId)  ->  Job   (due-block index, primary scan target for EndBlocker)
//	0x02 | creator (20 bytes)  | be8(jobId)  ->  1     (creator -> jobId membership)
//	0x03 | be8(jobId)                        ->  Job   (id-keyed canonical record, used by views)
//	0x04                                     ->  Params (JSON-encoded module params)
//	0x05                                     ->  be8(NextJobId)  (monotonic id allocator)
var (
	JobByDueBlockKeyPrefix = []byte{0x01}
	CreatorJobIndexPrefix  = []byte{0x02}
	JobByIDKeyPrefix       = []byte{0x03}
	ParamsKey              = []byte{0x04}
	NextJobIDKey           = []byte{0x05}
)

// JobByDueBlockKey returns the store key for a Job under the due-block index.
// Iterating the prefix `JobByDueBlockKeyPrefix | be8(executeAtBlock)` yields all
// jobs scheduled to fire at that block, in deterministic (insertion) order.
func JobByDueBlockKey(executeAtBlock, jobID uint64) []byte {
	key := make([]byte, 0, 1+8+8)
	key = append(key, JobByDueBlockKeyPrefix...)
	key = appendUint64BE(key, executeAtBlock)
	key = appendUint64BE(key, jobID)
	return key
}

// JobByDueBlockPrefix returns the iteration prefix for all jobs at a given block.
func JobByDueBlockPrefix(executeAtBlock uint64) []byte {
	key := make([]byte, 0, 1+8)
	key = append(key, JobByDueBlockKeyPrefix...)
	key = appendUint64BE(key, executeAtBlock)
	return key
}

// CreatorJobKey returns the store key for the (creator -> jobID) membership entry.
func CreatorJobKey(creator []byte, jobID uint64) []byte {
	key := make([]byte, 0, 1+len(creator)+8)
	key = append(key, CreatorJobIndexPrefix...)
	key = append(key, creator...)
	key = appendUint64BE(key, jobID)
	return key
}

// CreatorJobsPrefix returns the iteration prefix for all jobs owned by a creator.
func CreatorJobsPrefix(creator []byte) []byte {
	key := make([]byte, 0, 1+len(creator))
	key = append(key, CreatorJobIndexPrefix...)
	key = append(key, creator...)
	return key
}

// JobByIDKey returns the store key for the id-keyed canonical Job record.
func JobByIDKey(jobID uint64) []byte {
	key := make([]byte, 0, 1+8)
	key = append(key, JobByIDKeyPrefix...)
	key = appendUint64BE(key, jobID)
	return key
}

// appendUint64BE appends `v` to `buf` as 8-byte big-endian. Big-endian gives us
// lexicographic ordering that matches numeric ordering, which is what the
// EndBlocker iteration relies on.
func appendUint64BE(buf []byte, v uint64) []byte {
	var tmp [8]byte
	binary.BigEndian.PutUint64(tmp[:], v)
	return append(buf, tmp[:]...)
}
