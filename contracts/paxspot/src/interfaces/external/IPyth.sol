// SPDX-License-Identifier: MIT
pragma solidity ^0.8.24;

/// @title IPyth
/// @notice Minimal interface for Pyth Network price feeds (pull model).
///         Only the methods PaxSpot actually calls are declared here.
/// @dev Full Pyth interface: https://github.com/pyth-network/pyth-sdk-solidity
interface IPyth {
    struct Price {
        int64 price;
        uint64 conf;
        int32 expo;
        uint256 publishTime;
    }

    /// @notice Get the most recent price for a feed, reverting if stale beyond the caller's threshold.
    /// @param id The Pyth price feed ID.
    /// @param age Maximum acceptable staleness in seconds.
    /// @return price The price data.
    function getPriceNoOlderThan(bytes32 id, uint256 age) external view returns (Price memory price);

    /// @notice Get the most recent price regardless of staleness.
    /// @param id The Pyth price feed ID.
    /// @return price The price data.
    function getPriceUnsafe(bytes32 id) external view returns (Price memory price);

    /// @notice Update price feeds with fresh Pyth data (user/relayer submits as calldata).
    /// @param updateData The encoded price update data from Pyth's off-chain service.
    function updatePriceFeeds(bytes[] calldata updateData) external payable;

    /// @notice Get the fee required to update price feeds.
    /// @param updateData The encoded price update data.
    /// @return feeAmount The required fee in native token.
    function getUpdateFee(bytes[] calldata updateData) external view returns (uint256 feeAmount);
}
