// Copyright PaxLabs Ltd.(Paxeer Network)
// Paxeer Network Non-Commercial License 1.0 (ENCL-1.0)(https://github.com/Paxeer-Network/hyperpaxeer-os/blob/main/LICENSE_FAQ.md)

package coordinator

// Endpoint defines the identifiers for a chain's client, connection, and channel.
type Endpoint struct {
	ChainID      string
	ClientID     string
	ConnectionID string
	ChannelID    string
	PortID       string
}

// IBCConnection defines the connection between two chains.
type IBCConnection struct {
	EndpointA Endpoint
	EndpointB Endpoint
}
