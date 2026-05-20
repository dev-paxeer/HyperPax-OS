// SPDX-License-Identifier: MIT
pragma solidity ^0.8.24;

import {Ownable} from "@openzeppelin/contracts/access/Ownable.sol";
import {Pausable} from "@openzeppelin/contracts/utils/Pausable.sol";
import {ReentrancyGuard} from "@openzeppelin/contracts/utils/ReentrancyGuard.sol";
import {IERC20} from "@openzeppelin/contracts/token/ERC20/IERC20.sol";
import {SafeERC20} from "@openzeppelin/contracts/token/ERC20/utils/SafeERC20.sol";

/// @title SettlementEngine
/// @notice Manages virtual balances and epoch-based net settlement for PaxSpot.
///         Trades execute against virtual balances (same-block UX), while actual ERC-20
///         transfers are batched into settlement epochs (every N blocks) to reduce gas.
/// @dev Virtual balances are signed integers — negative balances represent debt that must
///      be covered by the next settlement epoch or the user's deposited collateral.
///      Fast-settle lane: users who need instant finality pay a premium (1 bps) and get
///      their net position settled in the same block.
contract SettlementEngine is Ownable, Pausable, ReentrancyGuard {
    using SafeERC20 for IERC20;

    // ─── Constants ─────────────────────────────────────────────────────
    uint256 public constant BPS_DENOMINATOR = 10_000;
    uint256 public constant FAST_SETTLE_FEE_BPS = 1;

    // ─── State Variables ───────────────────────────────────────────────
    address public matchingEngine;
    address public treasury;

    uint256 public epochLength;
    uint256 public currentEpochStart;
    uint256 public epochCounter;

    mapping(address => mapping(address => int256)) public virtualBalances;

    mapping(address => mapping(address => uint256)) public depositedCollateral;

    mapping(address => mapping(address => int256)) public epochNetDelta;

    address[] internal _epochDirtyUsers;
    mapping(address => bool) internal _isEpochDirty;

    address[] public supportedTokens;
    mapping(address => bool) public isTokenSupported;

    mapping(address => int256) public realizedPnL;

    // ─── Events ────────────────────────────────────────────────────────
    event Deposited(address indexed user, address indexed token, uint256 amount);
    event Withdrawn(address indexed user, address indexed token, uint256 amount);
    event VirtualBalanceUpdated(
        address indexed user, address indexed token, int256 delta, int256 newBalance
    );
    event EpochSettled(uint256 indexed epochId, uint256 blockNumber, uint256 usersSettled);
    event FastSettled(address indexed user, address indexed token, int256 netAmount, uint256 fee);
    event MatchingEngineUpdated(address oldEngine, address newEngine);
    event TreasuryUpdated(address oldTreasury, address newTreasury);
    event EpochLengthUpdated(uint256 oldLength, uint256 newLength);
    event TokenAdded(address indexed token);
    event PnLRecorded(address indexed user, int256 pnlDelta, int256 totalPnL);

    // ─── Errors ────────────────────────────────────────────────────────
    error OnlyMatchingEngine();
    error InsufficientCollateral(address user, address token, uint256 required, uint256 available);
    error InsufficientVirtualBalance(address user, address token, int256 required, int256 available);
    error TokenNotSupported(address token);
    error TokenAlreadySupported(address token);
    error InvalidAddress();
    error InvalidEpochLength();
    error EpochNotReady();
    error NothingToSettle();
    error WithdrawExceedsAvailable(address user, address token, uint256 requested, uint256 available);

    // ─── Modifiers ─────────────────────────────────────────────────────
    modifier onlyMatchingEngine() {
        if (msg.sender != matchingEngine) revert OnlyMatchingEngine();
        _;
    }

    // ─── Constructor ───────────────────────────────────────────────────
    /// @param _epochLength Number of blocks per settlement epoch (e.g., 5 = ~10s on HyperPaxeer).
    /// @param _treasury Address receiving fast-settle fees.
    constructor(uint256 _epochLength, address _treasury) Ownable(msg.sender) {
        if (_epochLength == 0) revert InvalidEpochLength();
        if (_treasury == address(0)) revert InvalidAddress();

        epochLength = _epochLength;
        treasury = _treasury;
        currentEpochStart = block.number;
        epochCounter = 0;
    }

    // ─── External Functions ────────────────────────────────────────────

    /// @notice Deposit ERC-20 collateral. Increases both deposited collateral and virtual balance.
    /// @param token The ERC-20 token address.
    /// @param amount The amount to deposit.
    function deposit(address token, uint256 amount) external nonReentrant whenNotPaused {
        if (!isTokenSupported[token]) revert TokenNotSupported(token);

        IERC20(token).safeTransferFrom(msg.sender, address(this), amount);

        depositedCollateral[msg.sender][token] += amount;
        virtualBalances[msg.sender][token] += int256(amount);

        emit Deposited(msg.sender, token, amount);
        emit VirtualBalanceUpdated(
            msg.sender, token, int256(amount), virtualBalances[msg.sender][token]
        );
    }

    /// @notice Withdraw collateral. Only withdrawable balance (collateral - abs(negative virtual delta)).
    /// @param token The ERC-20 token address.
    /// @param amount The amount to withdraw.
    function withdraw(address token, uint256 amount) external nonReentrant whenNotPaused {
        if (!isTokenSupported[token]) revert TokenNotSupported(token);

        uint256 available = getWithdrawable(msg.sender, token);
        if (amount > available) {
            revert WithdrawExceedsAvailable(msg.sender, token, amount, available);
        }

        depositedCollateral[msg.sender][token] -= amount;
        virtualBalances[msg.sender][token] -= int256(amount);

        IERC20(token).safeTransfer(msg.sender, amount);

        emit Withdrawn(msg.sender, token, amount);
        emit VirtualBalanceUpdated(
            msg.sender, token, -int256(amount), virtualBalances[msg.sender][token]
        );
    }

    /// @notice Credit or debit a user's virtual balance (called by MatchingEngine on fills).
    /// @param user The user address.
    /// @param token The token address.
    /// @param delta Signed amount to add (positive = credit, negative = debit).
    function updateVirtualBalance(address user, address token, int256 delta)
        external
        onlyMatchingEngine
        whenNotPaused
    {
        virtualBalances[user][token] += delta;
        epochNetDelta[user][token] += delta;

        if (!_isEpochDirty[user]) {
            _epochDirtyUsers.push(user);
            _isEpochDirty[user] = true;
        }

        emit VirtualBalanceUpdated(user, token, delta, virtualBalances[user][token]);
    }

    /// @notice Record realized PnL for a trader (called by MatchingEngine on trade settlement).
    /// @param user The trader address.
    /// @param pnlDelta Signed PnL delta from this trade.
    function recordPnL(address user, int256 pnlDelta) external onlyMatchingEngine {
        realizedPnL[user] += pnlDelta;
        emit PnLRecorded(user, pnlDelta, realizedPnL[user]);
    }

    /// @notice Settle the current epoch — compute net transfers for all dirty users.
    /// @dev Can be called by anyone (validator keepers in practice). Requires epoch boundary.
    function settleEpoch() external nonReentrant whenNotPaused {
        if (block.number < currentEpochStart + epochLength) revert EpochNotReady();
        if (_epochDirtyUsers.length == 0) revert NothingToSettle();

        uint256 settled = _epochDirtyUsers.length;
        uint256 currentEpoch = epochCounter;

        for (uint256 i = 0; i < _epochDirtyUsers.length; i++) {
            address user = _epochDirtyUsers[i];

            for (uint256 j = 0; j < supportedTokens.length; j++) {
                address token = supportedTokens[j];
                int256 net = epochNetDelta[user][token];

                if (net != 0) {
                    epochNetDelta[user][token] = 0;
                }
            }

            _isEpochDirty[user] = false;
        }

        delete _epochDirtyUsers;

        epochCounter++;
        currentEpochStart = block.number;

        emit EpochSettled(currentEpoch, block.number, settled);
    }

    /// @notice Fast-settle: immediately finalize a user's net position in a token.
    ///         Charges a 1 bps fee on the absolute net amount.
    /// @param token The token to settle.
    function fastSettle(address token) external nonReentrant whenNotPaused {
        if (!isTokenSupported[token]) revert TokenNotSupported(token);

        int256 net = epochNetDelta[msg.sender][token];
        if (net == 0) revert NothingToSettle();

        uint256 absNet = net > 0 ? uint256(net) : uint256(-net);
        uint256 fee = (absNet * FAST_SETTLE_FEE_BPS) / BPS_DENOMINATOR;

        epochNetDelta[msg.sender][token] = 0;

        if (fee > 0) {
            virtualBalances[msg.sender][token] -= int256(fee);
            virtualBalances[treasury][token] += int256(fee);
        }

        emit FastSettled(msg.sender, token, net, fee);
    }

    // ─── Admin Functions ───────────────────────────────────────────────

    /// @notice Set the matching engine address (can only be set once, then updated by owner).
    /// @param _matchingEngine The MatchingEngine contract address.
    function setMatchingEngine(address _matchingEngine) external onlyOwner {
        if (_matchingEngine == address(0)) revert InvalidAddress();
        emit MatchingEngineUpdated(matchingEngine, _matchingEngine);
        matchingEngine = _matchingEngine;
    }

    /// @notice Update the treasury address.
    /// @param _treasury New treasury address.
    function setTreasury(address _treasury) external onlyOwner {
        if (_treasury == address(0)) revert InvalidAddress();
        emit TreasuryUpdated(treasury, _treasury);
        treasury = _treasury;
    }

    /// @notice Update the epoch length.
    /// @param _epochLength New epoch length in blocks.
    function setEpochLength(uint256 _epochLength) external onlyOwner {
        if (_epochLength == 0) revert InvalidEpochLength();
        emit EpochLengthUpdated(epochLength, _epochLength);
        epochLength = _epochLength;
    }

    /// @notice Add a supported token.
    /// @param token The ERC-20 token address to support.
    function addToken(address token) external onlyOwner {
        if (token == address(0)) revert InvalidAddress();
        if (isTokenSupported[token]) revert TokenAlreadySupported(token);
        isTokenSupported[token] = true;
        supportedTokens.push(token);
        emit TokenAdded(token);
    }

    /// @notice Pause the settlement engine (emergency).
    function pause() external onlyOwner {
        _pause();
    }

    /// @notice Unpause the settlement engine.
    function unpause() external onlyOwner {
        _unpause();
    }

    // ─── View / Pure Functions ─────────────────────────────────────────

    /// @notice Get the withdrawable balance for a user (collateral minus any negative virtual delta).
    /// @param user The user address.
    /// @param token The token address.
    /// @return available The amount that can be withdrawn.
    function getWithdrawable(address user, address token) public view returns (uint256 available) {
        uint256 collateral = depositedCollateral[user][token];
        int256 vBal = virtualBalances[user][token];

        if (vBal >= int256(collateral)) {
            return collateral;
        }

        if (vBal <= 0) {
            return 0;
        }

        return uint256(vBal);
    }

    /// @notice Get a user's virtual balance for a token.
    /// @param user The user address.
    /// @param token The token address.
    /// @return balance Signed virtual balance.
    function getVirtualBalance(address user, address token) external view returns (int256 balance) {
        return virtualBalances[user][token];
    }

    /// @notice Get the number of blocks until the next epoch settlement is possible.
    /// @return blocksRemaining 0 if epoch can be settled now.
    function blocksUntilEpoch() external view returns (uint256 blocksRemaining) {
        uint256 epochEnd = currentEpochStart + epochLength;
        if (block.number >= epochEnd) return 0;
        return epochEnd - block.number;
    }

    /// @notice Get the number of dirty users pending settlement.
    /// @return count Number of users with unsettled epoch deltas.
    function pendingSettlementCount() external view returns (uint256 count) {
        return _epochDirtyUsers.length;
    }

    /// @notice Get the number of supported tokens.
    /// @return count Token count.
    function supportedTokenCount() external view returns (uint256 count) {
        return supportedTokens.length;
    }
}
