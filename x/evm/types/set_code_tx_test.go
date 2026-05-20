// Copyright PaxLabs Ltd.(Paxeer Network)
// Paxeer Network Non-Commercial License 1.0 (ENCL-1.0)(https://github.com/Paxeer-Network/hyperpaxeer-os/blob/main/LICENSE_FAQ.md)

// Tests for the EIP-7702 SetCodeTx proto wrapper. Coverage:
//   - Constructor pulls AuthList off the geth tx
//   - AsEthereumData roundtrip preserves payload + auth list
//   - Copy is deep wrt AuthList (mutation isolation)
//   - Validate enforces non-empty auth list, non-nil to, V parity
//   - TxType returns 0x04
//   - NewTxDataFromTx dispatches type 0x04 to NewSetCodeTx
//
// Cross-implementation vectors (viem / foundry) belong on the fork side
// (see paxeer-sdk/go-ethereum/core/types/tx_setcode_test.go) — this file
// only covers the wrapper / dispatch.

package types_test

import (
	"math/big"
	"testing"

	sdkmath "cosmossdk.io/math"

	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"

	"github.com/evmos/evmos/v18/x/evm/types"
)

// testAddr returns a stable non-zero common.Address for use across the file.
func testAddr() common.Address {
	return common.HexToAddress("0xdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef")
}

// buildGethSetCodeTx returns a signed-shape (V/R/S populated) geth SetCodeTx
// with a single authorization tuple. Not actually signed — populates the V/R/S
// fields with sentinel non-zero values so RawSignatureValues round-trips.
func buildGethSetCodeTx(t *testing.T) *ethtypes.Transaction {
	t.Helper()
	to := testAddr()
	auth := ethtypes.Authorization{
		ChainID: big.NewInt(125),
		Address: testAddr(),
		Nonce:   42,
		V:       big.NewInt(0),
		R:       big.NewInt(0x1234),
		S:       big.NewInt(0x5678),
	}
	inner := &ethtypes.SetCodeTx{
		ChainID:   big.NewInt(125),
		Nonce:     7,
		GasTipCap: big.NewInt(1),
		GasFeeCap: big.NewInt(2),
		Gas:       1_000_000,
		To:        &to,
		Value:     big.NewInt(0),
		Data:      []byte("payload"),
		AuthList:  []ethtypes.Authorization{auth},
		V:         big.NewInt(0),
		R:         big.NewInt(0x9abc),
		S:         big.NewInt(0xdef0),
	}
	return ethtypes.NewTx(inner)
}

func TestSetCodeTx_TxType(t *testing.T) {
	var tx types.SetCodeTx
	if got := tx.TxType(); got != ethtypes.SetCodeTxType {
		t.Fatalf("TxType: want 0x%02x, got 0x%02x", ethtypes.SetCodeTxType, got)
	}
}

func TestNewSetCodeTx_HappyPath(t *testing.T) {
	tx := buildGethSetCodeTx(t)
	wrapper, err := types.NewSetCodeTx(tx)
	if err != nil {
		t.Fatalf("NewSetCodeTx: %v", err)
	}
	if wrapper.Nonce != 7 {
		t.Fatalf("Nonce: want 7, got %d", wrapper.Nonce)
	}
	if wrapper.GasLimit != 1_000_000 {
		t.Fatalf("GasLimit: want 1_000_000, got %d", wrapper.GasLimit)
	}
	if wrapper.To != testAddr().Hex() {
		t.Fatalf("To: want %s, got %s", testAddr().Hex(), wrapper.To)
	}
	if string(wrapper.Data) != "payload" {
		t.Fatalf("Data: want 'payload', got %q", wrapper.Data)
	}
	if len(wrapper.AuthList) != 1 {
		t.Fatalf("AuthList len: want 1, got %d", len(wrapper.AuthList))
	}
	a := wrapper.AuthList[0]
	if a.Nonce != 42 {
		t.Fatalf("AuthList[0].Nonce: want 42, got %d", a.Nonce)
	}
	if a.Address != testAddr().Hex() {
		t.Fatalf("AuthList[0].Address: want %s, got %s", testAddr().Hex(), a.Address)
	}
	if a.ChainID == nil || a.ChainID.BigInt().Cmp(big.NewInt(125)) != 0 {
		t.Fatalf("AuthList[0].ChainID: want 125, got %v", a.ChainID)
	}
}

func TestNewSetCodeTx_RejectsWrongTxType(t *testing.T) {
	legacy := ethtypes.NewTx(&ethtypes.LegacyTx{
		Nonce: 1,
		To:    nil,
		Value: big.NewInt(0),
		Gas:   21000,
	})
	if _, err := types.NewSetCodeTx(legacy); err == nil {
		t.Fatal("NewSetCodeTx accepted a legacy tx; expected type mismatch error")
	}
}

func TestSetCodeTx_AsEthereumData_Roundtrip(t *testing.T) {
	src := buildGethSetCodeTx(t)
	wrapper, err := types.NewSetCodeTx(src)
	if err != nil {
		t.Fatalf("NewSetCodeTx: %v", err)
	}

	rebuilt := ethtypes.NewTx(wrapper.AsEthereumData())
	if rebuilt.Type() != ethtypes.SetCodeTxType {
		t.Fatalf("rebuilt Type: want 0x04, got 0x%02x", rebuilt.Type())
	}
	if rebuilt.Nonce() != src.Nonce() {
		t.Fatalf("Nonce mismatch: want %d, got %d", src.Nonce(), rebuilt.Nonce())
	}
	if rebuilt.Gas() != src.Gas() {
		t.Fatalf("Gas mismatch: want %d, got %d", src.Gas(), rebuilt.Gas())
	}
	if rebuilt.To() == nil || *rebuilt.To() != *src.To() {
		t.Fatalf("To mismatch: want %v, got %v", src.To(), rebuilt.To())
	}
	rebuiltAuths := rebuilt.SetCodeAuthorizations()
	srcAuths := src.SetCodeAuthorizations()
	if len(rebuiltAuths) != len(srcAuths) {
		t.Fatalf("auth list length: want %d, got %d", len(srcAuths), len(rebuiltAuths))
	}
	for i, a := range rebuiltAuths {
		if a.Address != srcAuths[i].Address {
			t.Fatalf("auth[%d].Address mismatch", i)
		}
		if a.Nonce != srcAuths[i].Nonce {
			t.Fatalf("auth[%d].Nonce mismatch: want %d, got %d", i, srcAuths[i].Nonce, a.Nonce)
		}
		if a.ChainID.Cmp(srcAuths[i].ChainID) != 0 {
			t.Fatalf("auth[%d].ChainID mismatch", i)
		}
	}
}

func TestSetCodeTx_Copy_DeepCopiesAuthList(t *testing.T) {
	src := buildGethSetCodeTx(t)
	wrapper, err := types.NewSetCodeTx(src)
	if err != nil {
		t.Fatalf("NewSetCodeTx: %v", err)
	}
	cpy, ok := wrapper.Copy().(*types.SetCodeTx)
	if !ok {
		t.Fatalf("Copy did not return *SetCodeTx, got %T", wrapper.Copy())
	}
	// Mutate the copy's auth list — original must be untouched.
	cpy.AuthList[0].Nonce = 9999
	if wrapper.AuthList[0].Nonce == 9999 {
		t.Fatal("AuthList shares backing memory with original — Copy is not deep")
	}
}

func TestSetCodeTx_Validate_RejectsEmptyAuthList(t *testing.T) {
	chainID := sdkmath.NewInt(125)
	zero := sdkmath.ZeroInt()
	one := sdkmath.OneInt()
	tx := types.SetCodeTx{
		ChainID:   &chainID,
		Nonce:     1,
		GasTipCap: &one,
		GasFeeCap: &one,
		GasLimit:  21000,
		To:        testAddr().Hex(),
		Amount:    &zero,
	}
	if err := tx.Validate(); err == nil {
		t.Fatal("Validate accepted empty auth list; expected error")
	}
}

func TestSetCodeTx_Validate_RejectsContractCreation(t *testing.T) {
	chainID := sdkmath.NewInt(125)
	zero := sdkmath.ZeroInt()
	one := sdkmath.OneInt()
	tx := types.SetCodeTx{
		ChainID:   &chainID,
		Nonce:     1,
		GasTipCap: &one,
		GasFeeCap: &one,
		GasLimit:  21000,
		To:        "", // EIP-7702 forbids contract creation
		Amount:    &zero,
		AuthList: []types.SetCodeAuthorization{{
			Address: testAddr().Hex(),
			Nonce:   0,
		}},
	}
	if err := tx.Validate(); err == nil {
		t.Fatal("Validate accepted empty To; expected contract-creation error")
	}
}

func TestSetCodeTx_Validate_RejectsLegacyParity(t *testing.T) {
	chainID := sdkmath.NewInt(125)
	zero := sdkmath.ZeroInt()
	one := sdkmath.OneInt()
	tx := types.SetCodeTx{
		ChainID:   &chainID,
		Nonce:     1,
		GasTipCap: &one,
		GasFeeCap: &one,
		GasLimit:  21000,
		To:        testAddr().Hex(),
		Amount:    &zero,
		AuthList: []types.SetCodeAuthorization{{
			Address: testAddr().Hex(),
			Nonce:   0,
			V:       big.NewInt(27).Bytes(), // legacy parity, illegal per EIP-7702 §3
			R:       big.NewInt(1).Bytes(),
			S:       big.NewInt(1).Bytes(),
		}},
	}
	if err := tx.Validate(); err == nil {
		t.Fatal("Validate accepted V=27 on auth tuple; expected parity error")
	}
}

func TestSetCodeTx_Validate_AcceptsValid(t *testing.T) {
	chainID := sdkmath.NewInt(125)
	zero := sdkmath.ZeroInt()
	one := sdkmath.OneInt()
	tx := types.SetCodeTx{
		ChainID:   &chainID,
		Nonce:     1,
		GasTipCap: &one,
		GasFeeCap: &one,
		GasLimit:  21000,
		To:        testAddr().Hex(),
		Amount:    &zero,
		AuthList: []types.SetCodeAuthorization{{
			Address: testAddr().Hex(),
			Nonce:   0,
			V:       []byte{0}, // y_parity = 0
			R:       big.NewInt(1).Bytes(),
			S:       big.NewInt(1).Bytes(),
		}},
	}
	if err := tx.Validate(); err != nil {
		t.Fatalf("Validate rejected valid SetCodeTx: %v", err)
	}
}

func TestNewTxDataFromTx_DispatchesType04(t *testing.T) {
	src := buildGethSetCodeTx(t)
	txData, err := types.NewTxDataFromTx(src)
	if err != nil {
		t.Fatalf("NewTxDataFromTx: %v", err)
	}
	if _, ok := txData.(*types.SetCodeTx); !ok {
		t.Fatalf("NewTxDataFromTx dispatched type-0x04 to %T, expected *SetCodeTx", txData)
	}
	if txData.TxType() != ethtypes.SetCodeTxType {
		t.Fatalf("TxData.TxType: want 0x04, got 0x%02x", txData.TxType())
	}
}
