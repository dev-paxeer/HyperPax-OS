package types

import (
	"fmt"
	"io"
	"math/big"

	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
)

const (
	TypeMsgSubmitPrice = "submit_price"
)

var _ sdk.Msg = &MsgSubmitPrice{}

// MsgSubmitPrice is submitted by validators to attest a price for a given market.
// Fields use proto-friendly types for proper wire encoding.
type MsgSubmitPrice struct {
	Signer     string `protobuf:"bytes,1,opt,name=signer,proto3" json:"signer,omitempty"`
	MarketId   []byte `protobuf:"bytes,2,opt,name=market_id,json=marketId,proto3" json:"market_id,omitempty"`
	Price      string `protobuf:"bytes,3,opt,name=price,proto3" json:"price,omitempty"`
	Confidence string `protobuf:"bytes,4,opt,name=confidence,proto3" json:"confidence,omitempty"`
}

// NewMsgSubmitPrice creates a new MsgSubmitPrice.
func NewMsgSubmitPrice(signer string, marketId [32]byte, price, confidence *big.Int) *MsgSubmitPrice {
	return &MsgSubmitPrice{
		Signer:     signer,
		MarketId:   marketId[:],
		Price:      price.String(),
		Confidence: confidence.String(),
	}
}

// GetMarketIdArray converts the []byte MarketId to a [32]byte for internal use.
func (msg *MsgSubmitPrice) GetMarketIdArray() [32]byte {
	var id [32]byte
	copy(id[:], msg.MarketId)
	return id
}

// GetPriceBigInt parses the Price string to *big.Int.
func (msg *MsgSubmitPrice) GetPriceBigInt() (*big.Int, bool) {
	return new(big.Int).SetString(msg.Price, 10)
}

// GetConfidenceBigInt parses the Confidence string to *big.Int.
func (msg *MsgSubmitPrice) GetConfidenceBigInt() (*big.Int, bool) {
	return new(big.Int).SetString(msg.Confidence, 10)
}

func (msg MsgSubmitPrice) Route() string { return RouterKey }
func (msg MsgSubmitPrice) Type() string  { return TypeMsgSubmitPrice }

func (msg MsgSubmitPrice) ValidateBasic() error {
	_, err := sdk.AccAddressFromBech32(msg.Signer)
	if err != nil {
		return sdkerrors.ErrInvalidAddress.Wrapf("invalid signer address: %s", err)
	}

	if len(msg.MarketId) != 32 {
		return ErrInvalidMarketId.Wrap("market id must be exactly 32 bytes")
	}
	empty := make([]byte, 32)
	if string(msg.MarketId) == string(empty) {
		return ErrInvalidMarketId.Wrap("market id cannot be zero")
	}

	price, ok := msg.GetPriceBigInt()
	if !ok || price.Sign() <= 0 {
		return ErrInvalidPrice.Wrap("price must be a positive integer string")
	}

	confidence, ok := msg.GetConfidenceBigInt()
	maxConfidence := new(big.Int).SetInt64(DefaultMaxConfidence)
	if !ok || confidence.Sign() <= 0 || confidence.Cmp(maxConfidence) > 0 {
		return ErrInvalidConfidence.Wrapf("confidence must be in (0, %s]", maxConfidence.String())
	}

	return nil
}

func (msg MsgSubmitPrice) GetSigners() []sdk.AccAddress {
	signer, _ := sdk.AccAddressFromBech32(msg.Signer)
	return []sdk.AccAddress{signer}
}

func (msg MsgSubmitPrice) GetSignBytes() []byte {
	return sdk.MustSortJSON(ModuleCdc.MustMarshalJSON(&msg))
}

func (*MsgSubmitPrice) ProtoMessage()      {}
func (msg *MsgSubmitPrice) Reset()         { *msg = MsgSubmitPrice{} }
func (msg *MsgSubmitPrice) String() string { return fmt.Sprintf("MsgSubmitPrice{signer:%s}", msg.Signer) }

// ─── Protobuf wire-format Marshal/Unmarshal ─────────────────────────────────

func (msg *MsgSubmitPrice) Marshal() ([]byte, error) {
	size := msg.Size()
	data := make([]byte, size)
	n, err := msg.MarshalTo(data)
	if err != nil {
		return nil, err
	}
	return data[:n], nil
}

func (msg *MsgSubmitPrice) MarshalTo(data []byte) (int, error) {
	return msg.MarshalToSizedBuffer(data[:msg.Size()])
}

func (msg *MsgSubmitPrice) MarshalToSizedBuffer(data []byte) (int, error) {
	i := len(data)
	// Field 4: confidence (string, tag = 0x22)
	if len(msg.Confidence) > 0 {
		i -= len(msg.Confidence)
		copy(data[i:], msg.Confidence)
		i = protoEncodeVarint(data, i, uint64(len(msg.Confidence)))
		i--
		data[i] = 0x22
	}
	// Field 3: price (string, tag = 0x1a)
	if len(msg.Price) > 0 {
		i -= len(msg.Price)
		copy(data[i:], msg.Price)
		i = protoEncodeVarint(data, i, uint64(len(msg.Price)))
		i--
		data[i] = 0x1a
	}
	// Field 2: market_id (bytes, tag = 0x12)
	if len(msg.MarketId) > 0 {
		i -= len(msg.MarketId)
		copy(data[i:], msg.MarketId)
		i = protoEncodeVarint(data, i, uint64(len(msg.MarketId)))
		i--
		data[i] = 0x12
	}
	// Field 1: signer (string, tag = 0x0a)
	if len(msg.Signer) > 0 {
		i -= len(msg.Signer)
		copy(data[i:], msg.Signer)
		i = protoEncodeVarint(data, i, uint64(len(msg.Signer)))
		i--
		data[i] = 0x0a
	}
	return len(data) - i, nil
}

func (msg *MsgSubmitPrice) Size() int {
	n := 0
	l := len(msg.Signer)
	if l > 0 {
		n += 1 + l + protoSizeVarint(uint64(l))
	}
	l = len(msg.MarketId)
	if l > 0 {
		n += 1 + l + protoSizeVarint(uint64(l))
	}
	l = len(msg.Price)
	if l > 0 {
		n += 1 + l + protoSizeVarint(uint64(l))
	}
	l = len(msg.Confidence)
	if l > 0 {
		n += 1 + l + protoSizeVarint(uint64(l))
	}
	return n
}

func (msg *MsgSubmitPrice) Unmarshal(data []byte) error {
	l := len(data)
	idx := 0
	for idx < l {
		// Read tag
		if idx >= l {
			return io.ErrUnexpectedEOF
		}
		tag := data[idx]
		idx++
		fieldNum := tag >> 3
		wireType := tag & 0x7

		if wireType != 2 { // all fields are length-delimited
			return fmt.Errorf("unexpected wire type %d for field %d", wireType, fieldNum)
		}

		// Read length varint
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
			msg.Signer = string(data[idx : idx+int(length)])
		case 2:
			msg.MarketId = make([]byte, length)
			copy(msg.MarketId, data[idx:idx+int(length)])
		case 3:
			msg.Price = string(data[idx : idx+int(length)])
		case 4:
			msg.Confidence = string(data[idx : idx+int(length)])
		default:
			// skip unknown fields
		}
		idx += int(length)
	}
	return nil
}

// ─── Proto encoding helpers ─────────────────────────────────────────────────

func protoEncodeVarint(data []byte, offset int, v uint64) int {
	for v >= 0x80 {
		offset--
		data[offset] = byte(v&0x7f | 0x80)
		v >>= 7
	}
	offset--
	data[offset] = byte(v)
	return offset
}

func protoSizeVarint(v uint64) int {
	n := 1
	for v >= 0x80 {
		v >>= 7
		n++
	}
	return n
}
