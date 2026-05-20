// Copyright PaxLabs Ltd.(Paxeer Network)
// Paxeer Network Non-Commercial License 1.0 (ENCL-1.0)(https://github.com/Paxeer-Network/hyperpaxeer-os/blob/main/LICENSE_FAQ.md)


package types

import sdk "github.com/cosmos/cosmos-sdk/types"

// CoinEq returns whether two Coins are equal.
// The IsEqual() method can panic.
func CoinEq(a, b sdk.Coins) bool {
	return a.IsAllLTE(b) && b.IsAllLTE(a)
}

// Max64 returns the maximum of its inputs.
func Max64(i, j int64) int64 {
	if i > j {
		return i
	}
	return j
}

// Min64 returns the minimum of its inputs.
func Min64(i, j int64) int64 {
	if i < j {
		return i
	}
	return j
}
