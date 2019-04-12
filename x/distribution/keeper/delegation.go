package keeper

import (
	"fmt"

	sdk "my-cosmos/cosmos-sdk/types"

	"my-cosmos/cosmos-sdk/x/distribution/types"
)

// initialize starting info for a new delegation
func (k Keeper) initializeDelegation(ctx sdk.Context, val sdk.ValAddress, del sdk.AccAddress) {
	// period has already been incremented - we want to store the period ended by this delegation action
	previousPeriod := k.GetValidatorCurrentRewards(ctx, val).Period - 1

	// increment reference count for the period we're going to track
	k.incrementReferenceCount(ctx, val, previousPeriod)

	validator := k.stakingKeeper.Validator(ctx, val)
	delegation := k.stakingKeeper.Delegation(ctx, del, val)

	// calculate delegation stake in tokens
	// we don't store directly, so multiply delegation shares * (tokens per share)
	// note: necessary to truncate so we don't allow withdrawing more rewards than owed
	stake := validator.ShareTokensTruncated(delegation.GetShares())
	k.SetDelegatorStartingInfo(ctx, val, del, types.NewDelegatorStartingInfo(previousPeriod, stake, uint64(ctx.BlockHeight())))
}

// calculate the rewards accrued by a delegation between two periods
func (k Keeper) calculateDelegationRewardsBetween(ctx sdk.Context, val sdk.Validator,
	startingPeriod, endingPeriod uint64, stake sdk.Dec) (rewards sdk.DecCoins) {
	// sanity check
	if startingPeriod > endingPeriod {
		panic("startingPeriod cannot be greater than endingPeriod")
	}

	// sanity check
	if stake.LT(sdk.ZeroDec()) {
		panic("stake should not be negative")
	}

	// return staking * (ending - starting)
	starting := k.GetValidatorHistoricalRewards(ctx, val.GetOperator(), startingPeriod)
	ending := k.GetValidatorHistoricalRewards(ctx, val.GetOperator(), endingPeriod)
	difference := ending.CumulativeRewardRatio.Sub(starting.CumulativeRewardRatio)
	if difference.IsAnyNegative() {
		panic("negative rewards should not be possible")
	}
	// note: necessary to truncate so we don't allow withdrawing more rewards than owed
	rewards = difference.MulDecTruncate(stake)
	return
}

// calculate the total rewards accrued by a delegation
func (k Keeper) calculateDelegationRewards(ctx sdk.Context, val sdk.Validator, del sdk.Delegation, endingPeriod uint64) (rewards sdk.DecCoins) {
	// fetch starting info for delegation
	startingInfo := k.GetDelegatorStartingInfo(ctx, del.GetValidatorAddr(), del.GetDelegatorAddr())

	if startingInfo.Height == uint64(ctx.BlockHeight()) {
		// started this height, no rewards yet
		return
	}

	startingPeriod := startingInfo.PreviousPeriod
	stake := startingInfo.Stake

	// iterate through slashes and withdraw with calculated staking for sub-intervals
	// these offsets are dependent on *when* slashes happen - namely, in BeginBlock, after rewards are allocated...
	// slashes which happened in the first block would have been before this delegation existed,
	// UNLESS they were slashes of a redelegation to this validator which was itself slashed
	// (from a fault committed by the redelegation source validator) earlier in the same BeginBlock
	startingHeight := startingInfo.Height
	// slashes this block happened after reward allocation, but we have to account for them for the stake sanity check below
	endingHeight := uint64(ctx.BlockHeight())
	if endingHeight > startingHeight {
		k.IterateValidatorSlashEventsBetween(ctx, del.GetValidatorAddr(), startingHeight, endingHeight,
			func(height uint64, event types.ValidatorSlashEvent) (stop bool) {
				endingPeriod := event.ValidatorPeriod
				if endingPeriod > startingPeriod {
					rewards = rewards.Add(k.calculateDelegationRewardsBetween(ctx, val, startingPeriod, endingPeriod, stake))
					// note: necessary to truncate so we don't allow withdrawing more rewards than owed
					stake = stake.MulTruncate(sdk.OneDec().Sub(event.Fraction))
					startingPeriod = endingPeriod
				}
				return false
			},
		)
	}

	// a stake sanity check - recalculated final stake should be less than or equal to current stake
	// here we cannot use Equals because stake is truncated when multiplied by slash fractions
	// we could only use equals if we had arbitrary-precision rationals
	currentStake := val.ShareTokens(del.GetShares())
	if stake.GT(currentStake) {
		panic(fmt.Sprintf("calculated final stake for delegator %s greater than current stake: %s, %s",
			del.GetDelegatorAddr(), stake, currentStake))
	}

	// calculate rewards for final period
	rewards = rewards.Add(k.calculateDelegationRewardsBetween(ctx, val, startingPeriod, endingPeriod, stake))

	return rewards
}

func (k Keeper) withdrawDelegationRewards(ctx sdk.Context, val sdk.Validator, del sdk.Delegation) sdk.Error {

	// check existence of delegator starting info
	// 检查委托人起始信息的存在
	if !k.HasDelegatorStartingInfo(ctx, del.GetValidatorAddr(), del.GetDelegatorAddr()) {
		return types.ErrNoDelegationDistInfo(k.codespace)
	}

	// end current period and calculate rewards
	// 结束当前期间并计算 [验证人]奖励
	//
	// 递增验证人的周期 ? (里面包含了 验证人的奖励 和 参考计数的更新)
	endingPeriod := k.incrementValidatorPeriod(ctx, val)

	// 计算 [委托人]奖励
	rewardsRaw := k.calculateDelegationRewards(ctx, val, del, endingPeriod)

	// 获取当前验证人的 优秀(出块) 奖励
	outstanding := k.GetValidatorOutstandingRewards(ctx, del.GetValidatorAddr())


	// defensive edge case may happen on the very final digits
	// of the decCoins due to operation order of the distribution mechanism.
	// 由于分配机制的操作顺序，防御边缘情况可能发生在decCoins的最终数字上.
	// 用委托人的奖励和当前验证人的奖励, 根据面额选择币 数量小的
	rewards := rewardsRaw.Intersect(outstanding)

	// 如果选择出来的新的 coins数组和 [委托人]的奖励不相等
	if !rewards.IsEqual(rewardsRaw) {
		logger := ctx.Logger().With("module", "x/distr")
		logger.Info(fmt.Sprintf("missing rewards rounding error, delegator %v"+
			"withdrawing rewards from validator %v, should have received %v, got %v",
			val.GetOperator(), del.GetDelegatorAddr(), rewardsRaw, rewards))
	}

	// decrement reference count of starting period
	// 减少起始期间的参考计数
	//
	// 获取与委托人相关的起始信息
	startingInfo := k.GetDelegatorStartingInfo(ctx, del.GetValidatorAddr(), del.GetDelegatorAddr())
	// 获取委托人的 previousPeriod
	startingPeriod := startingInfo.PreviousPeriod
	//  递减参考计数
	k.decrementReferenceCount(ctx, del.GetValidatorAddr(), startingPeriod)

	// truncate coins, return remainder to community pool
	// 截断硬币，将剩余部分归还社区奖励池
	// coins 为被取整之后的币, remainder 为被取整时所切掉的小数的累加
	coins, remainder := rewards.TruncateDecimal()

	// 给当前见证人 追加奖励 (出块奖励 - 委托人的奖励)
	k.SetValidatorOutstandingRewards(ctx, del.GetValidatorAddr(), outstanding.Sub(rewards))

	/**
	将被取整部分的币追加到社区奖励池中
	 */
	feePool := k.GetFeePool(ctx)
	feePool.CommunityPool = feePool.CommunityPool.Add(remainder)
	k.SetFeePool(ctx, feePool)

	// add coins to user account
	if !coins.IsZero() {
		// 获取 坚持质押的(委托人)地址
		withdrawAddr := k.GetDelegatorWithdrawAddr(ctx, del.GetDelegatorAddr())
		//追加设置进去, 并存储起来
		if _, _, err := k.bankKeeper.AddCoins(ctx, withdrawAddr, coins); err != nil {
			return err
		}
	}

	// remove delegator starting info
	// 移除掉当前委托人的 其实委托信息
	k.DeleteDelegatorStartingInfo(ctx, del.GetValidatorAddr(), del.GetDelegatorAddr())

	return nil
}
