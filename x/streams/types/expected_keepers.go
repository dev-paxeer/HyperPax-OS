// Copyright PaxLabs Ltd.(Paxeer Network)
// Paxeer Network Non-Commercial License 1.0 (ENCL-1.0)(https://github.com/Paxeer-Network/hyperpaxeer-os/blob/main/LICENSE_FAQ.md)


package types

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"

	evmtypes "github.com/evmos/evmos/v18/x/evm/types"
)

// AccountKeeper defines the expected interface for the auth/account module.
type AccountKeeper interface {
	GetSequence(ctx sdk.Context, addr sdk.AccAddress) (uint64, error)
	GetModuleAddress(name string) sdk.AccAddress
}

// BankKeeper defines the bank operations used by the streams module for native
// PAX escrow + payouts.
type BankKeeper interface {
	SendCoinsFromAccountToModule(ctx sdk.Context, senderAddr sdk.AccAddress, recipientModule string, amt sdk.Coins) error
	SendCoinsFromModuleToAccount(ctx sdk.Context, senderModule string, recipientAddr sdk.AccAddress, amt sdk.Coins) error
	SendCoinsFromModuleToModule(ctx sdk.Context, senderModule, recipientModule string, amt sdk.Coins) error
	GetBalance(ctx sdk.Context, addr sdk.AccAddress, denom string) sdk.Coin
}

// ERC20Keeper exposes the EVM-call helpers used to move ERC-20 tokens in and
// out of escrow.
//
// CallEVM is a thin wrapper that ABI-encodes the call. CallEVMWithData takes
// raw calldata. We use these for `transferFrom`, `transfer`, and `balanceOf`.
//
// See x/erc20/keeper/evm.go:142-224 for the canonical implementations.
type ERC20Keeper interface {
	CallEVM(
		ctx sdk.Context,
		abi abi.ABI,
		from, contract common.Address,
		commit bool,
		method string,
		args ...interface{},
	) (*evmtypes.MsgEthereumTxResponse, error)

	CallEVMWithData(
		ctx sdk.Context,
		from common.Address,
		contract *common.Address,
		data []byte,
		commit bool,
	) (*evmtypes.MsgEthereumTxResponse, error)
}
