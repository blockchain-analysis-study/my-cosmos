package keeper

import (
	"fmt"

	sdk "my-cosmos/cosmos-sdk/types"
	types "my-cosmos/cosmos-sdk/x/staking/types"
)

// Slash a validator for an infraction committed at a known height
// Find the contributing stake at that height and burn the specified slashFactor
// of it, updating unbonding delegations & redelegations appropriately
//
// CONTRACT:
//    slashFactor is non-negative
// CONTRACT:
//    Infraction was committed equal to or less than an unbonding period in the past,
//    so all unbonding delegations and redelegations from that height are stored
// CONTRACT:
//    Slash will not slash unbonded validators (for the above reason)
// CONTRACT:
//    Infraction was committed at the current height or at a past height,
//    not at a height in the future
/*
TODO 惩罚 实施

TODO 惩罚所扣除的钱是直接扣减 委托人和验证人的钱，但是这部分钱并没有被转到某个还在账户，而是直接减掉(等于 销毁了)
*/
func (k Keeper) Slash(ctx sdk.Context, consAddr sdk.ConsAddress, infractionHeight int64, power int64, slashFactor sdk.Dec) {
	logger := ctx.Logger().With("module", "x/staking")

	if slashFactor.LT(sdk.ZeroDec()) {
		panic(fmt.Errorf("attempted to slash with a negative slash factor: %v", slashFactor))
	}

	// Amount of slashing = slash slashFactor * power at time of infraction
	/*
	计算 惩罚扣减的 金额
	惩罚金额 = 惩罚分数值 * power(发生双签时的power)
	*/
	amount := sdk.TokensFromTendermintPower(power)
	slashAmountDec := amount.ToDec().Mul(slashFactor) // 这个就是 即将被 扣减的钱
	slashAmount := slashAmountDec.TruncateInt() // 扎差取整 TODO (这个是最终要被扣的钱)

	// ref https://my-cosmos/cosmos-sdk/issues/1348

	validator, found := k.GetValidatorByConsAddr(ctx, consAddr)
	if !found {
		// If not found, the validator must have been overslashed and removed - so we don't need to do anything
		// NOTE:  Correctness dependent on invariant that unbonding delegations / redelegations must also have been completely
		//        slashed in this case - which we don't explicitly check, but should be true.
		// Log the slash attempt for future reference (maybe we should tag it too)

		/*
		如果未找到，则必须对验证器进行斜切和删除 - 所以我们不需要做任何事

		在这种情况下，正确性取决于无约束的委托人 / 重新委托也必须完全削减 - 我们没有明确检查，但应该是真的。
		TODO 其实这里只是打印了个日志而已 还没这么做
		*/
		logger.Error(fmt.Sprintf(
			"WARNING: Ignored attempt to slash a nonexistent validator with address %s, we recommend you investigate immediately",
			consAddr))
		return
	}

	// should not be slashing an unbonded validator
	/*
	不应该削减一个没有绑定的验证人
	*/
	if validator.Status == sdk.Unbonded {
		panic(fmt.Sprintf("should not be slashing unbonded validator: %s", validator.GetOperator()))
	}

	/*
	获取 验证人地址
	*/
	operatorAddress := validator.GetOperator()

	// call the before-modification hook TODO 这个 staking 的函数还没被实现
	k.BeforeValidatorModified(ctx, operatorAddress)

	// Track remaining slash amount for the validator
	// This will decrease when we slash unbondings and
	// redelegations, as that stake has since unbonded
	/*
	跟踪验证器的剩余 削减量
	当我们削减无约束和重新授权时，这将减少，因为该股权已经没有约束
	*/
	remainingSlashAmount := slashAmount


	/*
	TODO 这一步先判断块高的合法性

	TODO 再逐个根据总体需要被惩罚扣除的钱 交由 该验证人之前所有的委托人分担惩罚

	TODO 最后剩下的交由 验证人来支付剩余的被惩罚的钱
	*/
	switch {
	/*
	这个是 被惩罚的块的 前一个块 (因为这个块中的奖励是在被惩罚这个块中发放的)
	大于当前最高快， 说明被惩罚的块在当前链上最高快之后， 那么数据是有问题的
	*/
	case infractionHeight > ctx.BlockHeight():

		// Can't slash infractions in the future
		//  TODO 不能惩罚一个来自未来的块
		panic(fmt.Sprintf(
			"impossible attempt to slash future infraction at height %d but we are at height %d",
			infractionHeight, ctx.BlockHeight()))
	/*
	如果需要被回滚出块奖励的块就是当前块，那么其实也是有问题的
	*/
	case infractionHeight == ctx.BlockHeight():

		// Special-case slash at current height for efficiency - we don't need to look through unbonding delegations or redelegations
		/*
		特殊情况：在当前高度 削减 以提高效率 - 我们不需要查看未绑定的授权或重新授权
		*/
		logger.Info(fmt.Sprintf(
			"slashing at current height %d, not scanning unbonding delegations & redelegations",
			infractionHeight))

	/*
	TODO 只有当被削减的块 小于当前块高时，才可以真正的被削减

	TODO 我这里有个疑问， 为什么不 削减 目前这个验证人身上的委托人的钱呢？
	*/
	case infractionHeight < ctx.BlockHeight():

		// Iterate through unbonding delegations from slashed validator
		/*
		通过被削减验证人 迭代 其下属的所有无约束的委托信息
		*/
		unbondingDelegations := k.GetUnbondingDelegationsFromValidator(ctx, operatorAddress)
		for _, unbondingDelegation := range unbondingDelegations {
			/*
			逐个 惩罚 削减 委托人

			TODO 如果在减持动作发生在 被惩罚块高之前，那么这部分委托信息不会被惩罚
			*/
			amountSlashed := k.slashUnbondingDelegation(ctx, unbondingDelegation, infractionHeight, slashFactor)
			if amountSlashed.IsZero() {
				continue
			}
			remainingSlashAmount = remainingSlashAmount.Sub(amountSlashed)
		}

		// Iterate through redelegations from slashed validator
		/*
		通过被削减的验证人  迭代 其下属所有从委托的人 (因为这些人之前委托过该验证人啊)
		*/
		redelegations := k.GetRedelegationsFromValidator(ctx, operatorAddress)
		for _, redelegation := range redelegations {
			/*
			逐个 惩罚削减 重置委托的委托人
			*/
			amountSlashed := k.slashRedelegation(ctx, validator, redelegation, infractionHeight, slashFactor)
			if amountSlashed.IsZero() {
				continue
			}
			remainingSlashAmount = remainingSlashAmount.Sub(amountSlashed)
		}
	}

	// cannot decrease balance below zero
	/*
	不能将 钱 降低到零以下
	扎差
	*/
	tokensToBurn := sdk.MinInt(remainingSlashAmount, validator.Tokens)
	tokensToBurn = sdk.MaxInt(tokensToBurn, sdk.ZeroInt()) // defensive.

	// we need to calculate the *effective* slash fraction for distribution
	/*
	我们需要计算分配的*有效* 削减分数(比例)
	*/
	if validator.Tokens.GT(sdk.ZeroInt()) {
		effectiveFraction := tokensToBurn.ToDec().QuoRoundUp(validator.Tokens.ToDec())
		// possible if power has changed
		if effectiveFraction.GT(sdk.OneDec()) {
			effectiveFraction = sdk.OneDec()
		}
		// call the before-slashed hook  主要做了，记录被惩罚的事件
		k.BeforeValidatorSlashed(ctx, operatorAddress, effectiveFraction)
	}

	// Deduct from validator's bonded tokens and update the validator.
	// The deducted tokens are returned to pool.NotBondedTokens.
	// TODO: Move the token accounting outside of `RemoveValidatorTokens` so it is less confusing
	/*

	TODO 这里才是真正削减验证人的钱
	验证人把剩余的需要 扣减的钱  补完
	*/
	validator = k.RemoveValidatorTokens(ctx, validator, tokensToBurn)
	pool := k.GetPool(ctx)
	// Burn the slashed tokens, which are now loose.
	pool.NotBondedTokens = pool.NotBondedTokens.Sub(tokensToBurn) // 销毁掉 所有撤销的总额
	k.SetPool(ctx, pool)

	// Log that a slash occurred!
	logger.Info(fmt.Sprintf(
		"validator %s slashed by slash factor of %s; burned %v tokens",
		validator.GetOperator(), slashFactor.String(), tokensToBurn))

	// TODO Return event(s), blocked on https://github.com/tendermint/tendermint/pull/1803
	return
}

// jail a validator
/**
惩罚锁定 验证人
 */
func (k Keeper) Jail(ctx sdk.Context, consAddr sdk.ConsAddress) {
	validator := k.mustGetValidatorByConsAddr(ctx, consAddr)
	// 做两件事： 更改验证人详情的 jailed字段为 true； 从权重队列中移除ValidatorId
	k.jailValidator(ctx, validator)
	logger := ctx.Logger().With("module", "x/staking")
	logger.Info(fmt.Sprintf("validator %s jailed", consAddr))
	// TODO Return event(s), blocked on https://github.com/tendermint/tendermint/pull/1803
	return
}

// unjail a validator
/**
解除惩罚锁定
 */
func (k Keeper) Unjail(ctx sdk.Context, consAddr sdk.ConsAddress) {
	validator := k.mustGetValidatorByConsAddr(ctx, consAddr)
	k.unjailValidator(ctx, validator)
	logger := ctx.Logger().With("module", "x/staking")
	logger.Info(fmt.Sprintf("validator %s unjailed", consAddr))
	// TODO Return event(s), blocked on https://github.com/tendermint/tendermint/pull/1803
	return
}

// slash an unbonding delegation and update the pool
// return the amount that would have been slashed assuming
// the unbonding delegation had enough stake to slash
// (the amount actually slashed may be less if there's
// insufficient stake remaining)
/*
削减某个验证人身上的那些委托减持中的钱
*/
func (k Keeper) slashUnbondingDelegation(ctx sdk.Context, unbondingDelegation types.UnbondingDelegation,
	infractionHeight int64, slashFactor sdk.Dec) (totalSlashAmount sdk.Int) {

	/*
	获取当前 block 的时间
	*/
	now := ctx.BlockHeader().Time
	totalSlashAmount = sdk.ZeroInt() // 先声明一个 返回变量

	// perform slashing on all entries within the unbonding delegation
	/*
	对unbonding委托中的所有条目执行削减
	*/
	for i, entry := range unbondingDelegation.Entries {

		// If unbonding started before this height, stake didn't contribute to infraction
		/*
		TODO 如果在减持动作发生在 被惩罚块高之前，那么这部分委托信息不会被惩罚
		*/
		if entry.CreationHeight < infractionHeight {
			continue
		}

		/*
		如果 发生减持的信息在当前时间之前，则(还没有真正的减持??)
		*/
		if entry.IsMature(now) {
			// Unbonding delegation no longer eligible for slashing, skip it
			// 无绑定授权不再有资格进行削减，跳过它
			/*
			TODO 意思是如果当前块需要在 被惩罚块之前 才给做？  沃日 ，这不是自相矛盾么， 看不懂啊
			*/
			continue
		}

		// Calculate slash amount proportional to stake contributing to infraction
		/*
		计算与导致违规的股权成比例的 削减金额
		削减百分比 * 本次减持的钱
		*/
		slashAmountDec := slashFactor.MulInt(entry.InitialBalance)
		slashAmount := slashAmountDec.TruncateInt() // 扎差
		totalSlashAmount = totalSlashAmount.Add(slashAmount) // 将这部分钱 叠加起来

		// Don't slash more tokens than held
		// Possible since the unbonding delegation may already
		// have been slashed, and slash amounts are calculated
		// according to stake held at time of infraction
		/*
		不要削减更多令牌而不是持有可能因为未绑定授权可能已被削减，并且削减金额是根据违规时持有的股权计算的
		*/
		unbondingSlashAmount := sdk.MinInt(slashAmount, entry.Balance)

		// Update unbonding delegation if necessary
		/*
		必要时更新 委托减持信息
		*/
		// 如果是 0 那么 跳过
		if unbondingSlashAmount.IsZero() {
			continue
		}
		/*
		开始 削减减持的钱(更新减持的钱)
		*/
		entry.Balance = entry.Balance.Sub(unbondingSlashAmount)
		unbondingDelegation.Entries[i] = entry
		k.SetUnbondingDelegation(ctx, unbondingDelegation)
		pool := k.GetPool(ctx)

		// Burn not-bonded tokens
		// Ref https://my-cosmos/cosmos-sdk/pull/1278#discussion_r198657760
		pool.NotBondedTokens = pool.NotBondedTokens.Sub(unbondingSlashAmount) // 将总的减持的计数去除掉 被惩罚削减 这部分
		k.SetPool(ctx, pool)
	}

	// TODO 可以看出来，减掉的钱是直接被销毁掉的
	return totalSlashAmount
}

// slash a redelegation and update the pool
// return the amount that would have been slashed assuming
// the unbonding delegation had enough stake to slash
// (the amount actually slashed may be less if there's
// insufficient stake remaining)
// nolint: unparam
/*
削减某个验证人身上的那些重置委托中的委托的钱
*/
func (k Keeper) slashRedelegation(ctx sdk.Context, validator types.Validator, redelegation types.Redelegation,
	infractionHeight int64, slashFactor sdk.Dec) (totalSlashAmount sdk.Int) {

	now := ctx.BlockHeader().Time
	totalSlashAmount = sdk.ZeroInt()

	// perform slashing on all entries within the redelegation
	/*
	对所有重置的委托执行削减
	*/
	for _, entry := range redelegation.Entries {

		// If redelegation started before this height, stake didn't contribute to infraction
		/*
		如果重置委托在被惩罚的块高之前，则跳过
		*/
		if entry.CreationHeight < infractionHeight {
			continue
		}

		/*
		TODO 沃日这个矛盾的意思，完全看不懂啊
		*/
		if entry.IsMature(now) {
			// Redelegation no longer eligible for slashing, skip it
			continue
		}

		// Calculate slash amount proportional to stake contributing to infraction
		/*
		根据削减的百分比，计算出被削减的值
		*/
		slashAmountDec := slashFactor.MulInt(entry.InitialBalance)
		slashAmount := slashAmountDec.TruncateInt() // 扎差
		totalSlashAmount = totalSlashAmount.Add(slashAmount) // 将这部分钱 叠加起来

		// Unbond from target validator
		sharesToUnbond := slashFactor.Mul(entry.SharesDst)
		if sharesToUnbond.IsZero() {
			continue
		}
		delegation, found := k.GetDelegation(ctx, redelegation.DelegatorAddress, redelegation.ValidatorDstAddress)
		if !found {
			// If deleted, delegation has zero shares, and we can't unbond any more
			continue
		}
		if sharesToUnbond.GT(delegation.Shares) {
			sharesToUnbond = delegation.Shares
		}

		/*
		TODO 解除重置委托中的部分钱，  减持委托
		*/
		tokensToBurn, err := k.unbond(ctx, redelegation.DelegatorAddress, redelegation.ValidatorDstAddress, sharesToUnbond)
		if err != nil {
			panic(fmt.Errorf("error unbonding delegator: %v", err))
		}

		// Burn not-bonded tokens
		pool := k.GetPool(ctx)
		pool.NotBondedTokens = pool.NotBondedTokens.Sub(tokensToBurn) // 减持的总数中 销毁掉被削减的这部分钱
		k.SetPool(ctx, pool)
	}

	// 返回 总共被削减的钱
	return totalSlashAmount
}
