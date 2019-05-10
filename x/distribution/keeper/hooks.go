package keeper

import (
	sdk "my-cosmos/cosmos-sdk/types"
)

// Wrapper struct
type Hooks struct {
	k Keeper
}

var _ sdk.StakingHooks = Hooks{}

// Create new distribution hooks
func (k Keeper) Hooks() Hooks { return Hooks{k} }

// nolint
// 初始化一个验证人的各种信息
func (h Hooks) AfterValidatorCreated(ctx sdk.Context, valAddr sdk.ValAddress) {
	// 往 全局 keeper 中追加 该validator
	val := h.k.stakingKeeper.Validator(ctx, valAddr)
	/**
	####### TODO  很重要的一步
	初始化新验证者的奖励

	1、设置了 历史奖励
	2、设置了 当前奖励
	3、设置了 累计佣金
	4、设置了 出块奖励
	*/
	h.k.initializeValidator(ctx, val)
}
func (h Hooks) BeforeValidatorModified(ctx sdk.Context, valAddr sdk.ValAddress) {
}
func (h Hooks) AfterValidatorRemoved(ctx sdk.Context, _ sdk.ConsAddress, valAddr sdk.ValAddress) {

	// fetch outstanding
	outstanding := h.k.GetValidatorOutstandingRewards(ctx, valAddr)

	// force-withdraw commission
	commission := h.k.GetValidatorAccumulatedCommission(ctx, valAddr)
	if !commission.IsZero() {
		// subtract from outstanding
		outstanding = outstanding.Sub(commission)

		// split into integral & remainder
		coins, remainder := commission.TruncateDecimal()

		// remainder to community pool
		feePool := h.k.GetFeePool(ctx)
		feePool.CommunityPool = feePool.CommunityPool.Add(remainder)
		h.k.SetFeePool(ctx, feePool)

		// add to validator account
		if !coins.IsZero() {

			accAddr := sdk.AccAddress(valAddr)
			withdrawAddr := h.k.GetDelegatorWithdrawAddr(ctx, accAddr)

			if _, _, err := h.k.bankKeeper.AddCoins(ctx, withdrawAddr, coins); err != nil {
				panic(err)
			}
		}
	}

	// add outstanding to community pool
	feePool := h.k.GetFeePool(ctx)
	feePool.CommunityPool = feePool.CommunityPool.Add(outstanding)
	h.k.SetFeePool(ctx, feePool)

	// delete outstanding
	h.k.DeleteValidatorOutstandingRewards(ctx, valAddr)

	// remove commission record
	h.k.DeleteValidatorAccumulatedCommission(ctx, valAddr)

	// clear slashes
	h.k.DeleteValidatorSlashEvents(ctx, valAddr)

	// clear historical rewards
	h.k.DeleteValidatorHistoricalRewards(ctx, valAddr)

	// clear current rewards
	h.k.DeleteValidatorCurrentRewards(ctx, valAddr)
}
func (h Hooks) BeforeDelegationCreated(ctx sdk.Context, delAddr sdk.AccAddress, valAddr sdk.ValAddress) {
	// 获取 验证人
	val := h.k.stakingKeeper.Validator(ctx, valAddr)

	// increment period
	// 增量验证人周期，返回刚刚结束的周期
	//
	// 这里除了处理 验证人的周期之外，还调整了验证人的出块奖励金额等等
	h.k.incrementValidatorPeriod(ctx, val)
}

// 更改委托信息 TODO 逻辑非常多的一部
func (h Hooks) BeforeDelegationSharesModified(ctx sdk.Context, delAddr sdk.AccAddress, valAddr sdk.ValAddress) {
	// 从存储中分别找到 验证人 和 委托人
	val := h.k.stakingKeeper.Validator(ctx, valAddr)
	del := h.k.stakingKeeper.Delegation(ctx, delAddr, valAddr)

	// withdraw delegation rewards (which also increments period)
	// (因为更改委托信息,所以需要撤回本周期获得的的奖励)退回委托奖励（也增加新的周期）
	if err := h.k.withdrawDelegationRewards(ctx, val, del); err != nil {
		panic(err)
	}
}
func (h Hooks) BeforeDelegationRemoved(ctx sdk.Context, delAddr sdk.AccAddress, valAddr sdk.ValAddress) {
	// nothing needed here since BeforeDelegationSharesModified will always also be called
}
func (h Hooks) AfterDelegationModified(ctx sdk.Context, delAddr sdk.AccAddress, valAddr sdk.ValAddress) {
	// create new delegation period record
	h.k.initializeDelegation(ctx, valAddr, delAddr)
}
func (h Hooks) AfterValidatorBeginUnbonding(ctx sdk.Context, _ sdk.ConsAddress, valAddr sdk.ValAddress) {
}
func (h Hooks) AfterValidatorBonded(ctx sdk.Context, _ sdk.ConsAddress, valAddr sdk.ValAddress) {
}
func (h Hooks) BeforeValidatorSlashed(ctx sdk.Context, valAddr sdk.ValAddress, fraction sdk.Dec) {
	// record the slash event
	h.k.updateValidatorSlashFraction(ctx, valAddr, fraction)
}
