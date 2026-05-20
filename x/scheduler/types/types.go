// Copyright PaxLabs Ltd.(Paxeer Network)
// Paxeer Network Non-Commercial License 1.0 (ENCL-1.0)(https://github.com/Paxeer-Network/hyperpaxeer-os/blob/main/LICENSE_FAQ.md)


package types

// Job represents a scheduled EVM call. Stored as JSON for v20; migrate to
// hand-crafted protobuf encoding (matching x/paxoracle/types/msgs.go::MsgSubmitPrice
// lines 100-223 for the template) before mainnet release if state size becomes
// a concern.
//
// `Creator` and `Target` are 20-byte Ethereum addresses.
// `Deposit` is encoded as a decimal string (avoids signed/variable-length issues).
type Job struct {
	// ID is the monotonic job identifier allocated at schedule-time.
	ID uint64 `json:"id"`
	// Creator is the EVM address that scheduled the job and pays for execution.
	Creator []byte `json:"creator"`
	// Target is the EVM address to call when the job fires.
	Target []byte `json:"target"`
	// CallData is the calldata passed to the target.
	CallData []byte `json:"call_data"`
	// ExecuteAtBlock is the block height at which the job becomes due.
	ExecuteAtBlock uint64 `json:"execute_at_block"`
	// GasLimit is the EVM gas limit applied to the scheduled call.
	GasLimit uint64 `json:"gas_limit"`
	// Deposit is the prepaid execution budget in `aPAX`, as a decimal string.
	Deposit string `json:"deposit"`
	// Active is true while the job is pending; flipped to false on cancel or
	// successful execution.
	Active bool `json:"active"`
}
