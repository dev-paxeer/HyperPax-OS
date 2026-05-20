// Copyright PaxLabs Ltd.(Paxeer Network)
// Paxeer Network Non-Commercial License 1.0 (ENCL-1.0)(https://github.com/Paxeer-Network/hyperpaxeer-os/blob/main/LICENSE_FAQ.md)

package types

import (
	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	proto "github.com/cosmos/gogoproto/proto"
)

// ModuleCdc references the global attestor module codec.
var ModuleCdc = codec.NewLegacyAmino()

func init() {
	RegisterLegacyAminoCodec(ModuleCdc)
	proto.RegisterType((*MsgUpdateTEERoots)(nil), "attestor.MsgUpdateTEERoots")
}

// RegisterLegacyAminoCodec registers MsgUpdateTEERoots on the legacy amino
// codec. The route name matches the gov proposal display string.
func RegisterLegacyAminoCodec(cdc *codec.LegacyAmino) {
	cdc.RegisterConcrete(&MsgUpdateTEERoots{}, "attestor/MsgUpdateTEERoots", nil)
}

// RegisterInterfaces registers the attestor module message types in the
// interface registry.
func RegisterInterfaces(registry codectypes.InterfaceRegistry) {
	registry.RegisterImplementations((*sdk.Msg)(nil),
		&MsgUpdateTEERoots{},
	)
}
