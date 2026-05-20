// SPDX-License-Identifier: MIT
pragma solidity ^0.8.24;

import "forge-std/Test.sol";
import {MockPyth} from "./mocks/MockPyth.sol";
import {MockERC20} from "./mocks/MockERC20.sol";
import {MockAllowanceProvider} from "./mocks/MockAllowanceProvider.sol";
import {OracleAdapter} from "../src/core/OracleAdapter.sol";
import {SettlementEngine} from "../src/core/SettlementEngine.sol";
import {MatchingEngine} from "../src/core/MatchingEngine.sol";
import {PaxSpotRouter} from "../src/core/PaxSpotRouter.sol";
import {PaxSpotTypes} from "../src/core/PaxSpotTypes.sol";
import {IOROBResolver} from "../src/interfaces/precompiles/IOROBResolver.sol";
import {IBatchClearing} from "../src/interfaces/precompiles/IBatchClearing.sol";
import {IPoFQScorer} from "../src/interfaces/precompiles/IPoFQScorer.sol";
import {IOracleAggregator} from "../src/interfaces/precompiles/IOracleAggregator.sol";

/// @dev Shared test base with precompile mocking, fixture deployment, and helpers.
abstract contract PaxSpotTestBase is Test {
    // ─── Precompile Addresses ────────────────────────────────────────
    address constant OROB_ADDR = 0x0000000000000000000000000000000000000901;
    address constant CLEARING_ADDR = 0x0000000000000000000000000000000000000902;
    address constant VOM_ADDR = 0x0000000000000000000000000000000000000903;
    address constant POFQ_ADDR = 0x0000000000000000000000000000000000000904;

    // ─── Actors ──────────────────────────────────────────────────────
    address internal owner = makeAddr("owner");
    address internal alice = makeAddr("alice");
    address internal bob = makeAddr("bob");
    address internal treasury = makeAddr("treasury");
    address internal keeper = makeAddr("keeper");

    // ─── Market Constants ────────────────────────────────────────────
    bytes32 internal constant MARKET_ETH_USDC = keccak256("ETH/USDC");
    bytes32 internal constant PYTH_ETH_FEED = keccak256("pyth-eth-usd");
    bytes32 internal constant VOM_ETH_MARKET = keccak256("vom-eth-usdc");
    int256 internal constant ORACLE_PRICE = 2000e18; // $2000
    uint256 internal constant EPOCH_LENGTH = 5;

    // ─── Contracts ───────────────────────────────────────────────────
    MockPyth internal mockPyth;
    MockERC20 internal baseToken;
    MockERC20 internal quoteToken;
    MockAllowanceProvider internal mockAllowance;
    OracleAdapter internal oracleAdapter;
    SettlementEngine internal settlementEngine;
    MatchingEngine internal matchingEngine;
    PaxSpotRouter internal router;

    // ─── Setup Helpers ───────────────────────────────────────────────

    /// @dev Deploy mocks + OracleAdapter only. Use for OracleAdapter-scoped tests.
    function _deployOracle() internal {
        vm.startPrank(owner);
        mockPyth = new MockPyth();
        oracleAdapter = new OracleAdapter(address(mockPyth), 60, 1);
        vm.stopPrank();
    }

    /// @dev Deploy the full stack: oracle, settlement, matching, router.
    function _deployFullStack() internal {
        vm.startPrank(owner);

        // Mocks
        mockPyth = new MockPyth();
        baseToken = new MockERC20("Wrapped ETH", "WETH", 18);
        quoteToken = new MockERC20("USD Coin", "USDC", 6);
        mockAllowance = new MockAllowanceProvider();

        // Core contracts
        oracleAdapter = new OracleAdapter(address(mockPyth), 60, 1);
        settlementEngine = new SettlementEngine(EPOCH_LENGTH, treasury);
        matchingEngine = new MatchingEngine(address(oracleAdapter), address(settlementEngine));
        router = new PaxSpotRouter(address(matchingEngine), address(settlementEngine), 0);

        // Wire contracts
        settlementEngine.setMatchingEngine(address(matchingEngine));
        matchingEngine.setRouter(address(router));

        // Add tokens
        settlementEngine.addToken(address(baseToken));
        settlementEngine.addToken(address(quoteToken));

        vm.stopPrank();
    }

    /// @dev Register the ETH/USDC market on oracle + matching engine.
    function _registerMarket() internal {
        vm.startPrank(owner);
        oracleAdapter.registerMarket(MARKET_ETH_USDC, PYTH_ETH_FEED, VOM_ETH_MARKET);
        matchingEngine.createMarket(
            MARKET_ETH_USDC,
            address(baseToken),
            address(quoteToken),
            PaxSpotTypes.MarketMode.CONTINUOUS,
            1e15, // minOrderSize: 0.001 ETH
            500, // maxOffsetBps: 5%
            10, // takerFeeBps: 0.10%
            -5 // makerRebateBps: -0.05%
        );
        vm.stopPrank();
    }

    /// @dev Set Pyth price fresh (publishTime = block.timestamp).
    function _setPythFresh(int64 price, int32 expo) internal {
        mockPyth.setPrice(PYTH_ETH_FEED, price, 100, expo, block.timestamp);
    }

    /// @dev Set Pyth price stale (publishTime far in the past).
    function _setPythStale() internal {
        vm.warp(1000);
        mockPyth.setPrice(PYTH_ETH_FEED, 200000000, 100, -8, 0);
    }

    // ─── Precompile Mock Helpers ─────────────────────────────────────

    /// @dev Mock OROB resolveOffset: price + (price * offsetBps / 10000)
    function _mockOROBResolve(int256 oraclePrice, int16 offsetBps) internal {
        int256 result = oraclePrice + (oraclePrice * int256(offsetBps)) / 10000;
        vm.mockCall(
            OROB_ADDR,
            abi.encodeCall(IOROBResolver.resolveOffset, (oraclePrice, offsetBps)),
            abi.encode(result)
        );
    }

    /// @dev Mock OROB resolveOffset for any inputs -> returns a reasonable default.
    function _mockOROBResolveDefault() internal {
        // Fallback: return oraclePrice unchanged. Tests that care about specific values
        // should call _mockOROBResolve with exact args.
        vm.mockCall(
            OROB_ADDR,
            abi.encodeWithSelector(IOROBResolver.resolveOffset.selector),
            abi.encode(ORACLE_PRICE)
        );
    }

    /// @dev Mock PoFQ scoreFill -> returns perfect score.
    function _mockPoFQDefault() internal {
        vm.mockCall(
            POFQ_ADDR,
            abi.encodeWithSelector(IPoFQScorer.scoreFill.selector),
            abi.encode(uint256(1e18))
        );
        vm.mockCall(
            POFQ_ADDR,
            abi.encodeWithSelector(IPoFQScorer.updateRollingScore.selector),
            abi.encode(uint256(1e18), uint256(1e18))
        );
    }

    /// @dev Mock VOM getValidatorPrice.
    function _mockVOM(bytes32 marketId, int256 price, uint256 quorum, uint256 ts) internal {
        vm.mockCall(
            VOM_ADDR,
            abi.encodeCall(IOracleAggregator.getValidatorPrice, (marketId)),
            abi.encode(price, quorum, ts)
        );
    }

    /// @dev Mock BatchClearing computeClearing.
    function _mockBatchClearing(
        int16 clearingOffsetBps,
        int256 clearingPrice,
        uint256 matchedVolume
    ) internal {
        vm.mockCall(
            CLEARING_ADDR,
            abi.encodeWithSelector(IBatchClearing.computeClearing.selector),
            abi.encode(clearingOffsetBps, clearingPrice, matchedVolume)
        );
    }

    /// @dev Fund a user with tokens and approve the settlement engine.
    function _fundAndApprove(address user, uint256 baseAmt, uint256 quoteAmt) internal {
        baseToken.mint(user, baseAmt);
        quoteToken.mint(user, quoteAmt);
        vm.startPrank(user);
        baseToken.approve(address(settlementEngine), type(uint256).max);
        quoteToken.approve(address(settlementEngine), type(uint256).max);
        vm.stopPrank();
    }

    /// @dev Deposit tokens into settlement engine for a user.
    function _depositForUser(address user, uint256 baseAmt, uint256 quoteAmt) internal {
        vm.startPrank(user);
        if (baseAmt > 0) settlementEngine.deposit(address(baseToken), baseAmt);
        if (quoteAmt > 0) settlementEngine.deposit(address(quoteToken), quoteAmt);
        vm.stopPrank();
    }

    /// @dev Advance block number by `n` blocks.
    function _advanceBlocks(uint256 n) internal {
        vm.roll(block.number + n);
    }
}
