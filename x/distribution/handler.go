package distribution

import (
	sdk "my-cosmos/cosmos-sdk/types"
	"my-cosmos/cosmos-sdk/x/distribution/keeper"
	"my-cosmos/cosmos-sdk/x/distribution/tags"
	"my-cosmos/cosmos-sdk/x/distribution/types"
)

/*
派发奖励 用
*/
func NewHandler(k keeper.Keeper) sdk.Handler {
	return func(ctx sdk.Context, msg sdk.Msg) sdk.Result {
		// NOTE msg already has validate basic run
		switch msg := msg.(type) {
		case types.MsgSetWithdrawAddress:

			/**

			 */
			return handleMsgModifyWithdrawAddress(ctx, msg, k)
		case types.MsgWithdrawDelegatorReward:

			/**
			提取委托奖励
			 */
			return handleMsgWithdrawDelegatorReward(ctx, msg, k)
		case types.MsgWithdrawValidatorCommission:

			/**
			提取质押佣金
			 */
			return handleMsgWithdrawValidatorCommission(ctx, msg, k)
		default:
			return sdk.ErrTxDecode("invalid message parse in distribution module").Result()
		}
	}
}

// These functions assume everything has been authenticated (ValidateBasic passed, and signatures checked)

// 这些函数假设所有内容都已经过身份验证（已通过ValidateBasic，并检查签名）
func handleMsgModifyWithdrawAddress(ctx sdk.Context, msg types.MsgSetWithdrawAddress, k keeper.Keeper) sdk.Result {

	err := k.SetWithdrawAddr(ctx, msg.DelegatorAddress, msg.WithdrawAddress)
	if err != nil {
		return err.Result()
	}

	tags := sdk.NewTags(
		tags.Delegator, []byte(msg.DelegatorAddress.String()),
	)
	return sdk.Result{
		Tags: tags,
	}
}

// 提取 委托人的处快奖励
func handleMsgWithdrawDelegatorReward(ctx sdk.Context, msg types.MsgWithdrawDelegatorReward, k keeper.Keeper) sdk.Result {

	// 来，我们开始处理
	err := k.WithdrawDelegationRewards(ctx, msg.DelegatorAddress, msg.ValidatorAddress)
	if err != nil {
		return err.Result()
	}

	tags := sdk.NewTags(
		tags.Delegator, []byte(msg.DelegatorAddress.String()),
		tags.Validator, []byte(msg.ValidatorAddress.String()),
	)
	return sdk.Result{
		Tags: tags,
	}
}

func handleMsgWithdrawValidatorCommission(ctx sdk.Context, msg types.MsgWithdrawValidatorCommission, k keeper.Keeper) sdk.Result {

	err := k.WithdrawValidatorCommission(ctx, msg.ValidatorAddress)
	if err != nil {
		return err.Result()
	}

	tags := sdk.NewTags(
		tags.Validator, []byte(msg.ValidatorAddress.String()),
	)
	return sdk.Result{
		Tags: tags,
	}
}
