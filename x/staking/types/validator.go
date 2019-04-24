package types

import (
	"bytes"
	"fmt"
	"strings"
	"time"

	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/crypto"
	tmtypes "github.com/tendermint/tendermint/types"

	"my-cosmos/cosmos-sdk/codec"
	sdk "my-cosmos/cosmos-sdk/types"
)

// nolint
const (
	// TODO: Why can't we just have one string description which can be JSON by convention
	MaxMonikerLength  = 70
	MaxIdentityLength = 3000
	MaxWebsiteLength  = 140
	MaxDetailsLength  = 280
)

// Validator defines the total amount of bond shares and their exchange rate to
// coins. Slashing results in a decrease in the exchange rate, allowing correct
// calculation of future undelegations without iterating over delegators.
// When coins are delegated to this validator, the validator is credited with a
// delegation whose number of bond shares is based on the amount of coins delegated
// divided by the current exchange rate. Voting power can be calculated as total
// bonded shares multiplied by exchange rate.
/**
Validator:
定义债券份额的总额及其对硬币的汇率。
削减导致汇率下降，允许正确
计算未来的未取消保护而不迭代委托人。
当硬币被委托给该验证器时，验证器被记入一个代表团，
该代表团的债券份额数基于所委托的硬币数量除以当前汇率。
投票权可以计算为总保税份额乘以汇率。
 */
// 这个就是经济模型的 验证人结构
type Validator struct {
	// 验证人的地址; 以JSON编码的bech
	/**
	Bech32是一种地址格式。 它由BIP173作为SegWit地址引入。
	Bech32由42个符号组成，以bc1开头。
	例如：bc1qa5ndt07z2lu7r2kl6zrffw362chj74vse76lq5
	Bech32地址本身就与SegWit兼容。该地址格式也称为“bc1地址”。

	虽然此地址格式已包含在某些实施中，但截至2017年12月，在更多软件支持该格式之前，建议不要使用地址格式
	 */
	OperatorAddress         sdk.ValAddress `json:"operator_address"`    // address of the validator's operator; bech encoded in JSON

	// 验证者的共识公钥; 以JSON编码的bech
	// 用于做共识时用的公钥
	ConsPubKey              crypto.PubKey  `json:"consensus_pubkey"`    // the consensus public key of the validator; bech encoded in JSON

	// 表示 当前验证人是否属于锁定状态
	// 这个貌似和 slash (惩罚机制) 相关的 (入狱？)
	Jailed                  bool           `json:"jailed"`              // has the validator been jailed from bonded status?

	// 验证器状态（被绑定/解除绑定/未被绑定） [这个主要是 解质押的时候 锁定 三周 用？]
	Status                  sdk.BondStatus `json:"status"`              // validator status (bonded/unbonding/unbonded)

	// 委托代币（包括自我授权）
	Tokens                  sdk.Int        `json:"tokens"`              // delegated tokens (incl. self-delegation)

	// 发给验证人的委托人发出的总委托的 占比？
	DelegatorShares         sdk.Dec        `json:"delegator_shares"`    // total shares issued to a validator's delegators

	// 当前 验证人的一些描述信息
	Description             Description    `json:"description"`         // description terms for the validator

	// 如果解开锁定，则此高度为该验证人解开锁定时的高度。
	UnbondingHeight         int64          `json:"unbonding_height"`    // if unbonding, height at which this validator has begun unbonding

	// 如果解开锁定，验证器完成解开锁定动作的最短时间
	UnbondingCompletionTime time.Time      `json:"unbonding_time"`      // if unbonding, min time for the validator to complete unbonding

	// 佣金参数 (佣金比例？)
	Commission              Commission     `json:"commission"`          // commission parameters

	// 验证人自声明的最小自委托门槛
	MinSelfDelegation       sdk.Int        `json:"min_self_delegation"` // validator's self declared minimum self delegation
}

// Validators is a collection of Validator
/**
验证人列表
 */
type Validators []Validator

func (v Validators) String() (out string) {
	for _, val := range v {
		out += val.String() + "\n"
	}
	return strings.TrimSpace(out)
}

// ToSDKValidators -  convenience function convert []Validators to []sdk.Validators
// ToSDKValidators  - 方便函数将[] Validators转换为[] sdk.Validators
// 哎，不就是 struct 转成 interface{} 么
func (v Validators) ToSDKValidators() (validators []sdk.Validator) {
	for _, val := range v {
		validators = append(validators, val)
	}
	return validators
}

// NewValidator - initialize a new validator
// 创建一个 验证人
func NewValidator(operator sdk.ValAddress, pubKey crypto.PubKey, description Description) Validator {
	return Validator{
		OperatorAddress:         operator,
		ConsPubKey:              pubKey,
		Jailed:                  false,

		// 初始值为, 未被锁定
		Status:                  sdk.Unbonded,
		Tokens:                  sdk.ZeroInt(),
		DelegatorShares:         sdk.ZeroDec(),
		Description:             description,
		UnbondingHeight:         int64(0),
		UnbondingCompletionTime: time.Unix(0, 0).UTC(),
		Commission:              NewCommission(sdk.ZeroDec(), sdk.ZeroDec(), sdk.ZeroDec()),
		MinSelfDelegation:       sdk.OneInt(),
	}
}

// return the redelegation
func MustMarshalValidator(cdc *codec.Codec, validator Validator) []byte {
	return cdc.MustMarshalBinaryLengthPrefixed(validator)
}

// unmarshal a redelegation from a store value
func MustUnmarshalValidator(cdc *codec.Codec, value []byte) Validator {
	validator, err := UnmarshalValidator(cdc, value)
	if err != nil {
		panic(err)
	}
	return validator
}

// unmarshal a redelegation from a store value
func UnmarshalValidator(cdc *codec.Codec, value []byte) (validator Validator, err error) {
	err = cdc.UnmarshalBinaryLengthPrefixed(value, &validator)
	return validator, err
}

// String returns a human readable string representation of a validator.
// 返回 验证人字符串形式
func (v Validator) String() string {
	// 解析出string类型的 公钥
	bechConsPubKey, err := sdk.Bech32ifyConsPub(v.ConsPubKey)
	if err != nil {
		panic(err)
	}

	// 直接按照自定义规则返回格式化string
	return fmt.Sprintf(`Validator
  Operator Address:           %s
  Validator Consensus Pubkey: %s
  Jailed:                     %v
  Status:                     %s
  Tokens:                     %s
  Delegator Shares:           %s
  Description:                %s
  Unbonding Height:           %d
  Unbonding Completion Time:  %v
  Minimum Self Delegation:    %v
  Commission:                 %s`, v.OperatorAddress, bechConsPubKey,
		v.Jailed, sdk.BondStatusToString(v.Status), v.Tokens,
		v.DelegatorShares, v.Description,
		v.UnbondingHeight, v.UnbondingCompletionTime, v.MinSelfDelegation, v.Commission)
}

// this is a helper struct used for JSON de- and encoding only
type bechValidator struct {
	OperatorAddress         sdk.ValAddress `json:"operator_address"`    // the bech32 address of the validator's operator
	ConsPubKey              string         `json:"consensus_pubkey"`    // the bech32 consensus public key of the validator
	Jailed                  bool           `json:"jailed"`              // has the validator been jailed from bonded status?
	Status                  sdk.BondStatus `json:"status"`              // validator status (bonded/unbonding/unbonded)
	Tokens                  sdk.Int        `json:"tokens"`              // delegated tokens (incl. self-delegation)
	DelegatorShares         sdk.Dec        `json:"delegator_shares"`    // total shares issued to a validator's delegators
	Description             Description    `json:"description"`         // description terms for the validator
	UnbondingHeight         int64          `json:"unbonding_height"`    // if unbonding, height at which this validator has begun unbonding
	UnbondingCompletionTime time.Time      `json:"unbonding_time"`      // if unbonding, min time for the validator to complete unbonding
	Commission              Commission     `json:"commission"`          // commission parameters
	MinSelfDelegation       sdk.Int        `json:"min_self_delegation"` // minimum self delegation
}

// MarshalJSON marshals the validator to JSON using Bech32
func (v Validator) MarshalJSON() ([]byte, error) {
	bechConsPubKey, err := sdk.Bech32ifyConsPub(v.ConsPubKey)
	if err != nil {
		return nil, err
	}

	return codec.Cdc.MarshalJSON(bechValidator{
		OperatorAddress:         v.OperatorAddress,
		ConsPubKey:              bechConsPubKey,
		Jailed:                  v.Jailed,
		Status:                  v.Status,
		Tokens:                  v.Tokens,
		DelegatorShares:         v.DelegatorShares,
		Description:             v.Description,
		UnbondingHeight:         v.UnbondingHeight,
		UnbondingCompletionTime: v.UnbondingCompletionTime,
		MinSelfDelegation:       v.MinSelfDelegation,
		Commission:              v.Commission,
	})
}

// UnmarshalJSON unmarshals the validator from JSON using Bech32
func (v *Validator) UnmarshalJSON(data []byte) error {
	bv := &bechValidator{}
	if err := codec.Cdc.UnmarshalJSON(data, bv); err != nil {
		return err
	}
	consPubKey, err := sdk.GetConsPubKeyBech32(bv.ConsPubKey)
	if err != nil {
		return err
	}
	*v = Validator{
		OperatorAddress:         bv.OperatorAddress,
		ConsPubKey:              consPubKey,
		Jailed:                  bv.Jailed,
		Tokens:                  bv.Tokens,
		Status:                  bv.Status,
		DelegatorShares:         bv.DelegatorShares,
		Description:             bv.Description,
		UnbondingHeight:         bv.UnbondingHeight,
		UnbondingCompletionTime: bv.UnbondingCompletionTime,
		Commission:              bv.Commission,
		MinSelfDelegation:       bv.MinSelfDelegation,
	}
	return nil
}

// only the vitals
func (v Validator) TestEquivalent(v2 Validator) bool {
	return v.ConsPubKey.Equals(v2.ConsPubKey) &&
		bytes.Equal(v.OperatorAddress, v2.OperatorAddress) &&
		v.Status.Equal(v2.Status) &&
		v.Tokens.Equal(v2.Tokens) &&
		v.DelegatorShares.Equal(v2.DelegatorShares) &&
		v.Description == v2.Description &&
		v.Commission.Equal(v2.Commission)
}

// return the TM validator address
func (v Validator) ConsAddress() sdk.ConsAddress {
	return sdk.ConsAddress(v.ConsPubKey.Address())
}

// constant used in flags to indicate that description field should not be updated
const DoNotModifyDesc = "[do-not-modify]"

// Description - description fields for a validator
/**
验证人的描述信息
验证人的字段
 */
type Description struct {
	// 验证人的名称
	Moniker  string `json:"moniker"`  // name
	// 可选的身份签名（例如UPort或Keybase）
	Identity string `json:"identity"` // optional identity signature (ex. UPort or Keybase)
	// 可选的网站链接（验证人的主页？）
	Website  string `json:"website"`  // optional website link
	// 一些描述信息
	Details  string `json:"details"`  // optional details
}

// NewDescription returns a new Description with the provided values.
func NewDescription(moniker, identity, website, details string) Description {
	return Description{
		Moniker:  moniker,
		Identity: identity,
		Website:  website,
		Details:  details,
	}
}

// UpdateDescription updates the fields of a given description. An error is
// returned if the resulting description contains an invalid length.
func (d Description) UpdateDescription(d2 Description) (Description, sdk.Error) {
	if d2.Moniker == DoNotModifyDesc {
		d2.Moniker = d.Moniker
	}
	if d2.Identity == DoNotModifyDesc {
		d2.Identity = d.Identity
	}
	if d2.Website == DoNotModifyDesc {
		d2.Website = d.Website
	}
	if d2.Details == DoNotModifyDesc {
		d2.Details = d.Details
	}

	return Description{
		Moniker:  d2.Moniker,
		Identity: d2.Identity,
		Website:  d2.Website,
		Details:  d2.Details,
	}.EnsureLength()
}

// EnsureLength ensures the length of a validator's description.
func (d Description) EnsureLength() (Description, sdk.Error) {
	if len(d.Moniker) > MaxMonikerLength {
		return d, ErrDescriptionLength(DefaultCodespace, "moniker", len(d.Moniker), MaxMonikerLength)
	}
	if len(d.Identity) > MaxIdentityLength {
		return d, ErrDescriptionLength(DefaultCodespace, "identity", len(d.Identity), MaxIdentityLength)
	}
	if len(d.Website) > MaxWebsiteLength {
		return d, ErrDescriptionLength(DefaultCodespace, "website", len(d.Website), MaxWebsiteLength)
	}
	if len(d.Details) > MaxDetailsLength {
		return d, ErrDescriptionLength(DefaultCodespace, "details", len(d.Details), MaxDetailsLength)
	}

	return d, nil
}

// ABCIValidatorUpdate returns an abci.ValidatorUpdate from a staking validator type
// with the full validator power
/**
ABCIValidatorUpdate:
从 staking的完整 validator类型返回abci.ValidatorUpdate
 */
func (v Validator) ABCIValidatorUpdate() abci.ValidatorUpdate {
	return abci.ValidatorUpdate{
		PubKey: tmtypes.TM2PB.PubKey(v.ConsPubKey),
		Power:  v.TendermintPower(),
	}
}

// ABCIValidatorUpdateZero returns an abci.ValidatorUpdate from a staking validator type
// with zero power used for validator updates.
func (v Validator) ABCIValidatorUpdateZero() abci.ValidatorUpdate {
	return abci.ValidatorUpdate{
		PubKey: tmtypes.TM2PB.PubKey(v.ConsPubKey),
		Power:  0,
	}
}

// UpdateStatus updates the location of the shares within a validator
// to reflect the new status
func (v Validator) UpdateStatus(pool Pool, NewStatus sdk.BondStatus) (Validator, Pool) {

	switch v.Status {
	case sdk.Unbonded:

		switch NewStatus {
		case sdk.Unbonded:
			return v, pool
		case sdk.Bonded:
			pool = pool.notBondedTokensToBonded(v.Tokens)
		}
	case sdk.Unbonding:

		switch NewStatus {
		case sdk.Unbonding:
			return v, pool
		case sdk.Bonded:
			pool = pool.notBondedTokensToBonded(v.Tokens)
		}
	case sdk.Bonded:

		switch NewStatus {
		case sdk.Bonded:
			return v, pool
		default:
			pool = pool.bondedTokensToNotBonded(v.Tokens)
		}
	}

	v.Status = NewStatus
	return v, pool
}

// removes tokens from a validator
func (v Validator) RemoveTokens(pool Pool, tokens sdk.Int) (Validator, Pool) {
	if tokens.IsNegative() {
		panic(fmt.Sprintf("should not happen: trying to remove negative tokens %v", tokens))
	}
	if v.Tokens.LT(tokens) {
		panic(fmt.Sprintf("should not happen: only have %v tokens, trying to remove %v", v.Tokens, tokens))
	}
	v.Tokens = v.Tokens.Sub(tokens)
	// TODO: It is not obvious from the name of the function that this will happen. Either justify or move outside.
	if v.Status == sdk.Bonded {
		pool = pool.bondedTokensToNotBonded(tokens)
	}
	return v, pool
}

// SetInitialCommission attempts to set a validator's initial commission. An
// error is returned if the commission is invalid.
func (v Validator) SetInitialCommission(commission Commission) (Validator, sdk.Error) {
	if err := commission.Validate(); err != nil {
		return v, err
	}

	v.Commission = commission
	return v, nil
}

// AddTokensFromDel adds tokens to a validator
// CONTRACT: Tokens are assumed to have come from not-bonded pool.
/**
AddTokensFromDel将币添加到验证人
合约：假设代币来自非绑定池。

钱被追加到token字段
钱的占比被追加到DelegatorShares字段
 */
func (v Validator) AddTokensFromDel(pool Pool, amount sdk.Int) (Validator, Pool, sdk.Dec) {

	// calculate the shares to issue
	// 计算发行 币？？
	var issuedShares sdk.Dec
	if v.DelegatorShares.IsZero() {
		// the first delegation to a validator sets the exchange rate to one
		// 这个是第一次被委托的时候，则当前验证人的被委托金额占比会直接去设置这个数. (精度是： 10^18)
		issuedShares = amount.ToDec()
	} else {
		// 计算当前 委托的token占有 验证人身上所有token的占比 ： 总占比 × 当前委托的token / 总的被委托的token == 当前委托的占比
		issuedShares = v.DelegatorShares.MulInt(amount).QuoInt(v.Tokens)
	}

	// 如果当前 验证人被锁定了
	if v.Status == sdk.Bonded {
		pool = pool.notBondedTokensToBonded(amount)
	}

	// 在该验证人的委托代币字段追加
	v.Tokens = v.Tokens.Add(amount)

	// 在委托人委托的钱除追加
	v.DelegatorShares = v.DelegatorShares.Add(issuedShares)

	return v, pool, issuedShares
}

// RemoveDelShares removes delegator shares from a validator.
// NOTE: because token fractions are left in the valiadator,
//       the exchange rate of future shares of this validator can increase.
// CONTRACT: Tokens are assumed to move to the not-bonded pool.
func (v Validator) RemoveDelShares(pool Pool, delShares sdk.Dec) (Validator, Pool, sdk.Int) {

	remainingShares := v.DelegatorShares.Sub(delShares)
	var issuedTokens sdk.Int
	if remainingShares.IsZero() {

		// last delegation share gets any trimmings
		issuedTokens = v.Tokens
		v.Tokens = sdk.ZeroInt()
	} else {

		// leave excess tokens in the validator
		// however fully use all the delegator shares
		issuedTokens = v.ShareTokens(delShares).TruncateInt()
		v.Tokens = v.Tokens.Sub(issuedTokens)
		if v.Tokens.IsNegative() {
			panic("attempting to remove more tokens than available in validator")
		}
	}

	v.DelegatorShares = remainingShares
	if v.Status == sdk.Bonded {
		pool = pool.bondedTokensToNotBonded(issuedTokens)
	}

	return v, pool, issuedTokens
}

// In some situations, the exchange rate becomes invalid, e.g. if
// Validator loses all tokens due to slashing. In this case,
// make all future delegations invalid.
/**
在某些情况下，每日佣金调整 变得无效 ?
例如 如果Validator由于削减而丢失所有令牌。
在这种情况下，使委托失效。
 */
func (v Validator) InvalidExRate() bool {
	// 如果 验证人身上的 token 为0，且被委托的钱 > 0.
	// 则 每日佣金调整 无效 ?
	return v.Tokens.IsZero() && v.DelegatorShares.IsPositive()
}

// calculate the token worth of provided shares
// 计算入参的数额的代币价值
func (v Validator) ShareTokens(shares sdk.Dec) sdk.Dec {
	// 要减持的股份 × 总质押的钱 / 总委托的股份 == 要减持的钱
	return (shares.MulInt(v.Tokens)).Quo(v.DelegatorShares)
}

// calculate the token worth of provided shares, truncated
func (v Validator) ShareTokensTruncated(shares sdk.Dec) sdk.Dec {
	return (shares.MulInt(v.Tokens)).QuoTruncate(v.DelegatorShares)
}

// get the bonded tokens which the validator holds
func (v Validator) BondedTokens() sdk.Int {
	if v.Status == sdk.Bonded {
		return v.Tokens
	}
	return sdk.ZeroInt()
}

// get the Tendermint Power
// a reduction of 10^6 from validator tokens is applied
func (v Validator) TendermintPower() int64 {

	// 如果该验证人处于 锁定期间则
	// 根据验证人身上的token数量计算 该验证人的Tendermint 权重值
	if v.Status == sdk.Bonded {
		return v.PotentialTendermintPower()
	}
	return 0
}

// potential Tendermint power
// 根据验证人身上的token数量计算 该验证人的Tendermint 权重值
func (v Validator) PotentialTendermintPower() int64 {
	// 根据验证人身上的token数量计算 该验证人的Tendermint 权重值
	return sdk.TokensToTendermintPower(v.Tokens)
}

// ensure fulfills the sdk validator types
var _ sdk.Validator = Validator{}

// nolint - for sdk.Validator
func (v Validator) GetJailed() bool               { return v.Jailed }
func (v Validator) GetMoniker() string            { return v.Description.Moniker }
func (v Validator) GetStatus() sdk.BondStatus     { return v.Status }
func (v Validator) GetOperator() sdk.ValAddress   { return v.OperatorAddress }
func (v Validator) GetConsPubKey() crypto.PubKey  { return v.ConsPubKey }
func (v Validator) GetConsAddr() sdk.ConsAddress  { return sdk.ConsAddress(v.ConsPubKey.Address()) }
func (v Validator) GetTokens() sdk.Int            { return v.Tokens }
func (v Validator) GetBondedTokens() sdk.Int      { return v.BondedTokens() }
func (v Validator) GetTendermintPower() int64     { return v.TendermintPower() }
func (v Validator) GetCommission() sdk.Dec        { return v.Commission.Rate }
func (v Validator) GetMinSelfDelegation() sdk.Int { return v.MinSelfDelegation }
func (v Validator) GetDelegatorShares() sdk.Dec   { return v.DelegatorShares }
