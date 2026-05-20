// Copyright PaxLabs Ltd.(Paxeer Network)
// Paxeer Network Non-Commercial License 1.0 (ENCL-1.0)(https://github.com/Paxeer-Network/hyperpaxeer-os/blob/main/LICENSE_FAQ.md)


package types

import (
	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/codec/types"
)

// ModuleCdc references the global scheduler module codec.
var ModuleCdc = codec.NewLegacyAmino()

// RegisterLegacyAminoCodec registers the necessary scheduler interfaces and
// concrete types on the provided LegacyAmino codec. The scheduler module has no
// Cosmos Msgs in v20 — scheduling happens via the EVM precompile — so this is a
// no-op stub kept for AppModuleBasic compliance.
func RegisterLegacyAminoCodec(_ *codec.LegacyAmino) {}

// RegisterInterfaces registers the scheduler module message types in the
// interface registry. No Msgs in v20 → no-op.
func RegisterInterfaces(_ types.InterfaceRegistry) {}
