// SPDX-License-Identifier: MIT
pragma solidity ^0.8.24;

/// @title IBatchClearing
/// @notice Interface for the BatchClearing precompile at 0x902.
///         Computes the uniform clearing price for a sealed-bid batch auction.
/// @dev Buy orders sorted by offset descending (most aggressive first).
///      Sell orders sorted by offset ascending (cheapest first).
interface IBatchClearing {
    struct Order {
        int16 offsetBps;
        uint128 size;
    }

    /// @notice Compute the uniform clearing price for a batch.
    /// @param oraclePrice Current oracle price (18 decimals).
    /// @param buyOrders Array of buy orders (sorted by offset descending).
    /// @param sellOrders Array of sell orders (sorted by offset ascending).
    /// @return clearingOffsetBps The clearing price as an OROB offset.
    /// @return clearingPrice The absolute clearing price (18 decimals).
    /// @return matchedVolume Total volume matched at clearing price.
    function computeClearing(int256 oraclePrice, Order[] calldata buyOrders, Order[] calldata sellOrders)
        external
        pure
        returns (int16 clearingOffsetBps, int256 clearingPrice, uint256 matchedVolume);
}
