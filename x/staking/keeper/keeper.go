package keeper

import (
	"container/list"

	"my-cosmos/cosmos-sdk/codec"
	sdk "my-cosmos/cosmos-sdk/types"

	"my-cosmos/cosmos-sdk/x/params"
	"my-cosmos/cosmos-sdk/x/staking/types"
)

const aminoCacheSize = 500

// keeper of the staking store
/**
经济模型的 管理器 (这个东西是全局的)
 */
type Keeper struct {

	// DB 类型的 key？
	storeKey           sdk.StoreKey
	storeTKey          sdk.StoreKey

	// 编码解码器
	cdc                *codec.Codec

	// 这个是管理账户的资产转移的管理器 ??
	bankKeeper         types.BankKeeper

	// 钩子的定义 (AOP一样的存在
	hooks              sdk.StakingHooks

	// 一个参数仓库？
	paramstore         params.Subspace

	/** TODO 下面这两个全局缓存用来，缓存验证人的 **/

	/** 全局保存的见证人 缓存 */
	validatorCache     map[string]cachedValidator
	// 一个go拓展库，list库
	// 这里用来存储 验证人列表
	validatorCacheList *list.List

	// codespace
	codespace sdk.CodespaceType
}

func NewKeeper(cdc *codec.Codec, key, tkey sdk.StoreKey, bk types.BankKeeper,
	paramstore params.Subspace, codespace sdk.CodespaceType) Keeper {

	keeper := Keeper{
		storeKey:           key,
		storeTKey:          tkey,
		cdc:                cdc,
		bankKeeper:         bk,
		paramstore:         paramstore.WithKeyTable(ParamKeyTable()),
		hooks:              nil,
		validatorCache:     make(map[string]cachedValidator, aminoCacheSize),
		validatorCacheList: list.New(),
		codespace:          codespace,
	}
	return keeper
}

// Set the validator hooks
func (k *Keeper) SetHooks(sh sdk.StakingHooks) *Keeper {
	if k.hooks != nil {
		panic("cannot set validator hooks twice")
	}
	k.hooks = sh
	return k
}

// return the codespace
func (k Keeper) Codespace() sdk.CodespaceType {
	return k.codespace
}

// get the pool
// 获取 Pool 实例
// Pool 记录这目前与验证人锁定和未锁定的流通 币数目
func (k Keeper) GetPool(ctx sdk.Context) (pool types.Pool) {
	store := ctx.KVStore(k.storeKey)
	b := store.Get(PoolKey)
	if b == nil {
		panic("stored pool should not have been nil")
	}
	k.cdc.MustUnmarshalBinaryLengthPrefixed(b, &pool)
	return
}

// set the pool
func (k Keeper) SetPool(ctx sdk.Context, pool types.Pool) {
	store := ctx.KVStore(k.storeKey)
	b := k.cdc.MustMarshalBinaryLengthPrefixed(pool)
	store.Set(PoolKey, b)
}

// Load the last total validator power.
func (k Keeper) GetLastTotalPower(ctx sdk.Context) (power sdk.Int) {
	store := ctx.KVStore(k.storeKey)
	b := store.Get(LastTotalPowerKey)
	if b == nil {
		return sdk.ZeroInt()
	}
	k.cdc.MustUnmarshalBinaryLengthPrefixed(b, &power)
	return
}

// Set the last total validator power.
func (k Keeper) SetLastTotalPower(ctx sdk.Context, power sdk.Int) {
	store := ctx.KVStore(k.storeKey)
	b := k.cdc.MustMarshalBinaryLengthPrefixed(power)
	store.Set(LastTotalPowerKey, b)
}
