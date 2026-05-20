package types

import (
	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	proto "github.com/cosmos/gogoproto/proto"
)

// ModuleCdc references the global paxoracle module codec.
var ModuleCdc = codec.NewLegacyAmino()

func init() {
	RegisterLegacyAminoCodec(ModuleCdc)
	proto.RegisterType((*MsgSubmitPrice)(nil), "paxoracle.MsgSubmitPrice")
}

// RegisterLegacyAminoCodec registers the necessary paxoracle interfaces and
// concrete types on the provided LegacyAmino codec.
func RegisterLegacyAminoCodec(cdc *codec.LegacyAmino) {
	cdc.RegisterConcrete(&MsgSubmitPrice{}, "paxoracle/MsgSubmitPrice", nil)
}

// RegisterInterfaces registers the paxoracle module message types in the interface registry.
func RegisterInterfaces(registry types.InterfaceRegistry) {
	registry.RegisterImplementations((*sdk.Msg)(nil),
		&MsgSubmitPrice{},
	)
}
