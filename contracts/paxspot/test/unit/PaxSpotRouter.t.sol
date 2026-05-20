// SPDX-License-Identifier: MIT
pragma solidity ^0.8.24;

import "../PaxSpotTestBase.sol";

contract PaxSpotRouterTest is PaxSpotTestBase {
    function setUp() public {
        _deployFullStack();
        _registerMarket();
        _setPythFresh(200000000000, -8); // $2000
        _mockOROBResolveDefault();
        _mockPoFQDefault();
        _fundAndApprove(alice, 100e18, 10_000_000e6);
        _fundAndApprove(bob, 100e18, 10_000_000e6);
        _depositForUser(alice, 50e18, 5_000_000e6);
        _depositForUser(bob, 50e18, 5_000_000e6);
    }

    // ─── Constructor ─────────────────────────────────────────────────

    function test_Constructor_SetsState() public view {
        assertEq(address(router.matchingEngine()), address(matchingEngine));
        assertEq(address(router.settlementEngine()), address(settlementEngine));
        assertEq(router.orderCooldownBlocks(), 0);
        assertEq(router.owner(), owner);
    }

    // ─── submitOrder ─────────────────────────────────────────────────

    function test_SubmitOrder_Success() public {
        vm.prank(alice);
        vm.expectEmit(true, true, true, true);
        emit PaxSpotRouter.OrderSubmitted(
            1, alice, MARKET_ETH_USDC, PaxSpotTypes.Side.BUY, PaxSpotTypes.OrderType.LIMIT, -10, 1e18
        );
        uint256 orderId = router.submitOrder(
            MARKET_ETH_USDC, PaxSpotTypes.Side.BUY, PaxSpotTypes.OrderType.LIMIT, -10, 1e18
        );

        assertEq(orderId, 1);
    }

    function test_SubmitOrder_UpdatesPosition() public {
        vm.prank(alice);
        router.submitOrder(
            MARKET_ETH_USDC, PaxSpotTypes.Side.BUY, PaxSpotTypes.OrderType.LIMIT, -10, 1e18
        );

        (int256 pos,) = router.getPosition(alice, MARKET_ETH_USDC);
        assertEq(pos, 1e18);
    }

    function test_SubmitOrder_SellReducesPosition() public {
        vm.prank(alice);
        router.submitOrder(
            MARKET_ETH_USDC, PaxSpotTypes.Side.BUY, PaxSpotTypes.OrderType.LIMIT, -10, 2e18
        );

        vm.prank(alice);
        _advanceBlocks(1);
        router.submitOrder(
            MARKET_ETH_USDC, PaxSpotTypes.Side.SELL, PaxSpotTypes.OrderType.LIMIT, 10, 1e18
        );

        (int256 pos,) = router.getPosition(alice, MARKET_ETH_USDC);
        assertEq(pos, 1e18);
    }

    function test_SubmitOrder_RecordsVolume() public {
        vm.prank(alice);
        router.submitOrder(
            MARKET_ETH_USDC, PaxSpotTypes.Side.BUY, PaxSpotTypes.OrderType.LIMIT, -10, 3e18
        );

        uint256 vol = router.getVolume(alice, 10);
        assertEq(vol, 3e18);
    }

    function test_RevertWhen_SubmitOrderZeroSize() public {
        vm.prank(alice);
        vm.expectRevert(PaxSpotRouter.InvalidSize.selector);
        router.submitOrder(
            MARKET_ETH_USDC, PaxSpotTypes.Side.BUY, PaxSpotTypes.OrderType.LIMIT, 0, 0
        );
    }

    function test_RevertWhen_SubmitOrderWhenPaused() public {
        vm.prank(owner);
        router.pause();

        vm.prank(alice);
        vm.expectRevert();
        router.submitOrder(
            MARKET_ETH_USDC, PaxSpotTypes.Side.BUY, PaxSpotTypes.OrderType.LIMIT, -10, 1e18
        );
    }

    // ─── Cooldown ────────────────────────────────────────────────────

    function test_Cooldown_EnforcedBetweenOrders() public {
        vm.prank(owner);
        router.setCooldown(3);

        // Advance past genesis so first order isn't blocked by lastOrderBlock=0 + cooldown > block.number
        vm.roll(100);

        vm.prank(alice);
        router.submitOrder(
            MARKET_ETH_USDC, PaxSpotTypes.Side.BUY, PaxSpotTypes.OrderType.LIMIT, -10, 1e18
        );
        uint256 lastBlock = router.lastOrderBlock(alice);

        // Before cooldown expires — should fail
        vm.roll(lastBlock + 2);
        vm.prank(alice);
        vm.expectRevert(); // CooldownActive
        router.submitOrder(
            MARKET_ETH_USDC, PaxSpotTypes.Side.BUY, PaxSpotTypes.OrderType.LIMIT, -10, 1e18
        );

        // At cooldown boundary — should succeed
        vm.roll(lastBlock + 3);
        vm.prank(alice);
        router.submitOrder(
            MARKET_ETH_USDC, PaxSpotTypes.Side.BUY, PaxSpotTypes.OrderType.LIMIT, -10, 1e18
        );
    }

    function test_Cooldown_ZeroMeansNoLimit() public {
        // Default cooldown is 0
        vm.startPrank(alice);
        router.submitOrder(
            MARKET_ETH_USDC, PaxSpotTypes.Side.BUY, PaxSpotTypes.OrderType.LIMIT, -10, 1e18
        );
        // Immediately another — should succeed
        router.submitOrder(
            MARKET_ETH_USDC, PaxSpotTypes.Side.BUY, PaxSpotTypes.OrderType.LIMIT, -10, 1e18
        );
        vm.stopPrank();
    }

    // ─── cancelOrder ─────────────────────────────────────────────────

    function test_CancelOrder_Success() public {
        vm.prank(alice);
        uint256 orderId = router.submitOrder(
            MARKET_ETH_USDC, PaxSpotTypes.Side.BUY, PaxSpotTypes.OrderType.LIMIT, -10, 1e18
        );

        vm.prank(alice);
        vm.expectEmit(true, true, false, false);
        emit PaxSpotRouter.OrderCancelRequested(orderId, alice);
        router.cancelOrder(orderId);
    }

    function test_RevertWhen_CancelOrderWhenPaused() public {
        vm.prank(alice);
        uint256 orderId = router.submitOrder(
            MARKET_ETH_USDC, PaxSpotTypes.Side.BUY, PaxSpotTypes.OrderType.LIMIT, -10, 1e18
        );

        vm.prank(owner);
        router.pause();

        vm.prank(alice);
        vm.expectRevert();
        router.cancelOrder(orderId);
    }

    // ─── Funded Wallet ───────────────────────────────────────────────

    function test_RegisterFundedWallet_Success() public {
        vm.prank(owner);
        vm.expectEmit(true, false, false, false);
        emit PaxSpotRouter.FundedWalletRegistered(alice);
        router.registerFundedWallet(alice);

        assertTrue(router.isFundedWallet(alice));
    }

    function test_RemoveFundedWallet_Success() public {
        vm.startPrank(owner);
        router.registerFundedWallet(alice);
        vm.expectEmit(true, false, false, false);
        emit PaxSpotRouter.FundedWalletRemoved(alice);
        router.removeFundedWallet(alice);
        vm.stopPrank();

        assertFalse(router.isFundedWallet(alice));
    }

    function test_RevertWhen_RegisterFundedWalletZeroAddress() public {
        vm.prank(owner);
        vm.expectRevert(PaxSpotRouter.InvalidAddress.selector);
        router.registerFundedWallet(address(0));
    }

    function test_RevertWhen_RegisterFundedWalletNotOwner() public {
        vm.prank(alice);
        vm.expectRevert();
        router.registerFundedWallet(alice);
    }

    function test_FundedWallet_AllowanceCheckPasses() public {
        vm.startPrank(owner);
        router.registerFundedWallet(alice);
        router.setAllowanceProvider(address(mockAllowance));
        vm.stopPrank();

        mockAllowance.setActive(alice, true);
        mockAllowance.setMaxPosition(alice, MARKET_ETH_USDC, 10e18);

        vm.prank(alice);
        router.submitOrder(
            MARKET_ETH_USDC, PaxSpotTypes.Side.BUY, PaxSpotTypes.OrderType.LIMIT, -10, 1e18
        );
    }

    function test_RevertWhen_FundedWalletNotActive() public {
        vm.startPrank(owner);
        router.registerFundedWallet(alice);
        router.setAllowanceProvider(address(mockAllowance));
        vm.stopPrank();

        mockAllowance.setActive(alice, false);

        vm.prank(alice);
        vm.expectRevert(abi.encodeWithSelector(PaxSpotRouter.FundedWalletNotActive.selector, alice));
        router.submitOrder(
            MARKET_ETH_USDC, PaxSpotTypes.Side.BUY, PaxSpotTypes.OrderType.LIMIT, -10, 1e18
        );
    }

    function test_RevertWhen_FundedWalletExceedsAllowance() public {
        vm.startPrank(owner);
        router.registerFundedWallet(alice);
        router.setAllowanceProvider(address(mockAllowance));
        vm.stopPrank();

        mockAllowance.setActive(alice, true);
        mockAllowance.setMaxPosition(alice, MARKET_ETH_USDC, 5e17); // max 0.5 ETH

        vm.prank(alice);
        vm.expectRevert(
            abi.encodeWithSelector(
                PaxSpotRouter.FundedWalletExceedsAllowance.selector,
                alice, MARKET_ETH_USDC, 1e18, 5e17
            )
        );
        router.submitOrder(
            MARKET_ETH_USDC, PaxSpotTypes.Side.BUY, PaxSpotTypes.OrderType.LIMIT, -10, 1e18
        );
    }

    function test_FundedWallet_NoProviderSkipsCheck() public {
        vm.prank(owner);
        router.registerFundedWallet(alice);
        // No allowance provider set — check is skipped

        vm.prank(alice);
        router.submitOrder(
            MARKET_ETH_USDC, PaxSpotTypes.Side.BUY, PaxSpotTypes.OrderType.LIMIT, -10, 1e18
        );
    }

    function test_NonFundedWallet_SkipsAllowanceCheck() public {
        vm.startPrank(owner);
        router.setAllowanceProvider(address(mockAllowance));
        vm.stopPrank();

        // Alice is NOT a funded wallet — no allowance check
        vm.prank(alice);
        router.submitOrder(
            MARKET_ETH_USDC, PaxSpotTypes.Side.BUY, PaxSpotTypes.OrderType.LIMIT, -10, 1e18
        );
    }

    // ─── IPaxSpotReader — getPoFQScore ───────────────────────────────

    function test_GetPoFQScore() public {
        _mockOROBResolve(ORACLE_PRICE, 0);

        vm.prank(alice);
        router.submitOrder(
            MARKET_ETH_USDC, PaxSpotTypes.Side.SELL, PaxSpotTypes.OrderType.LIMIT, 0, 1e18
        );

        vm.prank(bob);
        router.submitOrder(
            MARKET_ETH_USDC, PaxSpotTypes.Side.BUY, PaxSpotTypes.OrderType.LIMIT, 0, 1e18
        );

        (uint256 score, uint256 vol) = router.getPoFQScore(alice);
        assertEq(score, 1e18);
        assertGt(vol, 0);
    }

    // ─── IPaxSpotReader — getRealizedPnL ─────────────────────────────

    function test_GetRealizedPnL() public view {
        int256 pnl = router.getRealizedPnL(alice, address(quoteToken));
        assertEq(pnl, 0);
    }

    // ─── IPaxSpotReader — getPosition ────────────────────────────────

    function test_GetPosition_ReturnsCorrectValues() public {
        vm.prank(alice);
        router.submitOrder(
            MARKET_ETH_USDC, PaxSpotTypes.Side.BUY, PaxSpotTypes.OrderType.LIMIT, -10, 2e18
        );

        (int256 size, int256 avgEntry) = router.getPosition(alice, MARKET_ETH_USDC);
        assertEq(size, 2e18);
        assertEq(avgEntry, 0); // avgEntryPrice not updated in simplified implementation
    }

    // ─── IPaxSpotReader — getVolume ──────────────────────────────────

    function test_GetVolume_WindowedCorrectly() public {
        vm.prank(alice);
        router.submitOrder(
            MARKET_ETH_USDC, PaxSpotTypes.Side.BUY, PaxSpotTypes.OrderType.LIMIT, -10, 1e18
        );

        _advanceBlocks(5);

        vm.prank(alice);
        router.submitOrder(
            MARKET_ETH_USDC, PaxSpotTypes.Side.BUY, PaxSpotTypes.OrderType.LIMIT, -10, 2e18
        );

        // Window of 2 blocks — should only see the second order
        uint256 vol = router.getVolume(alice, 2);
        assertEq(vol, 2e18);

        // Window of 100 blocks — should see both
        uint256 totalVol = router.getVolume(alice, 100);
        assertEq(totalVol, 3e18);
    }

    // ─── Admin — setAllowanceProvider ────────────────────────────────

    function test_SetAllowanceProvider_Success() public {
        address newProvider = makeAddr("provider");
        vm.prank(owner);
        vm.expectEmit(false, false, false, true);
        emit PaxSpotRouter.AllowanceProviderUpdated(address(0), newProvider);
        router.setAllowanceProvider(newProvider);

        assertEq(address(router.allowanceProvider()), newProvider);
    }

    // ─── Admin — setCooldown ─────────────────────────────────────────

    function test_SetCooldown_Success() public {
        vm.prank(owner);
        vm.expectEmit(false, false, false, true);
        emit PaxSpotRouter.CooldownUpdated(0, 5);
        router.setCooldown(5);

        assertEq(router.orderCooldownBlocks(), 5);
    }

    function test_RevertWhen_SetCooldownNotOwner() public {
        vm.prank(alice);
        vm.expectRevert();
        router.setCooldown(5);
    }

    // ─── Pause / Unpause ─────────────────────────────────────────────

    function test_PauseUnpause() public {
        vm.prank(owner);
        router.pause();

        vm.prank(alice);
        vm.expectRevert();
        router.submitOrder(
            MARKET_ETH_USDC, PaxSpotTypes.Side.BUY, PaxSpotTypes.OrderType.LIMIT, -10, 1e18
        );

        vm.prank(owner);
        router.unpause();

        vm.prank(alice);
        router.submitOrder(
            MARKET_ETH_USDC, PaxSpotTypes.Side.BUY, PaxSpotTypes.OrderType.LIMIT, -10, 1e18
        );
    }

    // ─── Fuzz Tests ──────────────────────────────────────────────────

    function testFuzz_SubmitOrder_ValidSizes(uint128 size) public {
        size = uint128(bound(uint256(size), 1e15, 10e18));

        vm.prank(alice);
        uint256 orderId = router.submitOrder(
            MARKET_ETH_USDC, PaxSpotTypes.Side.BUY, PaxSpotTypes.OrderType.LIMIT, -10, size
        );

        assertGt(orderId, 0);
        (int256 pos,) = router.getPosition(alice, MARKET_ETH_USDC);
        assertEq(pos, int256(uint256(size)));
    }

    function testFuzz_CooldownRespected(uint256 cooldown) public {
        cooldown = bound(cooldown, 1, 100);
        vm.prank(owner);
        router.setCooldown(cooldown);

        // Advance past genesis to avoid lastOrderBlock=0 edge case
        vm.roll(200);

        vm.prank(alice);
        router.submitOrder(
            MARKET_ETH_USDC, PaxSpotTypes.Side.BUY, PaxSpotTypes.OrderType.LIMIT, -10, 1e18
        );
        uint256 lastBlock = router.lastOrderBlock(alice);

        // Still within cooldown — should fail
        if (cooldown > 1) {
            vm.roll(lastBlock + cooldown - 1);
            vm.prank(alice);
            vm.expectRevert(); // CooldownActive
            router.submitOrder(
                MARKET_ETH_USDC, PaxSpotTypes.Side.BUY, PaxSpotTypes.OrderType.LIMIT, -10, 1e18
            );
        }

        // At cooldown boundary — should succeed
        vm.roll(lastBlock + cooldown);
        vm.prank(alice);
        router.submitOrder(
            MARKET_ETH_USDC, PaxSpotTypes.Side.BUY, PaxSpotTypes.OrderType.LIMIT, -10, 1e18
        );
    }

    function testFuzz_FundedWalletAllowance(uint256 maxPos, uint128 orderSize) public {
        maxPos = bound(maxPos, 1e15, 100e18);
        orderSize = uint128(bound(uint256(orderSize), 1e15, maxPos));

        vm.startPrank(owner);
        router.registerFundedWallet(alice);
        router.setAllowanceProvider(address(mockAllowance));
        vm.stopPrank();

        mockAllowance.setActive(alice, true);
        mockAllowance.setMaxPosition(alice, MARKET_ETH_USDC, maxPos);

        vm.prank(alice);
        router.submitOrder(
            MARKET_ETH_USDC, PaxSpotTypes.Side.BUY, PaxSpotTypes.OrderType.LIMIT, -10, orderSize
        );

        (int256 pos,) = router.getPosition(alice, MARKET_ETH_USDC);
        assertEq(pos, int256(uint256(orderSize)));
    }

    // ─── Invariant: position tracking is symmetric ───────────────────

    function test_Invariant_PositionSymmetry() public {
        vm.prank(alice);
        router.submitOrder(
            MARKET_ETH_USDC, PaxSpotTypes.Side.BUY, PaxSpotTypes.OrderType.LIMIT, -10, 5e18
        );

        _advanceBlocks(1);
        vm.prank(alice);
        router.submitOrder(
            MARKET_ETH_USDC, PaxSpotTypes.Side.SELL, PaxSpotTypes.OrderType.LIMIT, 10, 5e18
        );

        (int256 pos,) = router.getPosition(alice, MARKET_ETH_USDC);
        assertEq(pos, 0, "Position should be zero after equal buy+sell");
    }
}
