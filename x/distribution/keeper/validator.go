package keeper

import (
	sdk "my-cosmos/cosmos-sdk/types"

	"my-cosmos/cosmos-sdk/x/distribution/types"
)

// initialize rewards for a new validator
/**
####### 很重要的一步
初始化新验证者的奖励

1、设置了 历史奖励
2、设置了 当前奖励 （出块/commit的投票奖励 - 佣金
3、设置了 累计佣金
4、设置了 出块/commit的投票奖励

 TODO 奖励的发放在: allocation.go 的  AllocateTokensToValidator（） 中
 */
func (k Keeper) initializeValidator(ctx sdk.Context, val sdk.Validator) {
	// set initial historical rewards (period 0) with reference count of 1
	// 设置初始历史奖励（期间0），引用次数为1 [初始的历史奖励为 0]
	//
	// History 的 period 从0开始记
	k.SetValidatorHistoricalRewards(ctx, val.GetOperator(), 0, types.NewValidatorHistoricalRewards(sdk.DecCoins{}, 1))

	// set current rewards (starting at period 1)
	// 设置当前奖励（从第1期开始） [初始的第一周期奖励为 0]
	// TODO 注意了，  出块奖励  和  block的commit的投票奖励  在cosmos中是，当前快发放上一个块的奖励的哦
	// TODO 这里这个当前奖励，指的是，出块/commit的投票奖励 - 佣金比占有的钱
	k.SetValidatorCurrentRewards(ctx, val.GetOperator(), types.NewValidatorCurrentRewards(sdk.DecCoins{}, 1))

	// set accumulated commission
	// 设定累计佣金 [初始的累积佣金为 0]
	k.SetValidatorAccumulatedCommission(ctx, val.GetOperator(), types.InitialValidatorAccumulatedCommission())

	// set outstanding rewards
	// 设定 出块/commit的投票奖励 [初始的出块奖励为 0]
	k.SetValidatorOutstandingRewards(ctx, val.GetOperator(), sdk.DecCoins{})
}

// increment validator period, returning the period just ended
// 增量验证人周期，返回刚刚结束的周期
//
// 这里除了处理 验证人的周期之外，还调整了验证人的出块奖励金额等等
func (k Keeper) incrementValidatorPeriod(ctx sdk.Context, val sdk.Validator) uint64 {
	// fetch current rewards
	// 获取验证人当前奖励 (出块/commit的投票奖励 - 佣金) (其实是 上一个块的奖励)
	rewards := k.GetValidatorCurrentRewards(ctx, val.GetOperator())

	// calculate current ratio
	// 计算流通比率
	var current sdk.DecCoins

	// 如果当前 验证人被质押的 token 为 0
	// 则 将验证人该得的钱 转到 社区奖励池中
	if val.GetTokens().IsZero() {

		// can't calculate ratio for zero-token validators
		// ergo we instead add to the community pool
		// 无法计算0 token 的验证人的比率
		// 我们改为添加到社区池
		feePool := k.GetFeePool(ctx)

		// 获取 验证人的 出块/commit的投票奖励
		outstanding := k.GetValidatorOutstandingRewards(ctx, val.GetOperator())

		// 将 验证人的当前奖励 转到 社区池中
		feePool.CommunityPool = feePool.CommunityPool.Add(rewards.Rewards)

		// TODO 理论上这个值应该就是 等于 累积佣金的值
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
	// 获取当前 验证人的历史奖励 (用当前周期 - 1)
	historical := k.GetValidatorHistoricalRewards(ctx, val.GetOperator(), rewards.Period-1).CumulativeRewardRatio

	// decrement reference count
	// 减少参考计数 （清算上一个块中的历史奖励标识）
	k.decrementReferenceCount(ctx, val.GetOperator(), rewards.Period-1)

	// set new historical rewards with reference count of 1
	//
	// 设置新的一轮历史奖励
	k.SetValidatorHistoricalRewards(ctx, val.GetOperator(), rewards.Period, types.NewValidatorHistoricalRewards(historical.Add(current), 1))

	// set current rewards, incrementing period by 1
	// 设置当前奖励，递增1
	// 预先设置下一个周期的 出块/commit的投票奖励, 这里先预置为 空 (因为这部分奖励是在 下一个块才会被发放的 查看 allocation.go 的 AllocateTokensToValidator() 方法)
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
	// 递减 (其实这个值最多只会是 1, 能被 -- 那肯定是 1 )
	historical.ReferenceCount--

	// 如果参考技术 == 0, 则直接删除 历史奖励信息
	if historical.ReferenceCount == 0 {
		k.DeleteValidatorHistoricalReward(ctx, valAddr, period)
	} else {
		// 否则设置 递减之后的 历史奖励信息
		k.SetValidatorHistoricalRewards(ctx, valAddr, period, historical)
	}
}


/*
TODO 记录惩罚事件
*/
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

	/*
	TODO 记录惩罚事件
	*/
	k.SetValidatorSlashEvent(ctx, valAddr, height, types.NewValidatorSlashEvent(endedPeriod, updatedFraction))
}
