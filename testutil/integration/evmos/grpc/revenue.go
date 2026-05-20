// Copyright PaxLabs Ltd.(Paxeer Network)
// Paxeer Network Non-Commercial License 1.0 (ENCL-1.0)(https://github.com/Paxeer-Network/hyperpaxeer-os/blob/main/LICENSE_FAQ.md)

package grpc

import (
	"context"

	"github.com/ethereum/go-ethereum/common"
	revtypes "github.com/evmos/evmos/v18/x/revenue/v1/types"
)

// GetRevenue returns the revenue for the given address.
func (gqh *IntegrationHandler) GetRevenue(address common.Address) (*revtypes.QueryRevenueResponse, error) {
	revenueClient := gqh.network.GetRevenueClient()
	return revenueClient.Revenue(context.Background(), &revtypes.QueryRevenueRequest{
		ContractAddress: address.String(),
	})
}

// GetRevenueParams gets the revenue module params.
func (gqh *IntegrationHandler) GetRevenueParams() (*revtypes.QueryParamsResponse, error) {
	revenueClient := gqh.network.GetRevenueClient()
	return revenueClient.Params(context.Background(), &revtypes.QueryParamsRequest{})
}
