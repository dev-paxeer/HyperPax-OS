// SPDX-License-Identifier: MIT
pragma solidity ^0.8.24;

import "forge-std/Script.sol";
import "../src/core/OracleAdapter.sol";
import "../src/core/SettlementEngine.sol";
import "../src/core/MatchingEngine.sol";
import "../src/core/PaxSpotRouter.sol";

contract DeployPaxSpot is Script {
    // ─── Configuration (override via env vars) ──────────────────────────
    // PYTH_ADDRESS: address of Pyth oracle (default: address(0) = VOM-only mode)
    // STALE_THRESHOLD: staleness threshold in seconds (default: 120)
    // MIN_VOM_QUORUM: minimum VOM attestations (default: 1)
    // EPOCH_LENGTH: blocks per settlement epoch (default: 5 = ~10s)
    // TREASURY: treasury address for fast-settle fees
    // ORDER_COOLDOWN: blocks between orders per address (default: 3)

    function run() external {
        uint256 deployerKey = vm.envUint("PRIVATE_KEY");
        address deployer = vm.addr(deployerKey);

        // Read configuration from environment
        address pythAddress = vm.envOr("PYTH_ADDRESS", address(0));
        uint256 staleThreshold = vm.envOr("STALE_THRESHOLD", uint256(120));
        uint256 minVomQuorum = vm.envOr("MIN_VOM_QUORUM", uint256(1));
        uint256 epochLength = vm.envOr("EPOCH_LENGTH", uint256(5));
        address treasury = vm.envOr("TREASURY", deployer);
        uint256 orderCooldown = vm.envOr("ORDER_COOLDOWN", uint256(3));

        console.log("=== PaxSpot Mainnet Deployment ===");
        console.log("Deployer:", deployer);
        console.log("Pyth:", pythAddress);
        if (pythAddress == address(0)) {
            console.log("  -> VOM-only mode (no Pyth)");
        }
        console.log("Stale threshold:", staleThreshold);
        console.log("Min VOM quorum:", minVomQuorum);
        console.log("Epoch length:", epochLength);
        console.log("Treasury:", treasury);
        console.log("Order cooldown:", orderCooldown);

        vm.startBroadcast(deployerKey);

        // 1. OracleAdapter (address(0) = VOM-only, no Pyth dependency)
        OracleAdapter oracle = new OracleAdapter(pythAddress, staleThreshold, minVomQuorum);
        console.log("OracleAdapter deployed at:", address(oracle));

        // 2. SettlementEngine
        SettlementEngine settlement = new SettlementEngine(epochLength, treasury);
        console.log("SettlementEngine deployed at:", address(settlement));

        // 3. MatchingEngine
        MatchingEngine matching = new MatchingEngine(address(oracle), address(settlement));
        console.log("MatchingEngine deployed at:", address(matching));

        // 4. PaxSpotRouter
        PaxSpotRouter router = new PaxSpotRouter(address(matching), address(settlement), orderCooldown);
        console.log("PaxSpotRouter deployed at:", address(router));

        // 5. Wire contracts together
        settlement.setMatchingEngine(address(matching));
        matching.setRouter(address(router));
        console.log("Contracts wired: settlement<->matching<->router");

        vm.stopBroadcast();

        // Write addresses to JSON for downstream consumption
        string memory json = string.concat(
            '{"OracleAdapter":"', vm.toString(address(oracle)),
            '","SettlementEngine":"', vm.toString(address(settlement)),
            '","MatchingEngine":"', vm.toString(address(matching)),
            '","PaxSpotRouter":"', vm.toString(address(router)),
            '"}'
        );
        vm.writeFile("scripts/deployed_addresses.json", json);
        console.log("Addresses written to scripts/deployed_addresses.json");
    }
}
