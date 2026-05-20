// Copyright PaxLabs Ltd.(Paxeer Network)
// Paxeer Network Non-Commercial License 1.0 (ENCL-1.0)(https://github.com/Paxeer-Network/hyperpaxeer-os/blob/main/LICENSE_FAQ.md)


package v21agent

const (
	// UpgradeName is the shared upgrade plan name for mainnet.
	UpgradeName = "v21-agent-payments"

	// MainnetUpgradeHeight is the HyperPaxeer mainnet block height at which the
	// upgrade will halt. Set when the v21 release binary is tagged.
	//
	// TODO(v21): set the real height before building the release binary.
	MainnetUpgradeHeight = 0

	// UpgradeInfo is the gov-proposal upgrade-info JSON announcing the binary
	// artifact for the upgrade. Substitute the final tag/checksum at release.
	UpgradeInfo = `'{"binaries":{"linux/amd64":"hyperpaxeer-v21-agent-payments-linux-amd64.tar.gz"}}'`
)

// NewlyActivePrecompiles is the list of precompile addresses to add to
// EVMParams.ActivePrecompiles during the v21 upgrade migration.
var NewlyActivePrecompiles = []string{
	"0x0000000000000000000000000000000000000906", // PaymentStreams
	"0x0000000000000000000000000000000000000907", // TEEAttestor
	"0x0000000000000000000000000000000000000908", // EIP712Helper
}
