// Copyright PaxLabs Ltd.(Paxeer Network)
// Paxeer Network Non-Commercial License 1.0 (ENCL-1.0)(https://github.com/Paxeer-Network/hyperpaxeer-os/blob/main/LICENSE_FAQ.md)

// Hand-crafted protobuf + TxData implementation for the EIP-7702 SetCodeTx
// (transaction type 0x04). Mirrors the paxoracle hand-crafted-proto pattern
// (see x/paxoracle/types/msgs.go and tx.pb.go) so we can ship type-0x04
// support in v20 WITHOUT regenerating ethermint/evm/v1/tx.proto. The proto
// regen ban is documented in AGENTS.md §4.3 and the v20 handoff Pass 2.
//
// The geth-side SetCodeTx (delegation marker, AuthorityHash, RecoverAuthority,
// ApplyAuthorizations) lives in our forked Paxeer-Network/go-ethereum at
// core/types/tx_setcode.go and core/state_transition_setcode.go — see the
// backport spec at app/upgrades/v20agent/geth_7702_backport.md.

package types

import (
	"errors"
	"fmt"
	"io"
	"math/big"

	errorsmod "cosmossdk.io/errors"
	sdkmath "cosmossdk.io/math"
	errortypes "github.com/cosmos/cosmos-sdk/types/errors"
	proto "github.com/cosmos/gogoproto/proto"

	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"

	"github.com/evmos/evmos/v18/types"
)

// Compile-time interface assertions.
var (
	_ TxData        = (*SetCodeTx)(nil)
	_ proto.Message = (*SetCodeTx)(nil)
	_ proto.Message = (*SetCodeAuthorization)(nil)
)

// ─── Type definitions ───────────────────────────────────────────────────────

// SetCodeTx is the chain's protobuf wrapper for an EIP-7702 transaction
// (type 0x04). Field numbers 1-13 mirror DynamicFeeTx where possible to keep
// ABCI tooling consistent; AuthList is field 14 (the only 7702-specific add).
type SetCodeTx struct {
	ChainID   *sdkmath.Int            `protobuf:"bytes,1,opt,name=chain_id,json=chainId,proto3" json:"chainID"`
	Nonce     uint64                  `protobuf:"varint,2,opt,name=nonce,proto3" json:"nonce,omitempty"`
	GasTipCap *sdkmath.Int            `protobuf:"bytes,3,opt,name=gas_tip_cap,json=gasTipCap,proto3" json:"gasTipCap,omitempty"`
	GasFeeCap *sdkmath.Int            `protobuf:"bytes,4,opt,name=gas_fee_cap,json=gasFeeCap,proto3" json:"gasFeeCap,omitempty"`
	GasLimit  uint64                  `protobuf:"varint,5,opt,name=gas,proto3" json:"gas,omitempty"`
	To        string                  `protobuf:"bytes,6,opt,name=to,proto3" json:"to,omitempty"`
	Amount    *sdkmath.Int            `protobuf:"bytes,7,opt,name=value,proto3" json:"value,omitempty"`
	Data      []byte                  `protobuf:"bytes,8,opt,name=data,proto3" json:"data,omitempty"`
	Accesses  AccessList              `protobuf:"bytes,9,rep,name=accesses,proto3" json:"accessList"`
	V         []byte                  `protobuf:"bytes,11,opt,name=v,proto3" json:"v,omitempty"`
	R         []byte                  `protobuf:"bytes,12,opt,name=r,proto3" json:"r,omitempty"`
	S         []byte                  `protobuf:"bytes,13,opt,name=s,proto3" json:"s,omitempty"`
	AuthList  []SetCodeAuthorization  `protobuf:"bytes,14,rep,name=auth_list,json=authList,proto3" json:"authList"`
}

// SetCodeAuthorization is one entry of an EIP-7702 authorization list. Fields
// match the geth-side ethtypes.Authorization 1:1 (chain_id, address, nonce,
// y_parity, r, s) but are wire-encoded protobuf-style for transit through
// codectypes.Any inside MsgEthereumTx.Data.
type SetCodeAuthorization struct {
	ChainID *sdkmath.Int `protobuf:"bytes,1,opt,name=chain_id,json=chainId,proto3" json:"chainID"`
	Address string       `protobuf:"bytes,2,opt,name=address,proto3" json:"address,omitempty"`
	Nonce   uint64       `protobuf:"varint,3,opt,name=nonce,proto3" json:"nonce,omitempty"`
	V       []byte       `protobuf:"bytes,4,opt,name=v,proto3" json:"v,omitempty"`
	R       []byte       `protobuf:"bytes,5,opt,name=r,proto3" json:"r,omitempty"`
	S       []byte       `protobuf:"bytes,6,opt,name=s,proto3" json:"s,omitempty"`
}

// ─── Constructor (from geth Transaction) ────────────────────────────────────

// NewSetCodeTx builds a SetCodeTx wrapper from a geth *ethtypes.Transaction.
// The caller MUST have verified tx.Type() == ethtypes.SetCodeTxType (0x04).
func NewSetCodeTx(tx *ethtypes.Transaction) (*SetCodeTx, error) {
	if tx.Type() != ethtypes.SetCodeTxType {
		return nil, fmt.Errorf("NewSetCodeTx: expected type 0x04, got 0x%02x", tx.Type())
	}
	txData := &SetCodeTx{
		Nonce:    tx.Nonce(),
		Data:     tx.Data(),
		GasLimit: tx.Gas(),
	}

	v, r, s := tx.RawSignatureValues()
	if to := tx.To(); to != nil {
		txData.To = to.Hex()
	}

	if tx.Value() != nil {
		amountInt, err := types.SafeNewIntFromBigInt(tx.Value())
		if err != nil {
			return nil, err
		}
		txData.Amount = &amountInt
	}
	if tx.GasFeeCap() != nil {
		gasFeeCapInt, err := types.SafeNewIntFromBigInt(tx.GasFeeCap())
		if err != nil {
			return nil, err
		}
		txData.GasFeeCap = &gasFeeCapInt
	}
	if tx.GasTipCap() != nil {
		gasTipCapInt, err := types.SafeNewIntFromBigInt(tx.GasTipCap())
		if err != nil {
			return nil, err
		}
		txData.GasTipCap = &gasTipCapInt
	}

	if al := tx.AccessList(); al != nil {
		txData.Accesses = NewAccessList(&al)
	}

	for _, a := range tx.SetCodeAuthorizations() {
		auth := SetCodeAuthorization{
			Address: a.Address.Hex(),
			Nonce:   a.Nonce,
		}
		if a.ChainID != nil {
			cid, err := types.SafeNewIntFromBigInt(a.ChainID)
			if err != nil {
				return nil, errorsmod.Wrap(err, "invalid authorization chain_id")
			}
			auth.ChainID = &cid
		}
		if a.V != nil {
			auth.V = a.V.Bytes()
		}
		if a.R != nil {
			auth.R = a.R.Bytes()
		}
		if a.S != nil {
			auth.S = a.S.Bytes()
		}
		txData.AuthList = append(txData.AuthList, auth)
	}

	txData.SetSignatureValues(tx.ChainId(), v, r, s)
	return txData, nil
}

// ─── TxData interface ───────────────────────────────────────────────────────

// TxType returns the EIP-7702 transaction type identifier (0x04).
func (tx *SetCodeTx) TxType() uint8 { return ethtypes.SetCodeTxType }

// Copy returns a deep copy of the tx data with all owned slices/pointers
// reallocated. Mirrors DynamicFeeTx.Copy with explicit AuthList deep-copy.
func (tx *SetCodeTx) Copy() TxData {
	cpy := &SetCodeTx{
		ChainID:   tx.ChainID,
		Nonce:     tx.Nonce,
		GasTipCap: tx.GasTipCap,
		GasFeeCap: tx.GasFeeCap,
		GasLimit:  tx.GasLimit,
		To:        tx.To,
		Amount:    tx.Amount,
		Data:      common.CopyBytes(tx.Data),
		Accesses:  tx.Accesses,
		V:         common.CopyBytes(tx.V),
		R:         common.CopyBytes(tx.R),
		S:         common.CopyBytes(tx.S),
	}
	if len(tx.AuthList) > 0 {
		cpy.AuthList = make([]SetCodeAuthorization, len(tx.AuthList))
		for i, a := range tx.AuthList {
			cpy.AuthList[i] = SetCodeAuthorization{
				ChainID: a.ChainID,
				Address: a.Address,
				Nonce:   a.Nonce,
				V:       common.CopyBytes(a.V),
				R:       common.CopyBytes(a.R),
				S:       common.CopyBytes(a.S),
			}
		}
	}
	return cpy
}

func (tx *SetCodeTx) GetChainID() *big.Int {
	if tx.ChainID == nil {
		return nil
	}
	return tx.ChainID.BigInt()
}

func (tx *SetCodeTx) GetAccessList() ethtypes.AccessList {
	if tx.Accesses == nil {
		return nil
	}
	return *tx.Accesses.ToEthAccessList()
}

func (tx *SetCodeTx) GetData() []byte { return common.CopyBytes(tx.Data) }
func (tx *SetCodeTx) GetGas() uint64  { return tx.GasLimit }

// GetGasPrice returns GasFeeCap, matching DynamicFeeTx semantics.
func (tx *SetCodeTx) GetGasPrice() *big.Int { return tx.GetGasFeeCap() }

func (tx *SetCodeTx) GetGasTipCap() *big.Int {
	if tx.GasTipCap == nil {
		return nil
	}
	return tx.GasTipCap.BigInt()
}

func (tx *SetCodeTx) GetGasFeeCap() *big.Int {
	if tx.GasFeeCap == nil {
		return nil
	}
	return tx.GasFeeCap.BigInt()
}

func (tx *SetCodeTx) GetValue() *big.Int {
	if tx.Amount == nil {
		return nil
	}
	return tx.Amount.BigInt()
}

func (tx *SetCodeTx) GetNonce() uint64 { return tx.Nonce }

func (tx *SetCodeTx) GetTo() *common.Address {
	if tx.To == "" {
		return nil
	}
	to := common.HexToAddress(tx.To)
	return &to
}

// AsEthereumData returns a geth-native ethtypes.SetCodeTx populated from this
// proto wrapper. The resulting struct is suitable for wrapping with
// ethtypes.NewTx for sighash/recovery flows.
func (tx *SetCodeTx) AsEthereumData() ethtypes.TxData {
	v, r, s := tx.GetRawSignatureValues()
	out := &ethtypes.SetCodeTx{
		ChainID:    tx.GetChainID(),
		Nonce:      tx.GetNonce(),
		GasTipCap:  tx.GetGasTipCap(),
		GasFeeCap:  tx.GetGasFeeCap(),
		Gas:        tx.GetGas(),
		To:         tx.GetTo(),
		Value:      tx.GetValue(),
		Data:       tx.GetData(),
		AccessList: tx.GetAccessList(),
		V:          v,
		R:          r,
		S:          s,
	}
	if len(tx.AuthList) > 0 {
		out.AuthList = make([]ethtypes.Authorization, len(tx.AuthList))
		for i, a := range tx.AuthList {
			out.AuthList[i] = ethtypes.Authorization{
				Address: common.HexToAddress(a.Address),
				Nonce:   a.Nonce,
			}
			if a.ChainID != nil {
				out.AuthList[i].ChainID = a.ChainID.BigInt()
			}
			if len(a.V) > 0 {
				out.AuthList[i].V = new(big.Int).SetBytes(a.V)
			}
			if len(a.R) > 0 {
				out.AuthList[i].R = new(big.Int).SetBytes(a.R)
			}
			if len(a.S) > 0 {
				out.AuthList[i].S = new(big.Int).SetBytes(a.S)
			}
		}
	}
	return out
}

func (tx *SetCodeTx) GetRawSignatureValues() (v, r, s *big.Int) {
	return rawSignatureValues(tx.V, tx.R, tx.S)
}

func (tx *SetCodeTx) SetSignatureValues(chainID, v, r, s *big.Int) {
	if v != nil {
		tx.V = v.Bytes()
	}
	if r != nil {
		tx.R = r.Bytes()
	}
	if s != nil {
		tx.S = s.Bytes()
	}
	if chainID != nil {
		chainIDInt := sdkmath.NewIntFromBigInt(chainID)
		tx.ChainID = &chainIDInt
	}
}

// Validate performs a stateless validation of the tx fields, mirroring
// DynamicFeeTx.Validate plus EIP-7702-specific checks.
func (tx SetCodeTx) Validate() error {
	if tx.GasTipCap == nil {
		return errorsmod.Wrap(ErrInvalidGasCap, "gas tip cap cannot be nil")
	}
	if tx.GasFeeCap == nil {
		return errorsmod.Wrap(ErrInvalidGasCap, "gas fee cap cannot be nil")
	}
	if tx.GasTipCap.IsNegative() {
		return errorsmod.Wrapf(ErrInvalidGasCap, "gas tip cap cannot be negative %s", tx.GasTipCap)
	}
	if tx.GasFeeCap.IsNegative() {
		return errorsmod.Wrapf(ErrInvalidGasCap, "gas fee cap cannot be negative %s", tx.GasFeeCap)
	}
	if !types.IsValidInt256(tx.GetGasTipCap()) {
		return errorsmod.Wrap(ErrInvalidGasCap, "out of bound")
	}
	if !types.IsValidInt256(tx.GetGasFeeCap()) {
		return errorsmod.Wrap(ErrInvalidGasCap, "out of bound")
	}
	if tx.GasFeeCap.LT(*tx.GasTipCap) {
		return errorsmod.Wrapf(ErrInvalidGasCap,
			"max priority fee per gas higher than max fee per gas (%s > %s)",
			tx.GasTipCap, tx.GasFeeCap)
	}
	if !types.IsValidInt256(tx.Fee()) {
		return errorsmod.Wrap(ErrInvalidGasFee, "out of bound")
	}

	if amount := tx.GetValue(); amount != nil {
		if amount.Sign() < 0 {
			return errorsmod.Wrapf(ErrInvalidAmount, "amount cannot be negative %s", amount)
		}
		if !types.IsValidInt256(amount) {
			return errorsmod.Wrap(ErrInvalidAmount, "out of bound")
		}
	}

	// EIP-7702 requires a non-empty destination — contract creation via
	// type 0x04 is forbidden at the consensus layer.
	if tx.To == "" {
		return errorsmod.Wrap(errortypes.ErrInvalidRequest,
			"set code tx must have a non-nil destination (contract creation forbidden by EIP-7702)")
	}
	if err := types.ValidateAddress(tx.To); err != nil {
		return errorsmod.Wrap(err, "invalid to address")
	}

	if tx.GetChainID() == nil {
		return errorsmod.Wrap(errortypes.ErrInvalidChainID,
			"chain ID must be present on SetCode txs")
	}

	// EIP-7702 requires the authorization list to be non-empty — a SetCodeTx
	// with zero authorizations does nothing and would just be an expensive
	// DynamicFeeTx. Reject early to keep the mempool clean.
	if len(tx.AuthList) == 0 {
		return errorsmod.Wrap(errortypes.ErrInvalidRequest,
			"set code tx authorization list must be non-empty")
	}

	for i, a := range tx.AuthList {
		if a.Address == "" {
			return errorsmod.Wrapf(errortypes.ErrInvalidAddress,
				"authorization[%d]: empty target address", i)
		}
		if err := types.ValidateAddress(a.Address); err != nil {
			return errorsmod.Wrapf(err, "authorization[%d]: invalid target address", i)
		}
		// Per EIP-7702 §3, y_parity (V) MUST be 0 or 1; signers using legacy
		// 27/28 parities are explicitly invalid for authorization tuples.
		if len(a.V) != 0 {
			vbi := new(big.Int).SetBytes(a.V)
			if vbi.BitLen() > 8 {
				return errorsmod.Wrapf(errortypes.ErrInvalidRequest,
					"authorization[%d]: V out of range", i)
			}
			vb := vbi.Uint64()
			if vb != 0 && vb != 1 {
				return errorsmod.Wrapf(errortypes.ErrInvalidRequest,
					"authorization[%d]: V must be 0 or 1, got %d", i, vb)
			}
		}
	}

	return nil
}

// Fee returns gasFeeCap * gasLimit.
func (tx SetCodeTx) Fee() *big.Int { return fee(tx.GetGasFeeCap(), tx.GasLimit) }

// Cost returns amount + gasFeeCap * gasLimit.
func (tx SetCodeTx) Cost() *big.Int { return cost(tx.Fee(), tx.GetValue()) }

// EffectiveGasPrice mirrors DynamicFeeTx.EffectiveGasPrice (1559 semantics).
func (tx *SetCodeTx) EffectiveGasPrice(baseFee *big.Int) *big.Int {
	return EffectiveGasPrice(baseFee, tx.GasFeeCap.BigInt(), tx.GasTipCap.BigInt())
}

// EffectiveFee returns effective_gasprice * gaslimit.
func (tx SetCodeTx) EffectiveFee(baseFee *big.Int) *big.Int {
	return fee(tx.EffectiveGasPrice(baseFee), tx.GasLimit)
}

// EffectiveCost returns amount + effective_gasprice * gaslimit.
func (tx SetCodeTx) EffectiveCost(baseFee *big.Int) *big.Int {
	return cost(tx.EffectiveFee(baseFee), tx.GetValue())
}

// ─── proto.Message interface (SetCodeTx) ────────────────────────────────────

func (*SetCodeTx) ProtoMessage()    {}
func (tx *SetCodeTx) Reset()        { *tx = SetCodeTx{} }
func (tx *SetCodeTx) String() string {
	return fmt.Sprintf("SetCodeTx{nonce:%d gas:%d to:%s auths:%d}",
		tx.Nonce, tx.GasLimit, tx.To, len(tx.AuthList))
}

// XXX_MessageName returns the canonical proto name for codec dispatch.
func (*SetCodeTx) XXX_MessageName() string { return "ethermint.evm.v1.SetCodeTx" }

// ─── proto.Message interface (SetCodeAuthorization) ─────────────────────────

func (*SetCodeAuthorization) ProtoMessage() {}
func (a *SetCodeAuthorization) Reset()      { *a = SetCodeAuthorization{} }
func (a *SetCodeAuthorization) String() string {
	return fmt.Sprintf("SetCodeAuthorization{addr:%s nonce:%d}", a.Address, a.Nonce)
}
func (*SetCodeAuthorization) XXX_MessageName() string {
	return "ethermint.evm.v1.SetCodeAuthorization"
}

// ─── Wire format: SetCodeTx ─────────────────────────────────────────────────

func (tx *SetCodeTx) Marshal() ([]byte, error) {
	size := tx.Size()
	data := make([]byte, size)
	n, err := tx.MarshalToSizedBuffer(data)
	if err != nil {
		return nil, err
	}
	return data[:n], nil
}

func (tx *SetCodeTx) MarshalTo(data []byte) (int, error) {
	return tx.MarshalToSizedBuffer(data[:tx.Size()])
}

func (tx *SetCodeTx) MarshalToSizedBuffer(data []byte) (int, error) {
	i := len(data)
	// Encode in REVERSE field-number order so each call writes from the tail
	// backwards (matches the paxoracle/gogoproto pattern).

	// Field 14: auth_list (repeated message, tag = 0x72)
	for j := len(tx.AuthList) - 1; j >= 0; j-- {
		size := tx.AuthList[j].Size()
		if size > 0 {
			n, err := tx.AuthList[j].MarshalToSizedBuffer(data[:i])
			if err != nil {
				return 0, err
			}
			i -= n
			i = protoEncodeSetCodeVarint(data, i, uint64(size))
			i--
			data[i] = 0x72
		}
	}
	// Field 13: s (bytes, tag = 0x6a)
	if err := setcodeMarshalBytesField(data, &i, tx.S, 0x6a); err != nil {
		return 0, err
	}
	// Field 12: r (bytes, tag = 0x62)
	if err := setcodeMarshalBytesField(data, &i, tx.R, 0x62); err != nil {
		return 0, err
	}
	// Field 11: v (bytes, tag = 0x5a)
	if err := setcodeMarshalBytesField(data, &i, tx.V, 0x5a); err != nil {
		return 0, err
	}
	// Field 9: accesses (repeated message, tag = 0x4a)
	for j := len(tx.Accesses) - 1; j >= 0; j-- {
		bz, err := tx.Accesses[j].Marshal()
		if err != nil {
			return 0, err
		}
		i -= len(bz)
		copy(data[i:], bz)
		i = protoEncodeSetCodeVarint(data, i, uint64(len(bz)))
		i--
		data[i] = 0x4a
	}
	// Field 8: data (bytes, tag = 0x42)
	if err := setcodeMarshalBytesField(data, &i, tx.Data, 0x42); err != nil {
		return 0, err
	}
	// Field 7: amount (sdkmath.Int, tag = 0x3a)
	if err := setcodeMarshalIntField(data, &i, tx.Amount, 0x3a); err != nil {
		return 0, err
	}
	// Field 6: to (string, tag = 0x32)
	if len(tx.To) > 0 {
		i -= len(tx.To)
		copy(data[i:], tx.To)
		i = protoEncodeSetCodeVarint(data, i, uint64(len(tx.To)))
		i--
		data[i] = 0x32
	}
	// Field 5: gas_limit (varint, tag = 0x28)
	if tx.GasLimit != 0 {
		i = protoEncodeSetCodeVarint(data, i, tx.GasLimit)
		i--
		data[i] = 0x28
	}
	// Field 4: gas_fee_cap (sdkmath.Int, tag = 0x22)
	if err := setcodeMarshalIntField(data, &i, tx.GasFeeCap, 0x22); err != nil {
		return 0, err
	}
	// Field 3: gas_tip_cap (sdkmath.Int, tag = 0x1a)
	if err := setcodeMarshalIntField(data, &i, tx.GasTipCap, 0x1a); err != nil {
		return 0, err
	}
	// Field 2: nonce (varint, tag = 0x10)
	if tx.Nonce != 0 {
		i = protoEncodeSetCodeVarint(data, i, tx.Nonce)
		i--
		data[i] = 0x10
	}
	// Field 1: chain_id (sdkmath.Int, tag = 0x0a)
	if err := setcodeMarshalIntField(data, &i, tx.ChainID, 0x0a); err != nil {
		return 0, err
	}
	return len(data) - i, nil
}

func (tx *SetCodeTx) Size() int {
	n := 0
	n += setcodeIntFieldSize(tx.ChainID, 1)
	if tx.Nonce != 0 {
		n += 1 + protoSizeSetCodeVarint(tx.Nonce)
	}
	n += setcodeIntFieldSize(tx.GasTipCap, 1)
	n += setcodeIntFieldSize(tx.GasFeeCap, 1)
	if tx.GasLimit != 0 {
		n += 1 + protoSizeSetCodeVarint(tx.GasLimit)
	}
	if l := len(tx.To); l > 0 {
		n += 1 + l + protoSizeSetCodeVarint(uint64(l))
	}
	n += setcodeIntFieldSize(tx.Amount, 1)
	if l := len(tx.Data); l > 0 {
		n += 1 + l + protoSizeSetCodeVarint(uint64(l))
	}
	for _, a := range tx.Accesses {
		bz, err := a.Marshal()
		if err == nil && len(bz) > 0 {
			n += 1 + len(bz) + protoSizeSetCodeVarint(uint64(len(bz)))
		}
	}
	if l := len(tx.V); l > 0 {
		n += 1 + l + protoSizeSetCodeVarint(uint64(l))
	}
	if l := len(tx.R); l > 0 {
		n += 1 + l + protoSizeSetCodeVarint(uint64(l))
	}
	if l := len(tx.S); l > 0 {
		n += 1 + l + protoSizeSetCodeVarint(uint64(l))
	}
	for _, a := range tx.AuthList {
		size := a.Size()
		if size > 0 {
			n += 1 + size + protoSizeSetCodeVarint(uint64(size))
		}
	}
	return n
}

func (tx *SetCodeTx) Unmarshal(data []byte) error {
	*tx = SetCodeTx{}
	l := len(data)
	idx := 0
	for idx < l {
		tag, n, err := protoDecodeSetCodeVarint(data[idx:])
		if err != nil {
			return err
		}
		idx += n
		fieldNum := tag >> 3
		wireType := tag & 0x7

		switch fieldNum {
		case 1: // chain_id
			val, consumed, err := setcodeReadIntField(data, idx, wireType)
			if err != nil {
				return err
			}
			tx.ChainID = val
			idx = consumed
		case 2: // nonce
			if wireType != 0 {
				return fmt.Errorf("set_code_tx: field 2 (nonce) wire type %d, want varint", wireType)
			}
			v, n, err := protoDecodeSetCodeVarint(data[idx:])
			if err != nil {
				return err
			}
			tx.Nonce = v
			idx += n
		case 3: // gas_tip_cap
			val, consumed, err := setcodeReadIntField(data, idx, wireType)
			if err != nil {
				return err
			}
			tx.GasTipCap = val
			idx = consumed
		case 4: // gas_fee_cap
			val, consumed, err := setcodeReadIntField(data, idx, wireType)
			if err != nil {
				return err
			}
			tx.GasFeeCap = val
			idx = consumed
		case 5: // gas_limit
			if wireType != 0 {
				return fmt.Errorf("set_code_tx: field 5 (gas_limit) wire type %d, want varint", wireType)
			}
			v, n, err := protoDecodeSetCodeVarint(data[idx:])
			if err != nil {
				return err
			}
			tx.GasLimit = v
			idx += n
		case 6: // to (string)
			s, consumed, err := setcodeReadLengthDelim(data, idx, wireType)
			if err != nil {
				return err
			}
			tx.To = string(s)
			idx = consumed
		case 7: // amount
			val, consumed, err := setcodeReadIntField(data, idx, wireType)
			if err != nil {
				return err
			}
			tx.Amount = val
			idx = consumed
		case 8: // data
			b, consumed, err := setcodeReadLengthDelim(data, idx, wireType)
			if err != nil {
				return err
			}
			tx.Data = append([]byte(nil), b...)
			idx = consumed
		case 9: // accesses (repeated message)
			b, consumed, err := setcodeReadLengthDelim(data, idx, wireType)
			if err != nil {
				return err
			}
			var at AccessTuple
			if err := at.Unmarshal(b); err != nil {
				return errorsmod.Wrap(err, "decode access tuple")
			}
			tx.Accesses = append(tx.Accesses, at)
			idx = consumed
		case 11: // v
			b, consumed, err := setcodeReadLengthDelim(data, idx, wireType)
			if err != nil {
				return err
			}
			tx.V = append([]byte(nil), b...)
			idx = consumed
		case 12: // r
			b, consumed, err := setcodeReadLengthDelim(data, idx, wireType)
			if err != nil {
				return err
			}
			tx.R = append([]byte(nil), b...)
			idx = consumed
		case 13: // s
			b, consumed, err := setcodeReadLengthDelim(data, idx, wireType)
			if err != nil {
				return err
			}
			tx.S = append([]byte(nil), b...)
			idx = consumed
		case 14: // auth_list (repeated message)
			b, consumed, err := setcodeReadLengthDelim(data, idx, wireType)
			if err != nil {
				return err
			}
			var auth SetCodeAuthorization
			if err := auth.Unmarshal(b); err != nil {
				return errorsmod.Wrap(err, "decode authorization")
			}
			tx.AuthList = append(tx.AuthList, auth)
			idx = consumed
		default:
			// Skip unknown field per proto3 forward-compat rules.
			consumed, err := setcodeSkipField(data, idx, wireType)
			if err != nil {
				return err
			}
			idx = consumed
		}
	}
	return nil
}

// ─── Wire format: SetCodeAuthorization ──────────────────────────────────────

func (a *SetCodeAuthorization) Marshal() ([]byte, error) {
	size := a.Size()
	data := make([]byte, size)
	n, err := a.MarshalToSizedBuffer(data)
	if err != nil {
		return nil, err
	}
	return data[:n], nil
}

func (a *SetCodeAuthorization) MarshalTo(data []byte) (int, error) {
	return a.MarshalToSizedBuffer(data[:a.Size()])
}

func (a *SetCodeAuthorization) MarshalToSizedBuffer(data []byte) (int, error) {
	i := len(data)
	// Field 6: s (bytes, tag = 0x32)
	if err := setcodeMarshalBytesField(data, &i, a.S, 0x32); err != nil {
		return 0, err
	}
	// Field 5: r (bytes, tag = 0x2a)
	if err := setcodeMarshalBytesField(data, &i, a.R, 0x2a); err != nil {
		return 0, err
	}
	// Field 4: v (bytes, tag = 0x22)
	if err := setcodeMarshalBytesField(data, &i, a.V, 0x22); err != nil {
		return 0, err
	}
	// Field 3: nonce (varint, tag = 0x18)
	if a.Nonce != 0 {
		i = protoEncodeSetCodeVarint(data, i, a.Nonce)
		i--
		data[i] = 0x18
	}
	// Field 2: address (string, tag = 0x12)
	if len(a.Address) > 0 {
		i -= len(a.Address)
		copy(data[i:], a.Address)
		i = protoEncodeSetCodeVarint(data, i, uint64(len(a.Address)))
		i--
		data[i] = 0x12
	}
	// Field 1: chain_id (sdkmath.Int, tag = 0x0a)
	if err := setcodeMarshalIntField(data, &i, a.ChainID, 0x0a); err != nil {
		return 0, err
	}
	return len(data) - i, nil
}

func (a *SetCodeAuthorization) Size() int {
	n := 0
	n += setcodeIntFieldSize(a.ChainID, 1)
	if l := len(a.Address); l > 0 {
		n += 1 + l + protoSizeSetCodeVarint(uint64(l))
	}
	if a.Nonce != 0 {
		n += 1 + protoSizeSetCodeVarint(a.Nonce)
	}
	if l := len(a.V); l > 0 {
		n += 1 + l + protoSizeSetCodeVarint(uint64(l))
	}
	if l := len(a.R); l > 0 {
		n += 1 + l + protoSizeSetCodeVarint(uint64(l))
	}
	if l := len(a.S); l > 0 {
		n += 1 + l + protoSizeSetCodeVarint(uint64(l))
	}
	return n
}

func (a *SetCodeAuthorization) Unmarshal(data []byte) error {
	*a = SetCodeAuthorization{}
	l := len(data)
	idx := 0
	for idx < l {
		tag, n, err := protoDecodeSetCodeVarint(data[idx:])
		if err != nil {
			return err
		}
		idx += n
		fieldNum := tag >> 3
		wireType := tag & 0x7
		switch fieldNum {
		case 1:
			val, consumed, err := setcodeReadIntField(data, idx, wireType)
			if err != nil {
				return err
			}
			a.ChainID = val
			idx = consumed
		case 2:
			s, consumed, err := setcodeReadLengthDelim(data, idx, wireType)
			if err != nil {
				return err
			}
			a.Address = string(s)
			idx = consumed
		case 3:
			if wireType != 0 {
				return fmt.Errorf("set_code_authorization: field 3 wire type %d, want varint", wireType)
			}
			v, n, err := protoDecodeSetCodeVarint(data[idx:])
			if err != nil {
				return err
			}
			a.Nonce = v
			idx += n
		case 4:
			b, consumed, err := setcodeReadLengthDelim(data, idx, wireType)
			if err != nil {
				return err
			}
			a.V = append([]byte(nil), b...)
			idx = consumed
		case 5:
			b, consumed, err := setcodeReadLengthDelim(data, idx, wireType)
			if err != nil {
				return err
			}
			a.R = append([]byte(nil), b...)
			idx = consumed
		case 6:
			b, consumed, err := setcodeReadLengthDelim(data, idx, wireType)
			if err != nil {
				return err
			}
			a.S = append([]byte(nil), b...)
			idx = consumed
		default:
			consumed, err := setcodeSkipField(data, idx, wireType)
			if err != nil {
				return err
			}
			idx = consumed
		}
	}
	return nil
}

// ─── gogoproto XXX_* dispatchers (so codectypes.Any roundtrips work) ────────

var xxx_messageInfo_SetCodeTx proto.InternalMessageInfo
var xxx_messageInfo_SetCodeAuthorization proto.InternalMessageInfo

func (m *SetCodeTx) XXX_Unmarshal(b []byte) error { return m.Unmarshal(b) }
func (m *SetCodeTx) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	if deterministic {
		return xxx_messageInfo_SetCodeTx.Marshal(b, m, deterministic)
	}
	b = b[:cap(b)]
	n, err := m.MarshalToSizedBuffer(b)
	if err != nil {
		return nil, err
	}
	return b[:n], nil
}
func (m *SetCodeTx) XXX_Merge(src proto.Message) { xxx_messageInfo_SetCodeTx.Merge(m, src) }
func (m *SetCodeTx) XXX_Size() int               { return m.Size() }
func (m *SetCodeTx) XXX_DiscardUnknown()         { xxx_messageInfo_SetCodeTx.DiscardUnknown(m) }

func (m *SetCodeAuthorization) XXX_Unmarshal(b []byte) error { return m.Unmarshal(b) }
func (m *SetCodeAuthorization) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	if deterministic {
		return xxx_messageInfo_SetCodeAuthorization.Marshal(b, m, deterministic)
	}
	b = b[:cap(b)]
	n, err := m.MarshalToSizedBuffer(b)
	if err != nil {
		return nil, err
	}
	return b[:n], nil
}
func (m *SetCodeAuthorization) XXX_Merge(src proto.Message) {
	xxx_messageInfo_SetCodeAuthorization.Merge(m, src)
}
func (m *SetCodeAuthorization) XXX_Size() int       { return m.Size() }
func (m *SetCodeAuthorization) XXX_DiscardUnknown() { xxx_messageInfo_SetCodeAuthorization.DiscardUnknown(m) }

// ─── Proto type registration ────────────────────────────────────────────────

func init() {
	proto.RegisterType((*SetCodeTx)(nil), "ethermint.evm.v1.SetCodeTx")
	proto.RegisterType((*SetCodeAuthorization)(nil), "ethermint.evm.v1.SetCodeAuthorization")
}

// ─── Wire-format helpers (private, prefixed to avoid collisions) ────────────

func protoEncodeSetCodeVarint(data []byte, offset int, v uint64) int {
	for v >= 0x80 {
		offset--
		data[offset] = byte(v&0x7f | 0x80)
		v >>= 7
	}
	offset--
	data[offset] = byte(v)
	return offset
}

func protoSizeSetCodeVarint(v uint64) int {
	n := 1
	for v >= 0x80 {
		v >>= 7
		n++
	}
	return n
}

func protoDecodeSetCodeVarint(data []byte) (uint64, int, error) {
	var v uint64
	for shift, n := uint(0), 0; ; shift += 7 {
		if n >= len(data) {
			return 0, 0, io.ErrUnexpectedEOF
		}
		b := data[n]
		n++
		v |= uint64(b&0x7F) << shift
		if b < 0x80 {
			return v, n, nil
		}
		if shift >= 63 {
			return 0, 0, errors.New("varint overflow")
		}
	}
}

func setcodeMarshalBytesField(data []byte, i *int, b []byte, tag byte) error {
	if len(b) == 0 {
		return nil
	}
	*i -= len(b)
	copy(data[*i:], b)
	*i = protoEncodeSetCodeVarint(data, *i, uint64(len(b)))
	*i--
	data[*i] = tag
	return nil
}

func setcodeMarshalIntField(data []byte, i *int, v *sdkmath.Int, tag byte) error {
	if v == nil {
		return nil
	}
	bz, err := v.Marshal()
	if err != nil {
		return err
	}
	if len(bz) == 0 {
		return nil
	}
	*i -= len(bz)
	copy(data[*i:], bz)
	*i = protoEncodeSetCodeVarint(data, *i, uint64(len(bz)))
	*i--
	data[*i] = tag
	return nil
}

func setcodeIntFieldSize(v *sdkmath.Int, tagBytes int) int {
	if v == nil {
		return 0
	}
	bz, err := v.Marshal()
	if err != nil || len(bz) == 0 {
		return 0
	}
	return tagBytes + len(bz) + protoSizeSetCodeVarint(uint64(len(bz)))
}

func setcodeReadLengthDelim(data []byte, idx int, wireType uint64) ([]byte, int, error) {
	if wireType != 2 {
		return nil, 0, fmt.Errorf("set_code_tx: expected length-delim, got wire type %d", wireType)
	}
	length, n, err := protoDecodeSetCodeVarint(data[idx:])
	if err != nil {
		return nil, 0, err
	}
	idx += n
	if uint64(idx)+length > uint64(len(data)) {
		return nil, 0, io.ErrUnexpectedEOF
	}
	return data[idx : idx+int(length)], idx + int(length), nil
}

func setcodeReadIntField(data []byte, idx int, wireType uint64) (*sdkmath.Int, int, error) {
	bz, consumed, err := setcodeReadLengthDelim(data, idx, wireType)
	if err != nil {
		return nil, 0, err
	}
	if len(bz) == 0 {
		return nil, consumed, nil
	}
	var v sdkmath.Int
	if err := v.Unmarshal(bz); err != nil {
		return nil, 0, err
	}
	return &v, consumed, nil
}

func setcodeSkipField(data []byte, idx int, wireType uint64) (int, error) {
	switch wireType {
	case 0: // varint
		_, n, err := protoDecodeSetCodeVarint(data[idx:])
		if err != nil {
			return 0, err
		}
		return idx + n, nil
	case 2: // length-delim
		_, consumed, err := setcodeReadLengthDelim(data, idx, wireType)
		return consumed, err
	default:
		return 0, fmt.Errorf("set_code_tx: cannot skip wire type %d", wireType)
	}
}
