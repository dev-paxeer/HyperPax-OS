// Copyright PaxLabs Ltd.(Paxeer Network)
// Paxeer Network Non-Commercial License 1.0 (ENCL-1.0)(https://github.com/Paxeer-Network/hyperpaxeer-os/blob/main/LICENSE_FAQ.md)

package vesting

const (
	// ErrDifferentFromOrigin is raised when the tx origin address is not the same as the vesting transaction initiator.
	ErrDifferentFromOrigin = "tx origin address %s does not match the from address %s"
	// ErrDifferentFunderOrigin is raised when the tx origin address is not the same as the vesting transaction funder.
	ErrDifferentFunderOrigin = "tx origin address %s does not match the funder address %s"
)
