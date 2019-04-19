package keeper

import (
	"bytes"
	"time"

	sdk "my-cosmos/cosmos-sdk/types"
	"my-cosmos/cosmos-sdk/x/staking/types"
)

// return a specific delegation
// 返回指定的委托人
func (k Keeper) GetDelegation(ctx sdk.Context,
	delAddr sdk.AccAddress, valAddr sdk.ValAddress) (
	delegation types.Delegation, found bool) {

	// 根据 委托人地址和 验证人地址从 db拉回委托人信息
	store := ctx.KVStore(k.storeKey)
	// 规则： prefix + delAddr + valAddr
	key := GetDelegationKey(delAddr, valAddr)
	value := store.Get(key)
	if value == nil {
		return delegation, false
	}

	delegation = types.MustUnmarshalDelegation(k.cdc, value)
	return delegation, true
}

// return all delegations used during genesis dump
// 返回在Genesis dump期间使用的所有委托人集
func (k Keeper) GetAllDelegations(ctx sdk.Context) (delegations []types.Delegation) {
	store := ctx.KVStore(k.storeKey)

	// 根据前缀 去DB中找所有委托人
	iterator := sdk.KVStorePrefixIterator(store, DelegationKey)
	defer iterator.Close()

	// 遍历所有委托人信息
	for ; iterator.Valid(); iterator.Next() {

		// 逐个解编码
		delegation := types.MustUnmarshalDelegation(k.cdc, iterator.Value())
		delegations = append(delegations, delegation)
	}
	return delegations
}

// return all delegations to a specific validator. Useful for querier.
func (k Keeper) GetValidatorDelegations(ctx sdk.Context, valAddr sdk.ValAddress) (delegations []types.Delegation) {
	store := ctx.KVStore(k.storeKey)
	iterator := sdk.KVStorePrefixIterator(store, DelegationKey)
	defer iterator.Close()

	for ; iterator.Valid(); iterator.Next() {
		delegation := types.MustUnmarshalDelegation(k.cdc, iterator.Value())
		if delegation.GetValidatorAddr().Equals(valAddr) {
			delegations = append(delegations, delegation)
		}
	}
	return delegations
}

// return a given amount of all the delegations from a delegator
func (k Keeper) GetDelegatorDelegations(ctx sdk.Context, delegator sdk.AccAddress,
	maxRetrieve uint16) (delegations []types.Delegation) {

	delegations = make([]types.Delegation, maxRetrieve)

	store := ctx.KVStore(k.storeKey)
	delegatorPrefixKey := GetDelegationsKey(delegator)
	iterator := sdk.KVStorePrefixIterator(store, delegatorPrefixKey)
	defer iterator.Close()

	i := 0
	for ; iterator.Valid() && i < int(maxRetrieve); iterator.Next() {
		delegation := types.MustUnmarshalDelegation(k.cdc, iterator.Value())
		delegations[i] = delegation
		i++
	}
	return delegations[:i] // trim if the array length < maxRetrieve
}

// set a delegation
func (k Keeper) SetDelegation(ctx sdk.Context, delegation types.Delegation) {
	store := ctx.KVStore(k.storeKey)
	b := types.MustMarshalDelegation(k.cdc, delegation)
	store.Set(GetDelegationKey(delegation.DelegatorAddress, delegation.ValidatorAddress), b)
}

// remove a delegation
func (k Keeper) RemoveDelegation(ctx sdk.Context, delegation types.Delegation) {
	// TODO: Consider calling hooks outside of the store wrapper functions, it's unobvious.
	k.BeforeDelegationRemoved(ctx, delegation.DelegatorAddress, delegation.ValidatorAddress)
	store := ctx.KVStore(k.storeKey)
	store.Delete(GetDelegationKey(delegation.DelegatorAddress, delegation.ValidatorAddress))
}

// return a given amount of all the delegator unbonding-delegations
func (k Keeper) GetUnbondingDelegations(ctx sdk.Context, delegator sdk.AccAddress,
	maxRetrieve uint16) (unbondingDelegations []types.UnbondingDelegation) {

	unbondingDelegations = make([]types.UnbondingDelegation, maxRetrieve)

	store := ctx.KVStore(k.storeKey)
	delegatorPrefixKey := GetUBDsKey(delegator)
	iterator := sdk.KVStorePrefixIterator(store, delegatorPrefixKey)
	defer iterator.Close()

	i := 0
	for ; iterator.Valid() && i < int(maxRetrieve); iterator.Next() {
		unbondingDelegation := types.MustUnmarshalUBD(k.cdc, iterator.Value())
		unbondingDelegations[i] = unbondingDelegation
		i++
	}
	return unbondingDelegations[:i] // trim if the array length < maxRetrieve
}

// return a unbonding delegation
func (k Keeper) GetUnbondingDelegation(ctx sdk.Context,
	delAddr sdk.AccAddress, valAddr sdk.ValAddress) (ubd types.UnbondingDelegation, found bool) {

	store := ctx.KVStore(k.storeKey)
	key := GetUBDKey(delAddr, valAddr)
	value := store.Get(key)
	if value == nil {
		return ubd, false
	}

	ubd = types.MustUnmarshalUBD(k.cdc, value)
	return ubd, true
}

// return all unbonding delegations from a particular validator
func (k Keeper) GetUnbondingDelegationsFromValidator(ctx sdk.Context, valAddr sdk.ValAddress) (ubds []types.UnbondingDelegation) {
	store := ctx.KVStore(k.storeKey)
	iterator := sdk.KVStorePrefixIterator(store, GetUBDsByValIndexKey(valAddr))
	defer iterator.Close()

	for ; iterator.Valid(); iterator.Next() {
		key := GetUBDKeyFromValIndexKey(iterator.Key())
		value := store.Get(key)
		ubd := types.MustUnmarshalUBD(k.cdc, value)
		ubds = append(ubds, ubd)
	}
	return ubds
}

// iterate through all of the unbonding delegations
func (k Keeper) IterateUnbondingDelegations(ctx sdk.Context, fn func(index int64, ubd types.UnbondingDelegation) (stop bool)) {
	store := ctx.KVStore(k.storeKey)
	iterator := sdk.KVStorePrefixIterator(store, UnbondingDelegationKey)
	defer iterator.Close()

	for i := int64(0); iterator.Valid(); iterator.Next() {
		ubd := types.MustUnmarshalUBD(k.cdc, iterator.Value())
		if stop := fn(i, ubd); stop {
			break
		}
		i++
	}
}

// HasMaxUnbondingDelegationEntries - check if unbonding delegation has maximum number of entries
func (k Keeper) HasMaxUnbondingDelegationEntries(ctx sdk.Context,
	delegatorAddr sdk.AccAddress, validatorAddr sdk.ValAddress) bool {

	ubd, found := k.GetUnbondingDelegation(ctx, delegatorAddr, validatorAddr)
	if !found {
		return false
	}
	return len(ubd.Entries) >= int(k.MaxEntries(ctx))
}

// set the unbonding delegation and associated index
func (k Keeper) SetUnbondingDelegation(ctx sdk.Context, ubd types.UnbondingDelegation) {
	store := ctx.KVStore(k.storeKey)
	bz := types.MustMarshalUBD(k.cdc, ubd)
	key := GetUBDKey(ubd.DelegatorAddress, ubd.ValidatorAddress)
	store.Set(key, bz)
	store.Set(GetUBDByValIndexKey(ubd.DelegatorAddress, ubd.ValidatorAddress), []byte{}) // index, store empty bytes
}

// remove the unbonding delegation object and associated index
func (k Keeper) RemoveUnbondingDelegation(ctx sdk.Context, ubd types.UnbondingDelegation) {
	store := ctx.KVStore(k.storeKey)
	key := GetUBDKey(ubd.DelegatorAddress, ubd.ValidatorAddress)
	store.Delete(key)
	store.Delete(GetUBDByValIndexKey(ubd.DelegatorAddress, ubd.ValidatorAddress))
}

// SetUnbondingDelegationEntry adds an entry to the unbonding delegation at
// the given addresses. It creates the unbonding delegation if it does not exist
func (k Keeper) SetUnbondingDelegationEntry(ctx sdk.Context,
	delegatorAddr sdk.AccAddress, validatorAddr sdk.ValAddress,
	creationHeight int64, minTime time.Time, balance sdk.Int) types.UnbondingDelegation {

	ubd, found := k.GetUnbondingDelegation(ctx, delegatorAddr, validatorAddr)
	if found {
		ubd.AddEntry(creationHeight, minTime, balance)
	} else {
		ubd = types.NewUnbondingDelegation(delegatorAddr, validatorAddr, creationHeight, minTime, balance)
	}
	k.SetUnbondingDelegation(ctx, ubd)
	return ubd
}

// unbonding delegation queue timeslice operations

// gets a specific unbonding queue timeslice. A timeslice is a slice of DVPairs
// corresponding to unbonding delegations that expire at a certain time.
func (k Keeper) GetUBDQueueTimeSlice(ctx sdk.Context, timestamp time.Time) (dvPairs []types.DVPair) {
	store := ctx.KVStore(k.storeKey)
	bz := store.Get(GetUnbondingDelegationTimeKey(timestamp))
	if bz == nil {
		return []types.DVPair{}
	}
	k.cdc.MustUnmarshalBinaryLengthPrefixed(bz, &dvPairs)
	return dvPairs
}

// Sets a specific unbonding queue timeslice.
func (k Keeper) SetUBDQueueTimeSlice(ctx sdk.Context, timestamp time.Time, keys []types.DVPair) {
	store := ctx.KVStore(k.storeKey)
	bz := k.cdc.MustMarshalBinaryLengthPrefixed(keys)
	store.Set(GetUnbondingDelegationTimeKey(timestamp), bz)
}

// Insert an unbonding delegation to the appropriate timeslice in the unbonding queue
func (k Keeper) InsertUBDQueue(ctx sdk.Context, ubd types.UnbondingDelegation,
	completionTime time.Time) {

	timeSlice := k.GetUBDQueueTimeSlice(ctx, completionTime)
	dvPair := types.DVPair{DelegatorAddress: ubd.DelegatorAddress, ValidatorAddress: ubd.ValidatorAddress}
	if len(timeSlice) == 0 {
		k.SetUBDQueueTimeSlice(ctx, completionTime, []types.DVPair{dvPair})
	} else {
		timeSlice = append(timeSlice, dvPair)
		k.SetUBDQueueTimeSlice(ctx, completionTime, timeSlice)
	}
}

// Returns all the unbonding queue timeslices from time 0 until endTime
func (k Keeper) UBDQueueIterator(ctx sdk.Context, endTime time.Time) sdk.Iterator {
	store := ctx.KVStore(k.storeKey)
	return store.Iterator(UnbondingQueueKey,
		sdk.InclusiveEndBytes(GetUnbondingDelegationTimeKey(endTime)))
}

// Returns a concatenated list of all the timeslices inclusively previous to
// currTime, and deletes the timeslices from the queue
func (k Keeper) DequeueAllMatureUBDQueue(ctx sdk.Context,
	currTime time.Time) (matureUnbonds []types.DVPair) {

	store := ctx.KVStore(k.storeKey)
	// gets an iterator for all timeslices from time 0 until the current Blockheader time
	unbondingTimesliceIterator := k.UBDQueueIterator(ctx, ctx.BlockHeader().Time)
	for ; unbondingTimesliceIterator.Valid(); unbondingTimesliceIterator.Next() {
		timeslice := []types.DVPair{}
		value := unbondingTimesliceIterator.Value()
		k.cdc.MustUnmarshalBinaryLengthPrefixed(value, &timeslice)
		matureUnbonds = append(matureUnbonds, timeslice...)
		store.Delete(unbondingTimesliceIterator.Key())
	}
	return matureUnbonds
}

// return a given amount of all the delegator redelegations
func (k Keeper) GetRedelegations(ctx sdk.Context, delegator sdk.AccAddress,
	maxRetrieve uint16) (redelegations []types.Redelegation) {
	redelegations = make([]types.Redelegation, maxRetrieve)

	store := ctx.KVStore(k.storeKey)
	delegatorPrefixKey := GetREDsKey(delegator)
	iterator := sdk.KVStorePrefixIterator(store, delegatorPrefixKey)
	defer iterator.Close()

	i := 0
	for ; iterator.Valid() && i < int(maxRetrieve); iterator.Next() {
		redelegation := types.MustUnmarshalRED(k.cdc, iterator.Value())
		redelegations[i] = redelegation
		i++
	}
	return redelegations[:i] // trim if the array length < maxRetrieve
}

// return a redelegation
func (k Keeper) GetRedelegation(ctx sdk.Context,
	delAddr sdk.AccAddress, valSrcAddr, valDstAddr sdk.ValAddress) (red types.Redelegation, found bool) {

	store := ctx.KVStore(k.storeKey)
	key := GetREDKey(delAddr, valSrcAddr, valDstAddr)
	value := store.Get(key)
	if value == nil {
		return red, false
	}

	red = types.MustUnmarshalRED(k.cdc, value)
	return red, true
}

// return all redelegations from a particular validator
func (k Keeper) GetRedelegationsFromValidator(ctx sdk.Context, valAddr sdk.ValAddress) (reds []types.Redelegation) {
	store := ctx.KVStore(k.storeKey)
	iterator := sdk.KVStorePrefixIterator(store, GetREDsFromValSrcIndexKey(valAddr))
	defer iterator.Close()

	for ; iterator.Valid(); iterator.Next() {
		key := GetREDKeyFromValSrcIndexKey(iterator.Key())
		value := store.Get(key)
		red := types.MustUnmarshalRED(k.cdc, value)
		reds = append(reds, red)
	}
	return reds
}

// check if validator is receiving a redelegation
func (k Keeper) HasReceivingRedelegation(ctx sdk.Context,
	delAddr sdk.AccAddress, valDstAddr sdk.ValAddress) bool {

	store := ctx.KVStore(k.storeKey)
	prefix := GetREDsByDelToValDstIndexKey(delAddr, valDstAddr)
	iterator := sdk.KVStorePrefixIterator(store, prefix)
	defer iterator.Close()

	return iterator.Valid()
}

// HasMaxRedelegationEntries - redelegation has maximum number of entries
func (k Keeper) HasMaxRedelegationEntries(ctx sdk.Context,
	delegatorAddr sdk.AccAddress, validatorSrcAddr,
	validatorDstAddr sdk.ValAddress) bool {

	red, found := k.GetRedelegation(ctx, delegatorAddr, validatorSrcAddr, validatorDstAddr)
	if !found {
		return false
	}
	return len(red.Entries) >= int(k.MaxEntries(ctx))
}

// set a redelegation and associated index
func (k Keeper) SetRedelegation(ctx sdk.Context, red types.Redelegation) {
	store := ctx.KVStore(k.storeKey)
	bz := types.MustMarshalRED(k.cdc, red)
	key := GetREDKey(red.DelegatorAddress, red.ValidatorSrcAddress, red.ValidatorDstAddress)
	store.Set(key, bz)
	store.Set(GetREDByValSrcIndexKey(red.DelegatorAddress, red.ValidatorSrcAddress, red.ValidatorDstAddress), []byte{})
	store.Set(GetREDByValDstIndexKey(red.DelegatorAddress, red.ValidatorSrcAddress, red.ValidatorDstAddress), []byte{})
}

// SetUnbondingDelegationEntry adds an entry to the unbonding delegation at
// the given addresses. It creates the unbonding delegation if it does not exist
func (k Keeper) SetRedelegationEntry(ctx sdk.Context,
	delegatorAddr sdk.AccAddress, validatorSrcAddr,
	validatorDstAddr sdk.ValAddress, creationHeight int64,
	minTime time.Time, balance sdk.Int,
	sharesSrc, sharesDst sdk.Dec) types.Redelegation {

	red, found := k.GetRedelegation(ctx, delegatorAddr, validatorSrcAddr, validatorDstAddr)
	if found {
		red.AddEntry(creationHeight, minTime, balance, sharesDst)
	} else {
		red = types.NewRedelegation(delegatorAddr, validatorSrcAddr,
			validatorDstAddr, creationHeight, minTime, balance, sharesDst)
	}
	k.SetRedelegation(ctx, red)
	return red
}

// iterate through all redelegations
func (k Keeper) IterateRedelegations(ctx sdk.Context, fn func(index int64, red types.Redelegation) (stop bool)) {
	store := ctx.KVStore(k.storeKey)
	iterator := sdk.KVStorePrefixIterator(store, RedelegationKey)
	defer iterator.Close()

	for i := int64(0); iterator.Valid(); iterator.Next() {
		red := types.MustUnmarshalRED(k.cdc, iterator.Value())
		if stop := fn(i, red); stop {
			break
		}
		i++
	}
}

// remove a redelegation object and associated index
func (k Keeper) RemoveRedelegation(ctx sdk.Context, red types.Redelegation) {
	store := ctx.KVStore(k.storeKey)
	redKey := GetREDKey(red.DelegatorAddress, red.ValidatorSrcAddress, red.ValidatorDstAddress)
	store.Delete(redKey)
	store.Delete(GetREDByValSrcIndexKey(red.DelegatorAddress, red.ValidatorSrcAddress, red.ValidatorDstAddress))
	store.Delete(GetREDByValDstIndexKey(red.DelegatorAddress, red.ValidatorSrcAddress, red.ValidatorDstAddress))
}

// redelegation queue timeslice operations

// Gets a specific redelegation queue timeslice. A timeslice is a slice of DVVTriplets corresponding to redelegations
// that expire at a certain time.
func (k Keeper) GetRedelegationQueueTimeSlice(ctx sdk.Context, timestamp time.Time) (dvvTriplets []types.DVVTriplet) {
	store := ctx.KVStore(k.storeKey)
	bz := store.Get(GetRedelegationTimeKey(timestamp))
	if bz == nil {
		return []types.DVVTriplet{}
	}
	k.cdc.MustUnmarshalBinaryLengthPrefixed(bz, &dvvTriplets)
	return dvvTriplets
}

// Sets a specific redelegation queue timeslice.
func (k Keeper) SetRedelegationQueueTimeSlice(ctx sdk.Context, timestamp time.Time, keys []types.DVVTriplet) {
	store := ctx.KVStore(k.storeKey)
	bz := k.cdc.MustMarshalBinaryLengthPrefixed(keys)
	store.Set(GetRedelegationTimeKey(timestamp), bz)
}

// Insert an redelegation delegation to the appropriate timeslice in the redelegation queue
func (k Keeper) InsertRedelegationQueue(ctx sdk.Context, red types.Redelegation,
	completionTime time.Time) {

	timeSlice := k.GetRedelegationQueueTimeSlice(ctx, completionTime)
	dvvTriplet := types.DVVTriplet{
		DelegatorAddress:    red.DelegatorAddress,
		ValidatorSrcAddress: red.ValidatorSrcAddress,
		ValidatorDstAddress: red.ValidatorDstAddress}

	if len(timeSlice) == 0 {
		k.SetRedelegationQueueTimeSlice(ctx, completionTime, []types.DVVTriplet{dvvTriplet})
	} else {
		timeSlice = append(timeSlice, dvvTriplet)
		k.SetRedelegationQueueTimeSlice(ctx, completionTime, timeSlice)
	}
}

// Returns all the redelegation queue timeslices from time 0 until endTime
func (k Keeper) RedelegationQueueIterator(ctx sdk.Context, endTime time.Time) sdk.Iterator {
	store := ctx.KVStore(k.storeKey)
	return store.Iterator(RedelegationQueueKey, sdk.InclusiveEndBytes(GetRedelegationTimeKey(endTime)))
}

// Returns a concatenated list of all the timeslices inclusively previous to
// currTime, and deletes the timeslices from the queue
func (k Keeper) DequeueAllMatureRedelegationQueue(ctx sdk.Context, currTime time.Time) (matureRedelegations []types.DVVTriplet) {
	store := ctx.KVStore(k.storeKey)
	// gets an iterator for all timeslices from time 0 until the current Blockheader time
	redelegationTimesliceIterator := k.RedelegationQueueIterator(ctx, ctx.BlockHeader().Time)
	for ; redelegationTimesliceIterator.Valid(); redelegationTimesliceIterator.Next() {
		timeslice := []types.DVVTriplet{}
		value := redelegationTimesliceIterator.Value()
		k.cdc.MustUnmarshalBinaryLengthPrefixed(value, &timeslice)
		matureRedelegations = append(matureRedelegations, timeslice...)
		store.Delete(redelegationTimesliceIterator.Key())
	}
	return matureRedelegations
}

// Perform a delegation, set/update everything necessary within the store.
/**
执行委托，设置/更新 store 中的所需一切。
 */
func (k Keeper) Delegate(ctx sdk.Context, delAddr sdk.AccAddress, bondAmt sdk.Int,
	validator types.Validator, subtractAccount bool) (newShares sdk.Dec, err sdk.Error) {

	// In some situations, the exchange rate becomes invalid, e.g. if
	// Validator loses all tokens due to slashing. In this case,
	// make all future delegations invalid.
	/**
	在某些情况下，佣金增幅 变为无效 ?
	例如 如果Validator由于削减而丢失所有令牌。 在这种情况下，使所有未来的代表团无效。
	*/
	if validator.InvalidExRate() {
		return sdk.ZeroDec(), types.ErrDelegatorShareExRateInvalid(k.Codespace())
	}



	// Get or create the delegation object
	// 获取或者创建一个 委托人实例
	// 根据委托人地址和验证人地址
	delegation, found := k.GetDelegation(ctx, delAddr, validator.OperatorAddress)
	if !found {
		// 如果找不到就创建一个
		delegation = types.NewDelegation(delAddr, validator.OperatorAddress, sdk.ZeroDec())
	}

	// call the appropriate hook if present
	// 调用对应的钩子函数
	// 如果找得到，那么是 修改委托
	if found {

		// 其实是调用app包的, BeforeDelegationSharesModified 函数
		// TODO 这一步超级重要， 里面处理的逻辑非常多
		// ################
		// ################
		// 这个函数主要处理的是  withdrawDelegationRewards
		// 退回委托奖励
		// TODO 对于发起一个委托来说，就是先要解除之前委托的奖励，重置委托
		k.BeforeDelegationSharesModified(ctx, delAddr, validator.OperatorAddress)
	} else {
		// 如果找不到，那么就是首次委托
		// 其实是调用app包的, BeforeDelegationCreated 函数
		k.BeforeDelegationCreated(ctx, delAddr, validator.OperatorAddress)
	}


	// 是否 减账户，标示？
	// 是否需要扣减 发起委托的 账户地址上的金额
	if subtractAccount {

		// 这里的 bankKeeper 是 bank.NewBaseKeeper 实现的
		_, err := k.bankKeeper.DelegateCoins(ctx, delegation.DelegatorAddress, sdk.Coins{sdk.NewCoin(k.GetParams(ctx).BondDenom, bondAmt)})
		if err != nil {
			return sdk.Dec{}, err
		}
	}

	/***
	###############
	###############
	将本次委托的金额关联到该验证人身上
	###############
	###############
	 */
	validator, newShares = k.AddValidatorTokensAndShares(ctx, validator, bondAmt)

	// Update delegation
	delegation.Shares = delegation.Shares.Add(newShares)
	// todo 设置 委托信息
	k.SetDelegation(ctx, delegation)

	// Call the after-modification hook
	// 其实是调用 app包的, BeforeDelegationCreated 函数
	// 其实里头最终只做了一件事, 初始化 委托起始信息
	k.AfterDelegationModified(ctx, delegation.DelegatorAddress, delegation.ValidatorAddress)

	return newShares, nil
}

// unbond a particular delegation and perform associated store operations
// 取消一个指定的委托人 和 写入存储中的动作 ？
func (k Keeper) unbond(ctx sdk.Context, delAddr sdk.AccAddress, valAddr sdk.ValAddress,
	shares sdk.Dec) (amount sdk.Int, err sdk.Error) {

	// check if a delegation object exists in the store
	// 检查该委托人是否存在 存储中
	delegation, found := k.GetDelegation(ctx, delAddr, valAddr)
	if !found {
		return amount, types.ErrNoDelegatorForAddress(k.Codespace())
	}

	// call the before-delegation-modified hook
	// AOP 钩子函数
	// ###############
	// ###############
	// ###############
	// TODO 这个是更新委托人相关的函数， 非常重要的一步，逻辑非常多
	// 这个函数主要处理的是  withdrawDelegationRewards
	k.BeforeDelegationSharesModified(ctx, delAddr, valAddr)

	// ensure that we have enough shares to remove
	// 确保之前委托的钱不比当前解除委托入参的钱小
	// 不能导致 多退
	if delegation.Shares.LT(shares) {
		return amount, types.ErrNotEnoughDelegationShares(k.Codespace(), delegation.Shares.String())
	}

	// get validator
	// 看看有没有验证人
	// 验证人都找不到，说明数据有毒啊
	// 那就不能解除委托
	validator, found := k.GetValidator(ctx, valAddr)
	if !found {
		return amount, types.ErrNoValidatorFound(k.Codespace())
	}

	// subtract shares from delegation
	// 减持委托的钱
	delegation.Shares = delegation.Shares.Sub(shares)

	isValidatorOperator := bytes.Equal(delegation.DelegatorAddress, validator.OperatorAddress)

	// if the delegation is the operator of the validator and undelegating will decrease the validator's self delegation below their minimum
	// trigger a jail validator
	//
	// 如果委托人地址就是是验证人地址，并且解除委托的钱会将验证者的自我委托减少到最小的质押限制？
	if isValidatorOperator && !validator.Jailed &&
		// 如果减持后的钱小于 质押的最小门槛
		validator.ShareTokens(delegation.Shares).TruncateInt().LT(validator.MinSelfDelegation) {

		// 封闭掉 验证人？？
		k.jailValidator(ctx, validator)

		// 获取验证人 信息
		validator = k.mustGetValidator(ctx, validator.OperatorAddress)
	}

	// remove the delegation
	// 如果当前委托人的委托的钱为 0, 则需要删除当前委托人信息
	if delegation.Shares.IsZero() {
		k.RemoveDelegation(ctx, delegation)
	} else {
		// 否则设置减持操作之后的当前委托人
		k.SetDelegation(ctx, delegation)
		// call the after delegation modification hook
		// 其实就是：初始化新委托的起始信息
		k.AfterDelegationModified(ctx, delegation.DelegatorAddress, delegation.ValidatorAddress)
	}

	// remove the shares and coins from the validator
	// 同时减少 验证人信息中的委托信息
	validator, amount = k.RemoveValidatorTokensAndShares(ctx, validator, shares)

	// 如果当前扣减完 委托金之后的验证人上的委托金额等于0 且 当前验证人属于 【未被绑定】 状态
	if validator.DelegatorShares.IsZero() && validator.Status == sdk.Unbonded {
		// if not unbonded, we must instead remove validator in EndBlocker once it finishes its unbonding period
		// 如果没有未绑定，一旦完成其未绑定期，我们必须在EndBlocker (区块处理完时)中删除验证器
		k.RemoveValidator(ctx, validator.OperatorAddress)
	}


	// 返回 验证人身上剩余的 token
	return amount, nil
}

// get info for begin functions: completionTime and CreationHeight
// 获取begin函数的信息：
// completionTime和CreationHeight： 完成的时间 和 创建时的块高
func (k Keeper) getBeginInfo(ctx sdk.Context, valSrcAddr sdk.ValAddress) (
	completionTime time.Time, height int64, completeNow bool) {


	// 根据 验证人地址获取 验证人
	validator, found := k.GetValidator(ctx, valSrcAddr)

	switch {
	// TODO: when would the validator not be found?
	// 什么时候会找不到验证人？？
	// 当验证人找不到 或者 该验证人被锁定时
	case !found || validator.Status == sdk.Bonded:

		// the longest wait - just unbonding period from now
		// 最长的等待 - 从现在开始进入解锁周期
		// 当前区块的时间戳 + 解锁的时间
		completionTime = ctx.BlockHeader().Time.Add(k.UnbondingTime(ctx))
		// 当前 块高
		height = ctx.BlockHeight()
		return completionTime, height, false

	/**
	如果当前验证人为 未被锁定状态
	 */
	case validator.Status == sdk.Unbonded:
		// 直接返回入参的 当前完成时间和块高, 及 是否是现在完成的标识位 true
		return completionTime, height, true

	/**
	如果 当前验证人为 解锁状态
	 */
	case validator.Status == sdk.Unbonding:
		// 使用验证人的 解锁更新时间作为 完成时间
		completionTime = validator.UnbondingCompletionTime
		// 使用验证人的 解锁块高作为 解锁块高
		height = validator.UnbondingHeight
		return completionTime, height, false

	default:
		panic("unknown validator status")
	}
}

// begin unbonding part or all of a delegation
// 开始解除部分或全部全部的委托
// TODO 超级重要的一步
func (k Keeper) Undelegate(ctx sdk.Context, delAddr sdk.AccAddress,
	valAddr sdk.ValAddress, sharesAmount sdk.Dec) (completionTime time.Time, sdkErr sdk.Error) {

	// create the unbonding delegation
	// 创建 解锁的委托信息
	// 获得验证人的 unbonding的completionTime和CreationHeight： 完成的时间 和 创建时的块高
	completionTime, height, completeNow := k.getBeginInfo(ctx, valAddr)

	// 更新 全局的Keeper 管理器的信息
	// 减持当前 sharesAmount 数额的委托金
	// returnAmount： 验证人身上剩余的 token
	returnAmount, err := k.unbond(ctx, delAddr, valAddr, sharesAmount)
	if err != nil {
		return completionTime, err
	}

	// 根据验证人身上剩余的 token 创建一个 coin实例
	balance := sdk.NewCoin(k.BondDenom(ctx), returnAmount)

	// no need to create the ubd object just complete now
	if completeNow {
		// track undelegation only when remaining or truncated shares are non-zero
		if !balance.IsZero() {
			if _, err := k.bankKeeper.UndelegateCoins(ctx, delAddr, sdk.Coins{balance}); err != nil {
				return completionTime, err
			}
		}

		return completionTime, nil
	}

	if k.HasMaxUnbondingDelegationEntries(ctx, delAddr, valAddr) {
		return time.Time{}, types.ErrMaxUnbondingDelegationEntries(k.Codespace())
	}

	ubd := k.SetUnbondingDelegationEntry(ctx, delAddr,
		valAddr, height, completionTime, returnAmount)

	k.InsertUBDQueue(ctx, ubd, completionTime)
	return completionTime, nil
}

// CompleteUnbonding completes the unbonding of all mature entries in the
// retrieved unbonding delegation object.
func (k Keeper) CompleteUnbonding(ctx sdk.Context, delAddr sdk.AccAddress,
	valAddr sdk.ValAddress) sdk.Error {

	ubd, found := k.GetUnbondingDelegation(ctx, delAddr, valAddr)
	if !found {
		return types.ErrNoUnbondingDelegation(k.Codespace())
	}

	ctxTime := ctx.BlockHeader().Time

	// loop through all the entries and complete unbonding mature entries
	for i := 0; i < len(ubd.Entries); i++ {
		entry := ubd.Entries[i]
		if entry.IsMature(ctxTime) {
			ubd.RemoveEntry(int64(i))
			i--

			// track undelegation only when remaining or truncated shares are non-zero
			if !entry.Balance.IsZero() {
				_, err := k.bankKeeper.UndelegateCoins(ctx, ubd.DelegatorAddress, sdk.Coins{sdk.NewCoin(k.GetParams(ctx).BondDenom, entry.Balance)})
				if err != nil {
					return err
				}
			}
		}
	}

	// set the unbonding delegation or remove it if there are no more entries
	if len(ubd.Entries) == 0 {
		k.RemoveUnbondingDelegation(ctx, ubd)
	} else {
		k.SetUnbondingDelegation(ctx, ubd)
	}

	return nil
}

// begin unbonding / redelegation; create a redelegation record
func (k Keeper) BeginRedelegation(ctx sdk.Context, delAddr sdk.AccAddress,
	valSrcAddr, valDstAddr sdk.ValAddress, sharesAmount sdk.Dec) (
	completionTime time.Time, errSdk sdk.Error) {

	if bytes.Equal(valSrcAddr, valDstAddr) {
		return time.Time{}, types.ErrSelfRedelegation(k.Codespace())
	}

	// check if this is a transitive redelegation
	if k.HasReceivingRedelegation(ctx, delAddr, valSrcAddr) {
		return time.Time{}, types.ErrTransitiveRedelegation(k.Codespace())
	}

	if k.HasMaxRedelegationEntries(ctx, delAddr, valSrcAddr, valDstAddr) {
		return time.Time{}, types.ErrMaxRedelegationEntries(k.Codespace())
	}

	returnAmount, err := k.unbond(ctx, delAddr, valSrcAddr, sharesAmount)
	if err != nil {
		return time.Time{}, err
	}

	if returnAmount.IsZero() {
		return time.Time{}, types.ErrVerySmallRedelegation(k.Codespace())
	}
	dstValidator, found := k.GetValidator(ctx, valDstAddr)
	if !found {
		return time.Time{}, types.ErrBadRedelegationDst(k.Codespace())
	}

	sharesCreated, err := k.Delegate(ctx, delAddr, returnAmount, dstValidator, false)
	if err != nil {
		return time.Time{}, err
	}

	// create the unbonding delegation
	completionTime, height, completeNow := k.getBeginInfo(ctx, valSrcAddr)

	if completeNow { // no need to create the redelegation object
		return completionTime, nil
	}

	red := k.SetRedelegationEntry(ctx, delAddr, valSrcAddr, valDstAddr,
		height, completionTime, returnAmount, sharesAmount, sharesCreated)
	k.InsertRedelegationQueue(ctx, red, completionTime)
	return completionTime, nil
}

// CompleteRedelegation completes the unbonding of all mature entries in the
// retrieved unbonding delegation object.
func (k Keeper) CompleteRedelegation(ctx sdk.Context, delAddr sdk.AccAddress,
	valSrcAddr, valDstAddr sdk.ValAddress) sdk.Error {

	red, found := k.GetRedelegation(ctx, delAddr, valSrcAddr, valDstAddr)
	if !found {
		return types.ErrNoRedelegation(k.Codespace())
	}

	ctxTime := ctx.BlockHeader().Time

	// loop through all the entries and complete mature redelegation entries
	for i := 0; i < len(red.Entries); i++ {
		entry := red.Entries[i]
		if entry.IsMature(ctxTime) {
			red.RemoveEntry(int64(i))
			i--
		}
	}

	// set the redelegation or remove it if there are no more entries
	if len(red.Entries) == 0 {
		k.RemoveRedelegation(ctx, red)
	} else {
		k.SetRedelegation(ctx, red)
	}

	return nil
}
