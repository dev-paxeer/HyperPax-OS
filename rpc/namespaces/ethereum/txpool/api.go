// Copyright PaxLabs Ltd.(Paxeer Network)
// Paxeer Network Non-Commercial License 1.0 (ENCL-1.0)(https://github.com/Paxeer-Network/hyperpaxeer-os/blob/main/LICENSE_FAQ.md)

package txpool

import (
	"context"
	"fmt"
	"math/big"

	"github.com/cometbft/cometbft/libs/log"
	tmrpcclient "github.com/cometbft/cometbft/rpc/client"
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	ethtypes "github.com/ethereum/go-ethereum/core/types"

	"github.com/evmos/evmos/v18/rpc/types"
	paxeertypes "github.com/evmos/evmos/v18/types"
	evmtypes "github.com/evmos/evmos/v18/x/evm/types"
)

// PublicAPI offers an API for the transaction pool. It only operates on data that is non-confidential.
type PublicAPI struct {
	logger    log.Logger
	clientCtx client.Context
}

// NewPublicAPI creates a new tx pool service that gives information about the transaction pool.
func NewPublicAPI(logger log.Logger, clientCtx client.Context) *PublicAPI {
	return &PublicAPI{
		logger:    logger.With("module", "txpool"),
		clientCtx: clientCtx,
	}
}

// Content returns the transactions contained within the transaction pool
func (api *PublicAPI) Content() (map[string]map[string]map[string]*types.RPCTransaction, error) {
	api.logger.Debug("txpool_content")
	content := map[string]map[string]map[string]*types.RPCTransaction{
		"pending": make(map[string]map[string]*types.RPCTransaction),
		"queued":  make(map[string]map[string]*types.RPCTransaction),
	}

	pendingTxs, err := api.getUnconfirmedEthTxs()
	if err != nil {
		api.logger.Debug("failed to get unconfirmed txs", "error", err.Error())
		return content, nil
	}

	chainID := api.getChainID()
	signer := ethtypes.LatestSignerForChainID(chainID)

	for _, ethMsg := range pendingTxs {
		tx := ethMsg.AsTransaction()
		from, err := signer.Sender(tx)
		if err != nil {
			continue
		}
		fromStr := from.Hex()
		nonce := fmt.Sprintf("%d", tx.Nonce())

		rpcTx, err := types.NewRPCTransaction(tx, common.Hash{}, 0, 0, nil, chainID)
		if err != nil {
			continue
		}

		if _, ok := content["pending"][fromStr]; !ok {
			content["pending"][fromStr] = make(map[string]*types.RPCTransaction)
		}
		content["pending"][fromStr][nonce] = rpcTx
	}

	return content, nil
}

// Inspect returns the content of the transaction pool and flattens it into an
// easily inspectable list.
func (api *PublicAPI) Inspect() (map[string]map[string]map[string]string, error) {
	api.logger.Debug("txpool_inspect")
	content := map[string]map[string]map[string]string{
		"pending": make(map[string]map[string]string),
		"queued":  make(map[string]map[string]string),
	}

	pendingTxs, err := api.getUnconfirmedEthTxs()
	if err != nil {
		api.logger.Debug("failed to get unconfirmed txs", "error", err.Error())
		return content, nil
	}

	chainID := api.getChainID()
	signer := ethtypes.LatestSignerForChainID(chainID)

	for _, ethMsg := range pendingTxs {
		tx := ethMsg.AsTransaction()
		from, err := signer.Sender(tx)
		if err != nil {
			continue
		}
		fromStr := from.Hex()
		nonce := fmt.Sprintf("%d", tx.Nonce())

		to := "contract creation"
		if tx.To() != nil {
			to = tx.To().Hex()
		}
		summary := fmt.Sprintf("%s: %v wei + %v gas x %v wei",
			to, tx.Value(), tx.Gas(), tx.GasPrice())

		if _, ok := content["pending"][fromStr]; !ok {
			content["pending"][fromStr] = make(map[string]string)
		}
		content["pending"][fromStr][nonce] = summary
	}

	return content, nil
}

// Status returns the number of pending and queued transaction in the pool.
func (api *PublicAPI) Status() map[string]hexutil.Uint {
	api.logger.Debug("txpool_status")

	mc, ok := api.clientCtx.Client.(tmrpcclient.MempoolClient)
	if !ok {
		return map[string]hexutil.Uint{
			"pending": hexutil.Uint(0),
			"queued":  hexutil.Uint(0),
		}
	}

	res, err := mc.NumUnconfirmedTxs(context.Background())
	if err != nil {
		api.logger.Debug("failed to get unconfirmed tx count", "error", err.Error())
		return map[string]hexutil.Uint{
			"pending": hexutil.Uint(0),
			"queued":  hexutil.Uint(0),
		}
	}

	return map[string]hexutil.Uint{
		"pending": hexutil.Uint(res.Count),
		"queued":  hexutil.Uint(0),
	}
}

// getUnconfirmedEthTxs fetches unconfirmed txs from the mempool and extracts EVM messages.
func (api *PublicAPI) getUnconfirmedEthTxs() ([]*evmtypes.MsgEthereumTx, error) {
	mc, ok := api.clientCtx.Client.(tmrpcclient.MempoolClient)
	if !ok {
		return nil, fmt.Errorf("invalid rpc client")
	}

	res, err := mc.UnconfirmedTxs(context.Background(), nil)
	if err != nil {
		return nil, err
	}

	var ethMsgs []*evmtypes.MsgEthereumTx
	for _, txBz := range res.Txs {
		tx, err := api.clientCtx.TxConfig.TxDecoder()(txBz)
		if err != nil {
			continue
		}
		for _, msg := range tx.GetMsgs() {
			ethMsg, ok := msg.(*evmtypes.MsgEthereumTx)
			if !ok {
				continue
			}
			ethMsgs = append(ethMsgs, ethMsg)
		}
	}

	return ethMsgs, nil
}

// getChainID returns the EIP-155 chain ID from the client context.
func (api *PublicAPI) getChainID() *big.Int {
	chainID, err := paxeertypes.ParseChainID(api.clientCtx.ChainID)
	if err != nil {
		return big.NewInt(0)
	}
	return chainID
}
