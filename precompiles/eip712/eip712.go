// Copyright PaxLabs Ltd.(Paxeer Network)
// Paxeer Network Non-Commercial License 1.0 (ENCL-1.0)(https://github.com/Paxeer-Network/hyperpaxeer-os/blob/main/LICENSE_FAQ.md)

package eip712

import (
	"embed"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/crypto"

	cmn "github.com/evmos/evmos/v18/precompiles/common"
)

var _ vm.PrecompiledContract = &Precompile{}

const PrecompileAddress = "0x0000000000000000000000000000000000000908"

const (
	HashTypedDataMethod      = "hashTypedData"
	DomainSeparatorMethod    = "domainSeparator"
	RecoverTypedSignerMethod = "recoverTypedSigner"
)

//go:embed abi.json
var f embed.FS

// Precompile implements the EIP-712 helper precompile (0x0908). Stateless,
// pure-crypto. Mirrors the shape of `precompiles/paxspot/orob/orob.go`:
// embed `abi.ABI`, no keeper.
//
// Methods (see abi.json for the full ABI):
//   - hashTypedData(ds, sh):      keccak256("\x19\x01" || ds || sh)
//   - domainSeparator(name, ver, chainId, verifyingContract):
//     keccak256(abi.encode(EIP712_DOMAIN_TYPEHASH, keccak256(name),
//     keccak256(version), chainId, verifyingContract))
//   - recoverTypedSigner(ds, sh, signature):
//     ecrecover over keccak256("\x19\x01" || ds || sh) using a
//     65-byte (r || s || v) signature. Returns the zero address on a
//     structurally-valid-but-unrecoverable signature so callers can
//     cheaply detect failure without parsing a revert reason.
type Precompile struct {
	abi.ABI
}

// eip712DomainTypeHash is `keccak256("EIP712Domain(string name,string version,uint256 chainId,address verifyingContract)")`.
// Computed once at package init so it doesn't allocate on every domainSeparator call.
var eip712DomainTypeHash = crypto.Keccak256Hash([]byte(
	"EIP712Domain(string name,string version,uint256 chainId,address verifyingContract)",
))

// NewPrecompile constructs the EIP-712 helper precompile.
func NewPrecompile() (*Precompile, error) {
	newABI, err := cmn.LoadABI(f, "abi.json")
	if err != nil {
		return nil, err
	}
	return &Precompile{ABI: newABI}, nil
}

// Address returns the precompile's EVM address.
func (Precompile) Address() common.Address {
	return common.HexToAddress(PrecompileAddress)
}

// RequiredGas returns the flat gas cost per method. Spec values:
//
//	hashTypedData       100  (two keccak256 hashes)
//	domainSeparator     200  (one keccak256 + string hashes)
//	recoverTypedSigner  400  (covers ecrecover)
func (p Precompile) RequiredGas(input []byte) uint64 {
	if len(input) < 4 {
		return 0
	}
	method, err := p.MethodById(input[:4])
	if err != nil {
		return 0
	}

	switch method.Name {
	case HashTypedDataMethod:
		return 100
	case DomainSeparatorMethod:
		return 200
	case RecoverTypedSignerMethod:
		return 400
	default:
		return 0
	}
}

// Run is the EVM entrypoint. Stateless precompile — no ctx, no keeper.
//
// The flat RequiredGas charge is consumed by the EVM frame BEFORE Run is
// invoked, so this body only has to do the arithmetic — no gas-charge dance.
func (p Precompile) Run(_ *vm.EVM, contract *vm.Contract, _ bool) (bz []byte, err error) {
	if len(contract.Input) < 4 {
		return nil, fmt.Errorf("input too short")
	}
	method, err := p.MethodById(contract.Input[:4])
	if err != nil {
		return nil, err
	}
	args, err := method.Inputs.Unpack(contract.Input[4:])
	if err != nil {
		return nil, err
	}

	switch method.Name {
	case HashTypedDataMethod:
		return p.handleHashTypedData(method, args)
	case DomainSeparatorMethod:
		return p.handleDomainSeparator(method, args)
	case RecoverTypedSignerMethod:
		return p.handleRecoverTypedSigner(method, args)
	default:
		return nil, fmt.Errorf("unknown method: %s", method.Name)
	}
}

// handleHashTypedData computes the EIP-712 final digest:
//
//	keccak256("\x19\x01" || domainSeparator || structHash)
//
// This is the value an off-chain signer wraps with their private key to
// produce the EIP-712 signature.
func (p Precompile) handleHashTypedData(method *abi.Method, args []interface{}) ([]byte, error) {
	if len(args) != 2 {
		return nil, fmt.Errorf("hashTypedData: expected 2 args, got %d", len(args))
	}
	ds, ok := args[0].([32]byte)
	if !ok {
		return nil, fmt.Errorf("hashTypedData: invalid domainSeparator type: %T", args[0])
	}
	sh, ok := args[1].([32]byte)
	if !ok {
		return nil, fmt.Errorf("hashTypedData: invalid structHash type: %T", args[1])
	}
	digest := eip712Digest(ds, sh)
	return method.Outputs.Pack(digest)
}

// eip712Digest is the canonical EIP-712 digest:
//
//	keccak256("\x19\x01" || domainSeparator || structHash)
//
// Exposed as a package-level helper so `recoverTypedSigner` can re-use it
// without re-encoding via the abi.Method machinery.
func eip712Digest(ds, sh [32]byte) [32]byte {
	buf := make([]byte, 2+32+32)
	buf[0] = 0x19
	buf[1] = 0x01
	copy(buf[2:34], ds[:])
	copy(buf[34:66], sh[:])
	return crypto.Keccak256Hash(buf)
}

// handleDomainSeparator computes the EIP-712 domain separator:
//
//	keccak256(abi.encode(
//	  EIP712Domain(string name,string version,uint256 chainId,address verifyingContract),
//	  keccak256(bytes(name)),
//	  keccak256(bytes(version)),
//	  chainId,
//	  verifyingContract,
//	))
//
// Field offsets follow `abi.encode` for the typed tuple: each field is left
// pad to 32 bytes, no length prefixes (these aren't dynamic-length args at
// this layer — name and version are pre-hashed).
func (p Precompile) handleDomainSeparator(method *abi.Method, args []interface{}) ([]byte, error) {
	if len(args) != 4 {
		return nil, fmt.Errorf("domainSeparator: expected 4 args, got %d", len(args))
	}
	name, ok := args[0].(string)
	if !ok {
		return nil, fmt.Errorf("domainSeparator: invalid name type: %T", args[0])
	}
	version, ok := args[1].(string)
	if !ok {
		return nil, fmt.Errorf("domainSeparator: invalid version type: %T", args[1])
	}
	chainID, ok := args[2].(*big.Int)
	if !ok {
		return nil, fmt.Errorf("domainSeparator: invalid chainId type: %T", args[2])
	}
	if chainID == nil {
		return nil, fmt.Errorf("domainSeparator: chainId is nil")
	}
	if chainID.Sign() < 0 {
		return nil, fmt.Errorf("domainSeparator: chainId must be non-negative")
	}
	verifyingContract, ok := args[3].(common.Address)
	if !ok {
		return nil, fmt.Errorf("domainSeparator: invalid verifyingContract type: %T", args[3])
	}

	nameHash := crypto.Keccak256Hash([]byte(name))
	versionHash := crypto.Keccak256Hash([]byte(version))

	// abi.encode is left-pad to 32 bytes for each fixed-size field.
	buf := make([]byte, 32*5)
	copy(buf[0:32], eip712DomainTypeHash[:])
	copy(buf[32:64], nameHash[:])
	copy(buf[64:96], versionHash[:])
	chainID.FillBytes(buf[96:128]) // panics if chainID > 2^256-1; ABI-bounded by uint256
	copy(buf[128+12:160], verifyingContract.Bytes())

	separator := crypto.Keccak256Hash(buf)
	return method.Outputs.Pack(separator)
}

// handleRecoverTypedSigner runs ecrecover over an EIP-712 digest. Signature
// is the canonical 65-byte (r || s || v) layout. The `v` byte is normalized
// to {0,1} (subtracting 27 if needed) before being passed to crypto.Ecrecover.
//
// Returns the zero address when:
//   - signature is not 65 bytes
//   - v is not in {0, 1, 27, 28}
//   - r/s/v fail Ecrecover validation
//
// Callers that want a strict error on failure should compare the returned
// address against common.Address{} before trusting it.
func (p Precompile) handleRecoverTypedSigner(method *abi.Method, args []interface{}) ([]byte, error) {
	if len(args) != 3 {
		return nil, fmt.Errorf("recoverTypedSigner: expected 3 args, got %d", len(args))
	}
	ds, ok := args[0].([32]byte)
	if !ok {
		return nil, fmt.Errorf("recoverTypedSigner: invalid domainSeparator type: %T", args[0])
	}
	sh, ok := args[1].([32]byte)
	if !ok {
		return nil, fmt.Errorf("recoverTypedSigner: invalid structHash type: %T", args[1])
	}
	sig, ok := args[2].([]byte)
	if !ok {
		return nil, fmt.Errorf("recoverTypedSigner: invalid signature type: %T", args[2])
	}
	if len(sig) != 65 {
		return method.Outputs.Pack(common.Address{})
	}

	// Copy because Ecrecover mutates the v byte in some go-ethereum versions
	// and we don't want to alter caller-owned memory.
	sigCopy := make([]byte, 65)
	copy(sigCopy, sig)
	// Normalize v from {27,28} -> {0,1} as Ecrecover expects.
	switch sigCopy[64] {
	case 27, 28:
		sigCopy[64] -= 27
	case 0, 1:
		// already normalized
	default:
		return method.Outputs.Pack(common.Address{})
	}

	digest := eip712Digest(ds, sh)
	pub, err := crypto.Ecrecover(digest[:], sigCopy)
	if err != nil || len(pub) != 65 {
		return method.Outputs.Pack(common.Address{})
	}
	// Drop the 0x04 uncompressed-pubkey prefix; last 20 bytes of keccak256 of
	// the remaining 64 bytes = the address.
	addrHash := crypto.Keccak256(pub[1:])
	var signer common.Address
	copy(signer[:], addrHash[12:])
	return method.Outputs.Pack(signer)
}
