package slashing

import (
	"my-cosmos/cosmos-sdk/codec"
	sdk "my-cosmos/cosmos-sdk/types"
)

var cdc = codec.New()

// verify interface at compile time
var _ sdk.Msg = &MsgUnjail{}

// MsgUnjail - struct for unjailing jailed validator
// 解禁入狱验证者
type MsgUnjail struct {
	ValidatorAddr sdk.ValAddress `json:"address"` // address of the validator operator
}

func NewMsgUnjail(validatorAddr sdk.ValAddress) MsgUnjail {
	return MsgUnjail{
		ValidatorAddr: validatorAddr,
	}
}

//nolint
func (msg MsgUnjail) Route() string { return RouterKey }
func (msg MsgUnjail) Type() string  { return "unjail" }
func (msg MsgUnjail) GetSigners() []sdk.AccAddress {
	return []sdk.AccAddress{sdk.AccAddress(msg.ValidatorAddr)}
}

// get the bytes for the message signer to sign on
func (msg MsgUnjail) GetSignBytes() []byte {
	bz := cdc.MustMarshalJSON(msg)
	return sdk.MustSortJSON(bz)
}

// quick validity check
func (msg MsgUnjail) ValidateBasic() sdk.Error {
	if msg.ValidatorAddr.Empty() {
		return ErrBadValidatorAddr(DefaultCodespace)
	}
	return nil
}
