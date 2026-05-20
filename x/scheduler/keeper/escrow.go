// Copyright PaxLabs Ltd.(Paxeer Network)
// Paxeer Network Non-Commercial License 1.0 (ENCL-1.0)(https://github.com/Paxeer-Network/hyperpaxeer-os/blob/main/LICENSE_FAQ.md)


package keeper

import (
	"math/big"

	errorsmod "cosmossdk.io/errors"
	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/ethereum/go-ethereum/common"

	evmtypes "github.com/evmos/evmos/v18/x/evm/types"
	"github.com/evmos/evmos/v18/x/scheduler/types"
)

// ModuleAccountAddress returns the bech32 address of the scheduler module
// account. The account holds escrowed deposits between Schedule and either
// Cancel or EndBlocker dispatch.
func (k Keeper) ModuleAccountAddress() sdk.AccAddress {
	return k.accountKeeper.GetModuleAddress(types.ModuleAccountName)
}

// EscrowDeposit moves `deposit` worth of the EVM denom from the payer's bank
// account to the scheduler module account. Called from
// precompiles/scheduler/scheduler.go::handleSchedule before invoking
// keeper.Schedule, so a rejected Schedule rolls the bank transfer back via
// the EVM precompile's revert path.
//
// Caller is responsible for cancelling the implicit msg.value transfer that
// the EVM made into the precompile address (via stateDB.SubBalance) before
// calling this. See the precompile's handleSchedule for the call-site comment.
func (k Keeper) EscrowDeposit(ctx sdk.Context, payer common.Address, deposit *big.Int) error {
	if deposit == nil || deposit.Sign() == 0 {
		return nil
	}
	if deposit.Sign() < 0 {
		return errorsmod.Wrap(types.ErrDepositTooLow, "deposit must be non-negative")
	}
	denom := evmtypes.DefaultEVMDenom
	coin := sdk.NewCoin(denom, sdkmath.NewIntFromBigInt(deposit))
	return k.bankKeeper.SendCoinsFromAccountToModule(
		ctx,
		sdk.AccAddress(payer.Bytes()),
		types.ModuleAccountName,
		sdk.NewCoins(coin),
	)
}

// RefundDeposit moves `amount` of the EVM denom from the scheduler module
// account back to `recipient`. A no-op when amount is zero or negative.
func (k Keeper) RefundDeposit(ctx sdk.Context, recipient common.Address, amount *big.Int) error {
	if amount == nil || amount.Sign() <= 0 {
		return nil
	}
	denom := evmtypes.DefaultEVMDenom
	coin := sdk.NewCoin(denom, sdkmath.NewIntFromBigInt(amount))
	return k.bankKeeper.SendCoinsFromModuleToAccount(
		ctx,
		types.ModuleAccountName,
		sdk.AccAddress(recipient.Bytes()),
		sdk.NewCoins(coin),
	)
}

// parseDeposit decodes a Job.Deposit string into a non-negative *big.Int. A
// malformed or empty string yields zero.
func parseDeposit(s string) *big.Int {
	if s == "" {
		return new(big.Int)
	}
	v, ok := new(big.Int).SetString(s, 10)
	if !ok || v.Sign() < 0 {
		return new(big.Int)
	}
	return v
}
