// SPDX-License-Identifier: MIT
pragma solidity ^0.8.24;

/// @title IPoFQScorer
/// @notice Interface for the PoFQ (Proof-of-Fill-Quality) precompile at 0x904.
///         Computes fill quality scores for vault/LP reputation tracking.
/// @dev All scores are 18-decimal fixed-point (0 = worst, 1e18 = perfect oracle match).
interface IPoFQScorer {
    struct Fill {
        int256 fillPrice;
        int256 oraclePrice;
        uint128 size;
    }

    /// @notice Compute fill quality score for a single fill.
    /// @param fillPrice Actual fill price (18 decimals).
    /// @param oraclePrice Oracle price at fill time (18 decimals).
    /// @return score Quality score (18 decimals, 0 = worst, 1e18 = perfect).
    function scoreFill(int256 fillPrice, int256 oraclePrice) external pure returns (uint256 score);

    /// @notice Compute volume-weighted average fill quality for a batch.
    /// @param fills Array of fills to score.
    /// @return avgScore Volume-weighted average score (18 decimals).
    /// @return totalVolume Total volume scored.
    function scoreBatch(Fill[] calldata fills) external pure returns (uint256 avgScore, uint256 totalVolume);

    /// @notice Update a rolling score with new batch data and exponential decay.
    /// @param currentScore Current rolling score (18 decimals).
    /// @param currentWeight Current weight (accumulated volume).
    /// @param newScore New batch score.
    /// @param newWeight New batch volume.
    /// @param decayBps Decay rate in bps (e.g., 100 = 1% decay per update).
    /// @return updatedScore New rolling score after decay + new data.
    /// @return updatedWeight New accumulated weight.
    function updateRollingScore(
        uint256 currentScore,
        uint256 currentWeight,
        uint256 newScore,
        uint256 newWeight,
        uint16 decayBps
    ) external pure returns (uint256 updatedScore, uint256 updatedWeight);
}
