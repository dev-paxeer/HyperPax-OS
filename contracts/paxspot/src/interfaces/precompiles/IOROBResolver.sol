// SPDX-License-Identifier: MIT
pragma solidity ^0.8.24;

/// @title IOROBResolver
/// @notice Interface for the OROB (Oracle-Relative Order Book) precompile at 0x901.
///         Resolves basis-point offsets from oracle price to absolute prices.
/// @dev All prices use 18 decimal fixed-point representation (int256).
interface IOROBResolver {
    /// @notice Resolve a basis-point offset to an absolute price.
    /// @param oraclePrice Current oracle price (18 decimals, signed).
    /// @param offsetBps Signed basis-point offset from oracle (-10000 to +10000).
    ///                  Negative = below oracle (buy side), Positive = above oracle (sell side).
    /// @return absolutePrice The resolved absolute price (18 decimals).
    function resolveOffset(int256 oraclePrice, int16 offsetBps) external pure returns (int256 absolutePrice);

    /// @notice Batch-resolve multiple offsets in one call.
    /// @param oraclePrice Current oracle price.
    /// @param offsetsBps Array of signed basis-point offsets.
    /// @return absolutePrices Array of resolved absolute prices.
    function resolveOffsetBatch(int256 oraclePrice, int16[] calldata offsetsBps)
        external
        pure
        returns (int256[] memory absolutePrices);

    /// @notice Convert an absolute price back to the nearest bps offset.
    /// @param oraclePrice Current oracle price.
    /// @param absolutePrice The absolute price to convert.
    /// @return offsetBps The nearest basis-point offset.
    function toOffset(int256 oraclePrice, int256 absolutePrice) external pure returns (int16 offsetBps);
}
