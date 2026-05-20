// SPDX-License-Identifier: MIT
pragma solidity ^0.8.24;

import {IPyth} from "../../src/interfaces/external/IPyth.sol";

/// @dev Mock Pyth oracle for unit tests. Allows setting arbitrary prices per feed.
contract MockPyth is IPyth {
    mapping(bytes32 => Price) internal _prices;
    bool public shouldRevert;

    function setPrice(bytes32 feedId, int64 price, uint64 conf, int32 expo, uint256 publishTime) external {
        _prices[feedId] = Price({price: price, conf: conf, expo: expo, publishTime: publishTime});
    }

    function setShouldRevert(bool _shouldRevert) external {
        shouldRevert = _shouldRevert;
    }

    function getPriceUnsafe(bytes32 id) external view returns (Price memory price) {
        if (shouldRevert) revert("MockPyth: forced revert");
        return _prices[id];
    }

    function getPriceNoOlderThan(bytes32 id, uint256 age) external view returns (Price memory price) {
        if (shouldRevert) revert("MockPyth: forced revert");
        Price memory p = _prices[id];
        require(block.timestamp - p.publishTime <= age, "MockPyth: stale");
        return p;
    }

    function updatePriceFeeds(bytes[] calldata) external payable {}

    function getUpdateFee(bytes[] calldata) external pure returns (uint256 feeAmount) {
        return 0;
    }
}
