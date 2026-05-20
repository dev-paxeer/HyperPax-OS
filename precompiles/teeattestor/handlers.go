// Copyright PaxLabs Ltd.(Paxeer Network)
// Paxeer Network Non-Commercial License 1.0 (ENCL-1.0)(https://github.com/Paxeer-Network/hyperpaxeer-os/blob/main/LICENSE_FAQ.md)


package teeattestor

import (
	"fmt"
	"math/big"

	errorsmod "cosmossdk.io/errors"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/ethereum/go-ethereum/accounts/abi"

	"github.com/evmos/evmos/v18/x/attestor/types"
)

// abiAttestation is the wire-format Solidity tuple for the verify outputs.
// Field names + types match `Attestation` in ITEEAttestor.sol exactly.
type abiAttestation struct {
	Family     uint8    `abi:"family"`
	MRTD       [32]byte `abi:"mrtd"`
	ReportData [32]byte `abi:"reportData"`
	Timestamp  uint64   `abi:"timestamp"`
	Debug      bool     `abi:"debug"`
}

// dispatchVerify is the family-aware dispatcher. Loads roots from the keeper,
// applies module-level policy (debug-allowed, max-attestation-age), then
// calls the per-family verify_ function.
func (p Precompile) dispatchVerify(ctx sdk.Context, family uint8, quote []byte) (types.Attestation, error) {
	if family > types.FamilyMax {
		return types.Attestation{}, errorsmod.Wrapf(types.ErrUnknownFamily, "family=%d", family)
	}

	roots := p.attestorKeeper.RootsForFamily(ctx, family)
	if len(roots) == 0 {
		return types.Attestation{}, errorsmod.Wrapf(types.ErrNoRootsLoaded, "family=%s", types.FamilyName(family))
	}

	var (
		att types.Attestation
		err error
	)
	switch family {
	case types.FamilyIntelTDX:
		att, err = verifyIntelTDX(roots, quote)
	case types.FamilyAMDSEVSNP:
		att, err = verifyAMDSEVSNP(roots, quote)
	case types.FamilyNVIDIAH100:
		att, err = verifyNVIDIA(roots, quote)
	case types.FamilyIntelSGX:
		att, err = verifyIntelSGX(roots, quote)
	default:
		return types.Attestation{}, errorsmod.Wrapf(types.ErrUnknownFamily, "family=%d", family)
	}
	if err != nil {
		return types.Attestation{}, err
	}

	// Apply module policy.
	params := p.attestorKeeper.GetParams(ctx)
	if att.Debug && !params.DebugAllowed {
		return types.Attestation{}, types.ErrDebugRejected
	}
	now := ctx.BlockTime().Unix()
	age := now - int64(att.Timestamp)
	if age > params.MaxAttestationAge {
		return types.Attestation{}, errorsmod.Wrapf(types.ErrAttestationStale, "age=%ds, max=%ds", age, params.MaxAttestationAge)
	}
	return att, nil
}

func toABI(att types.Attestation) abiAttestation {
	return abiAttestation{
		Family:     att.Family,
		MRTD:       att.MRTD,
		ReportData: att.ReportData,
		Timestamp:  att.Timestamp,
		Debug:      att.Debug,
	}
}

// handleVerify implements `verify(uint8 family, bytes quote)`.
func (p Precompile) handleVerify(
	ctx sdk.Context,
	method *abi.Method,
	args []interface{},
) ([]byte, error) {
	if len(args) != 2 {
		return nil, fmt.Errorf("verify: expected 2 args, got %d", len(args))
	}
	family, ok := args[0].(uint8)
	if !ok {
		return nil, fmt.Errorf("verify: invalid family type: %T", args[0])
	}
	quote, ok := args[1].([]byte)
	if !ok {
		return nil, fmt.Errorf("verify: invalid quote type: %T", args[1])
	}
	att, err := p.dispatchVerify(ctx, family, quote)
	if err != nil {
		return nil, err
	}
	return method.Outputs.Pack(toABI(att))
}

// handleVerifyAndExpect implements `verifyAndExpect(uint8 family, bytes quote, bytes32 expectedReportData)`.
// Identical to verify() except it additionally checks that the recovered
// report_data matches the caller's expected value.
func (p Precompile) handleVerifyAndExpect(
	ctx sdk.Context,
	method *abi.Method,
	args []interface{},
) ([]byte, error) {
	if len(args) != 3 {
		return nil, fmt.Errorf("verifyAndExpect: expected 3 args, got %d", len(args))
	}
	family, ok := args[0].(uint8)
	if !ok {
		return nil, fmt.Errorf("verifyAndExpect: invalid family type: %T", args[0])
	}
	quote, ok := args[1].([]byte)
	if !ok {
		return nil, fmt.Errorf("verifyAndExpect: invalid quote type: %T", args[1])
	}
	expected, ok := args[2].([32]byte)
	if !ok {
		return nil, fmt.Errorf("verifyAndExpect: invalid expectedReportData type: %T", args[2])
	}
	att, err := p.dispatchVerify(ctx, family, quote)
	if err != nil {
		return nil, err
	}
	if att.ReportData != expected {
		return nil, errorsmod.Wrap(types.ErrReportDataMismatch, "verifyAndExpect")
	}
	return method.Outputs.Pack(toABI(att))
}

// handleRootOf implements `rootOf(uint8 family, uint256 index)`.
func (p Precompile) handleRootOf(
	ctx sdk.Context,
	method *abi.Method,
	args []interface{},
) ([]byte, error) {
	if len(args) != 2 {
		return nil, fmt.Errorf("rootOf: expected 2 args, got %d", len(args))
	}
	family, ok := args[0].(uint8)
	if !ok {
		return nil, fmt.Errorf("rootOf: invalid family type: %T", args[0])
	}
	indexBig, ok := args[1].(*big.Int)
	if !ok {
		return nil, fmt.Errorf("rootOf: invalid index type: %T", args[1])
	}
	if !indexBig.IsUint64() {
		return nil, fmt.Errorf("rootOf: index out of uint64 range")
	}
	root, _ := p.attestorKeeper.RootOf(ctx, family, uint32(indexBig.Uint64()))
	if root == nil {
		root = []byte{}
	}
	return method.Outputs.Pack(root)
}

// handleRootCount implements `rootCount(uint8 family)`.
func (p Precompile) handleRootCount(
	ctx sdk.Context,
	method *abi.Method,
	args []interface{},
) ([]byte, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("rootCount: expected 1 arg, got %d", len(args))
	}
	family, ok := args[0].(uint8)
	if !ok {
		return nil, fmt.Errorf("rootCount: invalid family type: %T", args[0])
	}
	count := p.attestorKeeper.RootCount(ctx, family)
	return method.Outputs.Pack(new(big.Int).SetUint64(uint64(count)))
}
