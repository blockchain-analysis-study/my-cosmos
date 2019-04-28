package slashing

import (
	sdk "my-cosmos/cosmos-sdk/types"
	"my-cosmos/cosmos-sdk/x/slashing/tags"
)

/**
惩罚 机制相关
 */
func NewHandler(k Keeper) sdk.Handler {
	return func(ctx sdk.Context, msg sdk.Msg) sdk.Result {
		// NOTE msg already has validate basic run
		// 注意： msg已经验证了基本运行 ？？
		switch msg := msg.(type) {
		/**
		发起 解除监禁请求
		 */
		case MsgUnjail:
			return handleMsgUnjail(ctx, msg, k)
		default:
			return sdk.ErrTxDecode("invalid message parse in staking module").Result()
		}
	}
}

// Validators must submit a transaction to unjail itself after
// having been jailed (and thus unbonded) for downtime
// 验证者必须提交一个 交易，以便在因停工而被判入狱（因此没有绑定）后自行解雇
func handleMsgUnjail(ctx sdk.Context, msg MsgUnjail, k Keeper) sdk.Result {

	// 先获取当前验证人
	validator := k.validatorSet.Validator(ctx, msg.ValidatorAddr)
	if validator == nil {
		return ErrNoValidatorForAddress(k.codespace).Result()
	}

	// cannot be unjailed if no self-delegation exists
	// 如果不存在自委托详情，则不能解除 监禁
	// 因为数据有误啊
	selfDel := k.validatorSet.Delegation(ctx, sdk.AccAddress(msg.ValidatorAddr), msg.ValidatorAddr)
	if selfDel == nil {
		return ErrMissingSelfDelegation(k.codespace).Result()
	}

	// 如果该验证人自委托的token比该验证人定义的最小允许质押的钱还要少
	// 说明参数不合法，则不能解除 监禁
	if validator.ShareTokens(selfDel.GetShares()).TruncateInt().LT(validator.GetMinSelfDelegation()) {
		return ErrSelfDelegationTooLowToUnjail(k.codespace).Result()
	}

	// cannot be unjailed if not jailed
	// 或者本身就没有 被监禁的，也不能解除 监禁
	if !validator.GetJailed() {
		return ErrValidatorNotJailed(k.codespace).Result()
	}

	// 根据公钥获取节点地址
	consAddr := sdk.ConsAddress(validator.GetConsPubKey().Address())

	// 根据验证人地址查找对应的 签名信息
	// 如果找不到，则不能 解除 监禁
	info, found := k.getValidatorSigningInfo(ctx, consAddr)
	if !found {
		return ErrNoValidatorForAddress(k.codespace).Result()
	}

	// cannot be unjailed if tombstoned
	// 如果该验证人已经销毁了，则不能解除 监禁
	if info.Tombstoned {
		return ErrValidatorJailed(k.codespace).Result()
	}

	// cannot be unjailed until out of jail
	// 如果还在 监禁期，则不允许解除 监禁
	if ctx.BlockHeader().Time.Before(info.JailedUntil) {
		return ErrValidatorJailed(k.codespace).Result()
	}

	// unjail the validator
	/**
	解锁处于惩罚锁定期的 验证人
	 */
	k.validatorSet.Unjail(ctx, consAddr)

	tags := sdk.NewTags(
		tags.Action, tags.ActionValidatorUnjailed,
		tags.Validator, msg.ValidatorAddr.String(),
	)

	return sdk.Result{
		Tags: tags,
	}
}
