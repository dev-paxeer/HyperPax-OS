// Copyright PaxLabs Ltd.(Paxeer Network)
// Paxeer Network Non-Commercial License 1.0 (ENCL-1.0)(https://github.com/Paxeer-Network/hyperpaxeer-os/blob/main/LICENSE_FAQ.md)


package types

import "fmt"

// GenesisState defines the streams module's genesis state.
type GenesisState struct {
	Params       Params   `json:"params"`
	NextStreamID uint64   `json:"next_stream_id"`
	Streams      []Stream `json:"streams"`
}

// DefaultGenesisState returns the default genesis state.
func DefaultGenesisState() *GenesisState {
	return &GenesisState{
		Params:       DefaultParams(),
		NextStreamID: 1,
		Streams:      []Stream{},
	}
}

// Validate performs basic genesis state validation.
func (gs GenesisState) Validate() error {
	if err := gs.Params.Validate(); err != nil {
		return fmt.Errorf("invalid params: %w", err)
	}

	seen := make(map[uint64]bool)
	for _, s := range gs.Streams {
		if s.ID == 0 {
			return fmt.Errorf("stream has zero id")
		}
		if seen[s.ID] {
			return fmt.Errorf("duplicate stream id %d in genesis", s.ID)
		}
		seen[s.ID] = true
		if s.ID >= gs.NextStreamID {
			return fmt.Errorf("stream id %d >= next_stream_id %d", s.ID, gs.NextStreamID)
		}
	}

	return nil
}
