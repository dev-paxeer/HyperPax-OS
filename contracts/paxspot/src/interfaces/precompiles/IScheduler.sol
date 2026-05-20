// SPDX-License-Identifier: MIT
pragma solidity ^0.8.24;

/// @title IScheduler
/// @notice Interface for the Scheduler precompile at 0x0905.
/// @dev    Native cron. Schedule arbitrary EVM calls to fire at a future block
///         height with a deposit that pays for execution. The chain's
///         `x/scheduler` module's EndBlocker dispatches due jobs each block.
interface IScheduler {
    struct Job {
        uint256 id;
        address creator;
        address target;
        bytes   callData;
        uint64  executeAtBlock;
        uint64  gasLimit;
        uint256 deposit;
        bool    active;
    }

    /// @notice Register a job. `msg.value` funds the deposit.
    /// @dev    Reverts with `ErrInvalidGasLimit` if `gasLimit == 0`,
    ///         `ErrInvalidTarget` if `target == address(0)`,
    ///         `ErrPastBlock` if `executeAtBlock <= block.number`,
    ///         `ErrHorizonExceeded` if the schedule is too far in the future,
    ///         `ErrMaxJobsExceeded` if `msg.sender` already owns the cap,
    ///         `ErrDepositTooLow` if `msg.value < gasLimit*baseFee*minDepositFactor`.
    /// @return jobId Monotonic job identifier (1-based).
    function schedule(
        address target,
        bytes calldata callData,
        uint64 executeAtBlock,
        uint64 gasLimit
    ) external payable returns (uint256 jobId);

    /// @notice Cancel a pending job. Refunds the remaining deposit to the creator.
    /// @dev    Only `creator` may cancel.
    function cancel(uint256 jobId) external;

    /// @notice Move a pending job to a new execution height. Atomic
    ///         (cancel + reschedule). Cannot move a job into the past or beyond
    ///         the schedule horizon.
    function reschedule(uint256 jobId, uint64 newBlock) external;

    /// @notice View a single job.
    function getJob(uint256 jobId) external view returns (Job memory);

    /// @notice List all pending job IDs owned by `creator`.
    function pending(address creator) external view returns (uint256[] memory);

    // ── Events ──────────────────────────────────────────────────────────────

    event JobScheduled(
        uint256 indexed jobId,
        address indexed creator,
        address target,
        uint64  executeAtBlock,
        uint64  gasLimit,
        uint256 deposit
    );

    event JobCancelled(
        uint256 indexed jobId,
        address indexed creator,
        uint256 refund
    );

    event JobRescheduled(
        uint256 indexed jobId,
        uint64 newBlock
    );

    /// @notice Emitted by the EndBlocker after a successful dispatch.
    event JobExecuted(
        uint256 indexed jobId,
        address indexed creator,
        uint64  gasUsed,
        uint256 refund
    );

    /// @notice Emitted by the EndBlocker when a dispatch reverts. The block is
    ///         NOT halted; the deposit is refunded.
    event JobFailed(
        uint256 indexed jobId,
        address indexed creator,
        string  reason
    );
}
