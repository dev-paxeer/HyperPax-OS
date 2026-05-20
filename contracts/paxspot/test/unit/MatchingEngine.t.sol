// SPDX-License-Identifier: MIT
pragma solidity ^0.8.24;

import "../PaxSpotTestBase.sol";

contract MatchingEngineTest is PaxSpotTestBase {
    function setUp() public {
        _deployFullStack();
        _registerMarket();
        _setPythFresh(200000000000, -8); // $2000 fresh
        _mockOROBResolveDefault();
        _mockPoFQDefault();
        _fundAndApprove(alice, 100e18, 10_000_000e6);
        _fundAndApprove(bob, 100e18, 10_000_000e6);
        _depositForUser(alice, 50e18, 5_000_000e6);
        _depositForUser(bob, 50e18, 5_000_000e6);
    }

    // ─── Constructor ─────────────────────────────────────────────────

    function test_Constructor_SetsState() public view {
        assertEq(address(matchingEngine.oracle()), address(oracleAdapter));
        assertEq(address(matchingEngine.settlement()), address(settlementEngine));
        assertEq(matchingEngine.nextOrderId(), 1);
        assertEq(matchingEngine.owner(), owner);
    }

    // ─── createMarket ────────────────────────────────────────────────

    function test_CreateMarket_Success() public {
        bytes32 newMarket = keccak256("BTC/USDC");

        vm.prank(owner);
        vm.expectEmit(true, false, false, true);
        emit MatchingEngine.MarketCreated(
            newMarket, address(baseToken), address(quoteToken), PaxSpotTypes.MarketMode.CONTINUOUS
        );
        matchingEngine.createMarket(
            newMarket, address(baseToken), address(quoteToken),
            PaxSpotTypes.MarketMode.CONTINUOUS, 1e15, 500, 10, -5
        );

        assertEq(matchingEngine.marketCount(), 2);
    }

    function test_RevertWhen_CreateMarketAlreadyExists() public {
        vm.prank(owner);
        vm.expectRevert(abi.encodeWithSelector(MatchingEngine.MarketAlreadyExists.selector, MARKET_ETH_USDC));
        matchingEngine.createMarket(
            MARKET_ETH_USDC, address(baseToken), address(quoteToken),
            PaxSpotTypes.MarketMode.CONTINUOUS, 1e15, 500, 10, -5
        );
    }

    function test_RevertWhen_CreateMarketZeroTokens() public {
        vm.prank(owner);
        vm.expectRevert(MatchingEngine.InvalidMarketParams.selector);
        matchingEngine.createMarket(
            keccak256("X"), address(0), address(quoteToken),
            PaxSpotTypes.MarketMode.CONTINUOUS, 1e15, 500, 10, -5
        );
    }

    function test_RevertWhen_CreateMarketInvalidOffset() public {
        vm.prank(owner);
        vm.expectRevert(MatchingEngine.InvalidMarketParams.selector);
        matchingEngine.createMarket(
            keccak256("X"), address(baseToken), address(quoteToken),
            PaxSpotTypes.MarketMode.CONTINUOUS, 1e15, 0, 10, -5
        );
    }

    function test_RevertWhen_CreateMarketExcessiveFee() public {
        vm.prank(owner);
        vm.expectRevert(MatchingEngine.InvalidMarketParams.selector);
        matchingEngine.createMarket(
            keccak256("X"), address(baseToken), address(quoteToken),
            PaxSpotTypes.MarketMode.CONTINUOUS, 1e15, 500, 101, -5
        );
    }

    function test_RevertWhen_CreateMarketNotOwner() public {
        vm.prank(alice);
        vm.expectRevert();
        matchingEngine.createMarket(
            keccak256("X"), address(baseToken), address(quoteToken),
            PaxSpotTypes.MarketMode.CONTINUOUS, 1e15, 500, 10, -5
        );
    }

    // ─── placeOrder — continuous mode ────────────────────────────────

    function test_PlaceOrder_LimitBuy_EmitAndStore() public {
        vm.prank(alice);
        vm.expectEmit(true, true, true, true);
        emit MatchingEngine.OrderPlaced(
            1, MARKET_ETH_USDC, alice, PaxSpotTypes.Side.BUY, PaxSpotTypes.OrderType.LIMIT, -10, 1e18
        );
        uint256 orderId = router.submitOrder(
            MARKET_ETH_USDC, PaxSpotTypes.Side.BUY, PaxSpotTypes.OrderType.LIMIT, -10, 1e18
        );

        assertEq(orderId, 1);
        assertEq(matchingEngine.nextOrderId(), 2);

        (uint256 id, address trader, bytes32 mkt,,,int16 offset, uint128 size, uint128 filled, uint256 blockSub, bool active) =
            matchingEngine.orders(1);
        assertEq(id, 1);
        assertEq(trader, alice);
        assertEq(mkt, MARKET_ETH_USDC);
        assertEq(offset, -10);
        assertEq(size, 1e18);
        assertEq(filled, 0);
        assertGt(blockSub, 0);
        assertTrue(active);
    }

    function test_PlaceOrder_RestsOnBookWhenNoMatch() public {
        // Place a limit buy — no sells exist, so it rests
        vm.prank(alice);
        router.submitOrder(
            MARKET_ETH_USDC, PaxSpotTypes.Side.BUY, PaxSpotTypes.OrderType.LIMIT, -10, 1e18
        );

        assertEq(matchingEngine.buyOrderCount(MARKET_ETH_USDC), 1);
        assertEq(matchingEngine.sellOrderCount(MARKET_ETH_USDC), 0);
    }

    function test_RevertWhen_PlaceOrderMarketNotActive() public {
        bytes32 fakeMkt = keccak256("FAKE");
        vm.prank(alice);
        vm.expectRevert(abi.encodeWithSelector(MatchingEngine.MarketNotActive.selector, fakeMkt));
        router.submitOrder(fakeMkt, PaxSpotTypes.Side.BUY, PaxSpotTypes.OrderType.LIMIT, 0, 1e18);
    }

    function test_RevertWhen_PlaceOrderSizeTooSmall() public {
        vm.prank(alice);
        vm.expectRevert(abi.encodeWithSelector(MatchingEngine.OrderSizeTooSmall.selector, uint128(100), uint128(1e15)));
        router.submitOrder(
            MARKET_ETH_USDC, PaxSpotTypes.Side.BUY, PaxSpotTypes.OrderType.LIMIT, 0, 100
        );
    }

    function test_RevertWhen_PlaceOrderOffsetExceedsMax() public {
        vm.prank(alice);
        vm.expectRevert(abi.encodeWithSelector(MatchingEngine.OffsetExceedsMax.selector, int16(600), int16(500)));
        router.submitOrder(
            MARKET_ETH_USDC, PaxSpotTypes.Side.BUY, PaxSpotTypes.OrderType.LIMIT, 600, 1e18
        );
    }

    function test_RevertWhen_PlaceOrderNegativeOffsetExceedsMax() public {
        vm.prank(alice);
        vm.expectRevert(abi.encodeWithSelector(MatchingEngine.OffsetExceedsMax.selector, int16(-600), int16(500)));
        router.submitOrder(
            MARKET_ETH_USDC, PaxSpotTypes.Side.BUY, PaxSpotTypes.OrderType.LIMIT, -600, 1e18
        );
    }

    function test_RevertWhen_PlaceOrderNotRouter() public {
        vm.prank(alice);
        vm.expectRevert(MatchingEngine.OnlyRouter.selector);
        matchingEngine.placeOrder(
            alice, MARKET_ETH_USDC, PaxSpotTypes.Side.BUY, PaxSpotTypes.OrderType.LIMIT, 0, 1e18
        );
    }

    function test_RevertWhen_PlaceOrderWhenPaused() public {
        vm.prank(owner);
        matchingEngine.pause();

        vm.prank(alice);
        vm.expectRevert();
        router.submitOrder(
            MARKET_ETH_USDC, PaxSpotTypes.Side.BUY, PaxSpotTypes.OrderType.LIMIT, 0, 1e18
        );
    }

    // ─── Continuous matching ─────────────────────────────────────────

    function test_ContinuousMatch_BuyMatchesSell() public {
        // Mock OROB for specific offsets
        _mockOROBResolve(ORACLE_PRICE, -10); // buy at oracle - 10bps
        _mockOROBResolve(ORACLE_PRICE, -10); // sell also at -10bps => prices cross

        // Alice places a sell limit at -10 bps
        vm.prank(alice);
        router.submitOrder(
            MARKET_ETH_USDC, PaxSpotTypes.Side.SELL, PaxSpotTypes.OrderType.LIMIT, -10, 1e18
        );

        // Bob places a buy limit at -10 bps — should match against Alice's sell
        vm.prank(bob);
        vm.expectEmit(true, true, true, false);
        emit MatchingEngine.OrderFilled(2, 1, MARKET_ETH_USDC, 1e18, 0, 0);
        router.submitOrder(
            MARKET_ETH_USDC, PaxSpotTypes.Side.BUY, PaxSpotTypes.OrderType.LIMIT, -10, 1e18
        );

        // Both orders should be filled
        (,,,,,, uint128 sellSize, uint128 sellFilled,, bool sellActive) = matchingEngine.orders(1);
        assertEq(sellFilled, sellSize);
        assertFalse(sellActive);

        (,,,,,, uint128 buySize, uint128 buyFilled,, bool buyActive) = matchingEngine.orders(2);
        assertEq(buyFilled, buySize);
        assertFalse(buyActive);
    }

    function test_ContinuousMatch_PartialFill() public {
        _mockOROBResolve(ORACLE_PRICE, 0);

        // Alice sells 2 ETH
        vm.prank(alice);
        router.submitOrder(
            MARKET_ETH_USDC, PaxSpotTypes.Side.SELL, PaxSpotTypes.OrderType.LIMIT, 0, 2e18
        );

        // Bob buys 1 ETH — partial fill
        vm.prank(bob);
        router.submitOrder(
            MARKET_ETH_USDC, PaxSpotTypes.Side.BUY, PaxSpotTypes.OrderType.LIMIT, 0, 1e18
        );

        (,,,,,,,uint128 sellFilled,, bool sellActive) = matchingEngine.orders(1);
        assertEq(sellFilled, 1e18);
        assertTrue(sellActive); // still has 1 ETH remaining

        (,,,,,,,uint128 buyFilled,, bool buyActive) = matchingEngine.orders(2);
        assertEq(buyFilled, 1e18);
        assertFalse(buyActive);
    }

    function test_ContinuousMatch_NoCrossNoFill() public {
        // Sell at +100 bps, buy at -100 bps — no cross
        _mockOROBResolve(ORACLE_PRICE, 100);
        _mockOROBResolve(ORACLE_PRICE, -100);

        // Mock OROB for the specific case where buy tries to match against sell
        // buy resolves to oraclePrice - 1% = 1980e18
        // sell resolves to oraclePrice + 1% = 2020e18
        // buy < sell => no cross

        vm.prank(alice);
        router.submitOrder(
            MARKET_ETH_USDC, PaxSpotTypes.Side.SELL, PaxSpotTypes.OrderType.LIMIT, 100, 1e18
        );

        vm.prank(bob);
        router.submitOrder(
            MARKET_ETH_USDC, PaxSpotTypes.Side.BUY, PaxSpotTypes.OrderType.LIMIT, -100, 1e18
        );

        // Both should rest on book unfilled
        assertEq(matchingEngine.sellOrderCount(MARKET_ETH_USDC), 1);
        assertEq(matchingEngine.buyOrderCount(MARKET_ETH_USDC), 1);
    }

    function test_ContinuousMatch_MarketBuyFillsRestingSell() public {
        _mockOROBResolve(ORACLE_PRICE, 0);

        // Alice places a sell limit
        vm.prank(alice);
        router.submitOrder(
            MARKET_ETH_USDC, PaxSpotTypes.Side.SELL, PaxSpotTypes.OrderType.LIMIT, 0, 1e18
        );

        // Bob places a market buy — market orders use max price so they always cross
        vm.prank(bob);
        router.submitOrder(
            MARKET_ETH_USDC, PaxSpotTypes.Side.BUY, PaxSpotTypes.OrderType.MARKET, 0, 1e18
        );

        (,,,,,,,uint128 buyFilled,,) = matchingEngine.orders(2);
        assertEq(buyFilled, 1e18);
    }

    // ─── cancelOrder ─────────────────────────────────────────────────

    function test_CancelOrder_Success() public {
        vm.prank(alice);
        uint256 orderId = router.submitOrder(
            MARKET_ETH_USDC, PaxSpotTypes.Side.BUY, PaxSpotTypes.OrderType.LIMIT, -10, 1e18
        );

        vm.prank(alice);
        vm.expectEmit(true, true, false, false);
        emit MatchingEngine.OrderCancelled(orderId, alice);
        router.cancelOrder(orderId);

        (,,,,,,,,, bool active) = matchingEngine.orders(orderId);
        assertFalse(active);
    }

    function test_RevertWhen_CancelOrderNotOwner() public {
        vm.prank(alice);
        uint256 orderId = router.submitOrder(
            MARKET_ETH_USDC, PaxSpotTypes.Side.BUY, PaxSpotTypes.OrderType.LIMIT, -10, 1e18
        );

        vm.prank(bob);
        vm.expectRevert(abi.encodeWithSelector(MatchingEngine.NotOrderOwner.selector, orderId));
        router.cancelOrder(orderId);
    }

    function test_RevertWhen_CancelInactiveOrder() public {
        vm.prank(alice);
        uint256 orderId = router.submitOrder(
            MARKET_ETH_USDC, PaxSpotTypes.Side.BUY, PaxSpotTypes.OrderType.LIMIT, -10, 1e18
        );

        vm.prank(alice);
        router.cancelOrder(orderId);

        vm.prank(alice);
        vm.expectRevert(abi.encodeWithSelector(MatchingEngine.OrderNotActive.selector, orderId));
        router.cancelOrder(orderId);
    }

    // ─── setMarketMode ───────────────────────────────────────────────

    function test_SetMarketMode_Success() public {
        vm.prank(owner);
        vm.expectEmit(true, false, false, true);
        emit MatchingEngine.MarketModeChanged(
            MARKET_ETH_USDC, PaxSpotTypes.MarketMode.CONTINUOUS, PaxSpotTypes.MarketMode.BATCH
        );
        matchingEngine.setMarketMode(MARKET_ETH_USDC, PaxSpotTypes.MarketMode.BATCH);
    }

    function test_RevertWhen_SetMarketModeInactiveMarket() public {
        bytes32 fakeMkt = keccak256("FAKE");
        vm.prank(owner);
        vm.expectRevert(abi.encodeWithSelector(MatchingEngine.MarketNotActive.selector, fakeMkt));
        matchingEngine.setMarketMode(fakeMkt, PaxSpotTypes.MarketMode.BATCH);
    }

    // ─── Batch mode — placeOrder queues ──────────────────────────────

    function test_BatchMode_OrderQueued() public {
        // Switch to batch mode
        vm.prank(owner);
        matchingEngine.setMarketMode(MARKET_ETH_USDC, PaxSpotTypes.MarketMode.BATCH);

        vm.prank(alice);
        router.submitOrder(
            MARKET_ETH_USDC, PaxSpotTypes.Side.BUY, PaxSpotTypes.OrderType.LIMIT, 10, 1e18
        );

        vm.prank(bob);
        router.submitOrder(
            MARKET_ETH_USDC, PaxSpotTypes.Side.SELL, PaxSpotTypes.OrderType.LIMIT, -10, 1e18
        );

        (uint256 numBuys, uint256 numSells) = matchingEngine.batchQueueSize(MARKET_ETH_USDC);
        assertEq(numBuys, 1);
        assertEq(numSells, 1);
    }

    // ─── clearBatch ──────────────────────────────────────────────────

    function test_ClearBatch_MatchesOrders() public {
        vm.prank(owner);
        matchingEngine.setMarketMode(MARKET_ETH_USDC, PaxSpotTypes.MarketMode.BATCH);

        vm.prank(alice);
        router.submitOrder(
            MARKET_ETH_USDC, PaxSpotTypes.Side.BUY, PaxSpotTypes.OrderType.LIMIT, 10, 1e18
        );

        vm.prank(bob);
        router.submitOrder(
            MARKET_ETH_USDC, PaxSpotTypes.Side.SELL, PaxSpotTypes.OrderType.LIMIT, -10, 1e18
        );

        // Mock batch clearing result: 1 ETH matched at oracle price
        _mockBatchClearing(0, ORACLE_PRICE, 1e18);

        // Advance 1 block so clearBatch can proceed
        _advanceBlocks(1);

        vm.expectEmit(true, false, false, false);
        emit MatchingEngine.BatchCleared(MARKET_ETH_USDC, 0, ORACLE_PRICE, 1e18, 1, 1);
        matchingEngine.clearBatch(MARKET_ETH_USDC);

        // Queue should be cleared
        (uint256 numBuys, uint256 numSells) = matchingEngine.batchQueueSize(MARKET_ETH_USDC);
        assertEq(numBuys, 0);
        assertEq(numSells, 0);
    }

    function test_ClearBatch_EmptyQueuesNoOp() public {
        vm.prank(owner);
        matchingEngine.setMarketMode(MARKET_ETH_USDC, PaxSpotTypes.MarketMode.BATCH);

        // Only buys, no sells
        vm.prank(alice);
        router.submitOrder(
            MARKET_ETH_USDC, PaxSpotTypes.Side.BUY, PaxSpotTypes.OrderType.LIMIT, 10, 1e18
        );

        _advanceBlocks(1);
        matchingEngine.clearBatch(MARKET_ETH_USDC); // should not revert, just clean up
    }

    function test_RevertWhen_ClearBatchNotBatchMode() public {
        // Market is CONTINUOUS
        vm.expectRevert(abi.encodeWithSelector(MatchingEngine.BatchNotReady.selector, MARKET_ETH_USDC));
        matchingEngine.clearBatch(MARKET_ETH_USDC);
    }

    function test_RevertWhen_ClearBatchSameBlock() public {
        vm.prank(owner);
        matchingEngine.setMarketMode(MARKET_ETH_USDC, PaxSpotTypes.MarketMode.BATCH);

        vm.prank(alice);
        router.submitOrder(
            MARKET_ETH_USDC, PaxSpotTypes.Side.BUY, PaxSpotTypes.OrderType.LIMIT, 10, 1e18
        );

        vm.prank(bob);
        router.submitOrder(
            MARKET_ETH_USDC, PaxSpotTypes.Side.SELL, PaxSpotTypes.OrderType.LIMIT, -10, 1e18
        );

        _mockBatchClearing(0, ORACLE_PRICE, 1e18);
        _advanceBlocks(1);

        matchingEngine.clearBatch(MARKET_ETH_USDC);

        // Second clear in same block should fail
        vm.expectRevert(abi.encodeWithSelector(MatchingEngine.BatchNotReady.selector, MARKET_ETH_USDC));
        matchingEngine.clearBatch(MARKET_ETH_USDC);
    }

    // ─── PoFQ scoring ────────────────────────────────────────────────

    function test_PoFQUpdatedOnFill() public {
        _mockOROBResolve(ORACLE_PRICE, 0);

        vm.prank(alice);
        router.submitOrder(
            MARKET_ETH_USDC, PaxSpotTypes.Side.SELL, PaxSpotTypes.OrderType.LIMIT, 0, 1e18
        );

        vm.prank(bob);
        router.submitOrder(
            MARKET_ETH_USDC, PaxSpotTypes.Side.BUY, PaxSpotTypes.OrderType.LIMIT, 0, 1e18
        );

        // PoFQ scores should be updated (mocked to return 1e18)
        (uint256 aliceScore,) = matchingEngine.getPoFQScore(alice);
        (uint256 bobScore,) = matchingEngine.getPoFQScore(bob);
        assertEq(aliceScore, 1e18);
        assertEq(bobScore, 1e18);
    }

    // ─── View functions ──────────────────────────────────────────────

    function test_BuyOrderCount() public {
        vm.prank(alice);
        router.submitOrder(
            MARKET_ETH_USDC, PaxSpotTypes.Side.BUY, PaxSpotTypes.OrderType.LIMIT, -100, 1e18
        );

        assertEq(matchingEngine.buyOrderCount(MARKET_ETH_USDC), 1);
    }

    function test_SellOrderCount() public {
        vm.prank(alice);
        router.submitOrder(
            MARKET_ETH_USDC, PaxSpotTypes.Side.SELL, PaxSpotTypes.OrderType.LIMIT, 100, 1e18
        );

        assertEq(matchingEngine.sellOrderCount(MARKET_ETH_USDC), 1);
    }

    function test_MarketCount() public view {
        assertEq(matchingEngine.marketCount(), 1);
    }

    // ─── setRouter ───────────────────────────────────────────────────

    function test_SetRouter_Success() public {
        address newRouter = makeAddr("newRouter");
        vm.prank(owner);
        matchingEngine.setRouter(newRouter);
        assertEq(matchingEngine.router(), newRouter);
    }

    function test_RevertWhen_SetRouterNotOwner() public {
        vm.prank(alice);
        vm.expectRevert();
        matchingEngine.setRouter(alice);
    }

    // ─── Pause / Unpause ─────────────────────────────────────────────

    function test_PauseUnpause() public {
        vm.prank(owner);
        matchingEngine.pause();

        vm.prank(alice);
        vm.expectRevert();
        router.submitOrder(
            MARKET_ETH_USDC, PaxSpotTypes.Side.BUY, PaxSpotTypes.OrderType.LIMIT, 0, 1e18
        );

        vm.prank(owner);
        matchingEngine.unpause();

        vm.prank(alice);
        router.submitOrder(
            MARKET_ETH_USDC, PaxSpotTypes.Side.BUY, PaxSpotTypes.OrderType.LIMIT, -10, 1e18
        );
    }

    // ─── Fuzz Tests ──────────────────────────────────────────────────

    function testFuzz_PlaceOrder_SizeAboveMin(uint128 size) public {
        size = uint128(bound(uint256(size), 1e15, 10e18)); // minOrderSize to 10 ETH

        vm.prank(alice);
        uint256 orderId = router.submitOrder(
            MARKET_ETH_USDC, PaxSpotTypes.Side.BUY, PaxSpotTypes.OrderType.LIMIT, -10, size
        );

        (,,,,,,uint128 storedSize,,,) = matchingEngine.orders(orderId);
        assertEq(storedSize, size);
    }

    function testFuzz_PlaceOrder_OffsetWithinBounds(int16 offset) public {
        offset = int16(bound(int256(offset), -500, 500));

        _mockOROBResolve(ORACLE_PRICE, offset);

        vm.prank(alice);
        uint256 orderId = router.submitOrder(
            MARKET_ETH_USDC, PaxSpotTypes.Side.BUY, PaxSpotTypes.OrderType.LIMIT, offset, 1e18
        );

        (,,,,,int16 storedOffset,,,,) = matchingEngine.orders(orderId);
        assertEq(storedOffset, offset);
    }

    // ─── Invariant: nextOrderId strictly increases ───────────────────

    function test_Invariant_NextOrderIdMonotonic() public {
        assertEq(matchingEngine.nextOrderId(), 1);

        vm.prank(alice);
        router.submitOrder(
            MARKET_ETH_USDC, PaxSpotTypes.Side.BUY, PaxSpotTypes.OrderType.LIMIT, -10, 1e18
        );
        assertEq(matchingEngine.nextOrderId(), 2);

        vm.prank(bob);
        router.submitOrder(
            MARKET_ETH_USDC, PaxSpotTypes.Side.SELL, PaxSpotTypes.OrderType.LIMIT, 10, 1e18
        );
        assertEq(matchingEngine.nextOrderId(), 3);
    }
}
