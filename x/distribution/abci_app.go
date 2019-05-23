package distribution

import (
	abci "github.com/tendermint/tendermint/abci/types"

	sdk "my-cosmos/cosmos-sdk/types"
	"my-cosmos/cosmos-sdk/x/distribution/keeper"
)

// set the proposer for determining distribution during endblock
// TODO 由 tendermint 发起调用
// 在当前区块 endblock期间结束前，计算出 上一个块的 出块奖励
func BeginBlocker(ctx sdk.Context, req abci.RequestBeginBlock, k keeper.Keeper) {

	// determine the total power signing the block
	//
	// 计算出目标区块（上一个块）的的签名所对应的 validator 的 power
	var totalPower, sumPrecommitPower int64

	// tendermint 只是把 commit 信息发过来的
	// 遍历上一个块的commit 的vote
	for _, voteInfo := range req.LastCommitInfo.GetVotes() {

		// 将参与上一个区块的 commit 投票的 valitor的power进行叠加
		totalPower += voteInfo.Validator.Power
		// 判断上个块是否参与了 block的签名，是的话需要再叠加一次power
		if voteInfo.SignedLastBlock {
			sumPrecommitPower += voteInfo.Validator.Power
		}
	}

	// TODO this is Tendermint-dependent
	// ref https://my-cosmos/cosmos-sdk/issues/3095
	if ctx.BlockHeight() > 1 {
		/*
		TODO 获取出上一个区块的出块人
		*/
		previousProposer := k.GetPreviousProposerConsAddr(ctx)
		k.AllocateTokens(ctx, sumPrecommitPower, totalPower, previousProposer, req.LastCommitInfo.GetVotes())
	}

	// record the proposer for when we payout on the next block
	//
	// 记录我们在下一个区块支付时的提议者
	// TODO 就是在下一个块时，需要发放奖励的 当前块的 出块人
	consAddr := sdk.ConsAddress(req.Header.ProposerAddress)
	k.SetPreviousProposerConsAddr(ctx, consAddr)

}
