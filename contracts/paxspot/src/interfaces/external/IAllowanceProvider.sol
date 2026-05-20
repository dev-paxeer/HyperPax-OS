// SPDX-License-Identifier: MIT
pragma solidity ^0.8.24;

/// @title IAllowanceProvider
/// @notice Callback interface for querying per-address position limits set by the Argus risk engine.
///         PaxSpot calls this before accepting orders from funded smart wallets.
/// @dev Deployed and maintained by the AVM / Argus system. PaxSpot treats this as an external dependency.
interface IAllowanceProvider {
    /// @notice Get the maximum position size allowed for a wallet in a given market.
    /// @param wallet The funded smart wallet address.
    /// @param marketId The market identifier.
    /// @return maxSize Maximum absolute position size allowed (base asset units).
    function getMaxPosition(address wallet, bytes32 marketId) external view returns (uint256 maxSize);

    /// @notice Check whether a wallet is currently active (not frozen by risk engine).
    /// @param wallet The funded smart wallet address.
    /// @return active True if the wallet may trade; false if frozen/liquidated.
    function isActive(address wallet) external view returns (bool active);

    /// @notice Get the maximum notional drawdown allowed before auto-liquidation.
    /// @param wallet The funded smart wallet address.
    /// @return maxDrawdown Maximum drawdown in quote denomination.
    function getMaxDrawdown(address wallet) external view returns (uint256 maxDrawdown);
}
