// Copyright PaxLabs Ltd.(Paxeer Network)
// Paxeer Network Non-Commercial License 1.0 (ENCL-1.0)(https://github.com/Paxeer-Network/hyperpaxeer-os/blob/main/LICENSE_FAQ.md)

package keeper

import (
	"fmt"
	"math/big"
	"strings"

	errorsmod "cosmossdk.io/errors"
	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"

	evmtypes "github.com/evmos/evmos/v18/x/evm/types"
	"github.com/evmos/evmos/v18/x/streams/types"
)

// erc20TransferABI is a minimal ABI exposing transferFrom(from,to,amount) and
// transfer(to,amount) so the keeper can call these methods on arbitrary
// ERC-20 token contracts via erc20Keeper.CallEVM.
var erc20TransferABI abi.ABI

func init() {
	const src = `[
		{"name":"transferFrom","type":"function","stateMutability":"nonpayable",
		 "inputs":[{"name":"from","type":"address"},{"name":"to","type":"address"},{"name":"amount","type":"uint256"}],
		 "outputs":[{"name":"ok","type":"bool"}]},
		{"name":"transfer","type":"function","stateMutability":"nonpayable",
		 "inputs":[{"name":"to","type":"address"},{"name":"amount","type":"uint256"}],
		 "outputs":[{"name":"ok","type":"bool"}]}
	]`
	parsed, err := abi.JSON(strings.NewReader(src))
	if err != nil {
		panic(fmt.Sprintf("streams: failed to parse erc20 transfer ABI: %v", err))
	}
	erc20TransferABI = parsed
}

// ModuleAccountAddress returns the bech32 address of the streams module
// account. Cached in the keeper since the streams module account is queried
// on every escrow / payout.
func (k Keeper) ModuleAccountAddress() sdk.AccAddress {
	return k.accountKeeper.GetModuleAddress(types.ModuleAccountName)
}

// isNativeToken returns true when the stream token field is the zero address
// (native PAX in the EVM denom, no ERC-20 lookup required).
func isNativeToken(token []byte) bool {
	for _, b := range token {
		if b != 0 {
			return false
		}
	}
	return true
}

// escrowFromPayer moves `amount` of `token` from payer to the streams module
// account at open time. Native PAX uses bankKeeper.SendCoinsFromAccountToModule
// directly; ERC-20 uses erc20Keeper.CallEVM(transferFrom, payer, moduleAddr, amount).
//
// For native PAX the precompile's handleOpen MUST cancel the implicit msg.value
// transfer before calling — see precompiles/streams/streams.go::handleOpen.
// For ERC-20 the payer must have called `approve(streamsPrecompile, amount)`
// on the token contract beforehand.
func (k Keeper) escrowFromPayer(
	ctx sdk.Context,
	payer common.Address,
	token []byte,
	amount *big.Int,
) error {
	if amount == nil || amount.Sign() <= 0 {
		return nil
	}
	if isNativeToken(token) {
		coin := sdk.NewCoin(evmtypes.DefaultEVMDenom, sdkmath.NewIntFromBigInt(amount))
		return k.bankKeeper.SendCoinsFromAccountToModule(
			ctx,
			sdk.AccAddress(payer.Bytes()),
			types.ModuleAccountName,
			sdk.NewCoins(coin),
		)
	}
	moduleAddr := common.BytesToAddress(k.ModuleAccountAddress().Bytes())
	tokenAddr := common.BytesToAddress(token)
	_, err := k.erc20Keeper.CallEVM(
		ctx,
		erc20TransferABI,
		moduleAddr, // from = module account (it pays gas; not the value source)
		tokenAddr,
		true, // commit
		"transferFrom",
		payer,
		moduleAddr,
		amount,
	)
	if err != nil {
		return errorsmod.Wrap(err, "ERC-20 transferFrom payer -> streams module failed")
	}
	return nil
}

// payOutToPayee moves `amount` of `token` from the streams module account to
// payee. Used by Settle and Close. Symmetric inverse of escrowFromPayer.
func (k Keeper) payOutToPayee(
	ctx sdk.Context,
	payee common.Address,
	token []byte,
	amount *big.Int,
) error {
	if amount == nil || amount.Sign() <= 0 {
		return nil
	}
	if isNativeToken(token) {
		coin := sdk.NewCoin(evmtypes.DefaultEVMDenom, sdkmath.NewIntFromBigInt(amount))
		return k.bankKeeper.SendCoinsFromModuleToAccount(
			ctx,
			types.ModuleAccountName,
			sdk.AccAddress(payee.Bytes()),
			sdk.NewCoins(coin),
		)
	}
	moduleAddr := common.BytesToAddress(k.ModuleAccountAddress().Bytes())
	tokenAddr := common.BytesToAddress(token)
	_, err := k.erc20Keeper.CallEVM(
		ctx,
		erc20TransferABI,
		moduleAddr,
		tokenAddr,
		true,
		"transfer",
		payee,
		amount,
	)
	if err != nil {
		return errorsmod.Wrap(err, "ERC-20 transfer streams module -> payee failed")
	}
	return nil
}

// refundToPayer is the same as payOutToPayee but routed back to the payer at
// close time. Kept as a separate method for clarity at the call site.
func (k Keeper) refundToPayer(
	ctx sdk.Context,
	payer common.Address,
	token []byte,
	amount *big.Int,
) error {
	return k.payOutToPayee(ctx, payer, token, amount)
}
