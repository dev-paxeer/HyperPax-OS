// SPDX-License-Identifier: MIT
pragma solidity ^0.8.24;

import {IAllowanceProvider} from "../../src/interfaces/external/IAllowanceProvider.sol";

/// @dev Mock Argus risk engine for unit tests.
contract MockAllowanceProvider is IAllowanceProvider {
    mapping(address => bool) internal _active;
    mapping(address => mapping(bytes32 => uint256)) internal _maxPositions;
    mapping(address => uint256) internal _maxDrawdowns;

    function setActive(address wallet, bool active) external {
        _active[wallet] = active;
    }

    function setMaxPosition(address wallet, bytes32 marketId, uint256 maxSize) external {
        _maxPositions[wallet][marketId] = maxSize;
    }

    function setMaxDrawdown(address wallet, uint256 maxDrawdown) external {
        _maxDrawdowns[wallet] = maxDrawdown;
    }

    function isActive(address wallet) external view returns (bool active) {
        return _active[wallet];
    }

    function getMaxPosition(address wallet, bytes32 marketId) external view returns (uint256 maxSize) {
        return _maxPositions[wallet][marketId];
    }

    function getMaxDrawdown(address wallet) external view returns (uint256 maxDrawdown) {
        return _maxDrawdowns[wallet];
    }
}
