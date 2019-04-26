package gov

import (
	"fmt"

	sdk "my-cosmos/cosmos-sdk/types"
	"my-cosmos/cosmos-sdk/x/gov/tags"
)

// Called every block, process inflation, update validator set
func EndBlocker(ctx sdk.Context, keeper Keeper) sdk.Tags {
	logger := ctx.Logger().With("module", "x/gov")
	resTags := sdk.NewTags()

	/**
	根据当前区块时间
	遍历出所有 非激活期的提案 (已过期的)
	 */
	inactiveIterator := keeper.InactiveProposalQueueIterator(ctx, ctx.BlockHeader().Time)
	defer inactiveIterator.Close()

	// 遍历它们
	for ; inactiveIterator.Valid(); inactiveIterator.Next() {
		var proposalID uint64

		keeper.cdc.MustUnmarshalBinaryLengthPrefixed(inactiveIterator.Value(), &proposalID)
		inactiveProposal := keeper.GetProposal(ctx, proposalID)

		// 清除吊已过期的非激活提案
		keeper.DeleteProposal(ctx, proposalID)
		// 删除任何相关存款（烧毁）
		keeper.DeleteDeposits(ctx, proposalID) // delete any associated deposits (burned)

		// TODO 我还没看到把钱转回给质押账户啊？ 原因是当时质押时 根本没用到 AccountKeeper 管理这笔钱，当时转到 提案质押账户上其实也是做个记录的
		resTags = resTags.AppendTag(tags.ProposalID, fmt.Sprintf("%d", proposalID))
		resTags = resTags.AppendTag(tags.ProposalResult, tags.ActionProposalDropped)

		logger.Info(
			fmt.Sprintf("proposal %d (%s) didn't meet minimum deposit of %s (had only %s); deleted",
				inactiveProposal.GetProposalID(),
				inactiveProposal.GetTitle(),
				keeper.GetDepositParams(ctx).MinDeposit,
				inactiveProposal.GetTotalDeposit(),
			),
		)
	}

	// fetch active proposals whose voting periods have ended (are passed the block time)
	// 获取投票期结束的 所有激活的提案（通过当前块时间戳）
	activeIterator := keeper.ActiveProposalQueueIterator(ctx, ctx.BlockHeader().Time)
	defer activeIterator.Close()
	for ; activeIterator.Valid(); activeIterator.Next() {
		var proposalID uint64

		keeper.cdc.MustUnmarshalBinaryLengthPrefixed(activeIterator.Value(), &proposalID)
		// 跟填好获取出 激活的提案信息
		activeProposal := keeper.GetProposal(ctx, proposalID)


		passes, tallyResults := tally(ctx, keeper, activeProposal)

		var tagValue string
		if passes {
			keeper.RefundDeposits(ctx, activeProposal.GetProposalID())
			activeProposal.SetStatus(StatusPassed)
			tagValue = tags.ActionProposalPassed
		} else {
			keeper.DeleteDeposits(ctx, activeProposal.GetProposalID())
			activeProposal.SetStatus(StatusRejected)
			tagValue = tags.ActionProposalRejected
		}

		activeProposal.SetFinalTallyResult(tallyResults)
		keeper.SetProposal(ctx, activeProposal)
		keeper.RemoveFromActiveProposalQueue(ctx, activeProposal.GetVotingEndTime(), activeProposal.GetProposalID())

		logger.Info(
			fmt.Sprintf(
				"proposal %d (%s) tallied; passed: %v",
				activeProposal.GetProposalID(), activeProposal.GetTitle(), passes,
			),
		)

		resTags = resTags.AppendTag(tags.ProposalID, fmt.Sprintf("%d", proposalID))
		resTags = resTags.AppendTag(tags.ProposalResult, tagValue)
	}

	return resTags
}
