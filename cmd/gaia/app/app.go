package app

import (
	"fmt"
	"io"
	"os"
	"sort"

	abci "github.com/tendermint/tendermint/abci/types"
	cmn "github.com/tendermint/tendermint/libs/common"
	dbm "github.com/tendermint/tendermint/libs/db"
	"github.com/tendermint/tendermint/libs/log"

	// TODO: Remove once transfers are enabled.
	gaiabank "my-cosmos/cosmos-sdk/cmd/gaia/app/x/bank"

	bam "my-cosmos/cosmos-sdk/baseapp"
	"my-cosmos/cosmos-sdk/codec"
	sdk "my-cosmos/cosmos-sdk/types"
	"my-cosmos/cosmos-sdk/x/auth"
	"my-cosmos/cosmos-sdk/x/bank"
	distr "my-cosmos/cosmos-sdk/x/distribution"
	"my-cosmos/cosmos-sdk/x/gov"
	"my-cosmos/cosmos-sdk/x/mint"
	"my-cosmos/cosmos-sdk/x/params"
	"my-cosmos/cosmos-sdk/x/slashing"
	"my-cosmos/cosmos-sdk/x/staking"
)

const (
	appName = "GaiaApp"
	// DefaultKeyPass contains the default key password for genesis transactions
	DefaultKeyPass = "12345678"
)

// default home directories for expected binaries
var (
	DefaultCLIHome  = os.ExpandEnv("$HOME/.gaiacli")
	DefaultNodeHome = os.ExpandEnv("$HOME/.gaiad")
)

// Extended ABCI application
type GaiaApp struct {
	*bam.BaseApp
	cdc *codec.Codec

	// keys to access the substores
	keyMain          *sdk.KVStoreKey
	keyAccount       *sdk.KVStoreKey
	keyStaking       *sdk.KVStoreKey
	tkeyStaking      *sdk.TransientStoreKey
	keySlashing      *sdk.KVStoreKey
	keyMint          *sdk.KVStoreKey
	keyDistr         *sdk.KVStoreKey
	tkeyDistr        *sdk.TransientStoreKey
	keyGov           *sdk.KVStoreKey
	keyFeeCollection *sdk.KVStoreKey
	keyParams        *sdk.KVStoreKey
	tkeyParams       *sdk.TransientStoreKey

	// Manage getting and setting accounts
	accountKeeper       auth.AccountKeeper
	feeCollectionKeeper auth.FeeCollectionKeeper
	bankKeeper          bank.Keeper
	stakingKeeper       staking.Keeper  // staking.Keeper 其实就是 keeper.keeper
	slashingKeeper      slashing.Keeper
	mintKeeper          mint.Keeper
	distrKeeper         distr.Keeper
	govKeeper           gov.Keeper
	paramsKeeper        params.Keeper
}

// ##################
// ##################
// NewGaiaApp returns a reference to an initialized GaiaApp.
// 这个是gaia app实例
// 单独运行的话就是现在的 cosmos hub
// ##################
// ##################
func NewGaiaApp(logger log.Logger, db dbm.DB, traceStore io.Writer, loadLatest bool, baseAppOptions ...func(*bam.BaseApp)) *GaiaApp {
	// 什么都不管，进来先来一波 编码解码器实例化
	cdc := MakeCodec()

	// 实例化 baseApp
	// baseApp 使用 ABCI协议和底层tendermint 交互
	bApp := bam.NewBaseApp(appName, logger, db, auth.DefaultTxDecoder(cdc), baseAppOptions...)
	bApp.SetCommitMultiStoreTracer(traceStore)


	// 创建一个相关的APP，其它所有的APP都可以按照这个方法
	var app = &GaiaApp{
		// 继承了baseApp 实例
		BaseApp:          bApp,
		// 编解码器
		cdc:              cdc,

		// 各种 key ？？
		keyMain:          sdk.NewKVStoreKey(bam.MainStoreKey),
		keyAccount:       sdk.NewKVStoreKey(auth.StoreKey),
		keyStaking:       sdk.NewKVStoreKey(staking.StoreKey),
		tkeyStaking:      sdk.NewTransientStoreKey(staking.TStoreKey),
		keyMint:          sdk.NewKVStoreKey(mint.StoreKey),
		keyDistr:         sdk.NewKVStoreKey(distr.StoreKey),
		tkeyDistr:        sdk.NewTransientStoreKey(distr.TStoreKey),
		keySlashing:      sdk.NewKVStoreKey(slashing.StoreKey),
		keyGov:           sdk.NewKVStoreKey(gov.StoreKey),
		keyFeeCollection: sdk.NewKVStoreKey(auth.FeeStoreKey),
		keyParams:        sdk.NewKVStoreKey(params.StoreKey),
		tkeyParams:       sdk.NewTransientStoreKey(params.TStoreKey),
	}

	/**
	开始构建各种 管理器(keeper)
	 */
	app.paramsKeeper = params.NewKeeper(app.cdc, app.keyParams, app.tkeyParams)

	// define the accountKeeper
	// 定义一个账户 管理器
	// 帐户管理--从KVSTROE抽象
	app.accountKeeper = auth.NewAccountKeeper(
		app.cdc,
		app.keyAccount,
		app.paramsKeeper.Subspace(auth.DefaultParamspace),
		auth.ProtoBaseAccount,
	)

	// add handlers
	//添加各种操作——它们都从KVSTORE抽象出来,但是它们的抽象度更高，或者可以认为是accountMapper的更高一层

	// 一个基础的处理类 管理器
	app.bankKeeper = bank.NewBaseKeeper(
		app.accountKeeper,
		app.paramsKeeper.Subspace(bank.DefaultParamspace),
		bank.DefaultCodespace,
	)
	// FeeCollectionKeeper处理anteHandler中的费用收集和不同费用令牌的MinFees设置
	app.feeCollectionKeeper = auth.NewFeeCollectionKeeper(
		app.cdc,
		app.keyFeeCollection,
	)

	/**
	################
	################
	一个和经济模型相关的 管理器 (全局的) 其实staking.keeper 就是 keeper.keeper
	################
	################
	 */
	stakingKeeper := staking.NewKeeper(
		app.cdc,
		app.keyStaking, app.tkeyStaking,
		app.bankKeeper, app.paramsKeeper.Subspace(staking.DefaultParamspace),
		staking.DefaultCodespace,
	)

	// 这个不知道做什么的，应该是和 tendermint 共识有关 ？
	app.mintKeeper = mint.NewKeeper(app.cdc, app.keyMint,
		app.paramsKeeper.Subspace(mint.DefaultParamspace),
		&stakingKeeper, app.feeCollectionKeeper,
	)

	// distribute 指 发布新币的管理器 ？？
	// 派发奖励用
	app.distrKeeper = distr.NewKeeper(
		app.cdc,
		app.keyDistr,
		app.paramsKeeper.Subspace(distr.DefaultParamspace),
		app.bankKeeper, &stakingKeeper, app.feeCollectionKeeper,
		distr.DefaultCodespace,
	)

	//设置惩罚机制操作者
	app.slashingKeeper = slashing.NewKeeper(
		app.cdc,
		app.keySlashing,
		&stakingKeeper, app.paramsKeeper.Subspace(slashing.DefaultParamspace),
		slashing.DefaultCodespace,
	)

	/**
	这个是 链上治理 管理器
	 */
	app.govKeeper = gov.NewKeeper(
		app.cdc,
		app.keyGov,
		app.paramsKeeper, app.paramsKeeper.Subspace(gov.DefaultParamspace), app.bankKeeper, &stakingKeeper,
		gov.DefaultCodespace,
	)

	// register the staking hooks
	// NOTE: The stakingKeeper above is passed by reference, so that it can be
	// modified like below:
	/**
	注册赌注钩
	注意：上面的stakingKeeper通过引用传递，因此它可以
	修改如下：
	 */
	app.stakingKeeper = *stakingKeeper.SetHooks(
		NewStakingHooks(app.distrKeeper.Hooks(), app.slashingKeeper.Hooks()),
	)

	// register message routes
	//
	// TODO: Use standard bank router once transfers are enabled.

	// 这个是重点，在这里注册路由的句柄
	// ##################
	// ##################
	// 注册 各类路由
	// 启用传输后，使用标准银行路由器。
	// ##################
	// ##################
	app.Router().
		AddRoute(bank.RouterKey, gaiabank.NewHandler(app.bankKeeper)).
		// 经济模型相关
		AddRoute(staking.RouterKey, staking.NewHandler(app.stakingKeeper)).
		AddRoute(distr.RouterKey, distr.NewHandler(app.distrKeeper)).
		AddRoute(slashing.RouterKey, slashing.NewHandler(app.slashingKeeper)).

		// 链上治理相关
		AddRoute(gov.RouterKey, gov.NewHandler(app.govKeeper))


	app.QueryRouter().
		AddRoute(auth.QuerierRoute, auth.NewQuerier(app.accountKeeper)).
		AddRoute(distr.QuerierRoute, distr.NewQuerier(app.distrKeeper)).

		// 链上治理相关
		AddRoute(gov.QuerierRoute, gov.NewQuerier(app.govKeeper)).
		AddRoute(slashing.QuerierRoute, slashing.NewQuerier(app.slashingKeeper, app.cdc)).

		// 经济模型相关
		AddRoute(staking.QuerierRoute, staking.NewQuerier(app.stakingKeeper, app.cdc))

	// initialize BaseApp
	/**
	这里是 实例化了 base App
	 */
	// 从KV数据库加载相关数据--在当前版本中，IVAL存储是KVStore基础的实现
	app.MountStores(app.keyMain, app.keyAccount, app.keyStaking, app.keyMint, app.keyDistr,
		app.keySlashing, app.keyGov, app.keyFeeCollection, app.keyParams,
		app.tkeyParams, app.tkeyStaking, app.tkeyDistr,
	)

	/**
	TODO 重要

	TODO 注册各种重要的函数
	 */
	// 设置一个 初始化链的 func
	app.SetInitChainer(app.initChainer)

	// 设置一个 执行block中tx之前调用的 func
	app.SetBeginBlocker(app.BeginBlocker)

	// 设置一个 账户及外部token等等的 auth相关的 func
	// 设置权限控制句柄
	app.SetAnteHandler(auth.NewAnteHandler(app.accountKeeper, app.feeCollectionKeeper))

	// TODO 重要   关于诶个block 执行之后的 验证人信息变更全部在这里了 和tendermint交互的
	// 设置一个 执行 block中tx之后调用的 func
	app.SetEndBlocker(app.EndBlocker)

	// 再启动 gaia 是入参为 true
	// 表示：是否需要加载最新的应用程序版本
	if loadLatest {
		err := app.LoadLatestVersion(app.keyMain)
		if err != nil {
			cmn.Exit(err.Error())
		}
	}

	return app
}

// custom tx codec
// 自定义tx编解码器
// 将相关的编码器注册到相关的各方
func MakeCodec() *codec.Codec {
	// 创建一个编码解码器
	var cdc = codec.New()

	// 并把它注册到各个组件上
	bank.RegisterCodec(cdc)
	staking.RegisterCodec(cdc)
	distr.RegisterCodec(cdc)
	slashing.RegisterCodec(cdc)
	gov.RegisterCodec(cdc)
	auth.RegisterCodec(cdc)
	sdk.RegisterCodec(cdc)
	codec.RegisterCrypto(cdc)
	return cdc
}

// application updates every end block
/**
每个区块执行前都会调的

与 EndBlocker 相呼应

大家写过数据库的底层操作，这个东西应该和它非常类似，不外乎是Begin准备，End结束，清扫资源

TODO 这个由 tendermint 在处理每个块之前 回调cosmos 做的 rpc 交互
 */
func (app *GaiaApp) BeginBlocker(ctx sdk.Context, req abci.RequestBeginBlock) abci.ResponseBeginBlock {
	// mint new tokens for the previous block
	//
	mint.BeginBlocker(ctx, app.mintKeeper)

	// distribute rewards for the previous block
	/*
	TODO  分配前一个区块的奖励
	*/
	distr.BeginBlocker(ctx, req, app.distrKeeper)

	// slash anyone who double signed.
	// NOTE: This should happen after distr.BeginBlocker so that
	// there is nothing left over in the validator fee pool,
	// so as to keep the CanWithdrawInvariant invariant.
	// TODO: This should really happen at EndBlocker.
	/**
	惩罚任何双重签名的人。
	注意：这应该在distr.BeginBlocker之后发生
	验证器费用池中没有剩余任何内容，以保持CanWithdrawInvariant不变。
	TODO：这应该发生在EndBlocker上。
	 */
	// 在执行区块前 开始惩罚检查
	tags := slashing.BeginBlocker(ctx, req, app.slashingKeeper)

	return abci.ResponseBeginBlock{
		Tags: tags.ToKVPairs(),
	}
}

// tendermint 拉取 cosmos 实时的验证人列表
// TODO ################
// TODO 超级重要  这个才是由 tendermint 在每个块执行结束时 回调 cosmos 的 rpc交互
// application updates every end block
// nolint: unparam
// 在每个区块执行结束前 调用
// DeliverTx消息处理完成所有的交易后调用，主要用来对验证人集合的结果进行维护.
func (app *GaiaApp) EndBlocker(ctx sdk.Context, req abci.RequestEndBlock) abci.ResponseEndBlock {

	// 先调一波链上治理的接口
	tags := gov.EndBlocker(ctx, app.govKeeper)

	// TODO staking 的验证人变更的逻辑在这里哦
	validatorUpdates, endBlockerTags := staking.EndBlocker(ctx, app.stakingKeeper)
	tags = append(tags, endBlockerTags...)

	app.assertRuntimeInvariants()

	// 最后这些只是会被 返回到 tendermint层的
	return abci.ResponseEndBlock{
		ValidatorUpdates: validatorUpdates,
		Tags:             tags,
	}
}

// initialize store from a genesis state
func (app *GaiaApp) initFromGenesisState(ctx sdk.Context, genesisState GenesisState) []abci.ValidatorUpdate {
	genesisState.Sanitize()

	// load the accounts
	for _, gacc := range genesisState.Accounts {
		acc := gacc.ToAccount()
		acc = app.accountKeeper.NewAccount(ctx, acc) // set account number
		app.accountKeeper.SetAccount(ctx, acc)
	}

	// initialize distribution (must happen before staking)
	distr.InitGenesis(ctx, app.distrKeeper, genesisState.DistrData)

	// load the initial staking information
	validators, err := staking.InitGenesis(ctx, app.stakingKeeper, genesisState.StakingData)
	if err != nil {
		panic(err) // TODO find a way to do this w/o panics
	}

	// initialize module-specific stores
	auth.InitGenesis(ctx, app.accountKeeper, app.feeCollectionKeeper, genesisState.AuthData)
	bank.InitGenesis(ctx, app.bankKeeper, genesisState.BankData)
	slashing.InitGenesis(ctx, app.slashingKeeper, genesisState.SlashingData, genesisState.StakingData.Validators.ToSDKValidators())
	gov.InitGenesis(ctx, app.govKeeper, genesisState.GovData)
	mint.InitGenesis(ctx, app.mintKeeper, genesisState.MintData)

	// validate genesis state
	if err := GaiaValidateGenesisState(genesisState); err != nil {
		panic(err) // TODO find a way to do this w/o panics
	}

	if len(genesisState.GenTxs) > 0 {
		for _, genTx := range genesisState.GenTxs {
			var tx auth.StdTx
			err = app.cdc.UnmarshalJSON(genTx, &tx)
			if err != nil {
				panic(err)
			}
			bz := app.cdc.MustMarshalBinaryLengthPrefixed(tx)

			/**
			TODO 重要
			交易处理消息DeliverTx，
			它就是在区块开始被调用前，
			在这个接口中处理验证人签名的信息
			 */
			res := app.BaseApp.DeliverTx(bz)
			if !res.IsOK() {
				panic(res.Log)
			}
		}

		/*
		TODO 获取最新的  验证人队列
		*/
		validators = app.stakingKeeper.ApplyAndReturnValidatorSetUpdates(ctx)
	}
	return validators
}

// custom logic for gaia initialization
func (app *GaiaApp) initChainer(ctx sdk.Context, req abci.RequestInitChain) abci.ResponseInitChain {
	stateJSON := req.AppStateBytes
	// TODO is this now the whole genesis file?

	var genesisState GenesisState
	err := app.cdc.UnmarshalJSON(stateJSON, &genesisState)
	if err != nil {
		panic(err) // TODO https://my-cosmos/cosmos-sdk/issues/468
		// return sdk.ErrGenesisParse("").TraceCause(err, "")
	}

	validators := app.initFromGenesisState(ctx, genesisState)

	// sanity check
	if len(req.Validators) > 0 {
		if len(req.Validators) != len(validators) {
			panic(fmt.Errorf("len(RequestInitChain.Validators) != len(validators) (%d != %d)",
				len(req.Validators), len(validators)))
		}
		sort.Sort(abci.ValidatorUpdates(req.Validators))
		sort.Sort(abci.ValidatorUpdates(validators))
		for i, val := range validators {
			if !val.Equal(req.Validators[i]) {
				panic(fmt.Errorf("validators[%d] != req.Validators[%d] ", i, i))
			}
		}
	}

	// assert runtime invariants
	app.assertRuntimeInvariants()

	return abci.ResponseInitChain{
		Validators: validators,
	}
}

// load a particular height
func (app *GaiaApp) LoadHeight(height int64) error {
	return app.LoadVersion(height, app.keyMain)
}

// ______________________________________________________________________________________________

var _ sdk.StakingHooks = StakingHooks{}

// StakingHooks contains combined distribution and slashing hooks needed for the
// staking module.
type StakingHooks struct {
	dh distr.Hooks
	sh slashing.Hooks
}

func NewStakingHooks(dh distr.Hooks, sh slashing.Hooks) StakingHooks {
	return StakingHooks{dh, sh}
}

// nolint
/**
超级重要的 func 集, 都是一些AOP做法的 hook 函数
 */
func (h StakingHooks) AfterValidatorCreated(ctx sdk.Context, valAddr sdk.ValAddress) {
	// TODO 初始化一个验证人的各种信息
	//
	// 设置全局 keeper 中的 validator
	//
	// 1、设置了 历史奖励
	// 2、设置了 当前奖励
	// 3、设置了 累计佣金
	// 4、设置了 出块奖励
	h.dh.AfterValidatorCreated(ctx, valAddr)

	// 设置 addr -> pubkey
	h.sh.AfterValidatorCreated(ctx, valAddr)
}
func (h StakingHooks) BeforeValidatorModified(ctx sdk.Context, valAddr sdk.ValAddress) {
	h.dh.BeforeValidatorModified(ctx, valAddr)
	h.sh.BeforeValidatorModified(ctx, valAddr)
}
func (h StakingHooks) AfterValidatorRemoved(ctx sdk.Context, consAddr sdk.ConsAddress, valAddr sdk.ValAddress) {
	h.dh.AfterValidatorRemoved(ctx, consAddr, valAddr)
	h.sh.AfterValidatorRemoved(ctx, consAddr, valAddr)
}
func (h StakingHooks) AfterValidatorBonded(ctx sdk.Context, consAddr sdk.ConsAddress, valAddr sdk.ValAddress) {
	h.dh.AfterValidatorBonded(ctx, consAddr, valAddr)
	h.sh.AfterValidatorBonded(ctx, consAddr, valAddr)
}
func (h StakingHooks) AfterValidatorBeginUnbonding(ctx sdk.Context, consAddr sdk.ConsAddress, valAddr sdk.ValAddress) {
	h.dh.AfterValidatorBeginUnbonding(ctx, consAddr, valAddr)
	h.sh.AfterValidatorBeginUnbonding(ctx, consAddr, valAddr)
}
// 创建委托之前需要做的事
func (h StakingHooks) BeforeDelegationCreated(ctx sdk.Context, delAddr sdk.AccAddress, valAddr sdk.ValAddress) {
	h.dh.BeforeDelegationCreated(ctx, delAddr, valAddr)
	h.sh.BeforeDelegationCreated(ctx, delAddr, valAddr) // 未实现
}
func (h StakingHooks) BeforeDelegationSharesModified(ctx sdk.Context, delAddr sdk.AccAddress, valAddr sdk.ValAddress) {
	h.dh.BeforeDelegationSharesModified(ctx, delAddr, valAddr)
	h.sh.BeforeDelegationSharesModified(ctx, delAddr, valAddr)
}
func (h StakingHooks) BeforeDelegationRemoved(ctx sdk.Context, delAddr sdk.AccAddress, valAddr sdk.ValAddress) {
	h.dh.BeforeDelegationRemoved(ctx, delAddr, valAddr)
	h.sh.BeforeDelegationRemoved(ctx, delAddr, valAddr)
}
func (h StakingHooks) AfterDelegationModified(ctx sdk.Context, delAddr sdk.AccAddress, valAddr sdk.ValAddress) {
	h.dh.AfterDelegationModified(ctx, delAddr, valAddr)
	h.sh.AfterDelegationModified(ctx, delAddr, valAddr) // 未实现
}
func (h StakingHooks) BeforeValidatorSlashed(ctx sdk.Context, valAddr sdk.ValAddress, fraction sdk.Dec) {
	h.dh.BeforeValidatorSlashed(ctx, valAddr, fraction)
	h.sh.BeforeValidatorSlashed(ctx, valAddr, fraction)
}
