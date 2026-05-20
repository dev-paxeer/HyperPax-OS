package v19paxspot


const (
	// UpgradeName is the shared upgrade plan name for mainnet
	UpgradeName = "v19-paxspot"
	// MainnetUpgradeHeight defines the HyperPaxeer mainnet block height on which the upgrade will take place
	// NOTE: Set to the target halt height before building the release binary.
	// Current mainnet height is ~3,837,773. Pick a height ~500 blocks ahead (~17 min at 2s blocks).
	MainnetUpgradeHeight = 3_840_000
	// UpgradeInfo defines the binaries that will be used for the upgrade
	UpgradeInfo = `'{"binaries":{"linux/amd64":"hyperpaxeer-v19-paxspot-linux-amd64.tar.gz"}}'`
)
