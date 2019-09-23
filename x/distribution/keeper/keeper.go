package keeper

import (
	"my-cosmos/cosmos-sdk/codec"
	sdk "my-cosmos/cosmos-sdk/types"
	"my-cosmos/cosmos-sdk/x/distribution/types"
	"my-cosmos/cosmos-sdk/x/params"
)

// keeper of the staking store
type Keeper struct {
	storeKey            sdk.StoreKey
	cdc                 *codec.Codec
	paramSpace          params.Subspace
	bankKeeper          types.BankKeeper
	stakingKeeper       types.StakingKeeper
	feeCollectionKeeper types.FeeCollectionKeeper

	// codespace
	codespace sdk.CodespaceType
}

// create a new keeper
func NewKeeper(cdc *codec.Codec, key sdk.StoreKey, paramSpace params.Subspace, ck types.BankKeeper,
	sk types.StakingKeeper, fck types.FeeCollectionKeeper, codespace sdk.CodespaceType) Keeper {
	keeper := Keeper{
		storeKey:            key,
		cdc:                 cdc,
		paramSpace:          paramSpace.WithKeyTable(ParamKeyTable()),
		bankKeeper:          ck,
		stakingKeeper:       sk,
		feeCollectionKeeper: fck,
		codespace:           codespace,
	}
	return keeper
}

// set withdraw address
func (k Keeper) SetWithdrawAddr(ctx sdk.Context, delegatorAddr sdk.AccAddress, withdrawAddr sdk.AccAddress) sdk.Error {
	if !k.GetWithdrawAddrEnabled(ctx) {
		return types.ErrSetWithdrawAddrDisabled(k.codespace)
	}

	k.SetDelegatorWithdrawAddr(ctx, delegatorAddr, withdrawAddr)

	return nil
}

// withdraw rewards from a delegation
// 提取委托奖励
func (k Keeper) WithdrawDelegationRewards(ctx sdk.Context, delAddr sdk.AccAddress, valAddr sdk.ValAddress) sdk.Error {

	// 查询验证人详情
	val := k.stakingKeeper.Validator(ctx, valAddr)
	if val == nil {
		return types.ErrNoValidatorDistInfo(k.codespace)
	}

	// 查询委托信息
	del := k.stakingKeeper.Delegation(ctx, delAddr, valAddr)
	if del == nil {
		return types.ErrNoDelegationDistInfo(k.codespace)
	}

	// withdraw rewards
	// 提取委托奖励
	if err := k.withdrawDelegationRewards(ctx, val, del); err != nil {
		return err
	}

	// reinitialize the delegation
	//
	// 重新初始化委托人信息
	k.initializeDelegation(ctx, valAddr, delAddr)

	return nil
}

// withdraw validator commission
func (k Keeper) WithdrawValidatorCommission(ctx sdk.Context, valAddr sdk.ValAddress) sdk.Error {

	// fetch validator accumulated commission
	commission := k.GetValidatorAccumulatedCommission(ctx, valAddr)
	if commission.IsZero() {
		return types.ErrNoValidatorCommission(k.codespace)
	}

	coins, remainder := commission.TruncateDecimal()

	// leave remainder to withdraw later
	k.SetValidatorAccumulatedCommission(ctx, valAddr, remainder)

	// update outstanding
	outstanding := k.GetValidatorOutstandingRewards(ctx, valAddr)
	k.SetValidatorOutstandingRewards(ctx, valAddr, outstanding.Sub(sdk.NewDecCoins(coins)))

	if !coins.IsZero() {
		accAddr := sdk.AccAddress(valAddr)
		withdrawAddr := k.GetDelegatorWithdrawAddr(ctx, accAddr)

		if _, _, err := k.bankKeeper.AddCoins(ctx, withdrawAddr, coins); err != nil {
			return err
		}
	}

	return nil
}
