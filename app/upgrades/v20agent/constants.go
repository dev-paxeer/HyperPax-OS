// Copyright PaxLabs Ltd.(Paxeer Network)
// Paxeer Network Non-Commercial License 1.0 (ENCL-1.0)(https://github.com/Paxeer-Network/hyperpaxeer-os/blob/main/LICENSE_FAQ.md)


package v20agent

const (
	// UpgradeName is the shared upgrade plan name for mainnet.
	UpgradeName = "v20-agent-foundations"

	// MainnetUpgradeHeight is the HyperPaxeer mainnet block height at which the
	// upgrade will halt. Pick a height ~500 blocks ahead of the release binary
	// rollout (~17 min at 2s blocks) before tagging the release.
	//
	// TODO(v20): set the real height before building the release binary.
	MainnetUpgradeHeight = 0

	// EIP7702ActivationDelta is the number of blocks AFTER the upgrade height at
	// which EIP-7702 (SetCodeTx, type 0x04) becomes valid on consensus. Setting
	// this to a future block lets all validator binaries roll out before any
	// type-0x04 transaction can be accepted.
	//
	// 50_000 blocks ≈ 27.7 hours @ 2s blocks.
	EIP7702ActivationDelta = 50_000

	// UpgradeInfo is the gov-proposal upgrade-info JSON announcing the binary
	// artifact for the upgrade. Substitute the final tag/checksum at release.
	UpgradeInfo = `'{"binaries":{"linux/amd64":"hyperpaxeer-v20-agent-foundations-linux-amd64.tar.gz"}}'`
)

// NewlyActivePrecompiles is the list of precompile addresses to add to
// EVMParams.ActivePrecompiles during the v20 upgrade migration.
var NewlyActivePrecompiles = []string{
	"0x0000000000000000000000000000000000000905", // Scheduler
}
