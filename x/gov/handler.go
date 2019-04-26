package gov

import (
	"fmt"

	sdk "my-cosmos/cosmos-sdk/types"
	"my-cosmos/cosmos-sdk/x/gov/tags"
)

// Handle all "gov" type messages.
// 处理 治理相关
func NewHandler(keeper Keeper) sdk.Handler {
	return func(ctx sdk.Context, msg sdk.Msg) sdk.Result {
		switch msg := msg.(type) {
		/**
		发起治理提案的 预存款(预质押)
		 */
		case MsgDeposit:
			return handleMsgDeposit(ctx, keeper, msg)
		/**
		提交一个提案
		 */
		case MsgSubmitProposal:
			return handleMsgSubmitProposal(ctx, keeper, msg)
		/**
		对提案发起投票
		 */
		case MsgVote:
			return handleMsgVote(ctx, keeper, msg)
		default:
			errMsg := fmt.Sprintf("Unrecognized gov msg type: %T", msg)
			return sdk.ErrUnknownRequest(errMsg).Result()
		}
	}
}


/**
TODO 重要
提交一个提案
 */
func handleMsgSubmitProposal(ctx sdk.Context, keeper Keeper, msg MsgSubmitProposal) sdk.Result {
	// 根据入参，创建一个文本提案
	proposal := keeper.NewTextProposal(ctx, msg.Title, msg.Description, msg.ProposalType)

	// 获取一个自增的 提案ID
	proposalID := proposal.GetProposalID()
	proposalIDStr := fmt.Sprintf("%d", proposalID)

	// 添加提案质押
	// 返回是否激活提案状态为 投票周期
	err, votingStarted := keeper.AddDeposit(ctx, proposalID, msg.Proposer, msg.InitialDeposit)
	if err != nil {
		return err.Result()
	}

	/* 组装返回参数 */
	resTags := sdk.NewTags(
		tags.Proposer, []byte(msg.Proposer.String()),
		tags.ProposalID, proposalIDStr,
	)

	if votingStarted {
		resTags = resTags.AppendTag(tags.VotingPeriodStart, proposalIDStr)
	}

	return sdk.Result{
		Data: keeper.cdc.MustMarshalBinaryLengthPrefixed(proposalID),
		Tags: resTags,
	}
}

/**
发起治理 提案钱的质押
 */
func handleMsgDeposit(ctx sdk.Context, keeper Keeper, msg MsgDeposit) sdk.Result {

	//很明显在追加提案 质押时，需要判断该提案存不存在先
	err, votingStarted := keeper.AddDeposit(ctx, msg.ProposalID, msg.Depositor, msg.Amount)
	if err != nil {
		return err.Result()
	}

	proposalIDStr := fmt.Sprintf("%d", msg.ProposalID)
	resTags := sdk.NewTags(
		tags.Depositor, []byte(msg.Depositor.String()),
		tags.ProposalID, proposalIDStr,
	)

	if votingStarted {
		resTags = resTags.AppendTag(tags.VotingPeriodStart, proposalIDStr)
	}

	return sdk.Result{
		Tags: resTags,
	}
}


/**
对某个 治理提案进行 投票
 */
func handleMsgVote(ctx sdk.Context, keeper Keeper, msg MsgVote) sdk.Result {
	err := keeper.AddVote(ctx, msg.ProposalID, msg.Voter, msg.Option)
	if err != nil {
		return err.Result()
	}

	return sdk.Result{
		Tags: sdk.NewTags(
			tags.Voter, msg.Voter.String(),
			tags.ProposalID, fmt.Sprintf("%d", msg.ProposalID),
		),
	}
}
