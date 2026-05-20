// Copyright PaxLabs Ltd.(Paxeer Network)
// Paxeer Network Non-Commercial License 1.0 (ENCL-1.0)(https://github.com/Paxeer-Network/hyperpaxeer-os/blob/main/LICENSE_FAQ.md)


package keeper

import (
	"encoding/binary"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/evmos/evmos/v18/x/attestor/types"
)

// SetFamilyRoots replaces all roots for a given family in a single operation.
// This is the only way for state to mutate — gov-driven only — and matches the
// shape of the eventual `MsgUpdateTEERoots`.
//
// Returns an error if the family id is invalid.
func (k Keeper) SetFamilyRoots(ctx sdk.Context, family uint8, roots [][]byte) error {
	if family > types.FamilyMax {
		return types.ErrUnknownFamily
	}

	store := ctx.KVStore(k.storeKey)

	// Delete existing roots for the family.
	iter := sdk.KVStorePrefixIterator(store, types.RootsByFamilyPrefix(family))
	var oldKeys [][]byte
	for ; iter.Valid(); iter.Next() {
		oldKeys = append(oldKeys, append([]byte{}, iter.Key()...))
	}
	iter.Close()
	for _, k := range oldKeys {
		store.Delete(k)
	}

	// Write new roots.
	for i, r := range roots {
		store.Set(types.RootKey(family, uint32(i)), r)
	}

	// Update count.
	var buf [4]byte
	binary.BigEndian.PutUint32(buf[:], uint32(len(roots)))
	store.Set(types.RootCountKey(family), buf[:])

	return nil
}

// RootOf returns the i-th trusted root for the given family. Used by the
// precompile's `rootOf(family, index)` view method.
func (k Keeper) RootOf(ctx sdk.Context, family uint8, index uint32) ([]byte, bool) {
	if family > types.FamilyMax {
		return nil, false
	}
	store := ctx.KVStore(k.storeKey)
	bz := store.Get(types.RootKey(family, index))
	if bz == nil {
		return nil, false
	}
	return bz, true
}

// RootCount returns the number of trusted roots loaded for the family.
func (k Keeper) RootCount(ctx sdk.Context, family uint8) uint32 {
	if family > types.FamilyMax {
		return 0
	}
	store := ctx.KVStore(k.storeKey)
	bz := store.Get(types.RootCountKey(family))
	if bz == nil {
		return 0
	}
	return binary.BigEndian.Uint32(bz)
}

// RootsForFamily returns all roots for a family in index order. Used by the
// per-family verifiers (e.g. intel_tdx.go) to assemble the X.509 trust pool at
// verify time. NOT a hot-path operation — caller should cache per-block if it
// matters.
func (k Keeper) RootsForFamily(ctx sdk.Context, family uint8) [][]byte {
	count := k.RootCount(ctx, family)
	if count == 0 {
		return nil
	}
	roots := make([][]byte, 0, count)
	for i := uint32(0); i < count; i++ {
		if r, ok := k.RootOf(ctx, family, i); ok {
			roots = append(roots, r)
		}
	}
	return roots
}

// AllFamilyRoots returns all roots for all families. Used by ExportGenesis.
func (k Keeper) AllFamilyRoots(ctx sdk.Context) []types.FamilyRoots {
	out := make([]types.FamilyRoots, 0, types.FamilyMax+1)
	for fam := uint8(0); fam <= types.FamilyMax; fam++ {
		roots := k.RootsForFamily(ctx, fam)
		if len(roots) > 0 {
			out = append(out, types.FamilyRoots{Family: fam, Roots: roots})
		}
	}
	return out
}
