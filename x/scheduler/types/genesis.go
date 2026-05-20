// Copyright PaxLabs Ltd.(Paxeer Network)
// Paxeer Network Non-Commercial License 1.0 (ENCL-1.0)(https://github.com/Paxeer-Network/hyperpaxeer-os/blob/main/LICENSE_FAQ.md)


package types

import "fmt"

// GenesisState defines the scheduler module's genesis state.
type GenesisState struct {
	Params    Params `json:"params"`
	NextJobID uint64 `json:"next_job_id"`
	Jobs      []Job  `json:"jobs"`
}

// DefaultGenesisState returns the default genesis state.
func DefaultGenesisState() *GenesisState {
	return &GenesisState{
		Params:    DefaultParams(),
		NextJobID: 1,
		Jobs:      []Job{},
	}
}

// Validate performs basic genesis state validation.
func (gs GenesisState) Validate() error {
	if err := gs.Params.Validate(); err != nil {
		return fmt.Errorf("invalid params: %w", err)
	}

	seen := make(map[uint64]bool)
	for _, j := range gs.Jobs {
		if j.ID == 0 {
			return fmt.Errorf("job has zero id")
		}
		if seen[j.ID] {
			return fmt.Errorf("duplicate job id %d in genesis", j.ID)
		}
		seen[j.ID] = true
		if j.ID >= gs.NextJobID {
			return fmt.Errorf("job id %d >= next_job_id %d", j.ID, gs.NextJobID)
		}
	}

	return nil
}
