// Copyright PaxLabs Ltd.(Paxeer Network)
// Paxeer Network Non-Commercial License 1.0 (ENCL-1.0)(https://github.com/Paxeer-Network/hyperpaxeer-os/blob/main/LICENSE_FAQ.md)


package types

import (
	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/codec/types"
)

// ModuleCdc references the global streams module codec.
var ModuleCdc = codec.NewLegacyAmino()

// RegisterLegacyAminoCodec is a no-op; the streams module exposes no Cosmos
// Msgs in v21 (everything happens via the EVM precompile at 0x0906).
func RegisterLegacyAminoCodec(_ *codec.LegacyAmino) {}

// RegisterInterfaces is a no-op; see RegisterLegacyAminoCodec.
func RegisterInterfaces(_ types.InterfaceRegistry) {}
