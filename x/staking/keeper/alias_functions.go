package keeper

import (
	"fmt"

	sdk "my-cosmos/cosmos-sdk/types"
	"my-cosmos/cosmos-sdk/x/staking/types"
)

// Implements ValidatorSet
var _ sdk.ValidatorSet = Keeper{}

// iterate through the validator set and perform the provided function
func (k Keeper) IterateValidators(ctx sdk.Context, fn func(index int64, validator sdk.Validator) (stop bool)) {
	store := ctx.KVStore(k.storeKey)
	iterator := sdk.KVStorePrefixIterator(store, ValidatorsKey)
	defer iterator.Close()
	i := int64(0)
	for ; iterator.Valid(); iterator.Next() {
		validator := types.MustUnmarshalValidator(k.cdc, iterator.Value())
		stop := fn(i, validator) // XXX is this safe will the validator unexposed fields be able to get written to?
		if stop {
			break
		}
		i++
	}
}

// iterate through the bonded validator set and perform the provided function
func (k Keeper) IterateBondedValidatorsByPower(ctx sdk.Context, fn func(index int64, validator sdk.Validator) (stop bool)) {
	store := ctx.KVStore(k.storeKey)
	maxValidators := k.MaxValidators(ctx)

	// 获取所有 权重值的验证人 迭代器
	iterator := sdk.KVStoreReversePrefixIterator(store, ValidatorsByPowerIndexKey)
	defer iterator.Close()

	i := int64(0)
	for ; iterator.Valid() && i < int64(maxValidators); iterator.Next() {
		address := iterator.Value()
		// 回去验证人详情
		validator := k.mustGetValidator(ctx, address)

		// 查看验证人是都为锁定状态
		if validator.Status == sdk.Bonded {
			// 执行回调
			/**
			XXX是否可以将验证器未曝光的字段写入？
			 */
			stop := fn(i, validator) // XXX is this safe will the validator unexposed fields be able to get written to?
			if stop {
				break
			}
			i++
		}
	}
}

// iterate through the active validator set and perform the provided function
// 遍历所有 权重值验证人 ?
func (k Keeper) IterateLastValidators(ctx sdk.Context, fn func(index int64, validator sdk.Validator) (stop bool)) {
	iterator := k.LastValidatorsIterator(ctx)
	defer iterator.Close()
	i := int64(0)
	for ; iterator.Valid(); iterator.Next() {
		address := AddressFromLastValidatorPowerKey(iterator.Key())
		validator, found := k.GetValidator(ctx, address)
		if !found {
			panic(fmt.Sprintf("validator record not found for address: %v\n", address))
		}

		stop := fn(i, validator) // XXX is this safe will the validator unexposed fields be able to get written to?
		if stop {
			break
		}
		i++
	}
}

// get the sdk.validator for a particular address
// 从全局 Keeper 中获取 validator
// 无 则拉DB 的且缓存到 Keeper 中
// 有 则直接返回
func (k Keeper) Validator(ctx sdk.Context, address sdk.ValAddress) sdk.Validator {
	val, found := k.GetValidator(ctx, address)
	if !found {
		return nil
	}
	return val
}

// get the sdk.validator for a particular pubkey
func (k Keeper) ValidatorByConsAddr(ctx sdk.Context, addr sdk.ConsAddress) sdk.Validator {
	val, found := k.GetValidatorByConsAddr(ctx, addr)
	if !found {
		return nil
	}
	return val
}

// total staking tokens supply which is bonded
// 返回 pool 中记录的，目前所有 staking 锁定的 token数目
func (k Keeper) TotalBondedTokens(ctx sdk.Context) sdk.Int {
	pool := k.GetPool(ctx)
	return pool.BondedTokens
}

// total staking tokens supply bonded and unbonded
func (k Keeper) TotalTokens(ctx sdk.Context) sdk.Int {
	pool := k.GetPool(ctx)
	return pool.TokenSupply()
}

// the fraction of the staking tokens which are currently bonded
func (k Keeper) BondedRatio(ctx sdk.Context) sdk.Dec {
	pool := k.GetPool(ctx)
	return pool.BondedRatio()
}

// when minting new tokens
func (k Keeper) InflateSupply(ctx sdk.Context, newTokens sdk.Int) {
	pool := k.GetPool(ctx)
	pool.NotBondedTokens = pool.NotBondedTokens.Add(newTokens)
	k.SetPool(ctx, pool)
}

// Implements DelegationSet

var _ sdk.DelegationSet = Keeper{}

// Returns self as it is both a validatorset and delegationset
func (k Keeper) GetValidatorSet() sdk.ValidatorSet {
	return k
}

// get the delegation for a particular set of delegator and validator addresses
func (k Keeper) Delegation(ctx sdk.Context, addrDel sdk.AccAddress, addrVal sdk.ValAddress) sdk.Delegation {
	bond, ok := k.GetDelegation(ctx, addrDel, addrVal)
	if !ok {
		return nil
	}

	return bond
}

// iterate through all of the delegations from a delegator
//
// 迭代委托人的所有委托信息
func (k Keeper) IterateDelegations(ctx sdk.Context, delAddr sdk.AccAddress,

	// 入参回调函数
	fn func(index int64, del sdk.Delegation) (stop bool)) {


	// 先获取对应的存储实例
	store := ctx.KVStore(k.storeKey)
	// 获取 委托人的key前缀
	delegatorPrefixKey := GetDelegationsKey(delAddr)
	// 获取该委托人的所有委托信息
	iterator := sdk.KVStorePrefixIterator(store, delegatorPrefixKey) // smallest to largest
	defer iterator.Close()

	// 遍历该委托人的所有委托信息
	for i := int64(0); iterator.Valid(); iterator.Next() {
		del := types.MustUnmarshalDelegation(k.cdc, iterator.Value())

		// 调用回调函数
		stop := fn(i, del)
		if stop {
			break
		}
		i++
	}
}

// return all delegations used during genesis dump
// TODO: remove this func, change all usage for iterate functionality
func (k Keeper) GetAllSDKDelegations(ctx sdk.Context) (delegations []sdk.Delegation) {
	store := ctx.KVStore(k.storeKey)
	iterator := sdk.KVStorePrefixIterator(store, DelegationKey)
	defer iterator.Close()

	for ; iterator.Valid(); iterator.Next() {
		delegation := types.MustUnmarshalDelegation(k.cdc, iterator.Value())
		delegations = append(delegations, delegation)
	}
	return delegations
}
