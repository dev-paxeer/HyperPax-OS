// SPDX-License-Identifier: MIT
pragma solidity ^0.8.24;

/// @title PaxSpotTypes
/// @notice Shared type definitions used across PaxSpot core contracts.
library PaxSpotTypes {
    enum Side {
        BUY,
        SELL
    }

    enum OrderType {
        MARKET,
        LIMIT
    }

    enum MarketMode {
        CONTINUOUS,
        BATCH
    }

    struct Order {
        uint256 id;
        address trader;
        bytes32 marketId;
        Side side;
        OrderType orderType;
        int16 offsetBps;
        uint128 size;
        uint128 filledSize;
        uint256 blockSubmitted;
        bool active;
    }

    struct MarketState {
        bytes32 marketId;
        address baseToken;
        address quoteToken;
        MarketMode mode;
        bool active;
        uint128 minOrderSize;
        int16 maxOffsetBps;
        uint16 takerFeeBps;
        int16 makerRebateBps;
        uint256 batchModeUntilBlock;
    }

    struct FillResult {
        uint256 buyOrderId;
        uint256 sellOrderId;
        address buyer;
        address seller;
        uint128 fillSize;
        int256 fillPrice;
        int256 oraclePrice;
        uint256 fillBlock;
    }
}
