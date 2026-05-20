// SPDX-License-Identifier: MIT
pragma solidity ^0.8.24;

import {Ownable} from "@openzeppelin/contracts/access/Ownable.sol";
import {Pausable} from "@openzeppelin/contracts/utils/Pausable.sol";
import {ReentrancyGuard} from "@openzeppelin/contracts/utils/ReentrancyGuard.sol";
import {IPaxSpotReader} from "../interfaces/external/IPaxSpotReader.sol";
import {IAllowanceProvider} from "../interfaces/external/IAllowanceProvider.sol";
import {MatchingEngine} from "./MatchingEngine.sol";
import {SettlementEngine} from "./SettlementEngine.sol";
import {PaxSpotTypes} from "./PaxSpotTypes.sol";

/// @title PaxSpotRouter
/// @notice Order gateway for PaxSpot. Validates orders, enforces rate limits and smart wallet
///         allowances, then routes to the MatchingEngine. Implements IPaxSpotReader for Argus.
/// @dev All user-facing order submission goes through this contract.
///      Funded smart wallets are checked against IAllowanceProvider before order acceptance.
contract PaxSpotRouter is IPaxSpotReader, Ownable, Pausable, ReentrancyGuard {
    // ─── State Variables ───────────────────────────────────────────────
    MatchingEngine public matchingEngine;
    SettlementEngine public settlementEngine;
    IAllowanceProvider public allowanceProvider;

    mapping(address => uint256) public lastOrderBlock;
    uint256 public orderCooldownBlocks;

    mapping(address => bool) public isFundedWallet;

    mapping(address => mapping(bytes32 => int256)) public positions;
    mapping(address => mapping(bytes32 => int256)) public avgEntryPrices;

    mapping(address => mapping(uint256 => uint256)) internal _volumePerBlock;

    // ─── Events ────────────────────────────────────────────────────────
    event OrderSubmitted(
        uint256 indexed orderId,
        address indexed trader,
        bytes32 indexed marketId,
        PaxSpotTypes.Side side,
        PaxSpotTypes.OrderType orderType,
        int16 offsetBps,
        uint128 size
    );
    event OrderCancelRequested(uint256 indexed orderId, address indexed trader);
    event FundedWalletRegistered(address indexed wallet);
    event FundedWalletRemoved(address indexed wallet);
    event AllowanceProviderUpdated(address oldProvider, address newProvider);
    event CooldownUpdated(uint256 oldCooldown, uint256 newCooldown);

    // ─── Errors ────────────────────────────────────────────────────────
    error CooldownActive(address trader, uint256 blocksRemaining);
    error FundedWalletNotActive(address wallet);
    error FundedWalletExceedsAllowance(address wallet, bytes32 marketId, uint256 newSize, uint256 maxSize);
    error InvalidSize();
    error InvalidAddress();

    // ─── Constructor ───────────────────────────────────────────────────
    /// @param _matchingEngine The MatchingEngine contract address.
    /// @param _settlementEngine The SettlementEngine contract address.
    /// @param _orderCooldownBlocks Minimum blocks between orders from the same address (0 = no limit).
    constructor(address _matchingEngine, address _settlementEngine, uint256 _orderCooldownBlocks)
        Ownable(msg.sender)
    {
        matchingEngine = MatchingEngine(_matchingEngine);
        settlementEngine = SettlementEngine(_settlementEngine);
        orderCooldownBlocks = _orderCooldownBlocks;
    }

    // ─── External Functions ────────────────────────────────────────────

    /// @notice Submit a new order.
    /// @param marketId The market to trade.
    /// @param side BUY or SELL.
    /// @param orderType MARKET or LIMIT.
    /// @param offsetBps OROB offset from oracle in basis points.
    /// @param size Order size in base asset units.
    /// @return orderId The assigned order ID.
    function submitOrder(
        bytes32 marketId,
        PaxSpotTypes.Side side,
        PaxSpotTypes.OrderType orderType,
        int16 offsetBps,
        uint128 size
    ) external nonReentrant whenNotPaused returns (uint256 orderId) {
        if (size == 0) revert InvalidSize();

        if (orderCooldownBlocks > 0) {
            uint256 earliest = lastOrderBlock[msg.sender] + orderCooldownBlocks;
            if (block.number < earliest) {
                revert CooldownActive(msg.sender, earliest - block.number);
            }
        }

        if (isFundedWallet[msg.sender]) {
            _checkFundedWalletAllowance(msg.sender, marketId, side, size);
        }

        lastOrderBlock[msg.sender] = block.number;

        orderId = matchingEngine.placeOrder(msg.sender, marketId, side, orderType, offsetBps, size);

        _updatePosition(msg.sender, marketId, side, size);
        _recordVolume(msg.sender, size);

        emit OrderSubmitted(orderId, msg.sender, marketId, side, orderType, offsetBps, size);
    }

    /// @notice Cancel an existing order.
    /// @param orderId The order ID to cancel.
    function cancelOrder(uint256 orderId) external nonReentrant whenNotPaused {
        matchingEngine.cancelOrder(orderId, msg.sender);
        emit OrderCancelRequested(orderId, msg.sender);
    }

    // ─── IPaxSpotReader Implementation ─────────────────────────────────

    /// @inheritdoc IPaxSpotReader
    function getPoFQScore(address trader) external view returns (uint256 score, uint256 totalVolume) {
        return matchingEngine.getPoFQScore(trader);
    }

    /// @inheritdoc IPaxSpotReader
    function getRealizedPnL(address trader, address token) external view returns (int256 pnl) {
        return settlementEngine.realizedPnL(trader);
    }

    /// @inheritdoc IPaxSpotReader
    function getPosition(address trader, bytes32 marketId)
        external
        view
        returns (int256 size, int256 avgEntryPrice)
    {
        return (positions[trader][marketId], avgEntryPrices[trader][marketId]);
    }

    /// @inheritdoc IPaxSpotReader
    function getVolume(address trader, uint256 windowBlocks) external view returns (uint256 volume) {
        uint256 startBlock = block.number > windowBlocks ? block.number - windowBlocks : 0;
        for (uint256 b = startBlock; b <= block.number; b++) {
            volume += _volumePerBlock[trader][b];
        }
    }

    // ─── Admin Functions ───────────────────────────────────────────────

    /// @notice Register an address as a funded smart wallet (Argus-managed).
    /// @param wallet The wallet address.
    function registerFundedWallet(address wallet) external onlyOwner {
        if (wallet == address(0)) revert InvalidAddress();
        isFundedWallet[wallet] = true;
        emit FundedWalletRegistered(wallet);
    }

    /// @notice Remove a funded smart wallet registration.
    /// @param wallet The wallet address.
    function removeFundedWallet(address wallet) external onlyOwner {
        isFundedWallet[wallet] = false;
        emit FundedWalletRemoved(wallet);
    }

    /// @notice Set the IAllowanceProvider contract (Argus risk engine).
    /// @param _provider The allowance provider address.
    function setAllowanceProvider(address _provider) external onlyOwner {
        emit AllowanceProviderUpdated(address(allowanceProvider), _provider);
        allowanceProvider = IAllowanceProvider(_provider);
    }

    /// @notice Update the order cooldown period.
    /// @param _cooldownBlocks New cooldown in blocks.
    function setCooldown(uint256 _cooldownBlocks) external onlyOwner {
        emit CooldownUpdated(orderCooldownBlocks, _cooldownBlocks);
        orderCooldownBlocks = _cooldownBlocks;
    }

    /// @notice Pause order submission (emergency).
    function pause() external onlyOwner {
        _pause();
    }

    /// @notice Unpause order submission.
    function unpause() external onlyOwner {
        _unpause();
    }

    // ─── Internal Functions ────────────────────────────────────────────

    /// @dev Check funded wallet allowance against Argus risk engine.
    function _checkFundedWalletAllowance(
        address wallet,
        bytes32 marketId,
        PaxSpotTypes.Side side,
        uint128 size
    ) internal view {
        if (address(allowanceProvider) == address(0)) return;

        if (!allowanceProvider.isActive(wallet)) {
            revert FundedWalletNotActive(wallet);
        }

        uint256 maxSize = allowanceProvider.getMaxPosition(wallet, marketId);
        int256 currentPos = positions[wallet][marketId];

        int256 newPos;
        if (side == PaxSpotTypes.Side.BUY) {
            newPos = currentPos + int256(uint256(size));
        } else {
            newPos = currentPos - int256(uint256(size));
        }

        uint256 absNewPos = newPos >= 0 ? uint256(newPos) : uint256(-newPos);
        if (absNewPos > maxSize) {
            revert FundedWalletExceedsAllowance(wallet, marketId, absNewPos, maxSize);
        }
    }

    /// @dev Update position tracking (simplified — actual fill tracking requires callback from engine).
    function _updatePosition(address trader, bytes32 marketId, PaxSpotTypes.Side side, uint128 size) internal {
        if (side == PaxSpotTypes.Side.BUY) {
            positions[trader][marketId] += int256(uint256(size));
        } else {
            positions[trader][marketId] -= int256(uint256(size));
        }
    }

    /// @dev Record volume for the current block.
    function _recordVolume(address trader, uint128 size) internal {
        _volumePerBlock[trader][block.number] += uint256(size);
    }
}
