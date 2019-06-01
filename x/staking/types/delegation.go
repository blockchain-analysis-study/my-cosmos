package types

import (
	"bytes"
	"fmt"
	"strings"
	"time"

	"my-cosmos/cosmos-sdk/codec"
	sdk "my-cosmos/cosmos-sdk/types"
)

// DVPair is struct that just has a delegator-validator pair with no other data.
// It is intended to be used as a marshalable pointer. For example, a DVPair can be used to construct the
// key to getting an UnbondingDelegation from state.
/**
DVPair是一个只有一个【委托者 - 验证者】kv对,而没有其他数据的结构。
它旨在用作可编组指针。 例如，DVPair可用于构建从状态获取UnbondingDelegation的密钥。
 */
type DVPair struct {
	DelegatorAddress sdk.AccAddress
	ValidatorAddress sdk.ValAddress
}

// DVVTriplet is struct that just has a delegator-validator-validator triplet with no other data.
// It is intended to be used as a marshalable pointer. For example, a DVVTriplet can be used to construct the
// key to getting a Redelegation from state.
type DVVTriplet struct {
	DelegatorAddress    sdk.AccAddress
	ValidatorSrcAddress sdk.ValAddress
	ValidatorDstAddress sdk.ValAddress
}

// Delegation represents the bond with tokens held by an account. It is
// owned by one delegator, and is associated with the voting power of one
// validator.
/**
委托实例
 */
type Delegation struct {
	// 委托人地址
	DelegatorAddress sdk.AccAddress `json:"delegator_address"`
	// 被委托的验证人地址
	ValidatorAddress sdk.ValAddress `json:"validator_address"`
	// 委托的金额
	Shares           sdk.Dec        `json:"shares"`
}

// NewDelegation creates a new delegation object
func NewDelegation(delegatorAddr sdk.AccAddress, validatorAddr sdk.ValAddress,
	shares sdk.Dec) Delegation {

	return Delegation{
		DelegatorAddress: delegatorAddr,
		ValidatorAddress: validatorAddr,
		Shares:           shares,
	}
}

// return the delegation
func MustMarshalDelegation(cdc *codec.Codec, delegation Delegation) []byte {
	return cdc.MustMarshalBinaryLengthPrefixed(delegation)
}

// return the delegation
func MustUnmarshalDelegation(cdc *codec.Codec, value []byte) Delegation {
	delegation, err := UnmarshalDelegation(cdc, value)
	if err != nil {
		panic(err)
	}
	return delegation
}

// return the delegation
func UnmarshalDelegation(cdc *codec.Codec, value []byte) (delegation Delegation, err error) {
	err = cdc.UnmarshalBinaryLengthPrefixed(value, &delegation)
	return delegation, err
}

// nolint
func (d Delegation) Equal(d2 Delegation) bool {
	return bytes.Equal(d.DelegatorAddress, d2.DelegatorAddress) &&
		bytes.Equal(d.ValidatorAddress, d2.ValidatorAddress) &&
		d.Shares.Equal(d2.Shares)
}

// ensure fulfills the sdk validator types
var _ sdk.Delegation = Delegation{}

// nolint - for sdk.Delegation
func (d Delegation) GetDelegatorAddr() sdk.AccAddress { return d.DelegatorAddress }
func (d Delegation) GetValidatorAddr() sdk.ValAddress { return d.ValidatorAddress }
func (d Delegation) GetShares() sdk.Dec               { return d.Shares }

// String returns a human readable string representation of a Delegation.
func (d Delegation) String() string {
	return fmt.Sprintf(`Delegation:
  Delegator: %s
  Validator: %s
  Shares:    %s`, d.DelegatorAddress,
		d.ValidatorAddress, d.Shares)
}

// Delegations is a collection of delegations
type Delegations []Delegation

func (d Delegations) String() (out string) {
	for _, del := range d {
		out += del.String() + "\n"
	}
	return strings.TrimSpace(out)
}

// UnbondingDelegation stores all of a single delegator's unbonding bonds
// for a single validator in an time-ordered list
/**
UnbondingDelegation存储所有单个委托人的无约束债券
某个验证人身上的某个委托人的N次减持信息
 */
type UnbondingDelegation struct {
	// 委托人地址
	DelegatorAddress sdk.AccAddress             `json:"delegator_address"` // delegator
	// 该委托人所解除委托的 验证人地址
	ValidatorAddress sdk.ValAddress             `json:"validator_address"` // validator unbonding from operator addr
	// 所有【减持】委托的条目信息
	Entries          []UnbondingDelegationEntry `json:"entries"`           // unbonding delegation entries
}

// UnbondingDelegationEntry - entry to an UnbondingDelegation
/*
解除委托的条目信息 (减持的条目)
*/
type UnbondingDelegationEntry struct {
	// 解除(减持)委托时的块高
	CreationHeight int64     `json:"creation_height"` // height which the unbonding took place
	// 解除(减持)委托时的区块时间戳
	CompletionTime time.Time `json:"completion_time"` // time at which the unbonding delegation will complete


	/*
	这个是 发起减持时的钱
	*/
	InitialBalance sdk.Int   `json:"initial_balance"` // atoms initially sch eduled to receive at completion
	/*
	这个是如果有 惩罚而被削减的话，就是 减持时的钱 - 被惩罚的钱
	*/
	Balance        sdk.Int   `json:"balance"`         // atoms to receive at completion
}

// IsMature - is the current entry mature
/*
判断 目标时间 是否不在当前时间之后
TODO 等于 或者 在当前时间之前
*/
func (e UnbondingDelegationEntry) IsMature(currentTime time.Time) bool {
	return !e.CompletionTime.After(currentTime)
}

// NewUnbondingDelegation - create a new unbonding delegation object
/*
新建 减持条目的结构体，且追加第一个 减持条目单元
*/
func NewUnbondingDelegation(delegatorAddr sdk.AccAddress,
	validatorAddr sdk.ValAddress, creationHeight int64, minTime time.Time,
	balance sdk.Int) UnbondingDelegation {

	entry := NewUnbondingDelegationEntry(creationHeight, minTime, balance)
	return UnbondingDelegation{
		DelegatorAddress: delegatorAddr,
		ValidatorAddress: validatorAddr,
		Entries:          []UnbondingDelegationEntry{entry},
	}
}

// NewUnbondingDelegation - create a new unbonding delegation object
/*
创建一个 减持条目单元
*/
func NewUnbondingDelegationEntry(creationHeight int64, completionTime time.Time,
	balance sdk.Int) UnbondingDelegationEntry {

	return UnbondingDelegationEntry{
		CreationHeight: creationHeight,
		CompletionTime: completionTime,
		InitialBalance: balance,
		Balance:        balance,
	}
}

// AddEntry - append entry to the unbonding delegation
/*
向该委托的减持条目的结构体中 追加 减持条目单元
*/
func (d *UnbondingDelegation) AddEntry(creationHeight int64,
	minTime time.Time, balance sdk.Int) {

	entry := NewUnbondingDelegationEntry(creationHeight, minTime, balance)
	d.Entries = append(d.Entries, entry)
}

// RemoveEntry - remove entry at index i to the unbonding delegation
func (d *UnbondingDelegation) RemoveEntry(i int64) {
	d.Entries = append(d.Entries[:i], d.Entries[i+1:]...)
}

// return the unbonding delegation
func MustMarshalUBD(cdc *codec.Codec, ubd UnbondingDelegation) []byte {
	return cdc.MustMarshalBinaryLengthPrefixed(ubd)
}

// unmarshal a unbonding delegation from a store value
func MustUnmarshalUBD(cdc *codec.Codec, value []byte) UnbondingDelegation {
	ubd, err := UnmarshalUBD(cdc, value)
	if err != nil {
		panic(err)
	}
	return ubd
}

// unmarshal a unbonding delegation from a store value
func UnmarshalUBD(cdc *codec.Codec, value []byte) (ubd UnbondingDelegation, err error) {
	err = cdc.UnmarshalBinaryLengthPrefixed(value, &ubd)
	return ubd, err
}

// nolint
// inefficient but only used in testing
func (d UnbondingDelegation) Equal(d2 UnbondingDelegation) bool {
	bz1 := MsgCdc.MustMarshalBinaryLengthPrefixed(&d)
	bz2 := MsgCdc.MustMarshalBinaryLengthPrefixed(&d2)
	return bytes.Equal(bz1, bz2)
}

// String returns a human readable string representation of an UnbondingDelegation.
func (d UnbondingDelegation) String() string {
	out := fmt.Sprintf(`Unbonding Delegations between:
  Delegator:                 %s
  Validator:                 %s
	Entries:`, d.DelegatorAddress, d.ValidatorAddress)
	for i, entry := range d.Entries {
		out += fmt.Sprintf(`    Unbonding Delegation %d:
      Creation Height:           %v
      Min time to unbond (unix): %v
      Expected balance:          %s`, i, entry.CreationHeight,
			entry.CompletionTime, entry.Balance)
	}
	return out
}

// UnbondingDelegations is a collection of UnbondingDelegation
type UnbondingDelegations []UnbondingDelegation

func (ubds UnbondingDelegations) String() (out string) {
	for _, u := range ubds {
		out += u.String() + "\n"
	}
	return strings.TrimSpace(out)
}

// Redelegation contains the list of a particular delegator's
// redelegating bonds from a particular source validator to a
// particular destination validator
/*
某个验证人身上的某个委托的重置信息
*/
type Redelegation struct {
	DelegatorAddress    sdk.AccAddress      `json:"delegator_address"`     // delegator
	ValidatorSrcAddress sdk.ValAddress      `json:"validator_src_address"` // validator redelegation source operator addr
	ValidatorDstAddress sdk.ValAddress      `json:"validator_dst_address"` // validator redelegation destination operator addr
	Entries             []RedelegationEntry `json:"entries"`               // redelegation entries
}

// RedelegationEntry - entry to a Redelegation
/*
重置委托 条目单元
*/
type RedelegationEntry struct {
	CreationHeight int64     `json:"creation_height"` // height at which the redelegation took place
	CompletionTime time.Time `json:"completion_time"` // time at which the redelegation will complete
	InitialBalance sdk.Int   `json:"initial_balance"` // initial balance when redelegation started
	SharesDst      sdk.Dec   `json:"shares_dst"`      // amount of destination-validator shares created by redelegation
}

// NewRedelegation - create a new redelegation object
func NewRedelegation(delegatorAddr sdk.AccAddress, validatorSrcAddr,
	validatorDstAddr sdk.ValAddress, creationHeight int64,
	minTime time.Time, balance sdk.Int,
	sharesDst sdk.Dec) Redelegation {

	entry := NewRedelegationEntry(creationHeight,
		minTime, balance, sharesDst)

	return Redelegation{
		DelegatorAddress:    delegatorAddr,
		ValidatorSrcAddress: validatorSrcAddr,
		ValidatorDstAddress: validatorDstAddr,
		Entries:             []RedelegationEntry{entry},
	}
}

// NewRedelegation - create a new redelegation object
func NewRedelegationEntry(creationHeight int64,
	completionTime time.Time, balance sdk.Int,
	sharesDst sdk.Dec) RedelegationEntry {

	return RedelegationEntry{
		CreationHeight: creationHeight,
		CompletionTime: completionTime,
		InitialBalance: balance,
		SharesDst:      sharesDst,
	}
}

// IsMature - is the current entry mature
func (e RedelegationEntry) IsMature(currentTime time.Time) bool {
	return !e.CompletionTime.After(currentTime)
}

// AddEntry - append entry to the unbonding delegation
func (d *Redelegation) AddEntry(creationHeight int64,
	minTime time.Time, balance sdk.Int,
	sharesDst sdk.Dec) {

	entry := NewRedelegationEntry(creationHeight, minTime, balance, sharesDst)
	d.Entries = append(d.Entries, entry)
}

// RemoveEntry - remove entry at index i to the unbonding delegation
func (d *Redelegation) RemoveEntry(i int64) {
	d.Entries = append(d.Entries[:i], d.Entries[i+1:]...)
}

// return the redelegation
func MustMarshalRED(cdc *codec.Codec, red Redelegation) []byte {
	return cdc.MustMarshalBinaryLengthPrefixed(red)
}

// unmarshal a redelegation from a store value
func MustUnmarshalRED(cdc *codec.Codec, value []byte) Redelegation {
	red, err := UnmarshalRED(cdc, value)
	if err != nil {
		panic(err)
	}
	return red
}

// unmarshal a redelegation from a store value
func UnmarshalRED(cdc *codec.Codec, value []byte) (red Redelegation, err error) {
	err = cdc.UnmarshalBinaryLengthPrefixed(value, &red)
	return red, err
}

// nolint
// inefficient but only used in tests
func (d Redelegation) Equal(d2 Redelegation) bool {
	bz1 := MsgCdc.MustMarshalBinaryLengthPrefixed(&d)
	bz2 := MsgCdc.MustMarshalBinaryLengthPrefixed(&d2)
	return bytes.Equal(bz1, bz2)
}

// String returns a human readable string representation of a Redelegation.
func (d Redelegation) String() string {
	out := fmt.Sprintf(`Redelegations between:
  Delegator:                 %s
  Source Validator:          %s
  Destination Validator:     %s
  Entries:`, d.DelegatorAddress, d.ValidatorSrcAddress, d.ValidatorDstAddress)
	for i, entry := range d.Entries {
		out += fmt.Sprintf(`    Redelegation %d:
      Creation height:           %v
      Min time to unbond (unix): %v
      Dest Shares:               %s`, i, entry.CreationHeight,
			entry.CompletionTime, entry.SharesDst)
	}
	return out
}

// Redelegations are a collection of Redelegation
type Redelegations []Redelegation

func (d Redelegations) String() (out string) {
	for _, red := range d {
		out += red.String() + "\n"
	}
	return strings.TrimSpace(out)
}
