// SPDX-License-Identifier: MIT
pragma solidity ^0.8.24;

import {Ownable} from "@openzeppelin/contracts/access/Ownable.sol";
import {Pausable} from "@openzeppelin/contracts/utils/Pausable.sol";
import {ReentrancyGuard} from "@openzeppelin/contracts/utils/ReentrancyGuard.sol";
import {IOROBResolver} from "../interfaces/precompiles/IOROBResolver.sol";
import {IBatchClearing} from "../interfaces/precompiles/IBatchClearing.sol";
import {IPoFQScorer} from "../interfaces/precompiles/IPoFQScorer.sol";
import {OracleAdapter} from "./OracleAdapter.sol";
import {SettlementEngine} from "./SettlementEngine.sol";
import {PaxSpotTypes} from "./PaxSpotTypes.sol";

/// @title MatchingEngine
/// @notice Core order matching for PaxSpot. Supports two execution modes per market:
///         - Continuous: orders fill within the block they arrive (walk the book).
///         - Batch: sealed-bid block auction with uniform clearing price via precompile.
///         All orders are OROB (Oracle-Relative Order Book) — stored as bps offsets from oracle.
/// @dev Uses precompiles 0x901 (OROB), 0x902 (BatchClearing), 0x904 (PoFQ) for
///      gas-efficient computation. Fills update virtual balances in SettlementEngine.
contract MatchingEngine is Ownable, Pausable, ReentrancyGuard {
    using PaxSpotTypes for PaxSpotTypes.Side;

    // ─── Constants ─────────────────────────────────────────────────────
    IOROBResolver public constant OROB = IOROBResolver(0x0000000000000000000000000000000000000901);
    IBatchClearing public constant CLEARING = IBatchClearing(0x0000000000000000000000000000000000000902);
    IPoFQScorer public constant POFQ = IPoFQScorer(0x0000000000000000000000000000000000000904);

    uint256 public constant BPS_DENOMINATOR = 10_000;
    uint256 public constant PRICE_DECIMALS = 1e18;

    // ─── State Variables ───────────────────────────────────────────────
    OracleAdapter public oracle;
    SettlementEngine public settlement;
    address public router;

    uint256 public nextOrderId;

    mapping(bytes32 => PaxSpotTypes.MarketState) public marketStates;
    bytes32[] public marketList;

    mapping(uint256 => PaxSpotTypes.Order) public orders;

    mapping(bytes32 => uint256[]) internal _buyOrderIds;
    mapping(bytes32 => uint256[]) internal _sellOrderIds;

    mapping(bytes32 => IBatchClearing.Order[]) internal _batchBuys;
    mapping(bytes32 => IBatchClearing.Order[]) internal _batchSells;
    mapping(bytes32 => address[]) internal _batchBuyTraders;
    mapping(bytes32 => address[]) internal _batchSellTraders;
    mapping(bytes32 => uint256) internal _batchBlock;

    mapping(address => uint256) public poFQScores;
    mapping(address => uint256) public poFQWeights;

    mapping(bytes32 => uint256) public rollingVolume50;
    mapping(bytes32 => uint256) public lastVolumeBlock;

    // ─── Events ────────────────────────────────────────────────────────
    event MarketCreated(
        bytes32 indexed marketId, address baseToken, address quoteToken, PaxSpotTypes.MarketMode mode
    );
    event MarketModeChanged(bytes32 indexed marketId, PaxSpotTypes.MarketMode oldMode, PaxSpotTypes.MarketMode newMode);
    event OrderPlaced(
        uint256 indexed orderId,
        bytes32 indexed marketId,
        address indexed trader,
        PaxSpotTypes.Side side,
        PaxSpotTypes.OrderType orderType,
        int16 offsetBps,
        uint128 size
    );
    event OrderCancelled(uint256 indexed orderId, address indexed trader);
    event OrderFilled(
        uint256 indexed buyOrderId,
        uint256 indexed sellOrderId,
        bytes32 indexed marketId,
        uint128 fillSize,
        int256 fillPrice,
        int256 oraclePrice
    );
    event BatchCleared(
        bytes32 indexed marketId,
        int16 clearingOffsetBps,
        int256 clearingPrice,
        uint256 matchedVolume,
        uint256 numBuys,
        uint256 numSells
    );
    event PoFQUpdated(address indexed trader, uint256 newScore, uint256 newWeight);
    event MarketCircuitBreaker(bytes32 indexed marketId, uint256 blockNumber);

    // ─── Errors ────────────────────────────────────────────────────────
    error OnlyRouter();
    error MarketNotActive(bytes32 marketId);
    error MarketAlreadyExists(bytes32 marketId);
    error OrderNotActive(uint256 orderId);
    error NotOrderOwner(uint256 orderId);
    error OrderSizeTooSmall(uint128 size, uint128 minSize);
    error OffsetExceedsMax(int16 offset, int16 maxOffset);
    error InvalidMarketParams();
    error BatchNotReady(bytes32 marketId);
    error InsufficientVirtualBalance(address user, address token);
    error MarketOrderRequiresContinuousMode(bytes32 marketId);

    // ─── Modifiers ─────────────────────────────────────────────────────
    modifier onlyRouter() {
        if (msg.sender != router) revert OnlyRouter();
        _;
    }

    // ─── Constructor ───────────────────────────────────────────────────
    constructor(address _oracle, address _settlement) Ownable(msg.sender) {
        oracle = OracleAdapter(_oracle);
        settlement = SettlementEngine(_settlement);
        nextOrderId = 1;
    }

    // ─── External Functions (Router-only) ──────────────────────────────

    /// @notice Place a new order into the book. Called by PaxSpotRouter after validation.
    /// @param trader The order owner.
    /// @param marketId The market to trade.
    /// @param side BUY or SELL.
    /// @param orderType MARKET or LIMIT.
    /// @param offsetBps OROB offset from oracle (ignored for MARKET orders in continuous mode).
    /// @param size Order size in base asset units.
    /// @return orderId The assigned order ID.
    function placeOrder(
        address trader,
        bytes32 marketId,
        PaxSpotTypes.Side side,
        PaxSpotTypes.OrderType orderType,
        int16 offsetBps,
        uint128 size
    ) external onlyRouter whenNotPaused returns (uint256 orderId) {
        PaxSpotTypes.MarketState storage market = marketStates[marketId];
        if (!market.active) revert MarketNotActive(marketId);
        if (size < market.minOrderSize) revert OrderSizeTooSmall(size, market.minOrderSize);

        if (orderType == PaxSpotTypes.OrderType.LIMIT) {
            int16 absOffset = offsetBps >= 0 ? offsetBps : -offsetBps;
            if (absOffset > market.maxOffsetBps) revert OffsetExceedsMax(offsetBps, market.maxOffsetBps);
        }

        orderId = nextOrderId++;

        orders[orderId] = PaxSpotTypes.Order({
            id: orderId,
            trader: trader,
            marketId: marketId,
            side: side,
            orderType: orderType,
            offsetBps: offsetBps,
            size: size,
            filledSize: 0,
            blockSubmitted: block.number,
            active: true
        });

        emit OrderPlaced(orderId, marketId, trader, side, orderType, offsetBps, size);

        if (market.mode == PaxSpotTypes.MarketMode.CONTINUOUS) {
            _tryContinuousMatch(orderId, market);
        } else {
            _addToBatch(orderId, trader, marketId, side, offsetBps, size);
        }
    }

    /// @notice Cancel an active order.
    /// @param orderId The order to cancel.
    /// @param trader The order owner (verified by router).
    function cancelOrder(uint256 orderId, address trader) external onlyRouter {
        PaxSpotTypes.Order storage order = orders[orderId];
        if (!order.active) revert OrderNotActive(orderId);
        if (order.trader != trader) revert NotOrderOwner(orderId);

        order.active = false;
        emit OrderCancelled(orderId, trader);
    }

    /// @notice Trigger batch clearing for a market. Callable by anyone (validator keepers).
    /// @param marketId The market to clear.
    function clearBatch(bytes32 marketId) external nonReentrant whenNotPaused {
        PaxSpotTypes.MarketState storage market = marketStates[marketId];
        if (!market.active) revert MarketNotActive(marketId);
        if (market.mode != PaxSpotTypes.MarketMode.BATCH) revert BatchNotReady(marketId);
        if (_batchBlock[marketId] >= block.number) revert BatchNotReady(marketId);

        IBatchClearing.Order[] storage buys = _batchBuys[marketId];
        IBatchClearing.Order[] storage sells = _batchSells[marketId];

        if (buys.length == 0 || sells.length == 0) {
            _clearBatchState(marketId);
            return;
        }

        (int256 oraclePrice,,) = oracle.getPrice(marketId);

        (int16 clearingOffsetBps, int256 clearingPrice, uint256 matchedVolume) =
            CLEARING.computeClearing(oraclePrice, buys, sells);

        if (matchedVolume > 0) {
            _executeBatchFills(marketId, clearingPrice, oraclePrice, matchedVolume, market);
        }

        emit BatchCleared(marketId, clearingOffsetBps, clearingPrice, matchedVolume, buys.length, sells.length);

        _clearBatchState(marketId);
        _batchBlock[marketId] = block.number;
    }

    // ─── Admin Functions ───────────────────────────────────────────────

    /// @notice Create a new market.
    function createMarket(
        bytes32 marketId,
        address baseToken,
        address quoteToken,
        PaxSpotTypes.MarketMode mode,
        uint128 minOrderSize,
        int16 maxOffsetBps,
        uint16 takerFeeBps,
        int16 makerRebateBps
    ) external onlyOwner {
        if (marketStates[marketId].active) revert MarketAlreadyExists(marketId);
        if (baseToken == address(0) || quoteToken == address(0)) revert InvalidMarketParams();
        if (maxOffsetBps <= 0 || takerFeeBps > 100) revert InvalidMarketParams();

        marketStates[marketId] = PaxSpotTypes.MarketState({
            marketId: marketId,
            baseToken: baseToken,
            quoteToken: quoteToken,
            mode: mode,
            active: true,
            minOrderSize: minOrderSize,
            maxOffsetBps: maxOffsetBps,
            takerFeeBps: takerFeeBps,
            makerRebateBps: makerRebateBps,
            batchModeUntilBlock: 0
        });

        marketList.push(marketId);
        emit MarketCreated(marketId, baseToken, quoteToken, mode);
    }

    /// @notice Switch a market's execution mode.
    /// @param marketId The market to update.
    /// @param newMode CONTINUOUS or BATCH.
    function setMarketMode(bytes32 marketId, PaxSpotTypes.MarketMode newMode) external onlyOwner {
        PaxSpotTypes.MarketState storage market = marketStates[marketId];
        if (!market.active) revert MarketNotActive(marketId);

        PaxSpotTypes.MarketMode oldMode = market.mode;
        market.mode = newMode;
        emit MarketModeChanged(marketId, oldMode, newMode);
    }

    /// @notice Set the router contract address.
    /// @param _router The PaxSpotRouter contract.
    function setRouter(address _router) external onlyOwner {
        router = _router;
    }

    /// @notice Pause matching (emergency).
    function pause() external onlyOwner {
        _pause();
    }

    /// @notice Unpause matching.
    function unpause() external onlyOwner {
        _unpause();
    }

    // ─── View Functions ────────────────────────────────────────────────

    /// @notice Get a trader's current PoFQ score.
    /// @param trader The trader address.
    /// @return score Rolling fill quality score (18 decimals).
    /// @return weight Total volume contributing to the score.
    function getPoFQScore(address trader) external view returns (uint256 score, uint256 weight) {
        return (poFQScores[trader], poFQWeights[trader]);
    }

    /// @notice Get the number of resting buy orders for a market.
    /// @param marketId The market.
    /// @return count Number of buy orders in the book.
    function buyOrderCount(bytes32 marketId) external view returns (uint256 count) {
        return _buyOrderIds[marketId].length;
    }

    /// @notice Get the number of resting sell orders for a market.
    /// @param marketId The market.
    /// @return count Number of sell orders in the book.
    function sellOrderCount(bytes32 marketId) external view returns (uint256 count) {
        return _sellOrderIds[marketId].length;
    }

    /// @notice Get the number of markets.
    /// @return count Market count.
    function marketCount() external view returns (uint256 count) {
        return marketList.length;
    }

    /// @notice Get the batch queue sizes for a market.
    /// @param marketId The market.
    /// @return numBuys Number of queued buy orders.
    /// @return numSells Number of queued sell orders.
    function batchQueueSize(bytes32 marketId) external view returns (uint256 numBuys, uint256 numSells) {
        return (_batchBuys[marketId].length, _batchSells[marketId].length);
    }

    // ─── Internal Functions ────────────────────────────────────────────

    /// @dev Attempt to match a new order against resting orders in continuous mode.
    function _tryContinuousMatch(uint256 incomingOrderId, PaxSpotTypes.MarketState storage market) internal {
        PaxSpotTypes.Order storage incoming = orders[incomingOrderId];
        bytes32 marketId = incoming.marketId;

        (int256 oraclePrice,,) = oracle.getPrice(marketId);

        if (incoming.side == PaxSpotTypes.Side.BUY) {
            _matchAgainstBook(incoming, _sellOrderIds[marketId], oraclePrice, market);
            if (incoming.filledSize < incoming.size && incoming.orderType == PaxSpotTypes.OrderType.LIMIT) {
                _buyOrderIds[marketId].push(incomingOrderId);
            }
        } else {
            _matchAgainstBook(incoming, _buyOrderIds[marketId], oraclePrice, market);
            if (incoming.filledSize < incoming.size && incoming.orderType == PaxSpotTypes.OrderType.LIMIT) {
                _sellOrderIds[marketId].push(incomingOrderId);
            }
        }

        if (incoming.filledSize >= incoming.size) {
            incoming.active = false;
        }
    }

    /// @dev Walk the opposing book and fill where prices cross.
    function _matchAgainstBook(
        PaxSpotTypes.Order storage incoming,
        uint256[] storage opposingIds,
        int256 oraclePrice,
        PaxSpotTypes.MarketState storage market
    ) internal {
        uint128 remainingSize = incoming.size - incoming.filledSize;
        if (remainingSize == 0) return;

        int256 incomingAbsPrice;
        if (incoming.orderType == PaxSpotTypes.OrderType.MARKET) {
            incomingAbsPrice = incoming.side == PaxSpotTypes.Side.BUY ? type(int256).max : int256(0);
        } else {
            incomingAbsPrice = OROB.resolveOffset(oraclePrice, incoming.offsetBps);
        }

        for (uint256 i = 0; i < opposingIds.length && remainingSize > 0; i++) {
            PaxSpotTypes.Order storage resting = orders[opposingIds[i]];
            if (!resting.active) continue;

            uint128 restingRemaining = resting.size - resting.filledSize;
            if (restingRemaining == 0) {
                resting.active = false;
                continue;
            }

            int256 restingAbsPrice = OROB.resolveOffset(oraclePrice, resting.offsetBps);

            bool pricesCross;
            if (incoming.side == PaxSpotTypes.Side.BUY) {
                pricesCross = incomingAbsPrice >= restingAbsPrice;
            } else {
                pricesCross = incomingAbsPrice <= restingAbsPrice;
            }

            if (!pricesCross) continue;

            uint128 fillSize = remainingSize < restingRemaining ? remainingSize : restingRemaining;

            int256 fillPrice = restingAbsPrice;

            _executeFill(incoming, resting, fillSize, fillPrice, oraclePrice, market);

            remainingSize -= fillSize;

            if (resting.filledSize >= resting.size) {
                resting.active = false;
            }
        }
    }

    /// @dev Execute a single fill between two orders — update virtual balances, PoFQ, fees.
    function _executeFill(
        PaxSpotTypes.Order storage buyOrder,
        PaxSpotTypes.Order storage sellOrder,
        uint128 fillSize,
        int256 fillPrice,
        int256 oraclePrice,
        PaxSpotTypes.MarketState storage market
    ) internal {
        buyOrder.filledSize += fillSize;
        sellOrder.filledSize += fillSize;

        address buyer;
        address seller;

        if (buyOrder.side == PaxSpotTypes.Side.BUY) {
            buyer = buyOrder.trader;
            seller = sellOrder.trader;
        } else {
            buyer = sellOrder.trader;
            seller = buyOrder.trader;
        }

        int256 quoteAmount = (fillPrice * int256(uint256(fillSize))) / int256(PRICE_DECIMALS);
        uint256 absQuote = quoteAmount >= 0 ? uint256(quoteAmount) : uint256(-quoteAmount);

        uint256 takerFee = (absQuote * market.takerFeeBps) / BPS_DENOMINATOR;

        settlement.updateVirtualBalance(buyer, market.baseToken, int256(uint256(fillSize)));
        settlement.updateVirtualBalance(buyer, market.quoteToken, -quoteAmount - int256(takerFee));

        settlement.updateVirtualBalance(seller, market.baseToken, -int256(uint256(fillSize)));
        settlement.updateVirtualBalance(seller, market.quoteToken, quoteAmount);

        _updatePoFQ(buyer, fillPrice, oraclePrice, fillSize);
        _updatePoFQ(seller, fillPrice, oraclePrice, fillSize);

        emit OrderFilled(buyOrder.id, sellOrder.id, buyOrder.marketId, fillSize, fillPrice, oraclePrice);
    }

    /// @dev Add an order to the batch queue.
    function _addToBatch(
        uint256 orderId,
        address trader,
        bytes32 marketId,
        PaxSpotTypes.Side side,
        int16 offsetBps,
        uint128 size
    ) internal {
        IBatchClearing.Order memory batchOrder = IBatchClearing.Order({offsetBps: offsetBps, size: size});

        if (side == PaxSpotTypes.Side.BUY) {
            _batchBuys[marketId].push(batchOrder);
            _batchBuyTraders[marketId].push(trader);
        } else {
            _batchSells[marketId].push(batchOrder);
            _batchSellTraders[marketId].push(trader);
        }
    }

    /// @dev Execute fills from a batch clearing result.
    function _executeBatchFills(
        bytes32 marketId,
        int256 clearingPrice,
        int256 oraclePrice,
        uint256 matchedVolume,
        PaxSpotTypes.MarketState storage market
    ) internal {
        uint256 remainingVolume = matchedVolume;
        uint256 buyIdx = 0;
        uint256 sellIdx = 0;

        IBatchClearing.Order[] storage buys = _batchBuys[marketId];
        IBatchClearing.Order[] storage sells = _batchSells[marketId];

        while (remainingVolume > 0 && buyIdx < buys.length && sellIdx < sells.length) {
            uint128 fillSize = _minSize(buys[buyIdx].size, sells[sellIdx].size, uint128(remainingVolume));

            _settleBatchFill(
                marketId, buyIdx, sellIdx, fillSize, clearingPrice, oraclePrice, market
            );

            remainingVolume -= fillSize;
            buys[buyIdx].size -= fillSize;
            sells[sellIdx].size -= fillSize;

            if (buys[buyIdx].size == 0) buyIdx++;
            if (sells[sellIdx].size == 0) sellIdx++;
        }
    }

    /// @dev Settle a single batch fill — extracted to avoid stack-too-deep.
    function _settleBatchFill(
        bytes32 marketId,
        uint256 buyIdx,
        uint256 sellIdx,
        uint128 fillSize,
        int256 clearingPrice,
        int256 oraclePrice,
        PaxSpotTypes.MarketState storage market
    ) internal {
        address buyer = _batchBuyTraders[marketId][buyIdx];
        address seller = _batchSellTraders[marketId][sellIdx];

        int256 quoteAmount = (clearingPrice * int256(uint256(fillSize))) / int256(PRICE_DECIMALS);
        uint256 takerFee = _computeFee(quoteAmount, market.takerFeeBps);

        settlement.updateVirtualBalance(buyer, market.baseToken, int256(uint256(fillSize)));
        settlement.updateVirtualBalance(buyer, market.quoteToken, -quoteAmount - int256(takerFee));

        settlement.updateVirtualBalance(seller, market.baseToken, -int256(uint256(fillSize)));
        settlement.updateVirtualBalance(seller, market.quoteToken, quoteAmount);

        _updatePoFQ(buyer, clearingPrice, oraclePrice, fillSize);
        _updatePoFQ(seller, clearingPrice, oraclePrice, fillSize);
    }

    /// @dev Compute taker fee from a quote amount.
    function _computeFee(int256 quoteAmount, uint16 feeBps) internal pure returns (uint256) {
        uint256 absQuote = quoteAmount >= 0 ? uint256(quoteAmount) : uint256(-quoteAmount);
        return (absQuote * feeBps) / BPS_DENOMINATOR;
    }

    /// @dev Return the minimum of three uint128 values.
    function _minSize(uint128 a, uint128 b, uint128 c) internal pure returns (uint128) {
        uint128 min = a < b ? a : b;
        return min < c ? min : c;
    }

    /// @dev Update a trader's PoFQ rolling score via the precompile.
    function _updatePoFQ(address trader, int256 fillPrice, int256 oraclePrice, uint128 fillSize) internal {
        uint256 fillScore = POFQ.scoreFill(fillPrice, oraclePrice);

        (uint256 updatedScore, uint256 updatedWeight) =
            POFQ.updateRollingScore(poFQScores[trader], poFQWeights[trader], fillScore, fillSize, 100);

        poFQScores[trader] = updatedScore;
        poFQWeights[trader] = updatedWeight;

        emit PoFQUpdated(trader, updatedScore, updatedWeight);
    }

    /// @dev Clear batch state for a market.
    function _clearBatchState(bytes32 marketId) internal {
        delete _batchBuys[marketId];
        delete _batchSells[marketId];
        delete _batchBuyTraders[marketId];
        delete _batchSellTraders[marketId];
    }
}
