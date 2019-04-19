package keeper

import (
	sdk "my-cosmos/cosmos-sdk/types"

	"my-cosmos/cosmos-sdk/x/distribution/types"
)

// initialize rewards for a new validator
func (k Keeper) initializeValidator(ctx sdk.Context, val sdk.Validator) {
	// set initial historical rewards (period 0) with reference count of 1
	k.SetValidatorHistoricalRewards(ctx, val.GetOperator(), 0, types.NewValidatorHistoricalRewards(sdk.DecCoins{}, 1))

	// set current rewards (starting at period 1)
	k.SetValidatorCurrentRewards(ctx, val.GetOperator(), types.NewValidatorCurrentRewards(sdk.DecCoins{}, 1))

	// set accumulated commission
	k.SetValidatorAccumulatedCommission(ctx, val.GetOperator(), types.InitialValidatorAccumulatedCommission())

	// set outstanding rewards
	k.SetValidatorOutstandingRewards(ctx, val.GetOperator(), sdk.DecCoins{})
}

// increment validator period, returning the period just ended
// 增量验证人周期，返回刚刚结束的周期
//
// 这里除了处理 验证人的周期之外，还调整了验证人的出块奖励金额等等
func (k Keeper) incrementValidatorPeriod(ctx sdk.Context, val sdk.Validator) uint64 {
	// fetch current rewards
	// 获取验证人当前奖励
	rewards := k.GetValidatorCurrentRewards(ctx, val.GetOperator())

	// calculate current ratio
	// 计算流通比率
	var current sdk.DecCoins

	// 如果当前 验证人被质押的 token 为 0
	// 则，修复要证人该得的钱，转到 社区奖励池中
	if val.GetTokens().IsZero() {

		// can't calculate ratio for zero-token validators
		// ergo we instead add to the community pool
		// 无法计算0 token 的验证人的比率
		// 我们改为添加到社区池
		feePool := k.GetFeePool(ctx)

		// 获取 验证人的优秀奖励 ？？ 这个是什么鬼？ (应该是指： 出块奖励吧？)
		outstanding := k.GetValidatorOutstandingRewards(ctx, val.GetOperator())

		// 将当前生成的奖励追加到 尚未花费的社区资金池
		feePool.CommunityPool = feePool.CommunityPool.Add(rewards.Rewards)

		// 扣减吊验证人的 优秀奖励？ (我的理解是: 优秀奖励是不是指, 本来应该奖励给 验证人的出块奖励??)
		outstanding = outstanding.Sub(rewards.Rewards)

		// 分别更新
		k.SetFeePool(ctx, feePool)
		k.SetValidatorOutstandingRewards(ctx, val.GetOperator(), outstanding)

		// 实例化了一个空的 当时流通coins ？
		current = sdk.DecCoins{}
	} else {
		// note: necessary to truncate so we don't allow withdrawing more rewards than owed
		// 注意: 截断是必要的，所以我们不允许撤回比欠款更多的奖励
		// 直接截掉小数位
		current = rewards.Rewards.QuoDecTruncate(val.GetTokens().ToDec())
	}

	// fetch historical rewards for last period
	// 获取当前 验证人的上一期的历史奖励
	historical := k.GetValidatorHistoricalRewards(ctx, val.GetOperator(), rewards.Period-1).CumulativeRewardRatio

	// decrement reference count
	// 减少参考计数 （我一直想知道 参考计数 是做什么的？）
	k.decrementReferenceCount(ctx, val.GetOperator(), rewards.Period-1)

	// set new historical rewards with reference count of 1
	// 设置参考计数为1的新历史奖励
	// 用以前的历史奖励 + 当前奖励
	// 即：设置新的历史出块奖励
	k.SetValidatorHistoricalRewards(ctx, val.GetOperator(), rewards.Period, types.NewValidatorHistoricalRewards(historical.Add(current), 1))

	// set current rewards, incrementing period by 1
	// 设置当前奖励，递增1
	// 预先设置下一个周期的 奖励, 这里先预置为 空
	k.SetValidatorCurrentRewards(ctx, val.GetOperator(), types.NewValidatorCurrentRewards(sdk.DecCoins{}, rewards.Period+1))

	// 返回当前奖励 周期
	return rewards.Period
}

// increment the reference count for a historical rewards value
// 增加历史奖励值的引用计数
func (k Keeper) incrementReferenceCount(ctx sdk.Context, valAddr sdk.ValAddress, period uint64) {
	historical := k.GetValidatorHistoricalRewards(ctx, valAddr, period)
	if historical.ReferenceCount > 2 {
		panic("reference count should never exceed 2")
	}
	historical.ReferenceCount++
	k.SetValidatorHistoricalRewards(ctx, valAddr, period, historical)
}

// decrement the reference count for a historical rewards value, and delete if zero references remain
// 递减历史奖励值中的引用计数，如果保留零引用则删除
func (k Keeper) decrementReferenceCount(ctx sdk.Context, valAddr sdk.ValAddress, period uint64) {
	// 由存储中获取 当前 验证人指定周期的历史奖励
	historical := k.GetValidatorHistoricalRewards(ctx, valAddr, period)
	if historical.ReferenceCount == 0 {
		panic("cannot set negative reference count")
	}
	// 递减
	historical.ReferenceCount--

	// 如果参考技术 == 0, 则直接删除 历史奖励信息
	if historical.ReferenceCount == 0 {
		k.DeleteValidatorHistoricalReward(ctx, valAddr, period)
	} else {
		// 否则设置 递减之后的 历史奖励信息
		k.SetValidatorHistoricalRewards(ctx, valAddr, period, historical)
	}
}

func (k Keeper) updateValidatorSlashFraction(ctx sdk.Context, valAddr sdk.ValAddress, fraction sdk.Dec) {
	if fraction.GT(sdk.OneDec()) {
		panic("fraction greater than one")
	}
	height := uint64(ctx.BlockHeight())
	currentFraction := sdk.ZeroDec()
	endedPeriod := k.GetValidatorCurrentRewards(ctx, valAddr).Period - 1
	current, found := k.GetValidatorSlashEvent(ctx, valAddr, height)
	if found {
		// there has already been a slash event this height,
		// and we don't need to store more than one,
		// so just update the current slash fraction
		currentFraction = current.Fraction
	} else {
		val := k.stakingKeeper.Validator(ctx, valAddr)
		// increment current period
		endedPeriod = k.incrementValidatorPeriod(ctx, val)
		// increment reference count on period we need to track
		k.incrementReferenceCount(ctx, valAddr, endedPeriod)
	}
	currentMultiplicand := sdk.OneDec().Sub(currentFraction)
	newMultiplicand := sdk.OneDec().Sub(fraction)
	updatedFraction := sdk.OneDec().Sub(currentMultiplicand.Mul(newMultiplicand))
	if updatedFraction.LT(sdk.ZeroDec()) {
		panic("negative slash fraction")
	}
	k.SetValidatorSlashEvent(ctx, valAddr, height, types.NewValidatorSlashEvent(endedPeriod, updatedFraction))
}
