// SPDX-License-Identifier: MIT
pragma solidity ^0.8.24;

/// @title IPaxSpotReader
/// @notice Read-only interface exposed to the Argus VM (AVM) for querying PaxSpot state.
///         AVM indexes these reads to drive capital allocation, risk scoring, and drawdown policy.
/// @dev Implemented by PaxSpotRouter. AVM calls this via its EVM bridge.
interface IPaxSpotReader {
    /// @notice Get the current PoFQ rolling score for an address.
    /// @param trader The trader or vault address.
    /// @return score Rolling fill-quality score (18 decimals, 0–1e18).
    /// @return totalVolume Lifetime volume contributing to the score.
    function getPoFQScore(address trader) external view returns (uint256 score, uint256 totalVolume);

    /// @notice Get realized PnL for a trader in a specific token.
    /// @param trader The trader address.
    /// @param token The quote token address.
    /// @return pnl Signed realized PnL (positive = profit).
    function getRealizedPnL(address trader, address token) external view returns (int256 pnl);

    /// @notice Get a trader's current open position in a market.
    /// @param trader The trader address.
    /// @param marketId The market identifier.
    /// @return size Signed position size (positive = long, negative = short).
    /// @return avgEntryPrice Volume-weighted average entry price (18 decimals).
    function getPosition(address trader, bytes32 marketId) external view returns (int256 size, int256 avgEntryPrice);

    /// @notice Get a trader's total trading volume over a rolling window.
    /// @param trader The trader address.
    /// @param windowBlocks Number of blocks to look back.
    /// @return volume Total volume in quote denomination.
    function getVolume(address trader, uint256 windowBlocks) external view returns (uint256 volume);
}
