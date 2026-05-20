// SPDX-License-Identifier: MIT
pragma solidity ^0.8.24;

import "../PaxSpotTestBase.sol";

contract OracleAdapterTest is PaxSpotTestBase {
    function setUp() public {
        _deployOracle();
    }

    // ─── Constructor ─────────────────────────────────────────────────

    function test_Constructor_SetsStateCorrectly() public view {
        assertEq(address(oracleAdapter.pyth()), address(mockPyth));
        assertEq(oracleAdapter.staleThreshold(), 60);
        assertEq(oracleAdapter.minVomQuorum(), 1);
        assertEq(oracleAdapter.owner(), owner);
    }

    function test_RevertWhen_ConstructorZeroPyth() public {
        vm.prank(owner);
        vm.expectRevert(OracleAdapter.InvalidPythAddress.selector);
        new OracleAdapter(address(0), 60, 1);
    }

    function test_RevertWhen_ConstructorZeroStaleThreshold() public {
        vm.prank(owner);
        vm.expectRevert(OracleAdapter.InvalidStaleThreshold.selector);
        new OracleAdapter(address(mockPyth), 0, 1);
    }

    function test_RevertWhen_ConstructorZeroQuorum() public {
        vm.prank(owner);
        vm.expectRevert(OracleAdapter.InvalidQuorum.selector);
        new OracleAdapter(address(mockPyth), 60, 0);
    }

    // ─── registerMarket ──────────────────────────────────────────────

    function test_RegisterMarket_Success() public {
        vm.prank(owner);
        vm.expectEmit(true, false, false, true);
        emit OracleAdapter.MarketRegistered(MARKET_ETH_USDC, PYTH_ETH_FEED, VOM_ETH_MARKET);
        oracleAdapter.registerMarket(MARKET_ETH_USDC, PYTH_ETH_FEED, VOM_ETH_MARKET);

        (bytes32 pythFeedId, bytes32 vomMarketId, bool active) = oracleAdapter.markets(MARKET_ETH_USDC);
        assertTrue(active);
        assertEq(pythFeedId, PYTH_ETH_FEED);
        assertEq(vomMarketId, VOM_ETH_MARKET);
        assertEq(oracleAdapter.getMarketCount(), 1);
    }

    function test_RevertWhen_RegisterMarketAlreadyRegistered() public {
        vm.startPrank(owner);
        oracleAdapter.registerMarket(MARKET_ETH_USDC, PYTH_ETH_FEED, VOM_ETH_MARKET);
        vm.expectRevert(abi.encodeWithSelector(OracleAdapter.MarketAlreadyRegistered.selector, MARKET_ETH_USDC));
        oracleAdapter.registerMarket(MARKET_ETH_USDC, PYTH_ETH_FEED, VOM_ETH_MARKET);
        vm.stopPrank();
    }

    function test_RevertWhen_RegisterMarketNotOwner() public {
        vm.prank(alice);
        vm.expectRevert();
        oracleAdapter.registerMarket(MARKET_ETH_USDC, PYTH_ETH_FEED, VOM_ETH_MARKET);
    }

    // ─── deactivateMarket ────────────────────────────────────────────

    function test_DeactivateMarket_Success() public {
        vm.startPrank(owner);
        oracleAdapter.registerMarket(MARKET_ETH_USDC, PYTH_ETH_FEED, VOM_ETH_MARKET);
        vm.expectEmit(true, false, false, false);
        emit OracleAdapter.MarketDeactivated(MARKET_ETH_USDC);
        oracleAdapter.deactivateMarket(MARKET_ETH_USDC);
        vm.stopPrank();

        (,, bool active) = oracleAdapter.markets(MARKET_ETH_USDC);
        assertFalse(active);
    }

    function test_RevertWhen_DeactivateInactiveMarket() public {
        vm.prank(owner);
        vm.expectRevert(abi.encodeWithSelector(OracleAdapter.MarketNotActive.selector, MARKET_ETH_USDC));
        oracleAdapter.deactivateMarket(MARKET_ETH_USDC);
    }

    // ─── getPrice — Pyth fresh ───────────────────────────────────────

    function test_GetPrice_ReturnsPythWhenFresh() public {
        vm.prank(owner);
        oracleAdapter.registerMarket(MARKET_ETH_USDC, PYTH_ETH_FEED, VOM_ETH_MARKET);

        _setPythFresh(200000000000, -8); // $2000.00000000

        (int256 price, uint256 confidence, bool isFallback) = oracleAdapter.getPrice(MARKET_ETH_USDC);

        assertEq(price, 2000e18);
        assertEq(confidence, oracleAdapter.CONFIDENCE_HIGH());
        assertFalse(isFallback);
    }

    // ─── getPrice — Pyth stale, VOM fallback ─────────────────────────

    function test_GetPrice_FallsBackToVOMWhenPythStale() public {
        vm.prank(owner);
        oracleAdapter.registerMarket(MARKET_ETH_USDC, PYTH_ETH_FEED, VOM_ETH_MARKET);

        _setPythStale();
        _mockVOM(VOM_ETH_MARKET, 1999e18, 3, block.number);

        (int256 price, uint256 confidence, bool isFallback) = oracleAdapter.getPrice(MARKET_ETH_USDC);

        assertEq(price, 1999e18);
        assertEq(confidence, oracleAdapter.CONFIDENCE_REDUCED());
        assertTrue(isFallback);
    }

    // ─── getPrice — both unavailable ─────────────────────────────────

    function test_RevertWhen_BothOraclesUnavailable() public {
        vm.prank(owner);
        oracleAdapter.registerMarket(MARKET_ETH_USDC, PYTH_ETH_FEED, VOM_ETH_MARKET);

        _setPythStale();
        _mockVOM(VOM_ETH_MARKET, 0, 0, 0); // no quorum

        vm.expectRevert(abi.encodeWithSelector(OracleAdapter.OracleUnavailable.selector, MARKET_ETH_USDC));
        oracleAdapter.getPrice(MARKET_ETH_USDC);
    }

    // ─── getPrice — market not active ────────────────────────────────

    function test_RevertWhen_GetPriceMarketNotActive() public {
        vm.expectRevert(abi.encodeWithSelector(OracleAdapter.MarketNotActive.selector, MARKET_ETH_USDC));
        oracleAdapter.getPrice(MARKET_ETH_USDC);
    }

    // ─── getPrice — paused ───────────────────────────────────────────

    function test_RevertWhen_GetPriceWhenPaused() public {
        vm.startPrank(owner);
        oracleAdapter.registerMarket(MARKET_ETH_USDC, PYTH_ETH_FEED, VOM_ETH_MARKET);
        oracleAdapter.pause();
        vm.stopPrank();

        vm.expectRevert();
        oracleAdapter.getPrice(MARKET_ETH_USDC);
    }

    // ─── getPythPrice ────────────────────────────────────────────────

    function test_GetPythPrice_Success() public {
        vm.prank(owner);
        oracleAdapter.registerMarket(MARKET_ETH_USDC, PYTH_ETH_FEED, VOM_ETH_MARKET);
        _setPythFresh(200000000000, -8);

        int256 price = oracleAdapter.getPythPrice(MARKET_ETH_USDC);
        assertEq(price, 2000e18);
    }

    function test_RevertWhen_GetPythPriceStale() public {
        vm.prank(owner);
        oracleAdapter.registerMarket(MARKET_ETH_USDC, PYTH_ETH_FEED, VOM_ETH_MARKET);
        _setPythStale();

        vm.expectRevert(abi.encodeWithSelector(OracleAdapter.OracleUnavailable.selector, MARKET_ETH_USDC));
        oracleAdapter.getPythPrice(MARKET_ETH_USDC);
    }

    // ─── getVOMPrice ─────────────────────────────────────────────────

    function test_GetVOMPrice_Success() public {
        vm.prank(owner);
        oracleAdapter.registerMarket(MARKET_ETH_USDC, PYTH_ETH_FEED, VOM_ETH_MARKET);
        _mockVOM(VOM_ETH_MARKET, 2001e18, 5, block.number);

        (int256 price, uint256 quorum) = oracleAdapter.getVOMPrice(MARKET_ETH_USDC);
        assertEq(price, 2001e18);
        assertEq(quorum, 5);
    }

    function test_RevertWhen_GetVOMPriceInsufficientQuorum() public {
        vm.prank(owner);
        oracleAdapter.registerMarket(MARKET_ETH_USDC, PYTH_ETH_FEED, VOM_ETH_MARKET);
        _mockVOM(VOM_ETH_MARKET, 2001e18, 0, block.number);

        vm.expectRevert(abi.encodeWithSelector(OracleAdapter.InsufficientVomQuorum.selector, 0, 1));
        oracleAdapter.getVOMPrice(MARKET_ETH_USDC);
    }

    // ─── isPythFresh ─────────────────────────────────────────────────

    function test_IsPythFresh_ReturnsTrueWhenFresh() public {
        vm.prank(owner);
        oracleAdapter.registerMarket(MARKET_ETH_USDC, PYTH_ETH_FEED, VOM_ETH_MARKET);
        _setPythFresh(200000000000, -8);

        assertTrue(oracleAdapter.isPythFresh(MARKET_ETH_USDC));
    }

    function test_IsPythFresh_ReturnsFalseWhenStale() public {
        vm.prank(owner);
        oracleAdapter.registerMarket(MARKET_ETH_USDC, PYTH_ETH_FEED, VOM_ETH_MARKET);
        _setPythStale();

        assertFalse(oracleAdapter.isPythFresh(MARKET_ETH_USDC));
    }

    function test_IsPythFresh_ReturnsFalseWhenMarketNotActive() public {
        assertFalse(oracleAdapter.isPythFresh(MARKET_ETH_USDC));
    }

    // ─── Admin — setStaleThreshold ───────────────────────────────────

    function test_SetStaleThreshold_Success() public {
        vm.prank(owner);
        vm.expectEmit(false, false, false, true);
        emit OracleAdapter.StaleThresholdUpdated(60, 120);
        oracleAdapter.setStaleThreshold(120);

        assertEq(oracleAdapter.staleThreshold(), 120);
    }

    function test_RevertWhen_SetStaleThresholdZero() public {
        vm.prank(owner);
        vm.expectRevert(OracleAdapter.InvalidStaleThreshold.selector);
        oracleAdapter.setStaleThreshold(0);
    }

    function test_RevertWhen_SetStaleThresholdNotOwner() public {
        vm.prank(alice);
        vm.expectRevert();
        oracleAdapter.setStaleThreshold(120);
    }

    // ─── Admin — setMinVomQuorum ─────────────────────────────────────

    function test_SetMinVomQuorum_Success() public {
        vm.prank(owner);
        vm.expectEmit(false, false, false, true);
        emit OracleAdapter.MinVomQuorumUpdated(1, 3);
        oracleAdapter.setMinVomQuorum(3);

        assertEq(oracleAdapter.minVomQuorum(), 3);
    }

    function test_RevertWhen_SetMinVomQuorumZero() public {
        vm.prank(owner);
        vm.expectRevert(OracleAdapter.InvalidQuorum.selector);
        oracleAdapter.setMinVomQuorum(0);
    }

    // ─── Admin — setPythAddress ──────────────────────────────────────

    function test_SetPythAddress_Success() public {
        address newPyth = makeAddr("newPyth");
        vm.prank(owner);
        vm.expectEmit(false, false, false, true);
        emit OracleAdapter.PythAddressUpdated(address(mockPyth), newPyth);
        oracleAdapter.setPythAddress(newPyth);

        assertEq(address(oracleAdapter.pyth()), newPyth);
    }

    function test_RevertWhen_SetPythAddressZero() public {
        vm.prank(owner);
        vm.expectRevert(OracleAdapter.InvalidPythAddress.selector);
        oracleAdapter.setPythAddress(address(0));
    }

    // ─── Admin — pause / unpause ─────────────────────────────────────

    function test_PauseUnpause() public {
        vm.startPrank(owner);
        oracleAdapter.pause();
        oracleAdapter.registerMarket(MARKET_ETH_USDC, PYTH_ETH_FEED, VOM_ETH_MARKET);
        vm.stopPrank();

        _setPythFresh(200000000000, -8);
        vm.expectRevert();
        oracleAdapter.getPrice(MARKET_ETH_USDC);

        vm.prank(owner);
        oracleAdapter.unpause();

        (int256 price,,) = oracleAdapter.getPrice(MARKET_ETH_USDC);
        assertEq(price, 2000e18);
    }

    // ─── Pyth reverts gracefully ─────────────────────────────────────

    function test_GetPrice_PythRevertFallsBackToVOM() public {
        vm.prank(owner);
        oracleAdapter.registerMarket(MARKET_ETH_USDC, PYTH_ETH_FEED, VOM_ETH_MARKET);

        mockPyth.setShouldRevert(true);
        _mockVOM(VOM_ETH_MARKET, 1998e18, 2, block.number);

        (int256 price, uint256 confidence, bool isFallback) = oracleAdapter.getPrice(MARKET_ETH_USDC);

        assertEq(price, 1998e18);
        assertEq(confidence, oracleAdapter.CONFIDENCE_REDUCED());
        assertTrue(isFallback);
    }

    // ─── Price normalization edge cases ──────────────────────────────

    function test_GetPrice_NormalizesNegativeExpoCorrectly() public {
        vm.prank(owner);
        oracleAdapter.registerMarket(MARKET_ETH_USDC, PYTH_ETH_FEED, VOM_ETH_MARKET);

        // price=12345, expo=-2 => 12345 * 10^(18+(-2)) = 12345 * 10^16 = 123.45 * 10^18
        mockPyth.setPrice(PYTH_ETH_FEED, 12345, 100, -2, block.timestamp);

        (int256 price,,) = oracleAdapter.getPrice(MARKET_ETH_USDC);
        assertEq(price, 12345 * 1e16);
    }

    function test_GetPrice_NormalizesZeroExpoCorrectly() public {
        vm.prank(owner);
        oracleAdapter.registerMarket(MARKET_ETH_USDC, PYTH_ETH_FEED, VOM_ETH_MARKET);

        // price=50, expo=0 => 50 * 10^18
        mockPyth.setPrice(PYTH_ETH_FEED, 50, 100, 0, block.timestamp);

        (int256 price,,) = oracleAdapter.getPrice(MARKET_ETH_USDC);
        assertEq(price, 50e18);
    }

    function test_GetPrice_NegativePythPriceNormalizesToZero() public {
        vm.prank(owner);
        oracleAdapter.registerMarket(MARKET_ETH_USDC, PYTH_ETH_FEED, VOM_ETH_MARKET);

        mockPyth.setPrice(PYTH_ETH_FEED, -100, 100, -8, block.timestamp);

        // Negative Pyth price normalizes to 0. _tryPyth sees publishTime is fresh
        // so it returns (0, true). getPrice returns Pyth result with high confidence.
        (int256 price, uint256 confidence, bool isFallback) = oracleAdapter.getPrice(MARKET_ETH_USDC);
        assertEq(price, 0);
        assertEq(confidence, oracleAdapter.CONFIDENCE_HIGH());
        assertFalse(isFallback);
    }

    // ─── Fuzz Tests ──────────────────────────────────────────────────

    function testFuzz_SetStaleThreshold(uint256 threshold) public {
        threshold = bound(threshold, 1, type(uint128).max);
        vm.prank(owner);
        oracleAdapter.setStaleThreshold(threshold);
        assertEq(oracleAdapter.staleThreshold(), threshold);
    }

    function testFuzz_SetMinVomQuorum(uint256 quorum) public {
        quorum = bound(quorum, 1, type(uint64).max);
        vm.prank(owner);
        oracleAdapter.setMinVomQuorum(quorum);
        assertEq(oracleAdapter.minVomQuorum(), quorum);
    }

    function testFuzz_NormalizePythPrice(int64 price, int32 expo) public {
        // Bound to reasonable Pyth values
        price = int64(bound(int256(price), 1, 1e12));
        expo = int32(bound(int256(expo), -18, 0));

        vm.prank(owner);
        oracleAdapter.registerMarket(MARKET_ETH_USDC, PYTH_ETH_FEED, VOM_ETH_MARKET);
        mockPyth.setPrice(PYTH_ETH_FEED, price, 100, expo, block.timestamp);

        (int256 normalizedPrice,,) = oracleAdapter.getPrice(MARKET_ETH_USDC);
        assertGt(normalizedPrice, 0, "Normalized price should be positive for positive Pyth price");
    }

    // ─── Invariant: market count monotonically increases ─────────────

    function test_Invariant_MarketCountMonotonic() public {
        vm.startPrank(owner);
        assertEq(oracleAdapter.getMarketCount(), 0);

        oracleAdapter.registerMarket(MARKET_ETH_USDC, PYTH_ETH_FEED, VOM_ETH_MARKET);
        assertEq(oracleAdapter.getMarketCount(), 1);

        bytes32 market2 = keccak256("BTC/USDC");
        oracleAdapter.registerMarket(market2, keccak256("pyth-btc"), keccak256("vom-btc"));
        assertEq(oracleAdapter.getMarketCount(), 2);

        // Deactivating doesn't reduce count
        oracleAdapter.deactivateMarket(MARKET_ETH_USDC);
        assertEq(oracleAdapter.getMarketCount(), 2);
        vm.stopPrank();
    }
}
