// Copyright PaxLabs Ltd.(Paxeer Network)
// Paxeer Network Non-Commercial License 1.0 (ENCL-1.0)(https://github.com/Paxeer-Network/hyperpaxeer-os/blob/main/LICENSE_FAQ.md)


//go:build rocksdb
// +build rocksdb

package main

import (
	"sort"

	"github.com/linxGnu/grocksdb"
	"github.com/spf13/cobra"

	versiondbclient "github.com/crypto-org-chain/cronos/versiondb/client"
	"github.com/evmos/evmos/v18/app"
	"github.com/evmos/evmos/v18/cmd/evmosd/opendb"
)

// ChangeSetCmd returns a Cobra command for interacting with change sets.
// NOTE: this is only included in builds with rocksdb
func ChangeSetCmd() *cobra.Command {
	keys, _, _ := app.StoreKeys()
	storeNames := make([]string, 0, len(keys))
	for name := range keys {
		storeNames = append(storeNames, name)
	}
	sort.Strings(storeNames)

	return versiondbclient.ChangeSetGroupCmd(versiondbclient.Options{
		DefaultStores:  storeNames,
		OpenReadOnlyDB: opendb.OpenReadOnlyDB,
		AppRocksDBOptions: func(sstFileWriter bool) *grocksdb.Options {
			return opendb.NewRocksdbOptions(nil, sstFileWriter)
		},
	})
}
