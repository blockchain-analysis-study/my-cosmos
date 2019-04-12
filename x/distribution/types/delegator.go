package types

import (
	sdk "my-cosmos/cosmos-sdk/types"
)

// starting info for a delegator reward period
// tracks the previous validator period, the delegation's amount
// of staking token, and the creation height (to check later on
// if any slashes have occurred)
// NOTE that even though validators are slashed to whole staking tokens, the
// delegators within the validator may be left with less than a full token,
// thus sdk.Dec is used
/***
starting info：
是委托人的奖励期间追溯上一个验证人的期间, 委托人质押的token及发生委托时的高度.(为了检查后续是否有削减发生)

请注意，即使验证人s被削减了所有质押token, 验证人信息中的委托人可能会留下少于完整的标记，因此使用sdk.Dec (说的啥啊？
 */
type DelegatorStartingInfo struct {
	// 即: 委托人应当从这个周期作为减持的起始～
	PreviousPeriod uint64  `json:"previous_period"` // period at which the delegation should withdraw starting from
	// 委托人质押的token数量
	Stake          sdk.Dec `json:"stake"`           // amount of staking token delegated
	// 创建委托时的区块高度
	Height         uint64  `json:"height"`          // height at which delegation was created
}

// create a new DelegatorStartingInfo
func NewDelegatorStartingInfo(previousPeriod uint64, stake sdk.Dec, height uint64) DelegatorStartingInfo {
	return DelegatorStartingInfo{
		PreviousPeriod: previousPeriod,
		Stake:          stake,
		Height:         height,
	}
}
