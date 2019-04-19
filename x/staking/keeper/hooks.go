package keeper

import (
	sdk "my-cosmos/cosmos-sdk/types"
)

// AfterValidatorCreated - call hook if registered
func (k Keeper) AfterValidatorCreated(ctx sdk.Context, valAddr sdk.ValAddress) {
	if k.hooks != nil {
		k.hooks.AfterValidatorCreated(ctx, valAddr)
	}
}

// BeforeValidatorModified - call hook if registered
func (k Keeper) BeforeValidatorModified(ctx sdk.Context, valAddr sdk.ValAddress) {
	if k.hooks != nil {
		k.hooks.BeforeValidatorModified(ctx, valAddr)
	}
}

// AfterValidatorRemoved - call hook if registered
func (k Keeper) AfterValidatorRemoved(ctx sdk.Context, consAddr sdk.ConsAddress, valAddr sdk.ValAddress) {
	if k.hooks != nil {
		k.hooks.AfterValidatorRemoved(ctx, consAddr, valAddr)
	}
}

// AfterValidatorBonded - call hook if registered
func (k Keeper) AfterValidatorBonded(ctx sdk.Context, consAddr sdk.ConsAddress, valAddr sdk.ValAddress) {
	if k.hooks != nil {
		k.hooks.AfterValidatorBonded(ctx, consAddr, valAddr)
	}
}

// AfterValidatorBeginUnbonding - call hook if registered
func (k Keeper) AfterValidatorBeginUnbonding(ctx sdk.Context, consAddr sdk.ConsAddress, valAddr sdk.ValAddress) {
	if k.hooks != nil {
		k.hooks.AfterValidatorBeginUnbonding(ctx, consAddr, valAddr)
	}
}

// BeforeDelegationCreated - call hook if registered
func (k Keeper) BeforeDelegationCreated(ctx sdk.Context, delAddr sdk.AccAddress, valAddr sdk.ValAddress) {
	if k.hooks != nil {
		k.hooks.BeforeDelegationCreated(ctx, delAddr, valAddr)
	}
}

// BeforeDelegationSharesModified - call hook if registered
// TODO 这一步超级重要
// 逻辑非常多
// 这个函数主要处理的是  withdrawDelegationRewards
func (k Keeper) BeforeDelegationSharesModified(ctx sdk.Context, delAddr sdk.AccAddress, valAddr sdk.ValAddress) {
	if k.hooks != nil {
		// 其实是 keeper包的hook实例的 BeforeDelegationSharesModified
		k.hooks.BeforeDelegationSharesModified(ctx, delAddr, valAddr)
	}
}

// BeforeDelegationRemoved - call hook if registered
func (k Keeper) BeforeDelegationRemoved(ctx sdk.Context, delAddr sdk.AccAddress, valAddr sdk.ValAddress) {
	if k.hooks != nil {
		k.hooks.BeforeDelegationRemoved(ctx, delAddr, valAddr)
	}
}

// AfterDelegationModified - call hook if registered
func (k Keeper) AfterDelegationModified(ctx sdk.Context, delAddr sdk.AccAddress, valAddr sdk.ValAddress) {
	if k.hooks != nil {
		k.hooks.AfterDelegationModified(ctx, delAddr, valAddr)
	}
}

// BeforeValidatorSlashed - call hook if registered
func (k Keeper) BeforeValidatorSlashed(ctx sdk.Context, valAddr sdk.ValAddress, fraction sdk.Dec) {
	if k.hooks != nil {
		k.hooks.BeforeValidatorSlashed(ctx, valAddr, fraction)
	}
}
