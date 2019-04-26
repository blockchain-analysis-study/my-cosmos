package slashing

import (
	"fmt"
	"time"

	"github.com/tendermint/tendermint/crypto"

	"my-cosmos/cosmos-sdk/codec"
	sdk "my-cosmos/cosmos-sdk/types"
	"my-cosmos/cosmos-sdk/x/params"
)

// Keeper of the slashing store
type Keeper struct {
	storeKey     sdk.StoreKey
	cdc          *codec.Codec
	validatorSet sdk.ValidatorSet
	paramspace   params.Subspace

	// codespace
	codespace sdk.CodespaceType
}

// NewKeeper creates a slashing keeper
func NewKeeper(cdc *codec.Codec, key sdk.StoreKey, vs sdk.ValidatorSet, paramspace params.Subspace, codespace sdk.CodespaceType) Keeper {
	keeper := Keeper{
		storeKey:     key,
		cdc:          cdc,
		validatorSet: vs,
		paramspace:   paramspace.WithKeyTable(ParamKeyTable()),
		codespace:    codespace,
	}
	return keeper
}

// handle a validator signing two blocks at the same height
// power: power of the double-signing validator at the height of infraction
/**
TODO 惩罚 双签名的验证人
处理在同一高度签署两个块的验证人
power：双重签名验证器在违规高峰时的功率
 */
func (k Keeper) handleDoubleSign(ctx sdk.Context, addr crypto.Address, infractionHeight int64, timestamp time.Time, power int64) {
	logger := ctx.Logger().With("module", "x/slashing")

	// calculate the age of the evidence
	// 计算证据的年龄
	time := ctx.BlockHeader().Time
	age := time.Sub(timestamp)

	// fetch the validator public key
	// 获取验证人的公钥
	consAddr := sdk.ConsAddress(addr)
	pubkey, err := k.getPubkey(ctx, addr)
	if err != nil {
		// Ignore evidence that cannot be handled.
		// NOTE:
		// We used to panic with:
		// `panic(fmt.Sprintf("Validator consensus-address %v not found", consAddr))`,
		// but this couples the expectations of the app to both Tendermint and
		// the simulator.  Both are expected to provide the full range of
		// allowable but none of the disallowed evidence types.  Instead of
		// getting this coordination right, it is easier to relax the
		// constraints and ignore evidence that cannot be handled.
		return
	}

	// Reject evidence if the double-sign is too old
	// 如果双重标志太旧，则拒绝证据
	if age > k.MaxEvidenceAge(ctx) {
		logger.Info(fmt.Sprintf("Ignored double sign from %s at height %d, age of %d past max age of %d",
			pubkey.Address(), infractionHeight, age, k.MaxEvidenceAge(ctx)))
		return
	}

	// Get validator and signing info
	// 获取验证者和签名信息
	validator := k.validatorSet.ValidatorByConsAddr(ctx, consAddr)
	if validator == nil || validator.GetStatus() == sdk.Unbonded {
		// Defensive.
		// Simulation doesn't take unbonding periods into account, and
		// Tendermint might break this assumption at some point.
		/**
		防守。
		模拟不考虑未绑定周期，Tendermint可能会在某些时候打破这种假设。
		 */
		return
	}

	// fetch the validator signing info
	// 获取验证人的签名信息
	signInfo, found := k.getValidatorSigningInfo(ctx, consAddr)
	if !found {
		panic(fmt.Sprintf("Expected signing info for validator %s but not found", consAddr))
	}

	// validator is already tombstoned
	// 验证器已经被墓碑化了
	//
	// 就是说： 验证器是否已被逻辑删除（从验证器集中删除）
	if signInfo.Tombstoned {
		logger.Info(fmt.Sprintf("Ignored double sign from %s at height %d, validator already tombstoned", pubkey.Address(), infractionHeight))
		return
	}

	// double sign confirmed
	// 确认了是 双签
	logger.Info(fmt.Sprintf("Confirmed double sign from %s at height %d, age of %d", pubkey.Address(), infractionHeight, age))

	// We need to retrieve the stake distribution which signed the block, so we subtract ValidatorUpdateDelay from the evidence height.
	// Note that this *can* result in a negative "distributionHeight", up to -ValidatorUpdateDelay,
	// i.e. at the end of the pre-genesis block (none) = at the beginning of the genesis block.
	// That's fine since this is just used to filter unbonding delegations & redelegations.
	/**
	我们需要检索对块进行签名的桩号分布，因此我们从证据高度中减去ValidatorUpdateDelay。
	请注意，此* * *会导致负“distributionHeight”，最多为
	 ValidatorUpdateDelay，
	即在发生前的阻断结束时（无）=在发生阻滞的开始。
	这很好，因为这只是用来过滤无约束的授权和重新授权。
	 */
	distributionHeight := infractionHeight - sdk.ValidatorUpdateDelay

	// get the percentage slash penalty fraction
	// 得到百分比 惩罚分数
	fraction := k.SlashFractionDoubleSign(ctx)

	// Slash validator
	// `power` is the int64 power of the validator as provided to/by
	// Tendermint. This value is validator.Tokens as sent to Tendermint via
	// ABCI, and now received as evidence.
	// The fraction is passed in to separately to slash unbonding and rebonding delegations.
	/**
	惩罚验证器`power`是由Tendermint提供的验证人的int64权重。
	此值为validator.Tokens通过ABCI发送给Tendermint，现在作为 凭证被接收。
	将该分数分别传入以 惩罚未绑定和重新绑定的委托人s。
	 */
	k.validatorSet.Slash(ctx, consAddr, distributionHeight, power, fraction)

	// Jail validator if not already jailed
	// begin unbonding validator if not already unbonding (tombstone)
	/**
	如果在开始解除锁定时还没被监禁的 和如果已经 解锁的(已经被清除掉的验证人)，则监禁他
	 */
	if !validator.GetJailed() {
		/**
		TODO 进行惩罚锁定
		 */
		k.validatorSet.Jail(ctx, consAddr)
	}

	// Set tombstoned to be true
	// 设置该验证人的签名信息为 (该验证人已经被移除的标识)
	signInfo.Tombstoned = true

	// Set jailed until to be forever (max time)
	// 设置为监禁，直至永远（最长时间）
	signInfo.JailedUntil = DoubleSignJailEndTime

	// Set validator signing info
	// 保存 该验证人的签名信息
	k.SetValidatorSigningInfo(ctx, consAddr, signInfo)
}

// handle a validator signature, must be called once per validator per block
// TODO refactor to take in a consensus address, additionally should maybe just take in the pubkey too
/**
处理验证器签名，每个块每个验证器必须调用一次
TODO 重构采取共识地址，另外也许应该只是接受pubKey
 */
func (k Keeper) handleValidatorSignature(ctx sdk.Context, addr crypto.Address, power int64, signed bool) {
	logger := ctx.Logger().With("module", "x/slashing")
	// 获取当前块高
	height := ctx.BlockHeight()
	// 共识地址
	consAddr := sdk.ConsAddress(addr)
	// 获取 公钥
	pubkey, err := k.getPubkey(ctx, addr)
	if err != nil {
		panic(fmt.Sprintf("Validator consensus-address %v not found", consAddr))
	}

	// fetch signing info
	// 根据 验证人的共识addr 获取他对该block的签名
	signInfo, found := k.getValidatorSigningInfo(ctx, consAddr)
	if !found {
		panic(fmt.Sprintf("Expected signing info for validator %s but not found", consAddr))
	}

	// this is a relative index, so it counts blocks the validator *should* have signed
	// will use the 0-value default signing info if not present, except for start height
	/**
	这是一个相对索引，所以它计算块验证人 *应该* 已经签名将使用0值默认签名信息（如果不存在），除了起始高度
	 */
	// 当前索引 % 当前slash的时间窗口 ？？
	index := signInfo.IndexOffset % k.SignedBlocksWindow(ctx)
	signInfo.IndexOffset++

	// Update signed block bit array & counter
	// This counter just tracks the sum of the bit array
	// That way we avoid needing to read/write the whole array each time
	/**
	更新已签名的block 位 数组和计数器
	该计数器仅跟踪位阵列的总和
	这样我们就可以避免每次都需要读/写整个数组
	 */
	previous := k.getValidatorMissedBlockBitArray(ctx, consAddr, index)
	missed := !signed
	switch {
	case !previous && missed:
		// Array value has changed from not missed to missed, increment counter
		// 数组值已从未错过更改为错过，增量计数器
		// 啥呀 ？？
		k.setValidatorMissedBlockBitArray(ctx, consAddr, index, true)
		signInfo.MissedBlocksCounter++
	case previous && !missed:
		// Array value has changed from missed to not missed, decrement counter
		// 数组值已从错过更改为未错过，递减计数器
		// 又是啥啊？？
		k.setValidatorMissedBlockBitArray(ctx, consAddr, index, false)
		signInfo.MissedBlocksCounter--
	default:
		// Array value at this index has not changed, no need to update counter
	}

	if missed {
		logger.Info(fmt.Sprintf("Absent validator %s (%v) at height %d, %d missed, threshold %d", addr, pubkey, height, signInfo.MissedBlocksCounter, k.MinSignedPerWindow(ctx)))
	}

	minHeight := signInfo.StartHeight + k.SignedBlocksWindow(ctx)
	maxMissed := k.SignedBlocksWindow(ctx) - k.MinSignedPerWindow(ctx)

	// if we are past the minimum height and the validator has missed too many blocks, punish them
	/**
	如果我们超过最小高度并且验证人错过了太多的块，则惩罚它们
	 */
	if height > minHeight && signInfo.MissedBlocksCounter > maxMissed {
		validator := k.validatorSet.ValidatorByConsAddr(ctx, consAddr)
		if validator != nil && !validator.GetJailed() {

			// Downtime confirmed: slash and jail the validator
			// 确认停机时间：对验证器进行 惩罚和监禁
			logger.Info(fmt.Sprintf("Validator %s past min height of %d and below signed blocks threshold of %d",
				pubkey.Address(), minHeight, k.MinSignedPerWindow(ctx)))

			// We need to retrieve the stake distribution which signed the block, so we subtract ValidatorUpdateDelay from the evidence height,
			// and subtract an additional 1 since this is the LastCommit.
			// Note that this *can* result in a negative "distributionHeight" up to -ValidatorUpdateDelay-1,
			// i.e. at the end of the pre-genesis block (none) = at the beginning of the genesis block.
			// That's fine since this is just used to filter unbonding delegations & redelegations.
			/**
			我们需要检索对块进行签名的桩号分布，因此我们从证据高度中减去ValidatorUpdateDelay，
			并减去1，因为这是LastCommit。
			请注意，这* *可以导致负“distributionHeight”到-ValidatorUpdateDelay-1，
			即在发生前的阻断结束时（无）=在发生阻滞的开始。
			这很好，因为这只是用来过滤无约束的授权和重新授权。
			 */
			distributionHeight := height - sdk.ValidatorUpdateDelay - 1
			k.validatorSet.Slash(ctx, consAddr, distributionHeight, power, k.SlashFractionDowntime(ctx))
			k.validatorSet.Jail(ctx, consAddr)
			signInfo.JailedUntil = ctx.BlockHeader().Time.Add(k.DowntimeJailDuration(ctx))

			// We need to reset the counter & array so that the validator won't be immediately slashed for downtime upon rebonding.
			// 我们需要重置计数器和数组，以便在重新绑定时不会立即削减验证器的停机时间。
			signInfo.MissedBlocksCounter = 0
			signInfo.IndexOffset = 0
			k.clearValidatorMissedBlockBitArray(ctx, consAddr)
		} else {
			// Validator was (a) not found or (b) already jailed, don't slash
			// 验证者是（a）未找到或（b）已经入狱，不要削减
			logger.Info(fmt.Sprintf("Validator %s would have been slashed for downtime, but was either not found in store or already jailed",
				pubkey.Address()))
		}
	}

	// Set the updated signing info
	// 设置这个更新签名 信息
	k.SetValidatorSigningInfo(ctx, consAddr, signInfo)
}

func (k Keeper) addPubkey(ctx sdk.Context, pubkey crypto.PubKey) {
	addr := pubkey.Address()
	k.setAddrPubkeyRelation(ctx, addr, pubkey)
}

func (k Keeper) getPubkey(ctx sdk.Context, address crypto.Address) (crypto.PubKey, error) {
	store := ctx.KVStore(k.storeKey)
	var pubkey crypto.PubKey
	err := k.cdc.UnmarshalBinaryLengthPrefixed(store.Get(getAddrPubkeyRelationKey(address)), &pubkey)
	if err != nil {
		return nil, fmt.Errorf("address %v not found", address)
	}
	return pubkey, nil
}

func (k Keeper) setAddrPubkeyRelation(ctx sdk.Context, addr crypto.Address, pubkey crypto.PubKey) {
	store := ctx.KVStore(k.storeKey)
	bz := k.cdc.MustMarshalBinaryLengthPrefixed(pubkey)
	store.Set(getAddrPubkeyRelationKey(addr), bz)
}

func (k Keeper) deleteAddrPubkeyRelation(ctx sdk.Context, addr crypto.Address) {
	store := ctx.KVStore(k.storeKey)
	store.Delete(getAddrPubkeyRelationKey(addr))
}
