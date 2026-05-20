// Copyright PaxLabs Ltd.(Paxeer Network)
// Paxeer Network Non-Commercial License 1.0 (ENCL-1.0)(https://github.com/Paxeer-Network/hyperpaxeer-os/blob/main/LICENSE_FAQ.md)

package types

import (
	"math/big"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/ethereum/go-ethereum/common"

	evmtypes "github.com/evmos/evmos/v18/x/evm/types"
)

// AccountKeeper defines the expected interface for the auth/account module.
type AccountKeeper interface {
	GetSequence(ctx sdk.Context, addr sdk.AccAddress) (uint64, error)
	GetModuleAddress(name string) sdk.AccAddress
}

// BankKeeper defines the expected interface for refunds and deposit handling.
type BankKeeper interface {
	SendCoinsFromAccountToModule(ctx sdk.Context, senderAddr sdk.AccAddress, recipientModule string, amt sdk.Coins) error
	SendCoinsFromModuleToAccount(ctx sdk.Context, senderModule string, recipientAddr sdk.AccAddress, amt sdk.Coins) error
	GetBalance(ctx sdk.Context, addr sdk.AccAddress, denom string) sdk.Coin
}

// EVMKeeper defines the subset of the EVM keeper that the scheduler EndBlocker
// uses to dispatch scheduled calls. Mirrors `x/erc20/keeper.CallEVMWithData`
// (see x/erc20/keeper/evm.go:166-224 for the canonical implementation).
type EVMKeeper interface {
	// CallEVMWithData executes an EVM call from `from` to `contract` with the
	// given calldata. When `commit` is true, state changes are persisted.
	//
	// The scheduler always passes `commit = true` from inside EndBlocker, but
	// wraps each call in a CacheContext so a single failed scheduled call does
	// not roll back the entire block.
	CallEVMWithData(
		ctx sdk.Context,
		from common.Address,
		contract *common.Address,
		data []byte,
		commit bool,
	) (*evmtypes.MsgEthereumTxResponse, error)
}

// FeeMarketKeeper defines the subset of feemarket needed to compute the minimum
// required deposit at schedule time.
type FeeMarketKeeper interface {
	GetBaseFee(ctx sdk.Context) *big.Int
}
