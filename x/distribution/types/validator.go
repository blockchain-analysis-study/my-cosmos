package types

import (
	"fmt"
	"strings"

	sdk "my-cosmos/cosmos-sdk/types"
)

// historical rewards for a validator
// height is implicit within the store key
// cumulative reward ratio is the sum from the zeroeth period
// until this period of rewards / tokens, per the spec
// The reference count indicates the number of objects
// which might need to reference this historical entry
// at any point.
// ReferenceCount =
//    number of outstanding delegations which ended the associated period (and might need to read that record)
//  + number of slashes which ended the associated period (and might need to read that record)
//  + one per validator for the zeroeth period, set on initialization
/**

验证人的历史奖励
height 是隐含在 存储key中
累积奖励比率是零周期到当前(奖励/token)周期的总和
参考计数表明，可能需要在任何点引用此历史条目的对象数。

参考计数 = 在期间结束后的优秀委托人数量(中选委托人？)（可能需要阅读该记录） + 在期间结束后的削减数量 (什么是削减啊? 都削减些什么啊?) + 在零周期的每个验证器, 即在初始化时设置

 */
type ValidatorHistoricalRewards struct {
	// 验证人的累积奖励
	CumulativeRewardRatio sdk.DecCoins `json:"cumulative_reward_ratio"`
	// 参考计数 (做什么的？？)
	ReferenceCount        uint16       `json:"reference_count"`
}

// create a new ValidatorHistoricalRewards
func NewValidatorHistoricalRewards(cumulativeRewardRatio sdk.DecCoins, referenceCount uint16) ValidatorHistoricalRewards {
	return ValidatorHistoricalRewards{
		CumulativeRewardRatio: cumulativeRewardRatio,
		ReferenceCount:        referenceCount,
	}
}

// current rewards and current period for a validator
// kept as a running counter and incremented each block
// as long as the validator's tokens remain constant
/**
只要验证人的token保持不变, 验证人的当前奖励和当前周期将在每个递增的块中持续保持不变
 */
type ValidatorCurrentRewards struct {
	// 验证人的当前奖励
	Rewards sdk.DecCoins `json:"rewards"` // current rewards
	// 验证人的当前周期
	Period  uint64       `json:"period"`  // current period
}

// create a new ValidatorCurrentRewards
func NewValidatorCurrentRewards(rewards sdk.DecCoins, period uint64) ValidatorCurrentRewards {
	return ValidatorCurrentRewards{
		Rewards: rewards,
		Period:  period,
	}
}

// accumulated commission for a validator
// kept as a running counter, can be withdrawn at any time
type ValidatorAccumulatedCommission = sdk.DecCoins

// return the initial accumulated commission (zero)
func InitialValidatorAccumulatedCommission() ValidatorAccumulatedCommission {
	return ValidatorAccumulatedCommission{}
}

// validator slash event
// height is implicit within the store key
// needed to calculate appropriate amounts of staking token
// for delegations which withdraw after a slash has occurred
type ValidatorSlashEvent struct {
	ValidatorPeriod uint64  `json:"validator_period"` // period when the slash occurred
	Fraction        sdk.Dec `json:"fraction"`         // slash fraction
}

// create a new ValidatorSlashEvent
func NewValidatorSlashEvent(validatorPeriod uint64, fraction sdk.Dec) ValidatorSlashEvent {
	return ValidatorSlashEvent{
		ValidatorPeriod: validatorPeriod,
		Fraction:        fraction,
	}
}

func (vs ValidatorSlashEvent) String() string {
	return fmt.Sprintf(`Period:   %d
Fraction: %s`, vs.ValidatorPeriod, vs.Fraction)
}

// ValidatorSlashEvents is a collection of ValidatorSlashEvent
type ValidatorSlashEvents []ValidatorSlashEvent

func (vs ValidatorSlashEvents) String() string {
	out := "Validator Slash Events:\n"
	for i, sl := range vs {
		out += fmt.Sprintf(`  Slash %d:
    Period:   %d
    Fraction: %s
`, i, sl.ValidatorPeriod, sl.Fraction)
	}
	return strings.TrimSpace(out)
}

// outstanding (un-withdrawn) rewards for a validator
// inexpensive to track, allows simple sanity checks
type ValidatorOutstandingRewards = sdk.DecCoins
