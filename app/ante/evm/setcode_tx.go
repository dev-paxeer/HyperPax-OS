// Copyright PaxLabs Ltd.(Paxeer Network)
// Paxeer Network Non-Commercial License 1.0 (ENCL-1.0)(https://github.com/Paxeer-Network/hyperpaxeer-os/blob/main/LICENSE_FAQ.md)

package evm

import (
	errorsmod "cosmossdk.io/errors"
	sdk "github.com/cosmos/cosmos-sdk/types"
	errortypes "github.com/cosmos/cosmos-sdk/types/errors"

	ethtypes "github.com/ethereum/go-ethereum/core/types"

	evmtypes "github.com/evmos/evmos/v18/x/evm/types"
)

// EthSetCodeTxDecorator gates EIP-7702 (SetCodeTx, type 0x04) transactions
// at admission. Two responsibilities:
//
//  1. Reject type-0x04 txs before the chain reaches EIP7702BlockNumber.
//     The activation height is stored as a sidecar KV in the EVM keeper
//     (see x/evm/keeper/eip7702.go). Pre-activation rejection prevents
//     mempool pollution and avoids any chance of partial application
//     during the validator rollout window.
//
//  2. Structural validation of the authorization list. Per EIP-7702 §3:
//     - AuthList MUST be non-empty (an empty list is just an expensive
//     DynamicFeeTx and probably indicates a bug client-side).
//     - Per-tuple validation is performed by SetCodeTx.Validate() inside
//     the proto wrapper (x/evm/types/set_code_tx.go::Validate), which
//     also covers the V ∈ {0,1} check.
//
// The actual delegation-marker writes and per-authorization gas charges
// happen in x/evm/keeper/state_transition.go::ApplyMessageWithConfig via
// core.ApplyAuthorizations. This decorator does NOT touch state.
//
// Placement: after EthSigVerificationDecorator (so we know the outer-tx
// signature is valid) and before EthGasConsumeDecorator (so a rejected
// type-0x04 tx never pays gas).
type EthSetCodeTxDecorator struct {
	evmKeeper EVMKeeper
}

// NewEthSetCodeTxDecorator returns the v20-agent-foundations SetCodeTx ante
// decorator.
func NewEthSetCodeTxDecorator(ek EVMKeeper) EthSetCodeTxDecorator {
	return EthSetCodeTxDecorator{evmKeeper: ek}
}

// AnteHandle implements sdk.AnteDecorator. Walks each MsgEthereumTx in the
// outer Cosmos tx; for any type-0x04 inner tx it enforces (1) activation
// gating and (2) authorization-list shape. Non-7702 txs short-circuit
// through without any extra work.
func (d EthSetCodeTxDecorator) AnteHandle(
	ctx sdk.Context,
	tx sdk.Tx,
	simulate bool,
	next sdk.AnteHandler,
) (sdk.Context, error) {
	for i, msg := range tx.GetMsgs() {
		msgEthTx, ok := msg.(*evmtypes.MsgEthereumTx)
		if !ok {
			return ctx, errorsmod.Wrapf(errortypes.ErrUnknownRequest,
				"invalid message type %T at index %d, expected %T",
				msg, i, (*evmtypes.MsgEthereumTx)(nil))
		}

		txData, err := evmtypes.UnpackTxData(msgEthTx.Data)
		if err != nil {
			return ctx, errorsmod.Wrapf(err, "failed to unpack tx data for tx %d", i)
		}

		// Fast-path: only act on type-0x04 transactions.
		if txData.TxType() != ethtypes.SetCodeTxType {
			continue
		}

		// Step 1: activation gating. Reject pre-activation.
		if !d.evmKeeper.IsEIP7702Enabled(ctx) {
			return ctx, errorsmod.Wrapf(errortypes.ErrInvalidRequest,
				"EIP-7702 (SetCodeTx, type 0x04) not yet active on this chain; "+
					"see v20-agent-foundations upgrade for activation height")
		}

		// Step 2: structural validation. The wrapper's Validate() enforces
		// AuthList non-empty + per-tuple shape + V parity. We deliberately
		// re-run it here even though MsgEthereumTx.ValidateBasic may have
		// already done so — defense in depth on a consensus-critical path.
		setCodeData, ok := txData.(*evmtypes.SetCodeTx)
		if !ok {
			return ctx, errorsmod.Wrapf(errortypes.ErrInvalidRequest,
				"tx %d has type 0x04 but TxData is not *SetCodeTx (got %T)",
				i, txData)
		}
		if err := setCodeData.Validate(); err != nil {
			return ctx, errorsmod.Wrapf(err, "tx %d: set code tx validation failed", i)
		}
	}

	return next(ctx, tx, simulate)
}
