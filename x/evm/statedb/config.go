// Copyright PaxLabs Ltd.(Paxeer Network)
// Paxeer Network Non-Commercial License 1.0 (ENCL-1.0)(https://github.com/Paxeer-Network/hyperpaxeer-os/blob/main/LICENSE_FAQ.md)

package statedb

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/params"

	"github.com/evmos/evmos/v18/x/evm/types"
)

// TxConfig encapulates the readonly information of current tx for `StateDB`.
type TxConfig struct {
	BlockHash common.Hash // hash of current block
	TxHash    common.Hash // hash of current tx
	TxIndex   uint        // the index of current transaction
	LogIndex  uint        // the index of next log within current block
}

// NewTxConfig returns a TxConfig
func NewTxConfig(bhash, thash common.Hash, txIndex, logIndex uint) TxConfig {
	return TxConfig{
		BlockHash: bhash,
		TxHash:    thash,
		TxIndex:   txIndex,
		LogIndex:  logIndex,
	}
}

// NewEmptyTxConfig construct an empty TxConfig,
// used in context where there's no transaction, e.g. `eth_call`/`eth_estimateGas`.
func NewEmptyTxConfig(bhash common.Hash) TxConfig {
	return TxConfig{
		BlockHash: bhash,
		TxHash:    common.Hash{},
		TxIndex:   0,
		LogIndex:  0,
	}
}

// EVMConfig encapsulates common parameters needed to create an EVM to execute a message
// It's mainly to reduce the number of method parameters
type EVMConfig struct {
	Params      types.Params
	ChainConfig *params.ChainConfig
	CoinBase    common.Address
	BaseFee     *big.Int
	// SetCodeAuthList is the EIP-7702 authorization list pulled off the
	// original *ethtypes.Transaction in ApplyTransaction. When non-empty,
	// ApplyMessageWithConfig invokes core.ApplyAuthorizations against the
	// EVM's stateDB before the call frame executes. nil for any tx type
	// other than 0x04 (SetCodeTxType).
	SetCodeAuthList []ethtypes.Authorization
}
