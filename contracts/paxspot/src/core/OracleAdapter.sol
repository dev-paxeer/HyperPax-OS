// SPDX-License-Identifier: MIT
pragma solidity ^0.8.24;

import {Ownable} from "@openzeppelin/contracts/access/Ownable.sol";
import {Pausable} from "@openzeppelin/contracts/utils/Pausable.sol";
import {IOracleAggregator} from "../interfaces/precompiles/IOracleAggregator.sol";
import {IPyth} from "../interfaces/external/IPyth.sol";

/// @title OracleAdapter
/// @notice Two-tier oracle: Pyth (primary, pull-based) with Validator Oracle Module fallback.
///         Automatically detects Pyth staleness and switches to VOM consensus prices.
///         Pyth is optional — pass address(0) to run in VOM-only mode.
/// @dev Markets must be registered with both a Pyth feed ID and a VOM market ID.
///      When Pyth goes stale, the adapter signals the MatchingEngine to widen spread bands
///      and switch affected markets to batch-only mode.
contract OracleAdapter is Ownable, Pausable {
    // ─── Constants ─────────────────────────────────────────────────────
    IOracleAggregator public constant VOM = IOracleAggregator(0x0000000000000000000000000000000000000903);
    uint256 public constant PRICE_DECIMALS = 18;
    uint256 public constant CONFIDENCE_HIGH = 1e18;
    uint256 public constant CONFIDENCE_REDUCED = 7e17;

    // ─── State Variables ───────────────────────────────────────────────
    IPyth public pyth;
    uint256 public staleThreshold;
    uint256 public minVomQuorum;

    struct MarketConfig {
        bytes32 pythFeedId;
        bytes32 vomMarketId;
        bool active;
    }

    mapping(bytes32 => MarketConfig) public markets;
    bytes32[] public marketIds;

    // ─── Events ────────────────────────────────────────────────────────
    event MarketRegistered(bytes32 indexed marketId, bytes32 pythFeedId, bytes32 vomMarketId);
    event MarketDeactivated(bytes32 indexed marketId);
    event PythStale(bytes32 indexed marketId, uint256 lastUpdate, uint256 currentBlock);
    event FallbackToVOM(bytes32 indexed marketId, int256 price, uint256 quorum);
    event StaleThresholdUpdated(uint256 oldThreshold, uint256 newThreshold);
    event MinVomQuorumUpdated(uint256 oldQuorum, uint256 newQuorum);
    event PythAddressUpdated(address oldPyth, address newPyth);

    // ─── Errors ────────────────────────────────────────────────────────
    error MarketNotRegistered(bytes32 marketId);
    error MarketAlreadyRegistered(bytes32 marketId);
    error MarketNotActive(bytes32 marketId);
    error InsufficientVomQuorum(uint256 got, uint256 required);
    error InvalidStaleThreshold();
    error InvalidQuorum();
    error InvalidPythAddress();
    error OracleUnavailable(bytes32 marketId);

    // ─── Constructor ───────────────────────────────────────────────────
    /// @param _pyth Address of the Pyth oracle contract (address(0) = VOM-only mode).
    /// @param _staleThreshold Staleness threshold in seconds before fallback to VOM.
    /// @param _minVomQuorum Minimum number of validator attestations required for VOM.
    constructor(address _pyth, uint256 _staleThreshold, uint256 _minVomQuorum) Ownable(msg.sender) {
        if (_staleThreshold == 0) revert InvalidStaleThreshold();
        if (_minVomQuorum == 0) revert InvalidQuorum();

        pyth = IPyth(_pyth); // address(0) = Pyth disabled, VOM-only
        staleThreshold = _staleThreshold;
        minVomQuorum = _minVomQuorum;
    }

    // ─── External Functions ────────────────────────────────────────────

    /// @notice Get the current price for a market with automatic fallback.
    /// @param marketId The market identifier (e.g., keccak256("ETH/USDC")).
    /// @return price The price normalized to 18 decimals.
    /// @return confidence Confidence level (1e18 = high/Pyth, 7e17 = reduced/VOM).
    /// @return isFallback True if the price came from VOM instead of Pyth.
    function getPrice(bytes32 marketId)
        external
        view
        whenNotPaused
        returns (int256 price, uint256 confidence, bool isFallback)
    {
        MarketConfig memory config = markets[marketId];
        if (!config.active) revert MarketNotActive(marketId);

        (int256 pythPrice, bool pythFresh) = _tryPyth(config.pythFeedId);

        if (pythFresh) {
            return (pythPrice, CONFIDENCE_HIGH, false);
        }

        (int256 vomPrice, uint256 vomQuorum) = _tryVOM(config.vomMarketId);

        if (vomQuorum >= minVomQuorum) {
            return (vomPrice, CONFIDENCE_REDUCED, true);
        }

        revert OracleUnavailable(marketId);
    }

    /// @notice Get price from Pyth only (no fallback). Reverts if stale.
    /// @param marketId The market identifier.
    /// @return price Normalized price (18 decimals).
    function getPythPrice(bytes32 marketId) external view returns (int256 price) {
        MarketConfig memory config = markets[marketId];
        if (!config.active) revert MarketNotActive(marketId);

        (int256 p, bool fresh) = _tryPyth(config.pythFeedId);
        if (!fresh) revert OracleUnavailable(marketId);
        return p;
    }

    /// @notice Get price from VOM only (no Pyth). Reverts if quorum insufficient.
    /// @param marketId The market identifier.
    /// @return price VOM consensus price (18 decimals).
    /// @return quorum Number of attesting validators.
    function getVOMPrice(bytes32 marketId) external view returns (int256 price, uint256 quorum) {
        MarketConfig memory config = markets[marketId];
        if (!config.active) revert MarketNotActive(marketId);

        (int256 p, uint256 q) = _tryVOM(config.vomMarketId);
        if (q < minVomQuorum) revert InsufficientVomQuorum(q, minVomQuorum);
        return (p, q);
    }

    /// @notice Check whether Pyth data is fresh for a given market.
    /// @param marketId The market identifier.
    /// @return fresh True if Pyth price is within staleness threshold.
    function isPythFresh(bytes32 marketId) external view returns (bool fresh) {
        MarketConfig memory config = markets[marketId];
        if (!config.active) return false;
        (, fresh) = _tryPyth(config.pythFeedId);
    }

    // ─── Admin Functions ───────────────────────────────────────────────

    /// @notice Register a new market with its oracle feed IDs.
    /// @param marketId The market identifier.
    /// @param pythFeedId The Pyth price feed ID for this market.
    /// @param vomMarketId The VOM market ID (same as marketId by convention, but configurable).
    function registerMarket(bytes32 marketId, bytes32 pythFeedId, bytes32 vomMarketId) external onlyOwner {
        if (markets[marketId].active) revert MarketAlreadyRegistered(marketId);

        markets[marketId] = MarketConfig({pythFeedId: pythFeedId, vomMarketId: vomMarketId, active: true});

        marketIds.push(marketId);
        emit MarketRegistered(marketId, pythFeedId, vomMarketId);
    }

    /// @notice Deactivate a market (no price reads allowed).
    /// @param marketId The market identifier.
    function deactivateMarket(bytes32 marketId) external onlyOwner {
        if (!markets[marketId].active) revert MarketNotActive(marketId);
        markets[marketId].active = false;
        emit MarketDeactivated(marketId);
    }

    /// @notice Update the staleness threshold.
    /// @param _staleThreshold New threshold in seconds.
    function setStaleThreshold(uint256 _staleThreshold) external onlyOwner {
        if (_staleThreshold == 0) revert InvalidStaleThreshold();
        emit StaleThresholdUpdated(staleThreshold, _staleThreshold);
        staleThreshold = _staleThreshold;
    }

    /// @notice Update the minimum VOM quorum.
    /// @param _minVomQuorum New minimum quorum count.
    function setMinVomQuorum(uint256 _minVomQuorum) external onlyOwner {
        if (_minVomQuorum == 0) revert InvalidQuorum();
        emit MinVomQuorumUpdated(minVomQuorum, _minVomQuorum);
        minVomQuorum = _minVomQuorum;
    }

    /// @notice Update the Pyth contract address. Pass address(0) to disable Pyth.
    /// @param _pyth New Pyth contract address (address(0) = disable).
    function setPythAddress(address _pyth) external onlyOwner {
        emit PythAddressUpdated(address(pyth), _pyth);
        pyth = IPyth(_pyth);
    }

    /// @notice Pause all oracle reads (emergency).
    function pause() external onlyOwner {
        _pause();
    }

    /// @notice Unpause oracle reads.
    function unpause() external onlyOwner {
        _unpause();
    }

    // ─── View / Pure Functions ─────────────────────────────────────────

    /// @notice Get the total number of registered markets.
    /// @return count Number of markets (including deactivated).
    function getMarketCount() external view returns (uint256 count) {
        return marketIds.length;
    }

    // ─── Internal Functions ────────────────────────────────────────────

    /// @dev Try to read Pyth price. Returns (price, isFresh).
    ///      Returns (0, false) if Pyth is disabled (address(0)) or unavailable.
    function _tryPyth(bytes32 feedId) internal view returns (int256 price, bool fresh) {
        if (address(pyth) == address(0)) return (0, false);
        try pyth.getPriceUnsafe(feedId) returns (IPyth.Price memory p) {
            if (block.timestamp - p.publishTime <= staleThreshold) {
                price = _normalizePythPrice(p.price, p.expo);
                fresh = true;
            }
        } catch {
            // Pyth unavailable — fall through to VOM
        }
    }

    /// @dev Try to read VOM price via the precompile.
    function _tryVOM(bytes32 vomMarketId) internal view returns (int256 price, uint256 quorum) {
        try VOM.getValidatorPrice(vomMarketId) returns (int256 p, uint256 q, uint256) {
            return (p, q);
        } catch {
            return (0, 0);
        }
    }

    /// @dev Normalize a Pyth price (variable exponent) to 18-decimal fixed-point.
    ///      Pyth prices come as `price * 10^expo` where expo is typically negative (e.g., -8).
    ///      Target: price * 10^18.
    function _normalizePythPrice(int64 price, int32 expo) internal pure returns (int256) {
        if (price <= 0) return 0;

        int256 normalized = int256(price);
        int32 targetExpo = 18;
        int32 diff = targetExpo + expo;

        if (diff > 0) {
            normalized = normalized * int256(10 ** uint256(int256(diff)));
        } else if (diff < 0) {
            normalized = normalized / int256(10 ** uint256(int256(-diff)));
        }

        return normalized;
    }
}
