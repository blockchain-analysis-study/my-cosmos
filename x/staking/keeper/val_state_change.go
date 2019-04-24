package keeper

import (
	"bytes"
	"fmt"
	"sort"

	abci "github.com/tendermint/tendermint/abci/types"

	sdk "my-cosmos/cosmos-sdk/types"
	"my-cosmos/cosmos-sdk/x/staking/types"
)

// Apply and return accumulated updates to the bonded validator set. Also,
// * Updates the active valset as keyed by LastValidatorPowerKey.
// * Updates the total power as keyed by LastTotalPowerKey.
// * Updates validator status' according to updated powers.
// * Updates the fee pool bonded vs not-bonded tokens.
// * Updates relevant indices.
// It gets called once after genesis, another time maybe after genesis transactions,
// then once at every EndBlock.
//
// CONTRACT: Only validators with non-zero power or zero-power that were bonded
// at the previous block height or were removed from the validator set entirely
// are returned to Tendermint.
func (k Keeper) ApplyAndReturnValidatorSetUpdates(ctx sdk.Context) (updates []abci.ValidatorUpdate) {

	store := ctx.KVStore(k.storeKey)
	// 获取配置中最大验证人 数目
	maxValidators := k.GetParams(ctx).MaxValidators

	// 先声明一个 记录权重的缓存变量
	totalPower := sdk.ZeroInt()

	// Retrieve the last validator set.
	// The persistent set is updated later in this function.
	// (see LastValidatorPowerKey).
	/**
	取回(返回) 最新的 验证人集合 (最后一个时间窗口的？)
	该(持续/固化了的)集合在当前函数之后被更新
	(参考查看： LastValidatorPowerKey 这个key: 设置 验证人的最新的权重信息)
	 */
	// 这是一个 map
	last := k.getLastValidatorsByAddr(ctx)

	// Iterate over validators, highest power to lowest.
	// 按照权重索引 最高到最低 返回一个所有验证人的迭代器。
	iterator := sdk.KVStoreReversePrefixIterator(store, ValidatorsByPowerIndexKey)
	defer iterator.Close()
	/**
	TODO 迭代所有验证人
	 */
	for count := 0; iterator.Valid() && count < int(maxValidators); iterator.Next() {

		// fetch the validator
		// 获取对应的验证人信息
		valAddr := sdk.ValAddress(iterator.Value())
		validator := k.mustGetValidator(ctx, valAddr)

		// 如果当前验证人属于处在 slash(被惩罚)锁定中，则说明数据有问题了 ... panic
		if validator.Jailed {
			panic("should never retrieve a jailed validator from the power store")
		}

		// if we get to a zero-power validator (which we don't bond),
		// there are no more possible bonded validators
		/**
		如果我们得到一个 0 权重的验证人（我们没有绑定钱），就没有更多可能的绑定验证器

		TODO 其实就是遍历到 第一个出现 0 权重的验证人为止，或者没有0权重的需要继续把所有验证人遍历完
		 */
		if validator.PotentialTendermintPower() == 0 {
			break
		}

		// apply the appropriate state change if necessary
		/**
		必要时应用适当的状态更改
		 */
		switch validator.Status {
		case sdk.Unbonded:
			validator = k.unbondedToBonded(ctx, validator)
		case sdk.Unbonding:
			validator = k.unbondingToBonded(ctx, validator)
		case sdk.Bonded:
			// no state change
		default:
			panic("unexpected validator status")
		}

		// fetch the old power bytes
		// 获取所有 旧的权重 bytes
		var valAddrBytes [sdk.AddrLen]byte
		copy(valAddrBytes[:], valAddr[:])
		oldPowerBytes, found := last[valAddrBytes]

		// calculate the new power bytes
		// 计算当前 验证人的新的权重 bytes (根据 该验证人的 状态是否为 锁定期及身上的 token数目来定)
		newPower := validator.TendermintPower()
		newPowerBytes := k.cdc.MustMarshalBinaryLengthPrefixed(newPower)

		// update the validator set if power has changed
		// 如果权重又发生改变了，则需要更新 验证人集合
		// (全部收集到 updates 集合中，最后返回出去)
		if !found || !bytes.Equal(oldPowerBytes, newPowerBytes) {
			updates = append(updates, validator.ABCIValidatorUpdate())

			// set validator power on lookup index
			// 设置 验证人的最新的权重信息
			k.SetLastValidatorPower(ctx, valAddr, newPower)
		}

		// validator still in the validator set, so delete from the copy
		// 从last 这个map中删除 (删除只是不想影响后续的计算)
		delete(last, valAddrBytes)

		// keep count
		count++
		totalPower = totalPower.Add(sdk.NewInt(newPower))
	}

	// sort the no-longer-bonded validators
	// 对不再保留的验证者进行排序
	noLongerBonded := sortNoLongerBonded(last)

	// iterate through the sorted no-longer-bonded validators
	// 遍历不在保留的验证人
	for _, valAddrBytes := range noLongerBonded {

		// fetch the validator
		validator := k.mustGetValidator(ctx, sdk.ValAddress(valAddrBytes))

		// bonded to unbonding
		// 将 锁定状态更改为 正处于解锁状态
		k.bondedToUnbonding(ctx, validator)

		// delete from the bonded validator index
		// 清除掉 该验证人的最新权重信息
		k.DeleteLastValidatorPower(ctx, sdk.ValAddress(valAddrBytes))

		// update the validator set
		// 追加到更改集合 updates中，用于返回出去
		updates = append(updates, validator.ABCIValidatorUpdateZero())
	}

	// set total power on lookup index if there are any updates
	// 如果有任何更新，则在查找索引上设置总 权重
	if len(updates) > 0 {
		k.SetLastTotalPower(ctx, totalPower)
	}

	return updates
}

// Validator state transitions

func (k Keeper) bondedToUnbonding(ctx sdk.Context, validator types.Validator) types.Validator {
	if validator.Status != sdk.Bonded {
		panic(fmt.Sprintf("bad state transition bondedToUnbonding, validator: %v\n", validator))
	}
	return k.beginUnbondingValidator(ctx, validator)
}

func (k Keeper) unbondingToBonded(ctx sdk.Context, validator types.Validator) types.Validator {
	if validator.Status != sdk.Unbonding {
		panic(fmt.Sprintf("bad state transition unbondingToBonded, validator: %v\n", validator))
	}
	return k.bondValidator(ctx, validator)
}

func (k Keeper) unbondedToBonded(ctx sdk.Context, validator types.Validator) types.Validator {
	if validator.Status != sdk.Unbonded {
		panic(fmt.Sprintf("bad state transition unbondedToBonded, validator: %v\n", validator))
	}
	return k.bondValidator(ctx, validator)
}

// switches a validator from unbonding state to unbonded state
func (k Keeper) unbondingToUnbonded(ctx sdk.Context, validator types.Validator) types.Validator {
	if validator.Status != sdk.Unbonding {
		panic(fmt.Sprintf("bad state transition unbondingToBonded, validator: %v\n", validator))
	}
	return k.completeUnbondingValidator(ctx, validator)
}

// send a validator to jail
// 设置该验证人为 锁定
func (k Keeper) jailValidator(ctx sdk.Context, validator types.Validator) {
	if validator.Jailed {
		panic(fmt.Sprintf("cannot jail already jailed validator, validator: %v\n", validator))
	}

	validator.Jailed = true
	k.SetValidator(ctx, validator)
	k.DeleteValidatorByPowerIndex(ctx, validator)
}

// remove a validator from jail
func (k Keeper) unjailValidator(ctx sdk.Context, validator types.Validator) {
	if !validator.Jailed {
		panic(fmt.Sprintf("cannot unjail already unjailed validator, validator: %v\n", validator))
	}

	validator.Jailed = false
	k.SetValidator(ctx, validator)
	k.SetValidatorByPowerIndex(ctx, validator)
}

// perform all the store operations for when a validator status becomes bonded
func (k Keeper) bondValidator(ctx sdk.Context, validator types.Validator) types.Validator {

	// delete the validator by power index, as the key will change
	k.DeleteValidatorByPowerIndex(ctx, validator)

	// set the status
	pool := k.GetPool(ctx)
	validator, pool = validator.UpdateStatus(pool, sdk.Bonded)
	k.SetPool(ctx, pool)

	// save the now bonded validator record to the two referenced stores
	k.SetValidator(ctx, validator)
	k.SetValidatorByPowerIndex(ctx, validator)

	// delete from queue if present
	k.DeleteValidatorQueue(ctx, validator)

	// trigger hook
	k.AfterValidatorBonded(ctx, validator.ConsAddress(), validator.OperatorAddress)

	return validator
}

// perform all the store operations for when a validator begins unbonding
func (k Keeper) beginUnbondingValidator(ctx sdk.Context, validator types.Validator) types.Validator {

	params := k.GetParams(ctx)

	// delete the validator by power index, as the key will change
	k.DeleteValidatorByPowerIndex(ctx, validator)

	// sanity check
	if validator.Status != sdk.Bonded {
		panic(fmt.Sprintf("should not already be unbonded or unbonding, validator: %v\n", validator))
	}

	// set the status
	pool := k.GetPool(ctx)
	validator, pool = validator.UpdateStatus(pool, sdk.Unbonding)
	k.SetPool(ctx, pool)

	// set the unbonding completion time and completion height appropriately
	validator.UnbondingCompletionTime = ctx.BlockHeader().Time.Add(params.UnbondingTime)
	validator.UnbondingHeight = ctx.BlockHeader().Height

	// save the now unbonded validator record and power index
	k.SetValidator(ctx, validator)
	k.SetValidatorByPowerIndex(ctx, validator)

	// Adds to unbonding validator queue
	k.InsertValidatorQueue(ctx, validator)

	// trigger hook
	k.AfterValidatorBeginUnbonding(ctx, validator.ConsAddress(), validator.OperatorAddress)

	return validator
}

// perform all the store operations for when a validator status becomes unbonded
func (k Keeper) completeUnbondingValidator(ctx sdk.Context, validator types.Validator) types.Validator {
	pool := k.GetPool(ctx)
	validator, pool = validator.UpdateStatus(pool, sdk.Unbonded)
	k.SetPool(ctx, pool)
	k.SetValidator(ctx, validator)
	return validator
}

// map of operator addresses to serialized power
type validatorsByAddr map[[sdk.AddrLen]byte][]byte

// get the last validator set
func (k Keeper) getLastValidatorsByAddr(ctx sdk.Context) validatorsByAddr {
	last := make(validatorsByAddr)
	store := ctx.KVStore(k.storeKey)

	// 返回一个 最新的权重信息的验证人信息 的迭代器
	iterator := sdk.KVStorePrefixIterator(store, LastValidatorPowerKey)
	defer iterator.Close()
	// iterate over the last validator set index
	// 遍历迭代器，组装成 map 返回出去
	for ; iterator.Valid(); iterator.Next() {
		var valAddr [sdk.AddrLen]byte
		// extract the validator address from the key (prefix is 1-byte)
		copy(valAddr[:], iterator.Key()[1:])
		// power bytes is just the value
		powerBytes := iterator.Value()
		last[valAddr] = make([]byte, len(powerBytes))
		copy(last[valAddr][:], powerBytes[:])
	}
	return last
}

// given a map of remaining validators to previous bonded power
// returns the list of validators to be unbonded, sorted by operator address
func sortNoLongerBonded(last validatorsByAddr) [][]byte {
	// sort the map keys for determinism
	noLongerBonded := make([][]byte, len(last))
	index := 0
	for valAddrBytes := range last {
		valAddr := make([]byte, sdk.AddrLen)
		copy(valAddr[:], valAddrBytes[:])
		noLongerBonded[index] = valAddr
		index++
	}
	// sorted by address - order doesn't matter
	sort.SliceStable(noLongerBonded, func(i, j int) bool {
		// -1 means strictly less than
		return bytes.Compare(noLongerBonded[i], noLongerBonded[j]) == -1
	})
	return noLongerBonded
}
