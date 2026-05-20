// Copyright PaxLabs Ltd.(Paxeer Network)
// Paxeer Network Non-Commercial License 1.0 (ENCL-1.0)(https://github.com/Paxeer-Network/hyperpaxeer-os/blob/main/LICENSE_FAQ.md)


package types

import "encoding/binary"

const (
	// ModuleName defines the module name.
	ModuleName = "streams"

	// StoreKey defines the primary module store key.
	StoreKey = ModuleName

	// RouterKey defines the module's message routing key.
	RouterKey = ModuleName

	// ModuleAccountName is the account that holds escrowed stream funds.
	// Resolved via authtypes.NewModuleAddress(ModuleAccountName).
	ModuleAccountName = ModuleName
)

// KV store key prefixes.
//
// Layout:
//
//	0x01 | be8(streamId)              ->  Stream    (canonical record)
//	0x02 | payer (20)  | be8(streamId) ->  1        (payer index)
//	0x03 | payee (20)  | be8(streamId) ->  1        (payee index)
//	0x04                                ->  Params   (JSON)
//	0x05                                ->  be8(NextStreamId)
var (
	StreamByIDKeyPrefix = []byte{0x01}
	PayerStreamPrefix   = []byte{0x02}
	PayeeStreamPrefix   = []byte{0x03}
	ParamsKey           = []byte{0x04}
	NextStreamIDKey     = []byte{0x05}
)

// StreamByIDKey returns the store key for the canonical Stream record.
func StreamByIDKey(streamID uint64) []byte {
	key := make([]byte, 0, 1+8)
	key = append(key, StreamByIDKeyPrefix...)
	key = appendUint64BE(key, streamID)
	return key
}

// PayerStreamKey returns the store key for the (payer -> streamID) membership.
func PayerStreamKey(payer []byte, streamID uint64) []byte {
	key := make([]byte, 0, 1+len(payer)+8)
	key = append(key, PayerStreamPrefix...)
	key = append(key, payer...)
	key = appendUint64BE(key, streamID)
	return key
}

// PayerStreamsPrefix returns the iteration prefix for streams owned by payer.
func PayerStreamsPrefix(payer []byte) []byte {
	key := make([]byte, 0, 1+len(payer))
	key = append(key, PayerStreamPrefix...)
	key = append(key, payer...)
	return key
}

// PayeeStreamKey returns the store key for the (payee -> streamID) membership.
func PayeeStreamKey(payee []byte, streamID uint64) []byte {
	key := make([]byte, 0, 1+len(payee)+8)
	key = append(key, PayeeStreamPrefix...)
	key = append(key, payee...)
	key = appendUint64BE(key, streamID)
	return key
}

// PayeeStreamsPrefix returns the iteration prefix for streams owed to payee.
func PayeeStreamsPrefix(payee []byte) []byte {
	key := make([]byte, 0, 1+len(payee))
	key = append(key, PayeeStreamPrefix...)
	key = append(key, payee...)
	return key
}

func appendUint64BE(buf []byte, v uint64) []byte {
	var tmp [8]byte
	binary.BigEndian.PutUint64(tmp[:], v)
	return append(buf, tmp[:]...)
}
