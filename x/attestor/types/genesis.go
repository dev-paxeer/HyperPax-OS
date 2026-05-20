// Copyright PaxLabs Ltd.(Paxeer Network)
// Paxeer Network Non-Commercial License 1.0 (ENCL-1.0)(https://github.com/Paxeer-Network/hyperpaxeer-os/blob/main/LICENSE_FAQ.md)


package types

import "fmt"

// FamilyRoots groups the trusted roots for a single TEE family.
type FamilyRoots struct {
	Family uint8    `json:"family"`
	Roots  [][]byte `json:"roots"` // PEM-encoded certs OR raw DER pubkeys
}

// GenesisState defines the attestor module's genesis state.
//
// In v21 genesis ships with NO roots — they're loaded post-upgrade via
// MsgUpdateTEERoots gov proposals (D4 locked). This keeps the upgrade itself
// deterministic and reviewable.
type GenesisState struct {
	Params      Params        `json:"params"`
	FamilyRoots []FamilyRoots `json:"family_roots"`
}

// DefaultGenesisState returns the default genesis state — empty root sets.
func DefaultGenesisState() *GenesisState {
	return &GenesisState{
		Params:      DefaultParams(),
		FamilyRoots: []FamilyRoots{},
	}
}

// Validate performs basic genesis state validation.
func (gs GenesisState) Validate() error {
	if err := gs.Params.Validate(); err != nil {
		return fmt.Errorf("invalid params: %w", err)
	}

	seen := make(map[uint8]bool)
	for _, fr := range gs.FamilyRoots {
		if fr.Family > FamilyMax {
			return fmt.Errorf("invalid family id %d", fr.Family)
		}
		if seen[fr.Family] {
			return fmt.Errorf("duplicate family roots entry for family %d", fr.Family)
		}
		seen[fr.Family] = true
		for i, r := range fr.Roots {
			if len(r) == 0 {
				return fmt.Errorf("empty root at family=%d index=%d", fr.Family, i)
			}
		}
	}
	return nil
}
