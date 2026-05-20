// SPDX-License-Identifier: MIT
pragma solidity ^0.8.24;

import "forge-std/Script.sol";
import "../src/core/OracleAdapter.sol";
import "../src/core/SettlementEngine.sol";
import "../src/core/MatchingEngine.sol";
import "../src/core/PaxSpotRouter.sol";
import "../src/core/PaxSpotTypes.sol";
import {IPyth} from "../src/interfaces/external/IPyth.sol";
import {ERC20} from "@openzeppelin/contracts/token/ERC20/ERC20.sol";

// ─── Inline mocks (avoid cross-directory imports) ────────────────────────────

contract LocalMockPyth is IPyth {
    mapping(bytes32 => Price) internal _prices;

    function setPrice(bytes32 feedId, int64 price, uint64 conf, int32 expo, uint256 publishTime) external {
        _prices[feedId] = Price({price: price, conf: conf, expo: expo, publishTime: publishTime});
    }

    function getPriceUnsafe(bytes32 id) external view returns (Price memory price) {
        return _prices[id];
    }

    function getPriceNoOlderThan(bytes32 id, uint256 age) external view returns (Price memory price) {
        Price memory p = _prices[id];
        require(block.timestamp - p.publishTime <= age, "MockPyth: stale");
        return p;
    }

    function updatePriceFeeds(bytes[] calldata) external payable {}

    function getUpdateFee(bytes[] calldata) external pure returns (uint256 feeAmount) {
        return 0;
    }
}

contract LocalMockERC20 is ERC20 {
    uint8 internal _dec;

    constructor(string memory name_, string memory symbol_, uint8 decimals_) ERC20(name_, symbol_) {
        _dec = decimals_;
    }

    function mint(address to, uint256 amount) external {
        _mint(to, amount);
    }

    function decimals() public view override returns (uint8) {
        return _dec;
    }
}

// ─── Deploy script ───────────────────────────────────────────────────────────

contract DeployLocal is Script {
    function run() external {
        uint256 deployerKey = vm.envUint("PRIVATE_KEY");
        address deployer = vm.addr(deployerKey);

        uint256 staleThreshold = vm.envOr("STALE_THRESHOLD", uint256(120));
        uint256 minVomQuorum = vm.envOr("MIN_VOM_QUORUM", uint256(1));
        uint256 epochLength = vm.envOr("EPOCH_LENGTH", uint256(5));
        uint256 orderCooldown = vm.envOr("ORDER_COOLDOWN", uint256(0));

        console.log("=== PaxSpot Local Deployment ===");
        console.log("Deployer:", deployer);

        vm.startBroadcast(deployerKey);

        // ─── 1. Deploy mock infrastructure ──────────────────────────────
        LocalMockPyth mockPyth = new LocalMockPyth();
        console.log("MockPyth deployed at:", address(mockPyth));

        LocalMockERC20 baseToken = new LocalMockERC20("Test WETH", "tWETH", 18);
        console.log("tWETH deployed at:", address(baseToken));

        LocalMockERC20 quoteToken = new LocalMockERC20("Test USDC", "tUSDC", 6);
        console.log("tUSDC deployed at:", address(quoteToken));

        // ─── 2. Deploy PaxSpot core contracts ───────────────────────────
        OracleAdapter oracle = new OracleAdapter(address(mockPyth), staleThreshold, minVomQuorum);
        console.log("OracleAdapter deployed at:", address(oracle));

        SettlementEngine settlement = new SettlementEngine(epochLength, deployer);
        console.log("SettlementEngine deployed at:", address(settlement));

        MatchingEngine matching = new MatchingEngine(address(oracle), address(settlement));
        console.log("MatchingEngine deployed at:", address(matching));

        PaxSpotRouter router = new PaxSpotRouter(address(matching), address(settlement), orderCooldown);
        console.log("PaxSpotRouter deployed at:", address(router));

        // ─── 3. Wire contracts together ─────────────────────────────────
        // SettlementEngine needs to know the MatchingEngine
        settlement.setMatchingEngine(address(matching));
        console.log("SettlementEngine.matchingEngine set to:", address(matching));

        // MatchingEngine needs to know the Router
        matching.setRouter(address(router));
        console.log("MatchingEngine.router set to:", address(router));

        // ─── 4. Add supported tokens to SettlementEngine ───────────────
        settlement.addToken(address(baseToken));
        settlement.addToken(address(quoteToken));
        console.log("Tokens added to SettlementEngine");

        // ─── 5. Set up a test market: ETH/USDC ─────────────────────────
        bytes32 marketId = keccak256("ETH/USDC");
        bytes32 pythFeedId = bytes32(uint256(0xff61491a931112ddf1bd8147cd1b641375f79f5825126d665480874634fd0ace));

        // Register market on OracleAdapter
        oracle.registerMarket(marketId, pythFeedId, marketId);
        console.log("Market ETH/USDC registered on OracleAdapter");

        // Create market on MatchingEngine (continuous mode)
        matching.createMarket(
            marketId,
            address(baseToken),
            address(quoteToken),
            PaxSpotTypes.MarketMode.CONTINUOUS,
            uint128(1e15),   // minOrderSize: 0.001 ETH
            int16(500),      // maxOffsetBps: 5% from oracle
            uint16(5),       // takerFeeBps: 0.05%
            int16(2)         // makerRebateBps: 0.02%
        );
        console.log("Market ETH/USDC created on MatchingEngine (Continuous)");

        // ─── 6. Set initial mock price: ETH = $3500 ────────────────────
        // Pyth price: 3500_00000000 with expo -8 → $3500.00
        mockPyth.setPrice(
            pythFeedId,
            int64(350_000_000_000), // $3500.00 with 8 decimals
            uint64(100_000),        // confidence
            int32(-8),              // exponent
            block.timestamp         // publish time = now (fresh)
        );
        console.log("MockPyth price set: ETH = $3500.00");

        // ─── 7. Mint test tokens to deployer ────────────────────────────
        baseToken.mint(deployer, 1000 ether);          // 1000 tWETH
        quoteToken.mint(deployer, 10_000_000 * 1e6);   // 10M tUSDC
        console.log("Test tokens minted to deployer");

        vm.stopBroadcast();

        // ─── 8. Write addresses to JSON ─────────────────────────────────
        string memory json = string.concat(
            '{"MockPyth":"', vm.toString(address(mockPyth)),
            '","tWETH":"', vm.toString(address(baseToken)),
            '","tUSDC":"', vm.toString(address(quoteToken)),
            '","OracleAdapter":"', vm.toString(address(oracle)),
            '","SettlementEngine":"', vm.toString(address(settlement)),
            '","MatchingEngine":"', vm.toString(address(matching)),
            '","PaxSpotRouter":"', vm.toString(address(router)),
            '","marketId":"', vm.toString(marketId),
            '"}'
        );
        vm.writeFile("scripts/local_deployed.json", json);
        console.log("Addresses written to scripts/local_deployed.json");

        console.log("");
        console.log("=== Deployment complete ===");
        console.log("Next: run scripts/smoke_test.sh to verify");
    }
}
