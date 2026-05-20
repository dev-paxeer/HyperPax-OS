// SPDX-License-Identifier: MIT
pragma solidity ^0.8.24;

/// @title IInsuranceFund
/// @notice Interface for the insurance fund that absorbs protocol losses from oracle failure,
///         smart wallet bad debt, and settlement edge cases.
/// @dev Phase 1: deposit/withdraw only. Auto-payout triggers added in Phase 2.
interface IInsuranceFund {
    /// @notice Deposit funds into the insurance pool.
    /// @param token The ERC-20 token to deposit.
    /// @param amount The amount to deposit.
    function deposit(address token, uint256 amount) external;

    /// @notice Get the current balance of the insurance fund for a token.
    /// @param token The ERC-20 token address.
    /// @return balance Current fund balance.
    function getBalance(address token) external view returns (uint256 balance);

    /// @notice Request a payout from the insurance fund (restricted to MatchingEngine).
    /// @param token The ERC-20 token.
    /// @param amount The payout amount.
    /// @param recipient The address receiving the payout.
    function payout(address token, uint256 amount, address recipient) external;
}
