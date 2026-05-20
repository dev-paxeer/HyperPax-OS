// Copyright PaxLabs Ltd.(Paxeer Network)
// Paxeer Network Non-Commercial License 1.0 (ENCL-1.0)(https://github.com/Paxeer-Network/hyperpaxeer-os/blob/main/LICENSE_FAQ.md)


package network

import (
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types/v1"
	evmtypes "github.com/evmos/evmos/v18/x/evm/types"
	infltypes "github.com/evmos/evmos/v18/x/inflation/v1/types"
	revtypes "github.com/evmos/evmos/v18/x/revenue/v1/types"
)

func (n *IntegrationNetwork) UpdateEvmParams(params evmtypes.Params) error {
	return n.app.EvmKeeper.SetParams(n.ctx, params)
}

func (n *IntegrationNetwork) UpdateRevenueParams(params revtypes.Params) error {
	return n.app.RevenueKeeper.SetParams(n.ctx, params)
}

func (n *IntegrationNetwork) UpdateInflationParams(params infltypes.Params) error {
	return n.app.InflationKeeper.SetParams(n.ctx, params)
}

func (n *IntegrationNetwork) UpdateGovParams(params govtypes.Params) error {
	return n.app.GovKeeper.SetParams(n.ctx, params)
}
