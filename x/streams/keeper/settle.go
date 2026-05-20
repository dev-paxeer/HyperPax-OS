// Copyright PaxLabs Ltd.(Paxeer Network)
// Paxeer Network Non-Commercial License 1.0 (ENCL-1.0)(https://github.com/Paxeer-Network/hyperpaxeer-os/blob/main/LICENSE_FAQ.md)

package keeper

// Streams settle/close/updateRate implementations live in settle_impl.go.
// Escrow / payout helpers live in escrow.go. This file is kept as a marker
// for the keeper package layout — imports stay in the implementation files
// to avoid scattering cosmos-sdk + go-ethereum dependencies across stubs.
