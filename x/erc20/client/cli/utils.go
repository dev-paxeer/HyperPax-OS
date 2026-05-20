// Copyright PaxLabs Ltd.(Paxeer Network)
// Paxeer Network Non-Commercial License 1.0 (ENCL-1.0)(https://github.com/Paxeer-Network/hyperpaxeer-os/blob/main/LICENSE_FAQ.md)


package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/cosmos/cosmos-sdk/codec"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	"github.com/evmos/evmos/v18/x/erc20/types"
)

// ParseRegisterCoinProposal reads and parses a ParseRegisterCoinProposal from a file.
func ParseMetadata(cdc codec.JSONCodec, metadataFile string) ([]banktypes.Metadata, error) {
	proposalMetadata := types.ProposalMetadata{}

	contents, err := os.ReadFile(filepath.Clean(metadataFile))
	if err != nil {
		return nil, err
	}

	if err = cdc.UnmarshalJSON(contents, &proposalMetadata); err != nil {
		return nil, fmt.Errorf("failed to unmarshal proposal metadata: %w", err)
	}

	return proposalMetadata.Metadata, nil
}
