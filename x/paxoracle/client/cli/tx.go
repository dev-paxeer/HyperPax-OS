package cli

import (
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"

	"github.com/spf13/cobra"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/client/tx"

	"github.com/evmos/evmos/v18/x/paxoracle/types"
)

// GetTxCmd returns the transaction commands for the paxoracle module.
func GetTxCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:                        types.ModuleName,
		Short:                      "paxoracle transaction subcommands",
		DisableFlagParsing:         true,
		SuggestionsMinimumDistance: 2,
		RunE:                       client.ValidateCmd,
	}

	cmd.AddCommand(CmdSubmitPrice())
	return cmd
}

// CmdSubmitPrice returns a CLI command to submit a price attestation.
func CmdSubmitPrice() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "submit-price [market-id-hex] [price] [confidence]",
		Short: "Submit a price attestation for a market",
		Long: `Submit a price attestation as a validator.
market-id-hex: 32-byte market identifier as hex (with or without 0x prefix)
price: price with 18 decimal precision (e.g. 3500000000000000000000 for $3500)
confidence: confidence level with 18 decimal precision (e.g. 1000000000000000000 for 1.0)`,
		Args: cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			// Parse market ID (hex string → [32]byte)
			marketHex := strings.TrimPrefix(args[0], "0x")
			marketBytes, err := hex.DecodeString(marketHex)
			if err != nil {
				return fmt.Errorf("invalid market-id hex: %w", err)
			}
			if len(marketBytes) != 32 {
				return fmt.Errorf("market-id must be exactly 32 bytes (64 hex chars), got %d bytes", len(marketBytes))
			}
			var marketId [32]byte
			copy(marketId[:], marketBytes)

			// Parse price
			price, ok := new(big.Int).SetString(args[1], 10)
			if !ok {
				return fmt.Errorf("invalid price: %s", args[1])
			}

			// Parse confidence
			confidence, ok := new(big.Int).SetString(args[2], 10)
			if !ok {
				return fmt.Errorf("invalid confidence: %s", args[2])
			}

			signer := clientCtx.GetFromAddress().String()
			msg := types.NewMsgSubmitPrice(signer, marketId, price, confidence)

			if err := msg.ValidateBasic(); err != nil {
				return err
			}

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}

	flags.AddTxFlagsToCmd(cmd)
	return cmd
}
