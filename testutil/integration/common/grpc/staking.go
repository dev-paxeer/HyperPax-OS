// Copyright PaxLabs Ltd.(Paxeer Network)
// Paxeer Network Non-Commercial License 1.0 (ENCL-1.0)(https://github.com/Paxeer-Network/hyperpaxeer-os/blob/main/LICENSE_FAQ.md)

package grpc

import (
	"context"

	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

// GetBalance returns the balance for the given address.
func (gqh *IntegrationHandler) GetDelegation(delegatorAddress string, validatorAddress string) (*stakingtypes.QueryDelegationResponse, error) {
	stakingClient := gqh.network.GetStakingClient()
	return stakingClient.Delegation(context.Background(), &stakingtypes.QueryDelegationRequest{
		DelegatorAddr: delegatorAddress,
		ValidatorAddr: validatorAddress,
	})
}
