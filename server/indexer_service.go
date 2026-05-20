// Copyright PaxLabs Ltd.(Paxeer Network)
// Paxeer Network Non-Commercial License 1.0 (ENCL-1.0)(https://github.com/Paxeer-Network/hyperpaxeer-os/blob/main/LICENSE_FAQ.md)

package server

import (
	"context"
	"time"

	"github.com/cometbft/cometbft/libs/service"
	rpcclient "github.com/cometbft/cometbft/rpc/client"
	"github.com/cometbft/cometbft/types"

	evmostypes "github.com/evmos/evmos/v18/types"
)

const (
	ServiceName = "EVMIndexerService"

	NewBlockWaitTimeout = 60 * time.Second

	// MaxIndexerBacklog bounds how many historical blocks the live indexer
	// service will attempt to replay on startup. If the persisted indexer
	// cursor (LastIndexedBlock) is more than this many blocks behind chain
	// head, the cursor is jumped forward to (chainHead - MaxIndexerBacklog)
	// to avoid burning CPU/memory replaying ranges where ABCI responses
	// were pruned (e.g. legacy discard_abci_responses). Operators can
	// backfill historical ranges separately via the `evmosd index-eth-tx`
	// CLI subcommand.
	MaxIndexerBacklog = int64(500000)
)

// EVMIndexerService indexes transactions for json-rpc service.
type EVMIndexerService struct {
	service.BaseService

	txIdxr evmostypes.EVMTxIndexer
	client rpcclient.Client
}

// NewEVMIndexerService returns a new service instance.
func NewEVMIndexerService(
	txIdxr evmostypes.EVMTxIndexer,
	client rpcclient.Client,
) *EVMIndexerService {
	is := &EVMIndexerService{txIdxr: txIdxr, client: client}
	is.BaseService = *service.NewBaseService(nil, ServiceName, is)
	return is
}

// OnStart implements service.Service by subscribing for new blocks
// and indexing them by events.
func (eis *EVMIndexerService) OnStart() error {
	ctx := context.Background()
	status, err := eis.client.Status(ctx)
	if err != nil {
		return err
	}
	latestBlock := status.SyncInfo.LatestBlockHeight
	newBlockSignal := make(chan struct{}, 1)

	// Use SubscribeUnbuffered here to ensure both subscriptions does not get
	// canceled due to not pulling messages fast enough. Cause this might
	// sometimes happen when there are no other subscribers.
	blockHeadersChan, err := eis.client.Subscribe(
		ctx,
		ServiceName,
		types.QueryForEvent(types.EventNewBlockHeader).String(),
		0)
	if err != nil {
		return err
	}

	go func() {
		for {
			msg := <-blockHeadersChan
			eventDataHeader := msg.Data.(types.EventDataNewBlockHeader)
			if eventDataHeader.Header.Height > latestBlock {
				latestBlock = eventDataHeader.Header.Height
				// notify
				select {
				case newBlockSignal <- struct{}{}:
				default:
				}
			}
		}
	}()

	lastBlock, err := eis.txIdxr.LastIndexedBlock()
	if err != nil {
		return err
	}
	if lastBlock == -1 {
		lastBlock = latestBlock
	} else if latestBlock-lastBlock > MaxIndexerBacklog {
		// Persisted cursor is too far behind chain head. Jump forward to
		// avoid replaying potentially millions of heights with missing
		// ABCI data, which would starve the JSON-RPC handler.
		skipTo := latestBlock - MaxIndexerBacklog
		eis.Logger.Info(
			"indexer cursor too far behind chain head, jumping forward to bound replay work",
			"lastIndexed", lastBlock,
			"chainHead", latestBlock,
			"jumpTo", skipTo,
			"skipped", skipTo-lastBlock,
			"backlogBound", MaxIndexerBacklog,
		)
		lastBlock = skipTo
	}
	for {
		if latestBlock <= lastBlock {
			// nothing to index. wait for signal of new block
			select {
			case <-newBlockSignal:
			case <-time.After(NewBlockWaitTimeout):
			}
			continue
		}
		for i := lastBlock + 1; i <= latestBlock; i++ {
			block, err := eis.client.Block(ctx, &i)
			if err != nil {
				eis.Logger.Error("failed to fetch block, skipping height", "height", i, "err", err)
				lastBlock = i
				continue
			}
			blockResult, err := eis.client.BlockResults(ctx, &i)
			if err != nil {
				eis.Logger.Error("failed to fetch block result, skipping height", "height", i, "err", err)
				lastBlock = i
				continue
			}
			if err := eis.txIdxr.IndexBlock(block.Block, blockResult.TxsResults); err != nil {
				eis.Logger.Error("failed to index block", "height", i, "err", err)
			}
			lastBlock = blockResult.Height
		}
	}
}
