package slashing

import (
	"fmt"

	abci "github.com/tendermint/tendermint/abci/types"
	tmtypes "github.com/tendermint/tendermint/types"

	sdk "my-cosmos/cosmos-sdk/types"
)

// slashing begin block functionality
// 惩罚机制 开始 block 功能
func BeginBlocker(ctx sdk.Context, req abci.RequestBeginBlock, sk Keeper) sdk.Tags {

	// Iterate over all the validators which *should* have signed this block
	// store whether or not they have actually signed it and slash/unbond any
	// which have missed too many blocks in a row (downtime slashing)
	/**
	TODO
	迭代*应该*已经签署了这个Block存储的所有验证人，
	无论它们是否已经实际签名，
	并且 惩罚/取消绑定连续错过太多块的任何块（停机时间削减）
	 */
	// 逐个检查 当前块的 commit 信息中的 验证人 签名数 (vote 数)
	// 为了做惩罚用的
	for _, voteInfo := range req.LastCommitInfo.GetVotes() {
		sk.handleValidatorSignature(ctx, voteInfo.Validator.Address, voteInfo.Validator.Power, voteInfo.SignedLastBlock)
	}

	// Iterate through any newly discovered evidence of infraction
	// Slash any validators (and since-unbonded stake within the unbonding period)
	// who contributed to valid infractions
	/**
	迭代任何关于所有验证人的新发现的违规证据（以及在解绑期间的无staking股权）
	 */
	for _, evidence := range req.ByzantineValidators {
		switch evidence.Type {
		// 目前值惩罚这些 双签名的验证人
		case tmtypes.ABCIEvidenceTypeDuplicateVote:
			/**
			TODO
			处理验证器在同一高度签名两个块
			 */
			sk.handleDoubleSign(ctx, evidence.Validator.Address, evidence.Height, evidence.Time, evidence.Validator.Power)
		default:
			ctx.Logger().With("module", "x/slashing").Error(fmt.Sprintf("ignored unknown evidence type: %s", evidence.Type))
		}
	}

	return sdk.EmptyTags()
}
