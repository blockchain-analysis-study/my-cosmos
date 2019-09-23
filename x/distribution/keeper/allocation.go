package keeper

import (
	"fmt"

	abci "github.com/tendermint/tendermint/abci/types"

	sdk "my-cosmos/cosmos-sdk/types"
)

// allocate fees handles distribution of the collected fees
// TODO 计算出块奖励
/*
	TODO
	出块奖励计算依据：
	参与上一个块的block签名且参与了commit投票的验证人的power之和
	参与上一个块的commit投票的验证人的power之和
	上一个块的所有 commit的vote

*/
func (k Keeper) AllocateTokens(ctx sdk.Context, sumPrecommitPower, totalPower int64, proposer sdk.ConsAddress, votes []abci.VoteInfo) {
	logger := ctx.Logger().With("module", "x/distribution")

	// fetch collected fees & fee pool
	//
	// 获取 收集池中的金额(收集池中的金额，主要由，slashing之类或者扎差之类的操作攒来的钱)
	feesCollectedInt := k.feeCollectionKeeper.GetCollectedFees(ctx)
	feesCollected := sdk.NewDecCoins(feesCollectedInt)
	feePool := k.GetFeePool(ctx)

	// clear collected fees, which will now be distributed
	//
	// 清空 积攒的金额
	k.feeCollectionKeeper.ClearCollectedFees(ctx)

	// temporary workaround to keep CanWithdrawInvariant happy
	// general discussions here: https://my-cosmos/cosmos-sdk/issues/2906#issuecomment-441867634
	//
	// 保持Can Withdraw Invariant （可以撤回不变） 的临时解决方法
	// 一般性讨论：https://my-cosmos/cosmos-sdk/issues/2906#issuecomment-441867634
	//
	//
	// TODO 这里：如果叠加的总权重为0，那么 将攒来的钱直接投入 社区奖励池中
	if totalPower == 0 {
		feePool.CommunityPool = feePool.CommunityPool.Add(feesCollected)
		k.SetFeePool(ctx, feePool)
		return
	}

	// calculate fraction votes
	//
	// 计算根据入参的两个 power之和计算出一个 比分
	fractionVotes := sdk.NewDec(sumPrecommitPower).Quo(sdk.NewDec(totalPower))

	// calculate proposer reward
	/* 计算出块人的 奖励 */

	// 获取 进出奖励金额标准
	baseProposerReward := k.GetBaseProposerReward(ctx)
	// 获取奖金基础标准
	bonusProposerReward := k.GetBonusProposerReward(ctx)

	// 计算出一个比率
	proposerMultiplier := baseProposerReward.Add(bonusProposerReward.MulTruncate(fractionVotes))

	// 先从积攒的金额中瓜分一部分 钱
	proposerReward := feesCollected.MulDecTruncate(proposerMultiplier)

	// pay proposer
	remaining := feesCollected
	proposerValidator := k.stakingKeeper.ValidatorByConsAddr(ctx, proposer)
	if proposerValidator != nil {
		//  todo 发配奖励 （出块奖励）
		k.AllocateTokensToValidator (ctx, proposerValidator, proposerReward)
		remaining = remaining.Sub(proposerReward)
	} else {
		// proposer can be unknown if say, the unbonding period is 1 block, so
		// e.g. a validator undelegates at block X, it's removed entirely by
		// block X+1's endblock, then X+2 we need to refer to the previous
		// proposer for X+1, but we've forgotten about them.
		logger.Error(fmt.Sprintf(
			"WARNING: Attempt to allocate proposer rewards to unknown proposer %s. "+
				"This should happen only if the proposer unbonded completely within a single block, "+
				"which generally should not happen except in exceptional circumstances (or fuzz testing). "+
				"We recommend you investigate immediately.",
			proposer.String()))
	}

	/*  下面这个是 签名奖励 ??? */

	// calculate fraction allocated to validators
	//
	// 计算分配给验证人的分数
	communityTax := k.GetCommunityTax(ctx)
	voteMultiplier := sdk.OneDec().Sub(proposerMultiplier).Sub(communityTax)

	// allocate tokens proportionally to voting power
	//
	// TODO 按根据比例分配代币
	//
	// TODO consider parallelizing later, ref https://my-cosmos/cosmos-sdk/pull/3099#discussion_r246276376
	for _, vote := range votes {
		validator := k.stakingKeeper.ValidatorByConsAddr(ctx, vote.Validator.Address)

		// TODO consider microslashing for missing votes.
		// ref https://my-cosmos/cosmos-sdk/issues/2525#issuecomment-430838701
		powerFraction := sdk.NewDec(vote.Validator.Power).QuoTruncate(sdk.NewDec(totalPower))
		reward := feesCollected.MulDecTruncate(voteMultiplier).MulDecTruncate(powerFraction)
		reward = reward.Intersect(remaining)

		// todo 发配奖励 （对commit的签名奖励）
		k.AllocateTokensToValidator(ctx, validator, reward)
		remaining = remaining.Sub(reward)
	}

	// allocate community funding
	//
	// 将分配奖励之后的剩余的积攒的钱 直接投入 社区奖励池
	feePool.CommunityPool = feePool.CommunityPool.Add(remaining)
	k.SetFeePool(ctx, feePool)

}

// allocate tokens to a particular validator, splitting according to commission
// TODO 超级重要
// TODO 超级重要
// TODO 将 token 钱 分配给特定的验证器，根据佣金进行拆分
// TODO 超级重要
// TODO 超级重要
func (k Keeper) AllocateTokensToValidator(ctx sdk.Context, val sdk.Validator, tokens sdk.DecCoins) {

	// split tokens between validator and delegators according to commission
	//
	// 根据佣金在验证者和委托人之间分配 token
	commission := tokens.MulDec(val.GetCommission())  	// token * 佣金比 = X
	shared := tokens.Sub(commission)					// token - X

	// update current commission
	//
	// TODO 累积更新当前 验证人的 积累的佣金
	currentCommission := k.GetValidatorAccumulatedCommission(ctx, val.GetOperator())
	currentCommission = currentCommission.Add(commission)
	k.SetValidatorAccumulatedCommission(ctx, val.GetOperator(), currentCommission)

	// update current rewards
	//
	// TODO 累积更新当前验证人 累积奖励 （token - (token * 佣金比)）
	currentRewards := k.GetValidatorCurrentRewards(ctx, val.GetOperator())
	currentRewards.Rewards = currentRewards.Rewards.Add(shared)
	k.SetValidatorCurrentRewards(ctx, val.GetOperator(), currentRewards)

	// update outstanding rewards
	//
	// TODO 累积更新当前验证人的  累积出块奖励
	outstanding := k.GetValidatorOutstandingRewards(ctx, val.GetOperator())
	outstanding = outstanding.Add(tokens)
	k.SetValidatorOutstandingRewards(ctx, val.GetOperator(), outstanding)
}
