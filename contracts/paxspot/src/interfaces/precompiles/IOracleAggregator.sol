// SPDX-License-Identifier: MIT
pragma solidity ^0.8.24;

/// @title IOracleAggregator
/// @notice Interface for the OracleAggregator / VOM precompile at 0x903.
///         Provides stateless multi-feed aggregation and stateful validator oracle reads.
/// @dev getValidatorPrice reads from the x/paxoracle Cosmos SDK module state.
interface IOracleAggregator {
    struct PriceFeed {
        int256 price;
        uint256 confidence;
        uint256 timestamp;
    }

    /// @notice Aggregate multiple price feeds into a confidence-weighted median.
    /// @param feeds Array of price feeds.
    /// @return price Aggregated price (18 decimals).
    /// @return confidence Aggregated confidence (18 decimals, 0–1e18).
    function aggregate(PriceFeed[] calldata feeds) external pure returns (int256 price, uint256 confidence);

    /// @notice Get the validator consensus price for a market from the VOM.
    /// @dev Reads from on-chain state — validator attestations submitted via Cosmos tx.
    /// @param marketId The market identifier (e.g., keccak256("ETH/USDC")).
    /// @return price Median price from validator attestations (18 decimals).
    /// @return quorum Number of validators that attested.
    /// @return timestamp Block number of the oldest attestation in the quorum.
    function getValidatorPrice(bytes32 marketId)
        external
        view
        returns (int256 price, uint256 quorum, uint256 timestamp);

    /// @notice Submit a validator price attestation directly via EVM tx.
    /// @dev Caller must be an active validator. Bypasses Cosmos tx path.
    /// @param marketId The market identifier (e.g., keccak256("ETH/USDC")).
    /// @param price The attested price (18 decimals).
    /// @param confidence The confidence level (0, 1e18].
    /// @return success True if the submission was stored.
    function submitPrice(bytes32 marketId, int256 price, uint256 confidence)
        external
        returns (bool success);
}
