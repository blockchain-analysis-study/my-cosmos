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

	// 将提案的投票 结果用一个 map缓存
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

		// 入参为 第几个 和对应的 验证人信息
		func(index int64, validator sdk.Validator) (stop bool) {

		// 根据验证人地址key，收集验证人的治理投票信息
		currValidators[validator.GetOperator().String()] = newValidatorGovInfo(

			// 验证人的地址
			validator.GetOperator(),
			// 验证人 被质押/委托的 token
			validator.GetBondedTokens(),
			// 验证人的所有总 token 股份额度
			validator.GetDelegatorShares(),
			sdk.ZeroDec(),
			OptionEmpty,
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
		如果验证者，只需将其记录在 map中
		如果委托人 计算投票权
		 */
		valAddrStr := sdk.ValAddress(vote.Voter).String()

		// 根据vote中的 验证人地址去 map 中查找是否存在该验证人
		if val, ok := currValidators[valAddrStr]; ok {
			val.Vote = vote.Option
			currValidators[valAddrStr] = val
		} else {
			// iterate over all delegations from voter, deduct from any delegated-to validators
			keeper.ds.IterateDelegations(ctx, vote.Voter, func(index int64, delegation sdk.Delegation) (stop bool) {
				valAddrStr := delegation.GetValidatorAddr().String()

				if val, ok := currValidators[valAddrStr]; ok {
					val.DelegatorDeductions = val.DelegatorDeductions.Add(delegation.GetShares())
					currValidators[valAddrStr] = val

					delegatorShare := delegation.GetShares().Quo(val.DelegatorShares)
					votingPower := delegatorShare.MulInt(val.BondedTokens)

					results[vote.Option] = results[vote.Option].Add(votingPower)
					totalVotingPower = totalVotingPower.Add(votingPower)
				}

				return false
			})
		}

		keeper.deleteVote(ctx, vote.ProposalID, vote.Voter)
	}

	// iterate over the validators again to tally their voting power
	for _, val := range currValidators {
		if val.Vote == OptionEmpty {
			continue
		}

		sharesAfterDeductions := val.DelegatorShares.Sub(val.DelegatorDeductions)
		fractionAfterDeductions := sharesAfterDeductions.Quo(val.DelegatorShares)
		votingPower := fractionAfterDeductions.MulInt(val.BondedTokens)

		results[val.Vote] = results[val.Vote].Add(votingPower)
		totalVotingPower = totalVotingPower.Add(votingPower)
	}

	tallyParams := keeper.GetTallyParams(ctx)
	tallyResults = NewTallyResultFromMap(results)

	// TODO: Upgrade the spec to cover all of these cases & remove pseudocode.
	// If there is no staked coins, the proposal fails
	if keeper.vs.TotalBondedTokens(ctx).IsZero() {
		return false, tallyResults
	}
	// If there is not enough quorum of votes, the proposal fails
	percentVoting := totalVotingPower.Quo(keeper.vs.TotalBondedTokens(ctx).ToDec())
	if percentVoting.LT(tallyParams.Quorum) {
		return false, tallyResults
	}
	// If no one votes (everyone abstains), proposal fails
	if totalVotingPower.Sub(results[OptionAbstain]).Equal(sdk.ZeroDec()) {
		return false, tallyResults
	}
	// If more than 1/3 of voters veto, proposal fails
	if results[OptionNoWithVeto].Quo(totalVotingPower).GT(tallyParams.Veto) {
		return false, tallyResults
	}
	// If more than 1/2 of non-abstaining voters vote Yes, proposal passes
	if results[OptionYes].Quo(totalVotingPower.Sub(results[OptionAbstain])).GT(tallyParams.Threshold) {
		return true, tallyResults
	}
	// If more than 1/2 of non-abstaining voters vote No, proposal fails

	return false, tallyResults
}
