// Copyright PaxLabs Ltd.(Paxeer Network)
// Paxeer Network Non-Commercial License 1.0 (ENCL-1.0)(https://github.com/Paxeer-Network/hyperpaxeer-os/blob/main/LICENSE_FAQ.md)

package keeper

import (
	"encoding/binary"
	"encoding/json"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/evmos/evmos/v18/x/streams/types"
)

// SetStream writes a Stream to the canonical record + payer/payee indexes.
// Called from Open / UpdateRate / Settle (settle_impl.go) and from
// InitGenesis (genesis.go).
func (k Keeper) SetStream(ctx sdk.Context, s types.Stream) error {
	store := ctx.KVStore(k.storeKey)

	bz, err := json.Marshal(s)
	if err != nil {
		return err
	}

	store.Set(types.StreamByIDKey(s.ID), bz)
	store.Set(types.PayerStreamKey(s.Payer, s.ID), []byte{1})
	store.Set(types.PayeeStreamKey(s.Payee, s.ID), []byte{1})

	return nil
}

// GetStream returns the Stream with the given id, if any.
func (k Keeper) GetStream(ctx sdk.Context, streamID uint64) (types.Stream, bool) {
	store := ctx.KVStore(k.storeKey)
	bz := store.Get(types.StreamByIDKey(streamID))
	if bz == nil {
		return types.Stream{}, false
	}
	var s types.Stream
	if err := json.Unmarshal(bz, &s); err != nil {
		return types.Stream{}, false
	}
	return s, true
}

// DeleteStream removes a Stream from all three indexes. Used after `close`.
func (k Keeper) DeleteStream(ctx sdk.Context, s types.Stream) {
	store := ctx.KVStore(k.storeKey)
	store.Delete(types.StreamByIDKey(s.ID))
	store.Delete(types.PayerStreamKey(s.Payer, s.ID))
	store.Delete(types.PayeeStreamKey(s.Payee, s.ID))
}

// PayerStreamIDs returns all stream IDs owned by the given payer (20 bytes).
func (k Keeper) PayerStreamIDs(ctx sdk.Context, payer []byte) []uint64 {
	return k.streamIDsByPrefix(ctx, types.PayerStreamsPrefix(payer), 1+len(payer))
}

// PayeeStreamIDs returns all stream IDs owed to the given payee.
func (k Keeper) PayeeStreamIDs(ctx sdk.Context, payee []byte) []uint64 {
	return k.streamIDsByPrefix(ctx, types.PayeeStreamsPrefix(payee), 1+len(payee))
}

func (k Keeper) streamIDsByPrefix(ctx sdk.Context, prefix []byte, prefixLen int) []uint64 {
	store := ctx.KVStore(k.storeKey)
	iter := sdk.KVStorePrefixIterator(store, prefix)
	defer iter.Close()

	var ids []uint64
	for ; iter.Valid(); iter.Next() {
		key := iter.Key()
		if len(key) < prefixLen+8 {
			continue
		}
		id := binary.BigEndian.Uint64(key[prefixLen : prefixLen+8])
		ids = append(ids, id)
	}
	return ids
}

// CountStreamsByPayer returns the number of streams owned by a payer. Used to
// enforce Params.MaxStreamsPerPayer at open time.
func (k Keeper) CountStreamsByPayer(ctx sdk.Context, payer []byte) uint32 {
	store := ctx.KVStore(k.storeKey)
	iter := sdk.KVStorePrefixIterator(store, types.PayerStreamsPrefix(payer))
	defer iter.Close()

	var count uint32
	for ; iter.Valid(); iter.Next() {
		count++
	}
	return count
}

// ─── ID allocation ───────────────────────────────────────────────────────────

func (k Keeper) NextStreamID(ctx sdk.Context) uint64 {
	store := ctx.KVStore(k.storeKey)
	bz := store.Get(types.NextStreamIDKey)

	var next uint64
	if bz == nil {
		next = 1
	} else {
		next = binary.BigEndian.Uint64(bz)
		if next == 0 {
			next = 1
		}
	}

	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], next+1)
	store.Set(types.NextStreamIDKey, buf[:])

	return next
}

func (k Keeper) SetNextStreamID(ctx sdk.Context, next uint64) {
	store := ctx.KVStore(k.storeKey)
	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], next)
	store.Set(types.NextStreamIDKey, buf[:])
}

func (k Keeper) GetNextStreamID(ctx sdk.Context) uint64 {
	store := ctx.KVStore(k.storeKey)
	bz := store.Get(types.NextStreamIDKey)
	if bz == nil {
		return 1
	}
	return binary.BigEndian.Uint64(bz)
}

// GetAllStreams returns every stream in the store (id-keyed). Used only by
// ExportGenesis.
func (k Keeper) GetAllStreams(ctx sdk.Context) []types.Stream {
	store := ctx.KVStore(k.storeKey)
	iter := sdk.KVStorePrefixIterator(store, types.StreamByIDKeyPrefix)
	defer iter.Close()

	var streams []types.Stream
	for ; iter.Valid(); iter.Next() {
		var s types.Stream
		if err := json.Unmarshal(iter.Value(), &s); err != nil {
			continue
		}
		streams = append(streams, s)
	}
	return streams
}
