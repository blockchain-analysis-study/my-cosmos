package staking

import (
	"time"

	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/libs/common"
	tmtypes "github.com/tendermint/tendermint/types"

	sdk "my-cosmos/cosmos-sdk/types"
	"my-cosmos/cosmos-sdk/x/staking/keeper"
	"my-cosmos/cosmos-sdk/x/staking/tags"
	"my-cosmos/cosmos-sdk/x/staking/types"
)


/**
###########
很重要的一个函数：

返回一个根据入参去选择操作 经济模型相关的函数
质押
委托

###########
 */
func NewHandler(k keeper.Keeper) sdk.Handler {
	return func(ctx sdk.Context, msg sdk.Msg) sdk.Result {
		// NOTE msg already has validate basic run
		switch msg := msg.(type) {

		/**
		新建 验证人 （质押/自我授权委托）
		 */
		case types.MsgCreateValidator:
			return handleMsgCreateValidator(ctx, msg, k)

		/**
		更新 验证人
		 */
		case types.MsgEditValidator:
			return handleMsgEditValidator(ctx, msg, k)

		/**
		发起 委托
		 */
		case types.MsgDelegate:
			return handleMsgDelegate(ctx, msg, k)

		/**
		重置 委托
		 */
		case types.MsgBeginRedelegate:
			return handleMsgBeginRedelegate(ctx, msg, k)

		/**
		撤销 委托
		 */
		case types.MsgUndelegate:
			return handleMsgUndelegate(ctx, msg, k)
		default:
			return sdk.ErrTxDecode("invalid message parse in staking module").Result()
		}
	}
}

// Called every block, update validator set
func EndBlocker(ctx sdk.Context, k keeper.Keeper) ([]abci.ValidatorUpdate, sdk.Tags) {
	resTags := sdk.NewTags()

	// Calculate validator set changes.
	//
	// NOTE: ApplyAndReturnValidatorSetUpdates has to come before
	// UnbondAllMatureValidatorQueue.
	// This fixes a bug when the unbonding period is instant (is the case in
	// some of the tests). The test expected the validator to be completely
	// unbonded after the Endblocker (go from Bonded -> Unbonding during
	// ApplyAndReturnValidatorSetUpdates and then Unbonding -> Unbonded during
	// UnbondAllMatureValidatorQueue).
	validatorUpdates := k.ApplyAndReturnValidatorSetUpdates(ctx)

	// Unbond all mature validators from the unbonding queue.
	k.UnbondAllMatureValidatorQueue(ctx)

	// Remove all mature unbonding delegations from the ubd queue.
	matureUnbonds := k.DequeueAllMatureUBDQueue(ctx, ctx.BlockHeader().Time)
	for _, dvPair := range matureUnbonds {
		err := k.CompleteUnbonding(ctx, dvPair.DelegatorAddress, dvPair.ValidatorAddress)
		if err != nil {
			continue
		}

		resTags.AppendTags(sdk.NewTags(
			tags.Action, ActionCompleteUnbonding,
			tags.Delegator, dvPair.DelegatorAddress.String(),
			tags.SrcValidator, dvPair.ValidatorAddress.String(),
		))
	}

	// Remove all mature redelegations from the red queue.
	matureRedelegations := k.DequeueAllMatureRedelegationQueue(ctx, ctx.BlockHeader().Time)
	for _, dvvTriplet := range matureRedelegations {
		err := k.CompleteRedelegation(ctx, dvvTriplet.DelegatorAddress,
			dvvTriplet.ValidatorSrcAddress, dvvTriplet.ValidatorDstAddress)
		if err != nil {
			continue
		}

		resTags.AppendTags(sdk.NewTags(
			tags.Action, tags.ActionCompleteRedelegation,
			tags.Delegator, dvvTriplet.DelegatorAddress.String(),
			tags.SrcValidator, dvvTriplet.ValidatorSrcAddress.String(),
			tags.DstValidator, dvvTriplet.ValidatorDstAddress.String(),
		))
	}

	return validatorUpdates, resTags
}

// These functions assume everything has been authenticated,
// now we just perform action and save
/**
###########
创建一个验证人
###########
 */
// 这些函数假设所有内容都已经过身份验证，现在我们只执行操作并保存
func handleMsgCreateValidator(ctx sdk.Context, msg types.MsgCreateValidator, k keeper.Keeper) sdk.Result {
	// check to see if the pubkey or sender has been registered before
	// 先在验证人集里面，检查以前是否已注册pubkey或sender
	/**
	已经质押过，不能再次质押
	 */
	if _, found := k.GetValidator(ctx, msg.ValidatorAddress); found {
		return ErrValidatorOwnerExists(k.Codespace()).Result()
	}

	// 根据PubKey 去拿 也可以拿到的话，不给予 质押
	if _, found := k.GetValidatorByConsAddr(ctx, sdk.GetConsAddress(msg.PubKey)); found {
		return ErrValidatorPubKeyExists(k.Codespace()).Result()
	}

	// 校验入参的钱的面额 如果不等于 允许绑定的钱的面额
	// 其实就是入参的面额不等于 时价的话，不给予质押
	if msg.Value.Denom != k.GetParams(ctx).BondDenom {
		return ErrBadDenom(k.Codespace()).Result()
	}

	// 校验下入参的验证人信息的各个字段长度
	// 质押时填入的验证人的各个信息不符合的都不给予质押
	if _, err := msg.Description.EnsureLength(); err != nil {
		return err.Result()
	}

	// 获取共识入参
	if ctx.ConsensusParams() != nil {

		// 转换下公钥类型
		tmPubKey := tmtypes.TM2PB.PubKey(msg.PubKey)

		// 如果当前 pubkey的【类型】不在 共识允许的验证人公钥类型集里面
		// 需要返回Err
		if !common.StringInSlice(tmPubKey.Type, ctx.ConsensusParams().Validator.PubKeyTypes) {
			return ErrValidatorPubKeyTypeUnsupported(k.Codespace(),
				tmPubKey.Type,
				ctx.ConsensusParams().Validator.PubKeyTypes).Result()
		}
	}

	/**
	#########

	【重头戏】

	创建一个 验证人

	根据入参的 验证人地址、公钥、及描述信息
	#########
	 */
	validator := NewValidator(msg.ValidatorAddress, msg.PubKey, msg.Description)

	// 创建一个 佣金信息
	commission := NewCommissionWithTime(
		msg.Commission.Rate, msg.Commission.MaxRate,
		msg.Commission.MaxChangeRate, ctx.BlockHeader().Time,
	)

	// 把生成的佣金信息填充到验证人中
	validator, err := validator.SetInitialCommission(commission)
	if err != nil {
		return err.Result()
	}

	// 设置 验证人的 最小委托金
	validator.MinSelfDelegation = msg.MinSelfDelegation

	// 写入DB
	// 根据验证人addr 写入 validator
	k.SetValidator(ctx, validator)
	// 根据public key 转换成的 addr 写入 validator
	k.SetValidatorByConsAddr(ctx, validator)
	// 新的按照权重作为索引的 写入 validator
	k.SetNewValidatorByPowerIndex(ctx, validator)

	// call the after-creation hook
	// 在创建 验证人之后，调用 hook函数 (类似AOP之类的做法
	// 这里实际上是调用到了 Keeper包的 AfterValidatorCreated 函数
	//
	// ###########  TODO ############# 非常重要的一步
	//
	//	初始化新验证者的奖励
	//
	//1、设置了 历史奖励
	//2、设置了 当前奖励
	//3、设置了 累计佣金
	//4、设置了 出块奖励
	//
	k.AfterValidatorCreated(ctx, validator.OperatorAddress)

	// move coins from the msg.Address account to a (self-delegation) delegator account
	// the validator account and global shares are updated within here
	/**
	将币从msg.Address帐户移动到（自我授权）委托人帐户
	验证人帐户和全局共享在此处更新
	 */
	_, err = k.Delegate(ctx, msg.DelegatorAddress, msg.Value.Amount, validator, true)
	if err != nil {
		return err.Result()
	}

	// 组装成 pb 的Tags 结构，返回去
	tags := sdk.NewTags(
		tags.DstValidator, msg.ValidatorAddress.String(),
		tags.Moniker, msg.Description.Moniker,
		tags.Identity, msg.Description.Identity,
	)

	return sdk.Result{
		Tags: tags,
	}
}


/**
###########
更新一个验证人
###########
*/
func handleMsgEditValidator(ctx sdk.Context, msg types.MsgEditValidator, k keeper.Keeper) sdk.Result {
	// validator must already be registered
	// 该验证人必须是已经注册了的
	validator, found := k.GetValidator(ctx, msg.ValidatorAddress)
	if !found {
		return ErrNoValidatorFound(k.Codespace()).Result()
	}

	// replace all editable fields (clients should autofill existing values)
	// 更新验证人的各个字段
	description, err := validator.Description.UpdateDescription(msg.Description)
	if err != nil {
		return err.Result()
	}

	// 更新
	validator.Description = description

	// 入参的佣金比率 不为空，则更新佣金比
	if msg.CommissionRate != nil {
		commission, err := k.UpdateValidatorCommission(ctx, validator, *msg.CommissionRate)
		if err != nil {
			return err.Result()
		}

		// call the before-modification hook since we're about to update the commission
		// 因为我们更新了佣金，所以必须做一些前置处理 [该方法目前还未实现]
		k.BeforeValidatorModified(ctx, msg.ValidatorAddress)

		validator.Commission = commission
	}

	// 如果新入参的 最小自委托 不为nil
	if msg.MinSelfDelegation != nil {

		// 不允许调低 自委托门槛
		if !(*msg.MinSelfDelegation).GT(validator.MinSelfDelegation) {
			return ErrMinSelfDelegationDecreased(k.Codespace()).Result()
		}

		// 不允许通过把门槛调得比被质押/委托的钱 还要大 (否则就出问题啦，因为目前质押的竟然没有达到最低自委托门槛)
		if (*msg.MinSelfDelegation).GT(validator.Tokens) {
			return ErrSelfDelegationBelowMinimum(k.Codespace()).Result()
		}
		validator.MinSelfDelegation = (*msg.MinSelfDelegation)
	}

	// 〔更新验证人〕
	k.SetValidator(ctx, validator)

	tags := sdk.NewTags(
		tags.DstValidator, msg.ValidatorAddress.String(),
		tags.Moniker, description.Moniker,
		tags.Identity, description.Identity,
	)

	return sdk.Result{
		Tags: tags,
	}
}


/**
###########
发起一个委托
###########
*/
func handleMsgDelegate(ctx sdk.Context, msg types.MsgDelegate, k keeper.Keeper) sdk.Result {
	validator, found := k.GetValidator(ctx, msg.ValidatorAddress)
	if !found {
		return ErrNoValidatorFound(k.Codespace()).Result()
	}

	// 如果当前入参的委托的coin的面额 不等于当前 keeper 管理器中记录的 coin 的面额
	if msg.Value.Denom != k.GetParams(ctx).BondDenom {
		return ErrBadDenom(k.Codespace()).Result()
	}


	// 处理 委托相关
	_, err := k.Delegate(ctx, msg.DelegatorAddress, msg.Value.Amount, validator, true)
	if err != nil {
		return err.Result()
	}

	tags := sdk.NewTags(
		tags.Delegator, msg.DelegatorAddress.String(),
		tags.DstValidator, msg.ValidatorAddress.String(),
	)

	return sdk.Result{
		Tags: tags,
	}
}


/**
###########
解除一个委托
###########
*/
func handleMsgUndelegate(ctx sdk.Context, msg types.MsgUndelegate, k keeper.Keeper) sdk.Result {
	// 开始解除委托
	// TODO 重要的一步
	completionTime, err := k.Undelegate(ctx, msg.DelegatorAddress, msg.ValidatorAddress, msg.SharesAmount)
	if err != nil {
		return err.Result()
	}

	finishTime := types.MsgCdc.MustMarshalBinaryLengthPrefixed(completionTime)
	tags := sdk.NewTags(
		tags.Delegator, msg.DelegatorAddress.String(),
		tags.SrcValidator, msg.ValidatorAddress.String(),
		tags.EndTime, completionTime.Format(time.RFC3339),
	)

	return sdk.Result{Data: finishTime, Tags: tags}
}


/**
###########
重置一个委托
(所谓的重置委托，指，解除旧有验证人的委托，及添加新的验证人的委托)
###########
*/
func handleMsgBeginRedelegate(ctx sdk.Context, msg types.MsgBeginRedelegate, k keeper.Keeper) sdk.Result {

	// 开始重新委托
	completionTime, err := k.BeginRedelegation(ctx, msg.DelegatorAddress, msg.ValidatorSrcAddress,
		msg.ValidatorDstAddress, msg.SharesAmount)
	if err != nil {
		return err.Result()
	}

	// 解码完成时间
	finishTime := types.MsgCdc.MustMarshalBinaryLengthPrefixed(completionTime)
	resTags := sdk.NewTags(
		tags.Delegator, msg.DelegatorAddress.String(),
		tags.SrcValidator, msg.ValidatorSrcAddress.String(),
		tags.DstValidator, msg.ValidatorDstAddress.String(),
		tags.EndTime, completionTime.Format(time.RFC3339),
	)

	return sdk.Result{Data: finishTime, Tags: resTags}
}
