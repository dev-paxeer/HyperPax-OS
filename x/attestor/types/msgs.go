// Copyright PaxLabs Ltd.(Paxeer Network)
// Paxeer Network Non-Commercial License 1.0 (ENCL-1.0)(https://github.com/Paxeer-Network/hyperpaxeer-os/blob/main/LICENSE_FAQ.md)


package types

import (
	"encoding/json"
	"fmt"
	"io"

	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
)

const (
	TypeMsgUpdateTEERoots = "update_tee_roots"
)

var _ sdk.Msg = &MsgUpdateTEERoots{}

// MsgUpdateTEERoots replaces the trusted root certificates / public keys for
// a single TEE family. Gov-only — only the module authority may submit this
// message. The roots field is a list of PEM certificates (TDX, SEV-SNP, SGX)
// or DER public keys (NVIDIA), depending on the family. Empty roots argument
// is valid and clears the family's trust set.
type MsgUpdateTEERoots struct {
	Authority string   `protobuf:"bytes,1,opt,name=authority,proto3" json:"authority,omitempty"`
	Family    uint32   `protobuf:"varint,2,opt,name=family,proto3" json:"family,omitempty"`
	Roots     [][]byte `protobuf:"bytes,3,rep,name=roots,proto3" json:"roots,omitempty"`
}

// NewMsgUpdateTEERoots constructs an MsgUpdateTEERoots. `roots` is consumed by
// reference — callers should not retain it after the call.
func NewMsgUpdateTEERoots(authority string, family uint8, roots [][]byte) *MsgUpdateTEERoots {
	return &MsgUpdateTEERoots{
		Authority: authority,
		Family:    uint32(family),
		Roots:     roots,
	}
}

func (msg MsgUpdateTEERoots) Route() string { return RouterKey }
func (msg MsgUpdateTEERoots) Type() string  { return TypeMsgUpdateTEERoots }

// FamilyByte returns the Family field truncated to uint8 (the canonical
// per-family identifier — see types/keys.go).
func (msg MsgUpdateTEERoots) FamilyByte() uint8 { return uint8(msg.Family) }

func (msg MsgUpdateTEERoots) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(msg.Authority); err != nil {
		return sdkerrors.ErrInvalidAddress.Wrapf("invalid authority address: %s", err)
	}
	if msg.Family > uint32(FamilyMax) {
		return ErrUnknownFamily.Wrapf("family=%d", msg.Family)
	}
	for i, r := range msg.Roots {
		if len(r) == 0 {
			return fmt.Errorf("root %d is empty", i)
		}
	}
	return nil
}

func (msg MsgUpdateTEERoots) GetSigners() []sdk.AccAddress {
	addr, _ := sdk.AccAddressFromBech32(msg.Authority)
	return []sdk.AccAddress{addr}
}

// GetSignBytes uses canonical JSON for the legacy amino sign mode. Production
// gov path uses sign-mode-direct with the protobuf encoding below.
func (msg MsgUpdateTEERoots) GetSignBytes() []byte {
	bz, err := json.Marshal(msg)
	if err != nil {
		panic(err)
	}
	return sdk.MustSortJSON(bz)
}

func (*MsgUpdateTEERoots) ProtoMessage()      {}
func (msg *MsgUpdateTEERoots) Reset()         { *msg = MsgUpdateTEERoots{} }
func (msg *MsgUpdateTEERoots) String() string {
	return fmt.Sprintf("MsgUpdateTEERoots{authority:%s family:%d roots:%d}", msg.Authority, msg.Family, len(msg.Roots))
}

// ─── Hand-crafted protobuf wire-format Marshal/Unmarshal ────────────────────
//
// Field numbers + wire types match the proto declaration in the docstring at
// the top of this file. Mirrors the paxoracle MsgSubmitPrice template in
// `x/paxoracle/types/msgs.go`. NEVER reorder fields without bumping a
// migration — wire format is consensus-critical.

func (msg *MsgUpdateTEERoots) Marshal() ([]byte, error) {
	size := msg.Size()
	data := make([]byte, size)
	n, err := msg.MarshalTo(data)
	if err != nil {
		return nil, err
	}
	return data[:n], nil
}

func (msg *MsgUpdateTEERoots) MarshalTo(data []byte) (int, error) {
	return msg.MarshalToSizedBuffer(data[:msg.Size()])
}

func (msg *MsgUpdateTEERoots) MarshalToSizedBuffer(data []byte) (int, error) {
	i := len(data)
	// Field 3: roots (repeated bytes, tag = 0x1a)
	for j := len(msg.Roots) - 1; j >= 0; j-- {
		r := msg.Roots[j]
		i -= len(r)
		copy(data[i:], r)
		i = teeProtoEncodeVarint(data, i, uint64(len(r)))
		i--
		data[i] = 0x1a
	}
	// Field 2: family (varint, tag = 0x10)
	if msg.Family != 0 {
		i = teeProtoEncodeVarint(data, i, uint64(msg.Family))
		i--
		data[i] = 0x10
	}
	// Field 1: authority (string, tag = 0x0a)
	if len(msg.Authority) > 0 {
		i -= len(msg.Authority)
		copy(data[i:], msg.Authority)
		i = teeProtoEncodeVarint(data, i, uint64(len(msg.Authority)))
		i--
		data[i] = 0x0a
	}
	return len(data) - i, nil
}

func (msg *MsgUpdateTEERoots) Size() int {
	n := 0
	if l := len(msg.Authority); l > 0 {
		n += 1 + l + teeProtoSizeVarint(uint64(l))
	}
	if msg.Family != 0 {
		n += 1 + teeProtoSizeVarint(uint64(msg.Family))
	}
	for _, r := range msg.Roots {
		l := len(r)
		n += 1 + l + teeProtoSizeVarint(uint64(l))
	}
	return n
}

func (msg *MsgUpdateTEERoots) Unmarshal(data []byte) error {
	l := len(data)
	idx := 0
	for idx < l {
		tag := data[idx]
		idx++
		fieldNum := tag >> 3
		wireType := tag & 0x7

		switch wireType {
		case 0: // varint
			var v uint64
			for shift := uint(0); ; shift += 7 {
				if idx >= l {
					return io.ErrUnexpectedEOF
				}
				b := data[idx]
				idx++
				v |= uint64(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if fieldNum == 2 {
				msg.Family = uint32(v)
			}
		case 2: // length-delimited
			var length uint64
			for shift := uint(0); ; shift += 7 {
				if idx >= l {
					return io.ErrUnexpectedEOF
				}
				b := data[idx]
				idx++
				length |= uint64(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if idx+int(length) > l {
				return io.ErrUnexpectedEOF
			}
			switch fieldNum {
			case 1:
				msg.Authority = string(data[idx : idx+int(length)])
			case 3:
				root := make([]byte, length)
				copy(root, data[idx:idx+int(length)])
				msg.Roots = append(msg.Roots, root)
			}
			idx += int(length)
		default:
			return fmt.Errorf("unsupported wire type %d for field %d", wireType, fieldNum)
		}
	}
	return nil
}

func teeProtoEncodeVarint(data []byte, offset int, v uint64) int {
	for v >= 0x80 {
		offset--
		data[offset] = byte(v&0x7f | 0x80)
		v >>= 7
	}
	offset--
	data[offset] = byte(v)
	return offset
}

func teeProtoSizeVarint(v uint64) int {
	n := 1
	for v >= 0x80 {
		v >>= 7
		n++
	}
	return n
}
