// SPDX-License-Identifier: MIT
pragma solidity ^0.8.24;

import "../PaxSpotTestBase.sol";

contract SettlementEngineTest is PaxSpotTestBase {
    function setUp() public {
        _deployFullStack();
        _fundAndApprove(alice, 100e18, 1_000_000e6);
        _fundAndApprove(bob, 100e18, 1_000_000e6);
    }

    // ─── Constructor ─────────────────────────────────────────────────

    function test_Constructor_SetsStateCorrectly() public view {
        assertEq(settlementEngine.epochLength(), EPOCH_LENGTH);
        assertEq(settlementEngine.treasury(), treasury);
        assertEq(settlementEngine.owner(), owner);
        assertEq(settlementEngine.epochCounter(), 0);
    }

    function test_RevertWhen_ConstructorZeroEpochLength() public {
        vm.prank(owner);
        vm.expectRevert(SettlementEngine.InvalidEpochLength.selector);
        new SettlementEngine(0, treasury);
    }

    function test_RevertWhen_ConstructorZeroTreasury() public {
        vm.prank(owner);
        vm.expectRevert(SettlementEngine.InvalidAddress.selector);
        new SettlementEngine(5, address(0));
    }

    // ─── deposit ─────────────────────────────────────────────────────

    function test_Deposit_UpdatesCollateralAndVirtualBalance() public {
        vm.prank(alice);
        vm.expectEmit(true, true, false, true);
        emit SettlementEngine.Deposited(alice, address(quoteToken), 5000e6);
        settlementEngine.deposit(address(quoteToken), 5000e6);

        assertEq(settlementEngine.depositedCollateral(alice, address(quoteToken)), 5000e6);
        assertEq(settlementEngine.getVirtualBalance(alice, address(quoteToken)), int256(5000e6));
    }

    function test_Deposit_MultipleTimes_Accumulates() public {
        vm.startPrank(alice);
        settlementEngine.deposit(address(quoteToken), 1000e6);
        settlementEngine.deposit(address(quoteToken), 2000e6);
        vm.stopPrank();

        assertEq(settlementEngine.depositedCollateral(alice, address(quoteToken)), 3000e6);
        assertEq(settlementEngine.getVirtualBalance(alice, address(quoteToken)), int256(3000e6));
    }

    function test_RevertWhen_DepositUnsupportedToken() public {
        MockERC20 unsupported = new MockERC20("Bad", "BAD", 18);
        vm.prank(alice);
        vm.expectRevert(abi.encodeWithSelector(SettlementEngine.TokenNotSupported.selector, address(unsupported)));
        settlementEngine.deposit(address(unsupported), 100);
    }

    function test_RevertWhen_DepositWhenPaused() public {
        vm.prank(owner);
        settlementEngine.pause();

        vm.prank(alice);
        vm.expectRevert();
        settlementEngine.deposit(address(quoteToken), 100);
    }

    // ─── withdraw ────────────────────────────────────────────────────

    function test_Withdraw_Success() public {
        vm.startPrank(alice);
        settlementEngine.deposit(address(quoteToken), 5000e6);

        vm.expectEmit(true, true, false, true);
        emit SettlementEngine.Withdrawn(alice, address(quoteToken), 2000e6);
        settlementEngine.withdraw(address(quoteToken), 2000e6);
        vm.stopPrank();

        assertEq(settlementEngine.depositedCollateral(alice, address(quoteToken)), 3000e6);
        assertEq(settlementEngine.getVirtualBalance(alice, address(quoteToken)), int256(3000e6));
        assertEq(quoteToken.balanceOf(alice), 1_000_000e6 - 5000e6 + 2000e6);
    }

    function test_Withdraw_FullAmount() public {
        vm.startPrank(alice);
        settlementEngine.deposit(address(quoteToken), 5000e6);
        settlementEngine.withdraw(address(quoteToken), 5000e6);
        vm.stopPrank();

        assertEq(settlementEngine.depositedCollateral(alice, address(quoteToken)), 0);
        assertEq(settlementEngine.getVirtualBalance(alice, address(quoteToken)), 0);
    }

    function test_RevertWhen_WithdrawExceedsAvailable() public {
        vm.startPrank(alice);
        settlementEngine.deposit(address(quoteToken), 5000e6);

        vm.expectRevert(
            abi.encodeWithSelector(
                SettlementEngine.WithdrawExceedsAvailable.selector,
                alice, address(quoteToken), 6000e6, 5000e6
            )
        );
        settlementEngine.withdraw(address(quoteToken), 6000e6);
        vm.stopPrank();
    }

    function test_RevertWhen_WithdrawUnsupportedToken() public {
        MockERC20 unsupported = new MockERC20("Bad", "BAD", 18);
        vm.prank(alice);
        vm.expectRevert(abi.encodeWithSelector(SettlementEngine.TokenNotSupported.selector, address(unsupported)));
        settlementEngine.withdraw(address(unsupported), 100);
    }

    // ─── updateVirtualBalance ────────────────────────────────────────

    function test_UpdateVirtualBalance_OnlyMatchingEngine() public {
        vm.prank(address(matchingEngine));
        vm.expectEmit(true, true, false, true);
        emit SettlementEngine.VirtualBalanceUpdated(alice, address(quoteToken), 500e6, 500e6);
        settlementEngine.updateVirtualBalance(alice, address(quoteToken), 500e6);

        assertEq(settlementEngine.getVirtualBalance(alice, address(quoteToken)), 500e6);
    }

    function test_UpdateVirtualBalance_NegativeDelta() public {
        vm.startPrank(alice);
        settlementEngine.deposit(address(quoteToken), 1000e6);
        vm.stopPrank();

        vm.prank(address(matchingEngine));
        settlementEngine.updateVirtualBalance(alice, address(quoteToken), -300e6);

        assertEq(settlementEngine.getVirtualBalance(alice, address(quoteToken)), 700e6);
    }

    function test_RevertWhen_UpdateVirtualBalanceNotMatchingEngine() public {
        vm.prank(alice);
        vm.expectRevert(SettlementEngine.OnlyMatchingEngine.selector);
        settlementEngine.updateVirtualBalance(alice, address(quoteToken), 100);
    }

    function test_UpdateVirtualBalance_MarksDirty() public {
        vm.prank(address(matchingEngine));
        settlementEngine.updateVirtualBalance(alice, address(quoteToken), 100);

        assertEq(settlementEngine.pendingSettlementCount(), 1);
    }

    function test_UpdateVirtualBalance_SameUserNotDuplicatedInDirtyList() public {
        vm.startPrank(address(matchingEngine));
        settlementEngine.updateVirtualBalance(alice, address(quoteToken), 100);
        settlementEngine.updateVirtualBalance(alice, address(quoteToken), 200);
        vm.stopPrank();

        assertEq(settlementEngine.pendingSettlementCount(), 1);
    }

    // ─── recordPnL ───────────────────────────────────────────────────

    function test_RecordPnL_Success() public {
        vm.prank(address(matchingEngine));
        vm.expectEmit(true, false, false, true);
        emit SettlementEngine.PnLRecorded(alice, 500e6, 500e6);
        settlementEngine.recordPnL(alice, 500e6);

        assertEq(settlementEngine.realizedPnL(alice), 500e6);
    }

    function test_RecordPnL_NegativePnL() public {
        vm.startPrank(address(matchingEngine));
        settlementEngine.recordPnL(alice, 500e6);
        settlementEngine.recordPnL(alice, -200e6);
        vm.stopPrank();

        assertEq(settlementEngine.realizedPnL(alice), 300e6);
    }

    function test_RevertWhen_RecordPnLNotMatchingEngine() public {
        vm.prank(alice);
        vm.expectRevert(SettlementEngine.OnlyMatchingEngine.selector);
        settlementEngine.recordPnL(alice, 100);
    }

    // ─── settleEpoch ─────────────────────────────────────────────────

    function test_SettleEpoch_Success() public {
        // Create dirty state
        vm.prank(address(matchingEngine));
        settlementEngine.updateVirtualBalance(alice, address(quoteToken), 100);

        // Advance past epoch boundary
        _advanceBlocks(EPOCH_LENGTH);

        vm.expectEmit(true, false, false, true);
        emit SettlementEngine.EpochSettled(0, block.number, 1);
        settlementEngine.settleEpoch();

        assertEq(settlementEngine.epochCounter(), 1);
        assertEq(settlementEngine.pendingSettlementCount(), 0);
    }

    function test_SettleEpoch_MultipleUsers() public {
        vm.startPrank(address(matchingEngine));
        settlementEngine.updateVirtualBalance(alice, address(quoteToken), 100);
        settlementEngine.updateVirtualBalance(bob, address(quoteToken), -100);
        vm.stopPrank();

        assertEq(settlementEngine.pendingSettlementCount(), 2);

        _advanceBlocks(EPOCH_LENGTH);
        settlementEngine.settleEpoch();

        assertEq(settlementEngine.epochCounter(), 1);
        assertEq(settlementEngine.pendingSettlementCount(), 0);
    }

    function test_RevertWhen_SettleEpochTooEarly() public {
        vm.prank(address(matchingEngine));
        settlementEngine.updateVirtualBalance(alice, address(quoteToken), 100);

        vm.expectRevert(SettlementEngine.EpochNotReady.selector);
        settlementEngine.settleEpoch();
    }

    function test_RevertWhen_SettleEpochNothingToSettle() public {
        _advanceBlocks(EPOCH_LENGTH);

        vm.expectRevert(SettlementEngine.NothingToSettle.selector);
        settlementEngine.settleEpoch();
    }

    function test_SettleEpoch_ClearsEpochNetDelta() public {
        vm.prank(address(matchingEngine));
        settlementEngine.updateVirtualBalance(alice, address(quoteToken), 500);

        _advanceBlocks(EPOCH_LENGTH);
        settlementEngine.settleEpoch();

        // After settlement, epochNetDelta should be zero
        assertEq(settlementEngine.epochNetDelta(alice, address(quoteToken)), 0);
    }

    // ─── fastSettle ──────────────────────────────────────────────────

    function test_FastSettle_ChargesFee() public {
        vm.prank(address(matchingEngine));
        settlementEngine.updateVirtualBalance(alice, address(quoteToken), 10_000);

        vm.prank(alice);
        vm.expectEmit(true, true, false, true);
        // fee = 10000 * 1 / 10000 = 1
        emit SettlementEngine.FastSettled(alice, address(quoteToken), 10_000, 1);
        settlementEngine.fastSettle(address(quoteToken));

        // Fee deducted from user, credited to treasury
        assertEq(settlementEngine.getVirtualBalance(alice, address(quoteToken)), 10_000 - 1);
        assertEq(settlementEngine.getVirtualBalance(treasury, address(quoteToken)), 1);
    }

    function test_FastSettle_NegativeNetDelta() public {
        vm.prank(address(matchingEngine));
        settlementEngine.updateVirtualBalance(alice, address(quoteToken), -10_000);

        vm.prank(alice);
        settlementEngine.fastSettle(address(quoteToken));

        // Fee is 1 bps on abs(-10000) = 1
        assertEq(settlementEngine.getVirtualBalance(alice, address(quoteToken)), -10_000 - 1);
        assertEq(settlementEngine.getVirtualBalance(treasury, address(quoteToken)), 1);
    }

    function test_RevertWhen_FastSettleNothingToSettle() public {
        vm.prank(alice);
        vm.expectRevert(SettlementEngine.NothingToSettle.selector);
        settlementEngine.fastSettle(address(quoteToken));
    }

    function test_RevertWhen_FastSettleUnsupportedToken() public {
        MockERC20 unsupported = new MockERC20("Bad", "BAD", 18);
        vm.prank(alice);
        vm.expectRevert(abi.encodeWithSelector(SettlementEngine.TokenNotSupported.selector, address(unsupported)));
        settlementEngine.fastSettle(address(unsupported));
    }

    // ─── getWithdrawable ─────────────────────────────────────────────

    function test_GetWithdrawable_FullCollateralWhenNoDebt() public {
        vm.prank(alice);
        settlementEngine.deposit(address(quoteToken), 5000e6);

        assertEq(settlementEngine.getWithdrawable(alice, address(quoteToken)), 5000e6);
    }

    function test_GetWithdrawable_ReducedByNegativeVirtualBalance() public {
        vm.prank(alice);
        settlementEngine.deposit(address(quoteToken), 5000e6);

        // Simulate a trade loss: virtual balance drops below collateral
        vm.prank(address(matchingEngine));
        settlementEngine.updateVirtualBalance(alice, address(quoteToken), -2000e6);

        // vBal = 5000e6 - 2000e6 = 3000e6, collateral = 5000e6
        // withdrawable = min(vBal, collateral) = 3000e6
        assertEq(settlementEngine.getWithdrawable(alice, address(quoteToken)), 3000e6);
    }

    function test_GetWithdrawable_ZeroWhenVirtualBalanceNegative() public {
        // User with no deposit gets a negative virtual balance (debt)
        vm.prank(address(matchingEngine));
        settlementEngine.updateVirtualBalance(alice, address(quoteToken), -100);

        assertEq(settlementEngine.getWithdrawable(alice, address(quoteToken)), 0);
    }

    function test_GetWithdrawable_CollateralCapWhenVirtualExceedsCollateral() public {
        vm.prank(alice);
        settlementEngine.deposit(address(quoteToken), 1000e6);

        // Virtual balance gains (credits from trades)
        vm.prank(address(matchingEngine));
        settlementEngine.updateVirtualBalance(alice, address(quoteToken), 5000e6);

        // vBal = 1000e6 + 5000e6 = 6000e6, collateral = 1000e6
        // withdrawable = collateral = 1000e6 (capped)
        assertEq(settlementEngine.getWithdrawable(alice, address(quoteToken)), 1000e6);
    }

    // ─── Admin — setMatchingEngine ───────────────────────────────────

    function test_SetMatchingEngine_Success() public {
        address newEngine = makeAddr("newEngine");
        vm.prank(owner);
        vm.expectEmit(false, false, false, true);
        emit SettlementEngine.MatchingEngineUpdated(address(matchingEngine), newEngine);
        settlementEngine.setMatchingEngine(newEngine);

        assertEq(settlementEngine.matchingEngine(), newEngine);
    }

    function test_RevertWhen_SetMatchingEngineZero() public {
        vm.prank(owner);
        vm.expectRevert(SettlementEngine.InvalidAddress.selector);
        settlementEngine.setMatchingEngine(address(0));
    }

    function test_RevertWhen_SetMatchingEngineNotOwner() public {
        vm.prank(alice);
        vm.expectRevert();
        settlementEngine.setMatchingEngine(makeAddr("x"));
    }

    // ─── Admin — setTreasury ─────────────────────────────────────────

    function test_SetTreasury_Success() public {
        address newTreasury = makeAddr("newTreasury");
        vm.prank(owner);
        vm.expectEmit(false, false, false, true);
        emit SettlementEngine.TreasuryUpdated(treasury, newTreasury);
        settlementEngine.setTreasury(newTreasury);

        assertEq(settlementEngine.treasury(), newTreasury);
    }

    function test_RevertWhen_SetTreasuryZero() public {
        vm.prank(owner);
        vm.expectRevert(SettlementEngine.InvalidAddress.selector);
        settlementEngine.setTreasury(address(0));
    }

    // ─── Admin — setEpochLength ──────────────────────────────────────

    function test_SetEpochLength_Success() public {
        vm.prank(owner);
        vm.expectEmit(false, false, false, true);
        emit SettlementEngine.EpochLengthUpdated(EPOCH_LENGTH, 10);
        settlementEngine.setEpochLength(10);

        assertEq(settlementEngine.epochLength(), 10);
    }

    function test_RevertWhen_SetEpochLengthZero() public {
        vm.prank(owner);
        vm.expectRevert(SettlementEngine.InvalidEpochLength.selector);
        settlementEngine.setEpochLength(0);
    }

    // ─── Admin — addToken ────────────────────────────────────────────

    function test_AddToken_Success() public {
        MockERC20 newToken = new MockERC20("DAI", "DAI", 18);
        vm.prank(owner);
        vm.expectEmit(true, false, false, false);
        emit SettlementEngine.TokenAdded(address(newToken));
        settlementEngine.addToken(address(newToken));

        assertTrue(settlementEngine.isTokenSupported(address(newToken)));
        assertEq(settlementEngine.supportedTokenCount(), 3); // base + quote + DAI
    }

    function test_RevertWhen_AddTokenZeroAddress() public {
        vm.prank(owner);
        vm.expectRevert(SettlementEngine.InvalidAddress.selector);
        settlementEngine.addToken(address(0));
    }

    function test_RevertWhen_AddTokenAlreadySupported() public {
        vm.prank(owner);
        vm.expectRevert(
            abi.encodeWithSelector(SettlementEngine.TokenAlreadySupported.selector, address(baseToken))
        );
        settlementEngine.addToken(address(baseToken));
    }

    // ─── View functions ──────────────────────────────────────────────

    function test_BlocksUntilEpoch_ReturnsCorrectValues() public {
        assertEq(settlementEngine.blocksUntilEpoch(), EPOCH_LENGTH);

        _advanceBlocks(3);
        assertEq(settlementEngine.blocksUntilEpoch(), EPOCH_LENGTH - 3);

        _advanceBlocks(3);
        assertEq(settlementEngine.blocksUntilEpoch(), 0);
    }

    // ─── Fuzz Tests ──────────────────────────────────────────────────

    function testFuzz_Deposit(uint256 amount) public {
        amount = bound(amount, 1, 1_000_000e6);

        vm.prank(alice);
        settlementEngine.deposit(address(quoteToken), amount);

        assertEq(settlementEngine.depositedCollateral(alice, address(quoteToken)), amount);
        assertEq(settlementEngine.getVirtualBalance(alice, address(quoteToken)), int256(amount));
    }

    function testFuzz_DepositAndWithdraw(uint256 depositAmt, uint256 withdrawAmt) public {
        depositAmt = bound(depositAmt, 1, 1_000_000e6);
        withdrawAmt = bound(withdrawAmt, 0, depositAmt);

        vm.startPrank(alice);
        settlementEngine.deposit(address(quoteToken), depositAmt);
        if (withdrawAmt > 0) {
            settlementEngine.withdraw(address(quoteToken), withdrawAmt);
        }
        vm.stopPrank();

        assertEq(settlementEngine.depositedCollateral(alice, address(quoteToken)), depositAmt - withdrawAmt);
    }

    function testFuzz_FastSettleFee(int256 netDelta) public {
        netDelta = bound(netDelta, -1e18, 1e18);
        vm.assume(netDelta != 0);

        vm.prank(address(matchingEngine));
        settlementEngine.updateVirtualBalance(alice, address(quoteToken), netDelta);

        vm.prank(alice);
        settlementEngine.fastSettle(address(quoteToken));

        uint256 absNet = netDelta > 0 ? uint256(netDelta) : uint256(-netDelta);
        uint256 expectedFee = absNet / 10_000; // 1 bps

        assertEq(settlementEngine.getVirtualBalance(treasury, address(quoteToken)), int256(expectedFee));
        // epoch delta cleared
        assertEq(settlementEngine.epochNetDelta(alice, address(quoteToken)), 0);
    }

    // ─── Invariant: total virtual balance across users equals sum of deposits + deltas ──

    function test_Invariant_DepositWithdrawBalanceIntegrity() public {
        vm.prank(alice);
        settlementEngine.deposit(address(quoteToken), 10_000e6);

        vm.prank(bob);
        settlementEngine.deposit(address(quoteToken), 5_000e6);

        int256 totalVBal = settlementEngine.getVirtualBalance(alice, address(quoteToken))
            + settlementEngine.getVirtualBalance(bob, address(quoteToken));

        assertEq(totalVBal, int256(15_000e6));

        vm.prank(alice);
        settlementEngine.withdraw(address(quoteToken), 3_000e6);

        totalVBal = settlementEngine.getVirtualBalance(alice, address(quoteToken))
            + settlementEngine.getVirtualBalance(bob, address(quoteToken));

        assertEq(totalVBal, int256(12_000e6));
    }
}
