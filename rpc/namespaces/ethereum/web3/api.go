// Copyright PaxLabs Ltd.(Paxeer Network)
// Paxeer Network Non-Commercial License 1.0 (ENCL-1.0)(https://github.com/Paxeer-Network/hyperpaxeer-os/blob/main/LICENSE_FAQ.md)

package web3

import (
	"github.com/evmos/evmos/v18/version"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
)

// PublicAPI is the web3_ prefixed set of APIs in the Web3 JSON-RPC spec.
type PublicAPI struct{}

// NewPublicAPI creates an instance of the Web3 API.
func NewPublicAPI() *PublicAPI {
	return &PublicAPI{}
}

// ClientVersion returns the client version in the Web3 user agent format.
func (a *PublicAPI) ClientVersion() string {
	return version.Version()
}

// Sha3 returns the keccak-256 hash of the passed-in input.
func (a *PublicAPI) Sha3(input string) hexutil.Bytes {
	return crypto.Keccak256(hexutil.Bytes(input))
}
