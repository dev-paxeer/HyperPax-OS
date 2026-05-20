// Copyright PaxLabs Ltd.(Paxeer Network)
// Paxeer Network Non-Commercial License 1.0 (ENCL-1.0)(https://github.com/Paxeer-Network/hyperpaxeer-os/blob/main/LICENSE_FAQ.md)


package werc20

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
)

// EventDepositWithdraw defines the common event data for the WERC20 Deposit
// and Withdraw events.
type EventDepositWithdraw struct {
	// source or destination address
	Address common.Address
	// amount deposited or withdrawn
	Amount *big.Int
}
