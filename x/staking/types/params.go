package types

import (
	"bytes"
	"fmt"
	"time"

	"my-cosmos/cosmos-sdk/codec"
	sdk "my-cosmos/cosmos-sdk/types"
	"my-cosmos/cosmos-sdk/x/params"
)

const (
	// DefaultUnbondingTime reflects three weeks in seconds as the default
	// unbonding time.
	// TODO: Justify our choice of default here.
	DefaultUnbondingTime time.Duration = time.Second * 60 * 60 * 24 * 3

	// Default maximum number of bonded validators
	DefaultMaxValidators uint16 = 100

	// Default maximum entries in a UBD/RED pair
	DefaultMaxEntries uint16 = 7
)

// nolint - Keys for parameter access
// 参数访问的键
var (
	// 各个键的类型
	KeyUnbondingTime = []byte("UnbondingTime")
	KeyMaxValidators = []byte("MaxValidators")
	KeyMaxEntries    = []byte("KeyMaxEntries")
	KeyBondDenom     = []byte("BondDenom")
)

/**
TODO 这个是干嘛用的啊？
 */
var _ params.ParamSet = (*Params)(nil)

// Params defines the high level settings for staking
// Params定义了经济模型的高阶配置
type Params struct {

	// 解锁时长
	UnbondingTime time.Duration `json:"unbonding_time"` // time duration of unbonding

	// 最大验证人数量（max uint16 = 65535）
	MaxValidators uint16        `json:"max_validators"` // maximum number of validators (max uint16 = 65535)

	// 无绑定委托或重新委托的最大条目（单个 一对/三重奏）
	MaxEntries    uint16        `json:"max_entries"`    // max entries for either unbonding delegation or redelegation (per pair/trio)
	// note: we need to be a bit careful about potential overflow here, since this is user-determined
	// 注意：我们需要对这里的潜在溢出有点小心，因为这是由用户决定的

	// 质押的币面额
	BondDenom string `json:"bond_denom"` // bondable coin denomination
}

func NewParams(unbondingTime time.Duration, maxValidators, maxEntries uint16,
	bondDenom string) Params {

	return Params{
		UnbondingTime: unbondingTime,
		MaxValidators: maxValidators,
		MaxEntries:    maxEntries,
		BondDenom:     bondDenom,
	}
}

// Implements params.ParamSet
func (p *Params) ParamSetPairs() params.ParamSetPairs {
	return params.ParamSetPairs{
		{KeyUnbondingTime, &p.UnbondingTime},
		{KeyMaxValidators, &p.MaxValidators},
		{KeyMaxEntries, &p.MaxEntries},
		{KeyBondDenom, &p.BondDenom},
	}
}

// Equal returns a boolean determining if two Param types are identical.
// TODO: This is slower than comparing struct fields directly
func (p Params) Equal(p2 Params) bool {
	bz1 := MsgCdc.MustMarshalBinaryLengthPrefixed(&p)
	bz2 := MsgCdc.MustMarshalBinaryLengthPrefixed(&p2)
	return bytes.Equal(bz1, bz2)
}

// DefaultParams returns a default set of parameters.
func DefaultParams() Params {
	return NewParams(DefaultUnbondingTime, DefaultMaxValidators, DefaultMaxEntries, sdk.DefaultBondDenom)
}

// String returns a human readable string representation of the parameters.
func (p Params) String() string {
	return fmt.Sprintf(`Params:
  Unbonding Time:    %s
  Max Validators:    %d
  Max Entries:       %d
  Bonded Coin Denom: %s`, p.UnbondingTime,
		p.MaxValidators, p.MaxEntries, p.BondDenom)
}

// unmarshal the current staking params value from store key or panic
func MustUnmarshalParams(cdc *codec.Codec, value []byte) Params {
	params, err := UnmarshalParams(cdc, value)
	if err != nil {
		panic(err)
	}
	return params
}

// unmarshal the current staking params value from store key
func UnmarshalParams(cdc *codec.Codec, value []byte) (params Params, err error) {
	err = cdc.UnmarshalBinaryLengthPrefixed(value, &params)
	if err != nil {
		return
	}
	return
}

// validate a set of params
func (p Params) Validate() error {
	if p.BondDenom == "" {
		return fmt.Errorf("staking parameter BondDenom can't be an empty string")
	}
	if p.MaxValidators == 0 {
		return fmt.Errorf("staking parameter MaxValidators must be a positive integer")
	}
	return nil
}
