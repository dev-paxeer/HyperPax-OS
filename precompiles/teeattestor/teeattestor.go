// Copyright PaxLabs Ltd.(Paxeer Network)
// Paxeer Network Non-Commercial License 1.0 (ENCL-1.0)(https://github.com/Paxeer-Network/hyperpaxeer-os/blob/main/LICENSE_FAQ.md)

package teeattestor

import (
	"embed"
	"fmt"

	storetypes "github.com/cosmos/cosmos-sdk/store/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"

	cmn "github.com/evmos/evmos/v18/precompiles/common"
	attestorkeeper "github.com/evmos/evmos/v18/x/attestor/keeper"
)

var _ vm.PrecompiledContract = &Precompile{}

const PrecompileAddress = "0x0000000000000000000000000000000000000907"

const (
	VerifyMethod          = "verify"
	VerifyAndExpectMethod = "verifyAndExpect"
	RootOfMethod          = "rootOf"
	RootCountMethod       = "rootCount"
)

//go:embed abi.json
var f embed.FS

// Precompile implements the TEE Attestation precompile (0x0907).
//
// State (trusted root certs) lives in `x/attestor`. Each `verify(...)` call
// performs pure crypto over the input bytes + module-loaded roots — no other
// state writes. Determinism is preserved by avoiding `time.Now()` (use
// `ctx.BlockTime()` for staleness checks) and `crypto/rand` (verification
// requires no randomness).
//
// Per-family verifier implementations live in:
//   - precompiles/teeattestor/intel_tdx.go      (verifyIntelTDX)
//   - precompiles/teeattestor/amd_sev_snp.go    (verifyAMDSEVSNP)
//   - precompiles/teeattestor/nvidia.go         (verifyNVIDIA)
//   - precompiles/teeattestor/intel_sgx.go      (verifyIntelSGX)
//
// Each exports `verify<Family>(roots [][]byte, quote []byte) (Attestation, error)`.
// The shared envelope format + signature dispatch helpers live in envelope.go.
type Precompile struct {
	cmn.Precompile
	attestorKeeper attestorkeeper.Keeper
}

// NewPrecompile constructs the TEE Attestation precompile.
func NewPrecompile(attestorKeeper attestorkeeper.Keeper) (*Precompile, error) {
	newABI, err := cmn.LoadABI(f, "abi.json")
	if err != nil {
		return nil, err
	}

	return &Precompile{
		Precompile: cmn.Precompile{
			ABI:                  newABI,
			KvGasConfig:          storetypes.KVGasConfig(),
			TransientKVGasConfig: storetypes.TransientGasConfig(),
		},
		attestorKeeper: attestorKeeper,
	}, nil
}

func (Precompile) Address() common.Address {
	return common.HexToAddress(PrecompileAddress)
}

// IsTransaction returns false for every method; verification is a pure read
// of trusted roots + caller-provided bytes. STATICCALL-safe.
func (Precompile) IsTransaction(_ string) bool { return false }

// RequiredGas returns gas based on family. Defaults reflect the spec; once we
// have ctx in Run() we read `attestorKeeper.GetParams(ctx)` for live values.
func (p Precompile) RequiredGas(input []byte) uint64 {
	if len(input) < 4 {
		return 0
	}
	method, err := p.MethodById(input[:4])
	if err != nil {
		return 0
	}

	switch method.Name {
	case VerifyMethod, VerifyAndExpectMethod:
		// Conservative upper-bound; pessimistically charge for NVIDIA family.
		return 30_000 + 15_000
	case RootOfMethod, RootCountMethod:
		return 200
	default:
		return 0
	}
}

// Run is the EVM entrypoint. Decodes calldata, dispatches to the per-family
// verifier (or to keeper view methods for rootOf / rootCount). Verification
// is read-only — see envelope.go for the shared envelope format.
func (p Precompile) Run(evm *vm.EVM, contract *vm.Contract, readOnly bool) (bz []byte, err error) {
	ctx, _, method, initialGas, args, err := p.RunSetup(evm, contract, readOnly, p.IsTransaction)
	if err != nil {
		return nil, err
	}
	defer cmn.HandleGasError(ctx, contract, initialGas, &err)()

	switch method.Name {
	case VerifyMethod:
		bz, err = p.handleVerify(ctx, method, args)
	case VerifyAndExpectMethod:
		bz, err = p.handleVerifyAndExpect(ctx, method, args)
	case RootOfMethod:
		bz, err = p.handleRootOf(ctx, method, args)
	case RootCountMethod:
		bz, err = p.handleRootCount(ctx, method, args)
	default:
		return nil, fmt.Errorf("unknown method: %s", method.Name)
	}
	if err != nil {
		return nil, err
	}
	return bz, nil
}
