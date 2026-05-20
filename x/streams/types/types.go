// Copyright PaxLabs Ltd.(Paxeer Network)
// Paxeer Network Non-Commercial License 1.0 (ENCL-1.0)(https://github.com/Paxeer-Network/hyperpaxeer-os/blob/main/LICENSE_FAQ.md)


package types

// Stream represents a rate-based payment commitment from `Payer` to `Payee`.
//
// All numeric amounts are stored as decimal-string encodings of *big.Int to
// preserve precision and sidestep variable-length-int issues. The precompile
// converts to/from *big.Int at the API boundary.
//
// Custody model (D3 locked, Path = "escrow at open"):
//   - On `open(...)`: `Cap` worth of `Token` is moved from Payer into the
//     module account (or this module's bank balance for native PAX).
//   - On `settle/close`: amounts flow from module account → Payee.
//   - On `close`: any unclaimed `Cap - Settled` returns to Payer.
//
// Time bounds:
//   - StartTime / StopTime are unix seconds aligned with `ctx.BlockTime()`.
//   - StopTime == 0 means open-ended.
type Stream struct {
	// ID is the monotonic stream identifier allocated at open-time.
	ID uint64 `json:"id"`
	// Payer is the EVM address that funded the stream (20 bytes).
	Payer []byte `json:"payer"`
	// Payee is the EVM address that receives settlements (20 bytes).
	Payee []byte `json:"payee"`
	// Token is the EVM address of the ERC-20. Zero address (20 zero bytes) means
	// native PAX.
	Token []byte `json:"token"`
	// RatePerSecond is the streaming rate (token native units per second), as
	// decimal string.
	RatePerSecond string `json:"rate_per_second"`
	// Cap is the maximum lifetime payout (token native units), as decimal string.
	// Cap == "0" is REJECTED in v21 (allowance-pull is deferred).
	Cap string `json:"cap"`
	// StartTime is the unix-second timestamp at which accrual begins.
	StartTime uint64 `json:"start_time"`
	// StopTime is the unix-second timestamp at which accrual ends. 0 = open-ended.
	StopTime uint64 `json:"stop_time"`
	// Settled is the total amount already withdrawn to Payee, as decimal string.
	Settled string `json:"settled"`
	// Active is true while the stream is live; flipped on close.
	Active bool `json:"active"`
}
