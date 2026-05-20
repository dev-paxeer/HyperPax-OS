// Copyright PaxLabs Ltd.(Paxeer Network)
// Paxeer Network Non-Commercial License 1.0 (ENCL-1.0)(https://github.com/Paxeer-Network/hyperpaxeer-os/blob/main/LICENSE_FAQ.md)

package types

type CallType int

const (
	// RPC call type is used on requests to eth_estimateGas rpc API endpoint
	RPC CallType = iota + 1
	// Internal call type is used in case of smart contract methods calls
	Internal
)
