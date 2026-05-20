// Copyright PaxLabs Ltd.(Paxeer Network)
// Paxeer Network Non-Commercial License 1.0 (ENCL-1.0)(https://github.com/Paxeer-Network/hyperpaxeer-os/blob/main/LICENSE_FAQ.md)

package types

import paramtypes "github.com/cosmos/cosmos-sdk/x/params/types"

// Parameter keys
var (
	ParamStoreKeyEVMDenom            = []byte("EVMDenom")
	ParamStoreKeyEnableCreate        = []byte("EnableCreate")
	ParamStoreKeyEnableCall          = []byte("EnableCall")
	ParamStoreKeyExtraEIPs           = []byte("EnableExtraEIPs")
	ParamStoreKeyChainConfig         = []byte("ChainConfig")
	ParamStoreKeyAllowUnprotectedTxs = []byte("AllowUnprotectedTxs")
)

// Deprecated: ParamKeyTable returns the parameter key table.
// Usage of x/params to manage parameters is deprecated in favor of x/gov
// controlled execution of MsgUpdateParams messages. These types remain solely
// for migration purposes and will be removed in a future release.
func ParamKeyTable() paramtypes.KeyTable {
	return paramtypes.NewKeyTable().RegisterParamSet(&Params{})
}

// Deprecated: ParamSetPairs returns the parameter set pairs.
// Usage of x/params to manage parameters is deprecated in favor of x/gov
// controlled execution of MsgUpdateParams messages. These types remain solely
// for migration purposes and will be removed in a future release.
func (p *Params) ParamSetPairs() paramtypes.ParamSetPairs {
	return paramtypes.ParamSetPairs{
		paramtypes.NewParamSetPair(ParamStoreKeyEVMDenom, &p.EvmDenom, validateEVMDenom),
		paramtypes.NewParamSetPair(ParamStoreKeyEnableCreate, &p.EnableCreate, validateBool),
		paramtypes.NewParamSetPair(ParamStoreKeyEnableCall, &p.EnableCall, validateBool),
		paramtypes.NewParamSetPair(ParamStoreKeyExtraEIPs, &p.ExtraEIPs, validateEIPs),
		paramtypes.NewParamSetPair(ParamStoreKeyChainConfig, &p.ChainConfig, validateChainConfig),
		paramtypes.NewParamSetPair(ParamStoreKeyAllowUnprotectedTxs, &p.AllowUnprotectedTxs, validateBool),
	}
}
