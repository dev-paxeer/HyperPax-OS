// SPDX-License-Identifier: MIT
pragma solidity ^0.8.24;

/// @title JobTarget
/// @notice Minimal contract used by the Scheduler smoke test. Tracks how many
///         times it has been poked + by whom, so we can prove a scheduled job
///         actually fired.
contract JobTarget {
    uint256 public count;
    address public lastCaller;
    uint256 public lastBlock;

    event Poked(address indexed caller, uint256 count, uint256 blockNumber);

    function poke() external {
        unchecked {
            count += 1;
        }
        lastCaller = msg.sender;
        lastBlock = block.number;
        emit Poked(msg.sender, count, block.number);
    }
}
