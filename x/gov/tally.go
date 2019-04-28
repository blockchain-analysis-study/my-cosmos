package gov

import (
	sdk "my-cosmos/cosmos-sdk/types"
)

// validatorGovInfo used for tallying
// validatorGovInfo用于计算
type validatorGovInfo struct {
	// 验证人地址
	Address             sdk.ValAddress // address of the validator operator
	// 验证人被 质押/委托的tokens
	BondedTokens        sdk.Int        // Power of a Validator
	// 验证人的token的 股票额度
	DelegatorShares     sdk.Dec        // Total outstanding delegator shares
	// 代理人从验证人的委托人中独立投票扣除 ？？ 不懂
	DelegatorDeductions sdk.Dec        // Delegator deductions from validator's delegators voting independently

	// 验证人的治理提案获得的投票状态
	Vote                VoteOption     // Vote of the validator
}

func newValidatorGovInfo(address sdk.ValAddress, bondedTokens sdk.Int, delegatorShares,
	delegatorDeductions sdk.Dec, vote VoteOption) validatorGovInfo {

	return validatorGovInfo{
		Address:             address,
		BondedTokens:        bondedTokens,
		DelegatorShares:     delegatorShares,
		DelegatorDeductions: delegatorDeductions,
		Vote:                vote,
	}
}

// TODO: Break into several smaller functions for clarity
/**
tally：计算

为清晰起见，分成几个较小的功能
 */
func tally(ctx sdk.Context, keeper Keeper, proposal Proposal) (passes bool, tallyResults TallyResult) {

	/**
	返回结果,key（投票状态）-> value （该状态的得票权重）将提案的投票 结果用一个 map缓存
	 */
	results := make(map[VoteOption]sdk.Dec)
	results[OptionYes] = sdk.ZeroDec()
	results[OptionAbstain] = sdk.ZeroDec()
	results[OptionNo] = sdk.ZeroDec()
	results[OptionNoWithVeto] = sdk.ZeroDec()

	// 统计所有提案 投票的数目？
	totalVotingPower := sdk.ZeroDec()
	// 收集计算结果 ？？
	currValidators := make(map[string]validatorGovInfo)

	// fetch all the bonded validators, insert them into currValidators
	// 获取所有绑定的验证器，将它们插入currValidators
	keeper.vs.IterateBondedValidatorsByPower(ctx,

		// TODO 根据权重值遍历出当前最新的前N名验证人，并组成 ValidatorGovInfo信息，收集到 currValidators 中...
		// 入参为 第几个 和对应的 验证人信息
		func(index int64, validator sdk.Validator) (stop bool) {

		// 根据验证人地址key，收集验证人的治理投票信息
		//
		// 这么命名，表示 根据权重值获取到的 当前最新的验证人
		// TODO 因为 对治理提案发起投票的 (我认为应该只能是 验证人)
		// TODO
		currValidators[validator.GetOperator().String()] = newValidatorGovInfo(

			// 验证人的地址
			validator.GetOperator(),
			// 验证人 被质押/委托的 token
			validator.GetBondedTokens(),
			// 验证人的所有总 token 股份额度
			validator.GetDelegatorShares(),
			sdk.ZeroDec(), // 该验证人被
			OptionEmpty, // 治理投票信息默认为 Empty， 下面会填充的
		)
		return false
	})



	// iterate over all the votes
	// 迭代所有该提案的 投票
	votesIterator := keeper.GetVotes(ctx, proposal.GetProposalID())
	defer votesIterator.Close()
	for ; votesIterator.Valid(); votesIterator.Next() {
		vote := &Vote{}
		keeper.cdc.MustUnmarshalBinaryLengthPrefixed(votesIterator.Value(), vote)

		// if validator, just record it in the map
		// if delegator tally voting power
		/**
		如果该投票者是验证者，只需将其记录在 map中
		如果该投票者是委托人 计算投票权
		 */
		valAddrStr := sdk.ValAddress(vote.Voter).String()

		// TODO
		// 根据vote中的投票人地址去 map 中查找是否 该投票人为某个当前轮的验证人
		//
		// 如果存在, 将治理投票信息填充到 ValidatorGovInfo 的 Vote 字段
		if val, ok := currValidators[valAddrStr]; ok {
			val.Vote = vote.Option
			currValidators[valAddrStr] = val
		} else {
			// 否则： 去委托人里面找
			// iterate over all delegations from voter, deduct from any delegated-to validators
			// 从选民(发起治理投票者)地址的委托信息中获取出对应的验证人addr及委托信息
			//
			//
			keeper.ds.IterateDelegations(ctx, vote.Voter,

				// 入参的 回调函数
				func(index int64, delegation sdk.Delegation) (stop bool) {

				// 获取当前委托信息的 被委托的验证人
				valAddrStr := delegation.GetValidatorAddr().String()

				// 查看该验证人是否在 currValidators 这个ValidatorGovInfo的map中？
				// 如果存在，则 将该委托信息中的委托股份设置到ValidatorGovInfo信息的 DelegatorDeductions 字段
				if val, ok := currValidators[valAddrStr]; ok {
					// 用该委托占有该验证人的 股份设置为 DelegatorDeductions
					val.DelegatorDeductions = val.DelegatorDeductions.Add(delegation.GetShares())
					currValidators[valAddrStr] = val

					/**
					计算出 所委托的 toekn数目
					 */
					// 再计算出该委托份额，占用验证人的所有委托份额的 百分比
					delegatorShare := delegation.GetShares().Quo(val.DelegatorShares)
					// 用这个 百分比 乘于验证人的 总委托Token数
					votingPower := delegatorShare.MulInt(val.BondedTokens)

					// 使用这个token数目和 治理投票的状态值做相加
					results[vote.Option] = results[vote.Option].Add(votingPower)
					// 叠加全局的 Voting权重值
					totalVotingPower = totalVotingPower.Add(votingPower)
				}

				return false
			})
		}

		// 删除该治理的该选民的投票信息
		keeper.deleteVote(ctx, vote.ProposalID, vote.Voter)
	}

	// iterate over the validators again to tally their voting power
	//
	// 再次迭代 currValidators 以计算其投票权
	for _, val := range currValidators {
		if val.Vote == OptionEmpty {
			continue
		}

		// 计算该 验证人的所得的 非治理投票股权： Q
		sharesAfterDeductions := val.DelegatorShares.Sub(val.DelegatorDeductions)
		// 用Q/总的委托股权
		fractionAfterDeductions := sharesAfterDeductions.Quo(val.DelegatorShares)
		// 计算出该验证人的 非治理委托人的委托token
		votingPower := fractionAfterDeductions.MulInt(val.BondedTokens)

		//  用计算出来的 token追加到某状态的得票权重值上
		results[val.Vote] = results[val.Vote].Add(votingPower)
		// 叠加全局的 Voting权重值
		totalVotingPower = totalVotingPower.Add(votingPower)
	}

	tallyParams := keeper.GetTallyParams(ctx)

	/**
	构造返回参数
	 */
	tallyResults = NewTallyResultFromMap(results)

	// TODO: Upgrade the spec to cover all of these cases & remove pseudocode.
	// If there is no staked coins, the proposal fails
	// 返回 pool 中记录的，目前所有 staking 锁定的 token数目,如果为 0，则数据有问题
	if keeper.vs.TotalBondedTokens(ctx).IsZero() {
		return false, tallyResults
	}
	// If there is not enough quorum of votes, the proposal fails
	// 如果没有足够的票数，提案就失败了。
	//
	// 尼玛，这个计算我TM 硬是看不懂啊
	percentVoting := totalVotingPower.Quo(keeper.vs.TotalBondedTokens(ctx).ToDec())
	if percentVoting.LT(tallyParams.Quorum) {
		return false, tallyResults
	}
	// If no one votes (everyone abstains), proposal fails
	// 如果没有人投票（每个人弃权），提案失败
	if totalVotingPower.Sub(results[OptionAbstain]).Equal(sdk.ZeroDec()) {
		return false, tallyResults
	}
	// If more than 1/3 of voters veto, proposal fails
	// 如果超过1/3的选民否决，提案失败。
	if results[OptionNoWithVeto].Quo(totalVotingPower).GT(tallyParams.Veto) {
		return false, tallyResults
	}
	// If more than 1/2 of non-abstaining voters vote Yes, proposal passes
	// 如果超过1/2的非弃权选民投票赞成，则提案通过
	if results[OptionYes].Quo(totalVotingPower.Sub(results[OptionAbstain])).GT(tallyParams.Threshold) {
		return true, tallyResults
	}
	// If more than 1/2 of non-abstaining voters vote No, proposal fails
	// 如果超过1/2的非弃权选民投票否，提案将失败
	// TODO 这里我不明白 否决 (OptionNoWithVeto) 和 非弃权投NO (OptionNo) 的区别？

	return false, tallyResults
}
