// SPDX-License-Identifier: MIT
pragma solidity ^0.8.24;

/// @title IPaymentStreams
/// @notice Interface for the PaymentStreams precompile at 0x0906.
/// @dev    Native rate-based payment streams. O(1) settlement at any time.
///         Custody model: escrow at `open` (D3 locked).
///         `cap == 0` (uncapped) is REJECTED in v21 — allowance-pull is deferred.
interface IPaymentStreams {
    struct Stream {
        uint256 id;
        address payer;
        address payee;
        address token;          // address(0) = native PAX
        uint256 ratePerSecond;  // token native units / sec
        uint256 cap;            // total lifetime payout cap (must be > 0)
        uint64  startTime;      // unix seconds
        uint64  stopTime;       // unix seconds; 0 = open-ended
        uint256 settled;        // amount already withdrawn to payee
        bool    active;
    }

    /// @notice Open a new stream. Pulls `cap` of `token` from `msg.sender` to
    ///         the streams module account at this call.
    /// @dev    For native PAX, `msg.sender` must hold sufficient balance.
    ///         For ERC-20, `msg.sender` must have approved the streams module
    ///         account for at least `cap`.
    function open(
        address payee,
        address token,
        uint256 ratePerSecond,
        uint64  startTime,
        uint64  stopTime,
        uint256 cap
    ) external returns (uint256 streamId);

    /// @notice Pull all currently accrued funds. Anyone may call; funds always
    ///         go to `stream.payee`.
    /// @return paid Amount transferred to payee (may be 0 if nothing accrued).
    function settle(uint256 streamId) external returns (uint256 paid);

    /// @notice Stop the stream now: settles outstanding accrual, refunds the
    ///         unspent portion of `cap` to `stream.payer`, deletes the stream.
    ///         May be called by either payer or payee.
    /// @return finalPaid Amount transferred to payee at close time.
    function close(uint256 streamId) external returns (uint256 finalPaid);

    /// @notice Modify the streaming rate. Implicit settle-then-update so the
    ///         payee receives everything accrued at the OLD rate before the
    ///         new rate takes effect.
    /// @dev    Only `stream.payer` may call.
    function updateRate(uint256 streamId, uint256 newRate) external;

    /// @notice View the currently accrued (unsettled) amount for a stream.
    function accrued(uint256 streamId) external view returns (uint256);

    /// @notice View the full Stream struct.
    function getStream(uint256 streamId) external view returns (Stream memory);

    // ── Events ──────────────────────────────────────────────────────────────

    event StreamOpened(
        uint256 indexed streamId,
        address indexed payer,
        address indexed payee,
        address token,
        uint256 ratePerSecond,
        uint256 cap
    );

    /// @notice Emitted on settle/close. The (committed, paid) pair is the
    ///         input shape for the generalized PoFQ ReputationOracle —
    ///         committed = ratePerSecond * elapsed, paid = actual transfer.
    event StreamSettled(
        uint256 indexed streamId,
        address indexed payee,
        uint256 paid,
        uint256 committed
    );

    event StreamClosed(
        uint256 indexed streamId,
        uint256 finalPaid,
        uint256 refund
    );

    event StreamRateUpdated(
        uint256 indexed streamId,
        uint256 oldRate,
        uint256 newRate
    );
}
