package types

import (
	"fmt"
	"time"

	sdk "my-cosmos/cosmos-sdk/types"
)

type (
	// Commission defines a commission parameters for a given validator.
	// 验证人定义的佣金信息
	Commission struct {
		// 表示可以从 委托人身上抽取的佣金比
		Rate          sdk.Dec   `json:"rate"`            // the commission rate charged to delegators, as a fraction

		// 此验证人可以收取的最高佣金率
		MaxRate       sdk.Dec   `json:"max_rate"`        // maximum commission rate which this validator can ever charge, as a fraction

		// 验证人的佣金的每日最大增幅率
		MaxChangeRate sdk.Dec   `json:"max_change_rate"` // maximum daily increase of the validator commission, as a fraction

		// 信息更新时间
		UpdateTime    time.Time `json:"update_time"`     // the last time the commission rate was changed
	}

	// CommissionMsg defines a commission message to be used for creating a
	// validator.
	// CommissionMsg定义用于创建验证人的佣金消息。
	CommissionMsg struct {

		// 向委托人收取的佣金率
		Rate          sdk.Dec `json:"rate"`            // the commission rate charged to delegators, as a fraction

		// 验证人可以收取的最高佣金率
		MaxRate       sdk.Dec `json:"max_rate"`        // maximum commission rate which validator can ever charge, as a fraction

		// 验证人佣金的每日最大增幅
		MaxChangeRate sdk.Dec `json:"max_change_rate"` // maximum daily increase of the validator commission, as a fraction
	}
)

// NewCommissionMsg returns an initialized validator commission message.
func NewCommissionMsg(rate, maxRate, maxChangeRate sdk.Dec) CommissionMsg {
	return CommissionMsg{
		Rate:          rate,
		MaxRate:       maxRate,
		MaxChangeRate: maxChangeRate,
	}
}

// NewCommission returns an initialized validator commission.
func NewCommission(rate, maxRate, maxChangeRate sdk.Dec) Commission {
	return Commission{
		Rate:          rate,
		MaxRate:       maxRate,
		MaxChangeRate: maxChangeRate,
		UpdateTime:    time.Unix(0, 0).UTC(),
	}
}

// NewCommission returns an initialized validator commission with a specified
// update time which should be the current block BFT time.
func NewCommissionWithTime(rate, maxRate, maxChangeRate sdk.Dec, updatedAt time.Time) Commission {
	return Commission{
		Rate:          rate,
		MaxRate:       maxRate,
		MaxChangeRate: maxChangeRate,
		UpdateTime:    updatedAt,
	}
}

// Equal checks if the given Commission object is equal to the receiving
// Commission object.
func (c Commission) Equal(c2 Commission) bool {
	return c.Rate.Equal(c2.Rate) &&
		c.MaxRate.Equal(c2.MaxRate) &&
		c.MaxChangeRate.Equal(c2.MaxChangeRate) &&
		c.UpdateTime.Equal(c2.UpdateTime)
}

// String implements the Stringer interface for a Commission.
func (c Commission) String() string {
	return fmt.Sprintf("rate: %s, maxRate: %s, maxChangeRate: %s, updateTime: %s",
		c.Rate, c.MaxRate, c.MaxChangeRate, c.UpdateTime,
	)
}

// Validate performs basic sanity validation checks of initial commission
// parameters. If validation fails, an SDK error is returned.
func (c Commission) Validate() sdk.Error {
	switch {
	case c.MaxRate.LT(sdk.ZeroDec()):
		// max rate cannot be negative
		return ErrCommissionNegative(DefaultCodespace)

	case c.MaxRate.GT(sdk.OneDec()):
		// max rate cannot be greater than 1
		return ErrCommissionHuge(DefaultCodespace)

	case c.Rate.LT(sdk.ZeroDec()):
		// rate cannot be negative
		return ErrCommissionNegative(DefaultCodespace)

	case c.Rate.GT(c.MaxRate):
		// rate cannot be greater than the max rate
		return ErrCommissionGTMaxRate(DefaultCodespace)

	case c.MaxChangeRate.LT(sdk.ZeroDec()):
		// change rate cannot be negative
		return ErrCommissionChangeRateNegative(DefaultCodespace)

	case c.MaxChangeRate.GT(c.MaxRate):
		// change rate cannot be greater than the max rate
		return ErrCommissionChangeRateGTMaxRate(DefaultCodespace)
	}

	return nil
}

// ValidateNewRate performs basic sanity validation checks of a new commission
// rate. If validation fails, an SDK error is returned.
/**
校验 新设置的佣金比合法性
ValidateNewRate执行新佣金率的基本健全性验证检查。
如果验证失败，则返回错误。
 */
func (c Commission) ValidateNewRate(newRate sdk.Dec, blockTime time.Time) sdk.Error {
	switch {

	// 如果当前更新时间(拿了当前区块时间)和上次更新的时间相差不到 24 小时
	// 则, 不给更新
	case blockTime.Sub(c.UpdateTime).Hours() < 24:
		// new rate cannot be changed more than once within 24 hours
		return ErrCommissionUpdateTime(DefaultCodespace)

	// 	如果新的佣金比是 负数, 则返回失败
	case newRate.LT(sdk.ZeroDec()):
		// new rate cannot be negative
		return ErrCommissionNegative(DefaultCodespace)

	// 如果佣金比大于	验证人最大允许的佣金比, 则返回失败
	case newRate.GT(c.MaxRate):
		// new rate cannot be greater than the max rate
		return ErrCommissionGTMaxRate(DefaultCodespace)

		// TODO: why do we need an absolute value, do we care if the validator decreases their rate rapidly?
	// 如果新的佣金比和就有佣金比的涨跌幅度大于允许的涨跌幅, 则返回失败
	case newRate.Sub(c.Rate).Abs().GT(c.MaxChangeRate):
		// new rate % points change cannot be greater than the max change rate
		return ErrCommissionGTMaxChangeRate(DefaultCodespace)
	}

	return nil
}
