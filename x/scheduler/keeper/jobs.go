// Copyright PaxLabs Ltd.(Paxeer Network)
// Paxeer Network Non-Commercial License 1.0 (ENCL-1.0)(https://github.com/Paxeer-Network/hyperpaxeer-os/blob/main/LICENSE_FAQ.md)

package keeper

import (
	"encoding/binary"
	"encoding/json"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/evmos/evmos/v18/x/scheduler/types"
)

// ─── Job CRUD ────────────────────────────────────────────────────────────────

// SetJob writes a Job to all three indexes (id-keyed, due-block-keyed, creator).
// Called from Keeper.Schedule (schedule.go) and Keeper.Reschedule and from
// InitGenesis (genesis.go). Reference pattern:
// x/paxoracle/keeper/price.go::SetPriceSubmission.
func (k Keeper) SetJob(ctx sdk.Context, j types.Job) error {
	store := ctx.KVStore(k.storeKey)

	bz, err := json.Marshal(j)
	if err != nil {
		return err
	}

	// Primary record by id.
	store.Set(types.JobByIDKey(j.ID), bz)

	// Due-block index — copy of the record so EndBlocker doesn't have to
	// dereference back into the id-keyed store on every iteration.
	store.Set(types.JobByDueBlockKey(j.ExecuteAtBlock, j.ID), bz)

	// Creator membership — value byte is just a marker.
	store.Set(types.CreatorJobKey(j.Creator, j.ID), []byte{1})

	return nil
}

// GetJob returns the Job with the given id, if any.
func (k Keeper) GetJob(ctx sdk.Context, jobID uint64) (types.Job, bool) {
	store := ctx.KVStore(k.storeKey)
	bz := store.Get(types.JobByIDKey(jobID))
	if bz == nil {
		return types.Job{}, false
	}
	var j types.Job
	if err := json.Unmarshal(bz, &j); err != nil {
		return types.Job{}, false
	}
	return j, true
}

// DeleteJob removes a Job from all three indexes.
func (k Keeper) DeleteJob(ctx sdk.Context, j types.Job) {
	store := ctx.KVStore(k.storeKey)
	store.Delete(types.JobByIDKey(j.ID))
	store.Delete(types.JobByDueBlockKey(j.ExecuteAtBlock, j.ID))
	store.Delete(types.CreatorJobKey(j.Creator, j.ID))
}

// ─── Iterators ───────────────────────────────────────────────────────────────

// IterateJobsDueAt walks every Job whose ExecuteAtBlock equals the given height,
// invoking `cb(j)` for each. Returning true stops the iteration. Used by the
// EndBlocker (see keeper/abci.go) to dispatch due jobs.
func (k Keeper) IterateJobsDueAt(ctx sdk.Context, height uint64, cb func(j types.Job) bool) {
	store := ctx.KVStore(k.storeKey)
	iter := sdk.KVStorePrefixIterator(store, types.JobByDueBlockPrefix(height))
	defer iter.Close()

	for ; iter.Valid(); iter.Next() {
		var j types.Job
		if err := json.Unmarshal(iter.Value(), &j); err != nil {
			// Skip malformed entries — should never happen but don't halt.
			continue
		}
		if cb(j) {
			return
		}
	}
}

// PendingJobIDsByCreator returns the list of all pending job IDs for a given
// creator EVM address (20 bytes). Order is by job ID ascending.
func (k Keeper) PendingJobIDsByCreator(ctx sdk.Context, creator []byte) []uint64 {
	store := ctx.KVStore(k.storeKey)
	iter := sdk.KVStorePrefixIterator(store, types.CreatorJobsPrefix(creator))
	defer iter.Close()

	prefixLen := len(types.CreatorJobIndexPrefix) + len(creator)
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

// CountJobsByCreator returns the number of pending jobs for a creator. Used to
// enforce Params.MaxJobsPerCreator at schedule time.
func (k Keeper) CountJobsByCreator(ctx sdk.Context, creator []byte) uint32 {
	store := ctx.KVStore(k.storeKey)
	iter := sdk.KVStorePrefixIterator(store, types.CreatorJobsPrefix(creator))
	defer iter.Close()

	var count uint32
	for ; iter.Valid(); iter.Next() {
		count++
	}
	return count
}

// ─── ID allocation ───────────────────────────────────────────────────────────

// NextJobID atomically allocates the next monotonic job ID and persists the
// successor for the next caller. Genesis seeds this to 1 (see types.DefaultGenesisState).
func (k Keeper) NextJobID(ctx sdk.Context) uint64 {
	store := ctx.KVStore(k.storeKey)
	bz := store.Get(types.NextJobIDKey)

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
	store.Set(types.NextJobIDKey, buf[:])

	return next
}

// SetNextJobID is used by InitGenesis to restore the counter.
func (k Keeper) SetNextJobID(ctx sdk.Context, next uint64) {
	store := ctx.KVStore(k.storeKey)
	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], next)
	store.Set(types.NextJobIDKey, buf[:])
}

// GetNextJobID returns the current counter without advancing it. Used by
// ExportGenesis.
func (k Keeper) GetNextJobID(ctx sdk.Context) uint64 {
	store := ctx.KVStore(k.storeKey)
	bz := store.Get(types.NextJobIDKey)
	if bz == nil {
		return 1
	}
	return binary.BigEndian.Uint64(bz)
}

// GetAllJobs returns every pending Job in the store (id-keyed). Used only by
// ExportGenesis. Do not call on the hot path.
func (k Keeper) GetAllJobs(ctx sdk.Context) []types.Job {
	store := ctx.KVStore(k.storeKey)
	iter := sdk.KVStorePrefixIterator(store, types.JobByIDKeyPrefix)
	defer iter.Close()

	var jobs []types.Job
	for ; iter.Valid(); iter.Next() {
		var j types.Job
		if err := json.Unmarshal(iter.Value(), &j); err != nil {
			continue
		}
		jobs = append(jobs, j)
	}
	return jobs
}
