package baseapp

import (
	"fmt"
	"io"
	"reflect"
	"runtime/debug"
	"strings"

	"errors"

	"github.com/gogo/protobuf/proto"

	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/crypto/tmhash"
	dbm "github.com/tendermint/tendermint/libs/db"
	"github.com/tendermint/tendermint/libs/log"

	"my-cosmos/cosmos-sdk/codec"
	"my-cosmos/cosmos-sdk/store"
	sdk "my-cosmos/cosmos-sdk/types"
	"my-cosmos/cosmos-sdk/version"
)

// Key to store the consensus params in the main store.
var mainConsensusParamsKey = []byte("consensus_params")

// Enum mode for app.runTx
type runTxMode uint8

const (
	// Check a transaction
	// 检查一个 tx
	runTxModeCheck runTxMode = iota
	// Simulate a transaction
	// 模拟一个 tx
	runTxModeSimulate runTxMode = iota
	// Deliver a transaction
	// 交付一个 tx （真正处理一笔 tx）
	runTxModeDeliver runTxMode = iota

	// MainStoreKey is the string representation of the main store
	// MainStoreKey是 主要存储的字符串表示形式 (也就是 key)
	MainStoreKey = "main"
)

// BaseApp reflects the ABCI application implementation.
type BaseApp struct {
	// initialized on creation
	logger      log.Logger
	name        string               // application name from abci.Info
	db          dbm.DB               // common DB backend
	cms         sdk.CommitMultiStore // Main (uncached) state
	router      Router               // handle any kind of message
	queryRouter QueryRouter          // router for redirecting query calls
	txDecoder   sdk.TxDecoder        // unmarshal []byte into sdk.Tx

	// set upon LoadVersion or LoadLatestVersion.
	baseKey *sdk.KVStoreKey // Main KVStore in cms

	anteHandler    sdk.AnteHandler  // ante handler for fee and auth
	initChainer    sdk.InitChainer  // initialize state with validators and state blob

	/*
	这些方法有底层的 tendermint 来发起 rpc 调用的哦
	*/
	beginBlocker   sdk.BeginBlocker // logic to run before any txs
	endBlocker     sdk.EndBlocker   // logic to run after all txs, and to determine valset changes

	addrPeerFilter sdk.PeerFilter   // filter peers by address and port
	idPeerFilter   sdk.PeerFilter   // filter peers by node ID
	fauxMerkleMode bool             // if true, IAVL MountStores uses MountStoresDB for simulation speed.

	// --------------------
	// Volatile state
	// checkState is set on initialization and reset on Commit.
	// deliverState is set in InitChain and BeginBlock and cleared on Commit.
	// See methods setCheckState and setDeliverState.
	checkState   *state          // for CheckTx
	deliverState *state          // for DeliverTx
	voteInfos    []abci.VoteInfo // absent validators from begin block  没有来自开始块的验证器 ??

	// consensus params 共识参数 ？？
	// TODO: Move this in the future to baseapp param store on main store.
	consensusParams *abci.ConsensusParams

	// The minimum gas prices a validator is willing to accept for processing a
	// transaction. This is mainly used for DoS and spam prevention.
	//
	// 验证人 愿意接受处理交易的最低 gas价格。 这主要用于DoS和垃圾邮件预防。
	minGasPrices sdk.DecCoins

	// flag for sealing options and parameters to a BaseApp
	// 用于密封BaseApp的选项和参数的标志
	sealed bool
}

var _ abci.Application = (*BaseApp)(nil)

// NewBaseApp returns a reference to an initialized BaseApp. It accepts a
// variadic number of option functions, which act on the BaseApp to set
// configuration choices.
//
// NOTE: The db is used to store the version number for now.
func NewBaseApp(
	name string, logger log.Logger, db dbm.DB, txDecoder sdk.TxDecoder, options ...func(*BaseApp),
) *BaseApp {

	app := &BaseApp{
		logger:         logger,
		name:           name,
		db:             db,
		cms:            store.NewCommitMultiStore(db),
		router:         NewRouter(),
		queryRouter:    NewQueryRouter(),
		txDecoder:      txDecoder,
		fauxMerkleMode: false,
	}
	for _, option := range options {
		option(app)
	}

	return app
}

// Name returns the name of the BaseApp.
func (app *BaseApp) Name() string {
	return app.name
}

// Logger returns the logger of the BaseApp.
func (app *BaseApp) Logger() log.Logger {
	return app.logger
}

// SetCommitMultiStoreTracer sets the store tracer on the BaseApp's underlying
// CommitMultiStore.
func (app *BaseApp) SetCommitMultiStoreTracer(w io.Writer) {
	app.cms.SetTracer(w)
}

// MountStores mounts all IAVL or DB stores to the provided keys in the BaseApp
// multistore.
/**
MountStore:
将所有IAVL或DB存储安装到BaseApp多存储中提供的密钥。
 */
func (app *BaseApp) MountStores(keys ...sdk.StoreKey) {
	for _, key := range keys {
		switch key.(type) {
		case *sdk.KVStoreKey:
			if !app.fauxMerkleMode {
				app.MountStore(key, sdk.StoreTypeIAVL)
			} else {
				// StoreTypeDB doesn't do anything upon commit, and it doesn't
				// retain history, but it's useful for faster simulation.
				app.MountStore(key, sdk.StoreTypeDB)
			}
		case *sdk.TransientStoreKey:
			app.MountStore(key, sdk.StoreTypeTransient)
		default:
			panic("Unrecognized store key type " + reflect.TypeOf(key).Name())
		}
	}
}



// ############ 存储相关
// MountStoreWithDB mounts a store to the provided key in the BaseApp
// multistore, using a specified DB.
func (app *BaseApp) MountStoreWithDB(key sdk.StoreKey, typ sdk.StoreType, db dbm.DB) {
	app.cms.MountStoreWithDB(key, typ, db)
}

// MountStore mounts a store to the provided key in the BaseApp multistore,
// using the default DB.
func (app *BaseApp) MountStore(key sdk.StoreKey, typ sdk.StoreType) {
	app.cms.MountStoreWithDB(key, typ, nil)
}

// LoadLatestVersion loads the latest application version. It will panic if
// called more than once on a running BaseApp.
// LoadLatestVersion:
// 加载最新的应用程序版本。 如果在正在运行的BaseApp上多次调用它将会出现混乱。
func (app *BaseApp) LoadLatestVersion(baseKey *sdk.KVStoreKey) error {
	err := app.cms.LoadLatestVersion()
	if err != nil {
		return err
	}
	return app.initFromMainStore(baseKey)
}

// LoadVersion loads the BaseApp application version. It will panic if called
// more than once on a running baseapp.
func (app *BaseApp) LoadVersion(version int64, baseKey *sdk.KVStoreKey) error {
	err := app.cms.LoadVersion(version)
	if err != nil {
		return err
	}
	return app.initFromMainStore(baseKey)
}

// LastCommitID returns the last CommitID of the multistore.
func (app *BaseApp) LastCommitID() sdk.CommitID {
	return app.cms.LastCommitID()
}

// LastBlockHeight returns the last committed block height.
func (app *BaseApp) LastBlockHeight() int64 {
	return app.cms.LastCommitID().Version
}

// initializes the remaining logic from app.cms
func (app *BaseApp) initFromMainStore(baseKey *sdk.KVStoreKey) error {
	mainStore := app.cms.GetKVStore(baseKey)
	if mainStore == nil {
		return errors.New("baseapp expects MultiStore with 'main' KVStore")
	}

	// memoize baseKey
	if app.baseKey != nil {
		panic("app.baseKey expected to be nil; duplicate init?")
	}
	app.baseKey = baseKey

	// Load the consensus params from the main store. If the consensus params are
	// nil, it will be saved later during InitChain.
	//
	// TODO: assert that InitChain hasn't yet been called.
	consensusParamsBz := mainStore.Get(mainConsensusParamsKey)
	if consensusParamsBz != nil {
		var consensusParams = &abci.ConsensusParams{}

		err := proto.Unmarshal(consensusParamsBz, consensusParams)
		if err != nil {
			panic(err)
		}

		app.setConsensusParams(consensusParams)
	}

	// needed for `gaiad export`, which inits from store but never calls initchain
	app.setCheckState(abci.Header{})
	app.Seal()

	return nil
}

func (app *BaseApp) setMinGasPrices(gasPrices sdk.DecCoins) {
	app.minGasPrices = gasPrices
}

// Router returns the router of the BaseApp.
func (app *BaseApp) Router() Router {
	if app.sealed {
		// We cannot return a router when the app is sealed because we can't have
		// any routes modified which would cause unexpected routing behavior.
		panic("Router() on sealed BaseApp")
	}
	return app.router
}

// QueryRouter returns the QueryRouter of a BaseApp.
func (app *BaseApp) QueryRouter() QueryRouter { return app.queryRouter }

// Seal seals a BaseApp. It prohibits any further modifications to a BaseApp.
func (app *BaseApp) Seal() { app.sealed = true }

// IsSealed returns true if the BaseApp is sealed and false otherwise.
func (app *BaseApp) IsSealed() bool { return app.sealed }

// setCheckState sets checkState with the cached multistore and
// the context wrapping it.
// It is called by InitChain() and Commit()
func (app *BaseApp) setCheckState(header abci.Header) {
	ms := app.cms.CacheMultiStore()
	app.checkState = &state{
		ms:  ms,
		ctx: sdk.NewContext(ms, header, true, app.logger).WithMinGasPrices(app.minGasPrices),
	}
}

// setCheckState sets checkState with the cached multistore and
// the context wrapping it.
// It is called by InitChain() and BeginBlock(),
// and deliverState is set nil on Commit().
func (app *BaseApp) setDeliverState(header abci.Header) {
	ms := app.cms.CacheMultiStore()
	app.deliverState = &state{
		ms:  ms,
		ctx: sdk.NewContext(ms, header, false, app.logger),
	}
}

// setConsensusParams memoizes the consensus params.
func (app *BaseApp) setConsensusParams(consensusParams *abci.ConsensusParams) {
	app.consensusParams = consensusParams
}

// setConsensusParams stores the consensus params to the main store.
func (app *BaseApp) storeConsensusParams(consensusParams *abci.ConsensusParams) {
	consensusParamsBz, err := proto.Marshal(consensusParams)
	if err != nil {
		panic(err)
	}
	mainStore := app.cms.GetKVStore(app.baseKey)
	mainStore.Set(mainConsensusParamsKey, consensusParamsBz)
}

// getMaximumBlockGas gets the maximum gas from the consensus params.
func (app *BaseApp) getMaximumBlockGas() (maxGas uint64) {
	if app.consensusParams == nil || app.consensusParams.Block == nil {
		return 0
	}
	return uint64(app.consensusParams.Block.MaxGas)
}

// ----------------------------------------------------------------------------
// ABCI

// TODO 下面大部分方法都是 交由 tendermint 发起回调的 rpc 函数

// Info implements the ABCI interface.
func (app *BaseApp) Info(req abci.RequestInfo) abci.ResponseInfo {
	lastCommitID := app.cms.LastCommitID()

	return abci.ResponseInfo{
		Data:             app.name,
		LastBlockHeight:  lastCommitID.Version,
		LastBlockAppHash: lastCommitID.Hash,
	}
}

// SetOption implements the ABCI interface.
func (app *BaseApp) SetOption(req abci.RequestSetOption) (res abci.ResponseSetOption) {
	// TODO: Implement!
	return
}

// InitChain implements the ABCI interface. It runs the initialization logic
// directly on the CommitMultiStore.
/**
TODO 重要的入口 交由 tendermint 发起调用
InitChain实现了ABCI接口。
它直接在CommitMultiStore上运行初始化逻辑。


它会被Tendermint在启动时调用一次，
用来初始化各种相关的Message，
比如共识层的参数和最初的验证人的集合数据。
当然，肯定还会有决定信息处理的方式.


在调用这个函数的期间，对这些信息进行填充并存储
 */
func (app *BaseApp) InitChain(req abci.RequestInitChain) (res abci.ResponseInitChain) {
	// stash the consensus params in the cms main store and memoize
	if req.ConsensusParams != nil {
		app.setConsensusParams(req.ConsensusParams)
		app.storeConsensusParams(req.ConsensusParams)
	}

	initHeader := abci.Header{ChainID: req.ChainId, Time: req.Time}

	// initialize the deliver state and check state with a correct header
	app.setDeliverState(initHeader)
	app.setCheckState(initHeader)

	if app.initChainer == nil {
		return
	}

	// add block gas meter for any genesis transactions (allow infinite gas)
	app.deliverState.ctx = app.deliverState.ctx.
		WithBlockGasMeter(sdk.NewInfiniteGasMeter())

	res = app.initChainer(app.deliverState.ctx, req)

	// NOTE: We don't commit, but BeginBlock for block 1 starts from this
	// deliverState.
	return
}

// FilterPeerByAddrPort filters peers by address/port.
func (app *BaseApp) FilterPeerByAddrPort(info string) abci.ResponseQuery {
	if app.addrPeerFilter != nil {
		return app.addrPeerFilter(info)
	}
	return abci.ResponseQuery{}
}

// FilterPeerByIDfilters peers by node ID.
func (app *BaseApp) FilterPeerByID(info string) abci.ResponseQuery {
	if app.idPeerFilter != nil {
		return app.idPeerFilter(info)
	}
	return abci.ResponseQuery{}
}

// Splits a string path using the delimiter '/'.
// e.g. "this/is/funny" becomes []string{"this", "is", "funny"}
func splitPath(requestPath string) (path []string) {
	path = strings.Split(requestPath, "/")
	// first element is empty string
	if len(path) > 0 && path[0] == "" {
		path = path[1:]
	}
	return path
}

// Query implements the ABCI interface. It delegates to CommitMultiStore if it
// implements Queryable.
func (app *BaseApp) Query(req abci.RequestQuery) (res abci.ResponseQuery) {
	path := splitPath(req.Path)
	if len(path) == 0 {
		msg := "no query path provided"
		return sdk.ErrUnknownRequest(msg).QueryResult()
	}

	switch path[0] {
	// "/app" prefix for special application queries
	case "app":
		return handleQueryApp(app, path, req)

	case "store":
		return handleQueryStore(app, path, req)

	case "p2p":
		return handleQueryP2P(app, path, req)

	case "custom":
		return handleQueryCustom(app, path, req)
	}

	msg := "unknown query path"
	return sdk.ErrUnknownRequest(msg).QueryResult()
}

func handleQueryApp(app *BaseApp, path []string, req abci.RequestQuery) (res abci.ResponseQuery) {
	if len(path) >= 2 {
		var result sdk.Result

		switch path[1] {
		case "simulate":
			txBytes := req.Data
			tx, err := app.txDecoder(txBytes)
			if err != nil {
				result = err.Result()
			} else {
				result = app.Simulate(txBytes, tx)
			}

		case "version":
			return abci.ResponseQuery{
				Code:      uint32(sdk.CodeOK),
				Codespace: string(sdk.CodespaceRoot),
				Value:     []byte(version.Version),
			}

		default:
			result = sdk.ErrUnknownRequest(fmt.Sprintf("Unknown query: %s", path)).Result()
		}

		value := codec.Cdc.MustMarshalBinaryLengthPrefixed(result)
		return abci.ResponseQuery{
			Code:      uint32(sdk.CodeOK),
			Codespace: string(sdk.CodespaceRoot),
			Value:     value,
		}
	}

	msg := "Expected second parameter to be either simulate or version, neither was present"
	return sdk.ErrUnknownRequest(msg).QueryResult()
}

func handleQueryStore(app *BaseApp, path []string, req abci.RequestQuery) (res abci.ResponseQuery) {
	// "/store" prefix for store queries
	queryable, ok := app.cms.(sdk.Queryable)
	if !ok {
		msg := "multistore doesn't support queries"
		return sdk.ErrUnknownRequest(msg).QueryResult()
	}

	req.Path = "/" + strings.Join(path[1:], "/")
	return queryable.Query(req)
}

func handleQueryP2P(app *BaseApp, path []string, _ abci.RequestQuery) (res abci.ResponseQuery) {
	// "/p2p" prefix for p2p queries
	if len(path) >= 4 {
		cmd, typ, arg := path[1], path[2], path[3]
		switch cmd {
		case "filter":
			switch typ {
			case "addr":
				return app.FilterPeerByAddrPort(arg)
			case "id":
				return app.FilterPeerByID(arg)
			}
		default:
			msg := "Expected second parameter to be filter"
			return sdk.ErrUnknownRequest(msg).QueryResult()
		}
	}

	msg := "Expected path is p2p filter <addr|id> <parameter>"
	return sdk.ErrUnknownRequest(msg).QueryResult()
}

func handleQueryCustom(app *BaseApp, path []string, req abci.RequestQuery) (res abci.ResponseQuery) {
	// path[0] should be "custom" because "/custom" prefix is required for keeper
	// queries.
	//
	// The queryRouter routes using path[1]. For example, in the path
	// "custom/gov/proposal", queryRouter routes using "gov".
	if len(path) < 2 || path[1] == "" {
		return sdk.ErrUnknownRequest("No route for custom query specified").QueryResult()
	}

	querier := app.queryRouter.Route(path[1])
	if querier == nil {
		return sdk.ErrUnknownRequest(fmt.Sprintf("no custom querier found for route %s", path[1])).QueryResult()
	}

	// cache wrap the commit-multistore for safety
	ctx := sdk.NewContext(
		app.cms.CacheMultiStore(), app.checkState.ctx.BlockHeader(), true, app.logger,
	).WithMinGasPrices(app.minGasPrices)

	// Passes the rest of the path as an argument to the querier.
	//
	// For example, in the path "custom/gov/proposal/test", the gov querier gets
	// []string{"proposal", "test"} as the path.
	resBytes, err := querier(ctx, path[2:], req)
	if err != nil {
		return abci.ResponseQuery{
			Code:      uint32(err.Code()),
			Codespace: string(err.Codespace()),
			Log:       err.ABCILog(),
		}
	}

	return abci.ResponseQuery{
		Code:  uint32(sdk.CodeOK),
		Value: resBytes,
	}
}

// BeginBlock implements the ABCI application interface.
// TODO 交由tendermint 发起 rpc调用
func (app *BaseApp) BeginBlock(req abci.RequestBeginBlock) (res abci.ResponseBeginBlock) {
	if app.cms.TracingEnabled() {
		app.cms.SetTracingContext(sdk.TraceContext(
			map[string]interface{}{"blockHeight": req.Header.Height},
		))
	}

	// Initialize the DeliverTx state. If this is the first block, it should
	// already be initialized in InitChain. Otherwise app.deliverState will be
	// nil, since it is reset on Commit.
	if app.deliverState == nil {
		app.setDeliverState(req.Header)
	} else {
		// In the first block, app.deliverState.ctx will already be initialized
		// by InitChain. Context is now updated with Header information.
		app.deliverState.ctx = app.deliverState.ctx.
			WithBlockHeader(req.Header).
			WithBlockHeight(req.Header.Height)
	}

	// add block gas meter
	var gasMeter sdk.GasMeter
	if maxGas := app.getMaximumBlockGas(); maxGas > 0 {
		gasMeter = sdk.NewGasMeter(maxGas)
	} else {
		gasMeter = sdk.NewInfiniteGasMeter()
	}

	app.deliverState.ctx = app.deliverState.ctx.WithBlockGasMeter(gasMeter)

	if app.beginBlocker != nil {
		res = app.beginBlocker(app.deliverState.ctx, req)
	}

	// set the signed validators for addition to context in deliverTx
	app.voteInfos = req.LastCommitInfo.GetVotes()
	return
}

// CheckTx implements the ABCI interface. It runs the "basic checks" to see
// whether or not a transaction can possibly be executed, first decoding, then
// the ante handler (which checks signatures/fees/ValidateBasic), then finally
// the route match to see whether a handler exists.
//
// NOTE:CheckTx does not run the actual Msg handler function(s).
/**
TODO 由 tendermint 来调用

CheckTx实现了ABCI接口。
它运行“基本检查”以查看是否可以执行事务，
首先解码，然后是ante处理程序（检查签名/费用/ ValidateBasic），
然后最终路由匹配以查看是否存在处理程序。

注意：CheckTx不运行实际的Msg处理函数。


CheckTx用于交易池。它只运行AnteHandler （handler累函数调用之前被调用的函数类型）。
消息处理直到交易已经被提交到区块时才开始处理的代价是非常之高的。
AnteHandler对发送者授权，确保他们有足够的手续费去支付。
如果之后交易失败，发送者仍然会支付这笔费用
 */
func (app *BaseApp) CheckTx(txBytes []byte) (res abci.ResponseCheckTx) {
	var result sdk.Result

	tx, err := app.txDecoder(txBytes)
	if err != nil {
		result = err.Result()
	} else {
		// 执行交易检查
		result = app.runTx(runTxModeCheck, txBytes, tx)
	}

	return abci.ResponseCheckTx{
		Code:      uint32(result.Code),
		Data:      result.Data,
		Log:       result.Log,
		GasWanted: int64(result.GasWanted), // TODO: Should type accept unsigned ints?
		GasUsed:   int64(result.GasUsed),   // TODO: Should type accept unsigned ints?
		Tags:      result.Tags,
	}
}

// DeliverTx implements the ABCI interface.
/*
TODO 这个才是 处理交易的 函数 （由 底层的tendermint 调用）
和 BeginBlock 及EndBlock 息息相关
*/
func (app *BaseApp) DeliverTx(txBytes []byte) (res abci.ResponseDeliverTx) {
	var result sdk.Result

	tx, err := app.txDecoder(txBytes)
	if err != nil {
		result = err.Result()
	} else {
		/*
		TODO 超级主要
		根据 交易类型 执行交易
		*/
		result = app.runTx(runTxModeDeliver, txBytes, tx)
	}

	return abci.ResponseDeliverTx{
		Code:      uint32(result.Code),
		Codespace: string(result.Codespace),
		Data:      result.Data,
		Log:       result.Log,
		GasWanted: int64(result.GasWanted), // TODO: Should type accept unsigned ints?
		GasUsed:   int64(result.GasUsed),   // TODO: Should type accept unsigned ints?
		Tags:      result.Tags,
	}
}

// validateBasicTxMsgs executes basic validator calls for messages.
// validateBasicTxMsgs :执行基本验证器 检查逻辑。
func validateBasicTxMsgs(msgs []sdk.Msg) sdk.Error {
	if msgs == nil || len(msgs) == 0 {
		return sdk.ErrUnknownRequest("Tx.GetMsgs() must return at least one message in list")
	}

	for _, msg := range msgs {
		// Validate the Msg.
		err := msg.ValidateBasic()
		if err != nil {
			return err
		}
	}

	return nil
}

// retrieve the context for the tx w/ txBytes and other memoized values.
func (app *BaseApp) getContextForTx(mode runTxMode, txBytes []byte) (ctx sdk.Context) {
	ctx = app.getState(mode).ctx.
		WithTxBytes(txBytes).
		WithVoteInfos(app.voteInfos).
		WithConsensusParams(app.consensusParams)

	if mode == runTxModeSimulate {
		ctx, _ = ctx.CacheContext()
	}

	return
}

// runMsgs iterates through all the messages and executes them.
func (app *BaseApp) runMsgs(ctx sdk.Context, msgs []sdk.Msg, mode runTxMode) (result sdk.Result) {
	idxlogs := make([]sdk.ABCIMessageLog, 0, len(msgs)) // a list of JSON-encoded logs with msg index

	var data []byte   // NOTE: we just append them all (?!)
	var tags sdk.Tags // also just append them all
	var code sdk.CodeType
	var codespace sdk.CodespaceType

	for msgIdx, msg := range msgs {
		// match message route
		msgRoute := msg.Route()
		handler := app.router.Route(msgRoute)
		if handler == nil {
			return sdk.ErrUnknownRequest("Unrecognized Msg type: " + msgRoute).Result()
		}

		var msgResult sdk.Result

		// skip actual execution for CheckTx mode
		if mode != runTxModeCheck {
			msgResult = handler(ctx, msg)
		}

		// NOTE: GasWanted is determined by ante handler and GasUsed by the GasMeter.

		// Result.Data must be length prefixed in order to separate each result
		data = append(data, msgResult.Data...)
		tags = append(tags, sdk.MakeTag(sdk.TagAction, msg.Type()))
		tags = append(tags, msgResult.Tags...)

		idxLog := sdk.ABCIMessageLog{MsgIndex: msgIdx, Log: msgResult.Log}

		// stop execution and return on first failed message
		if !msgResult.IsOK() {
			idxLog.Success = false
			idxlogs = append(idxlogs, idxLog)

			code = msgResult.Code
			codespace = msgResult.Codespace
			break
		}

		idxLog.Success = true
		idxlogs = append(idxlogs, idxLog)
	}

	logJSON := codec.Cdc.MustMarshalJSON(idxlogs)
	result = sdk.Result{
		Code:      code,
		Codespace: codespace,
		Data:      data,
		Log:       strings.TrimSpace(string(logJSON)),
		GasUsed:   ctx.GasMeter().GasConsumed(),
		Tags:      tags,
	}

	return result
}

// Returns the applicantion's deliverState if app is in runTxModeDeliver,
// otherwise it returns the application's checkstate.
func (app *BaseApp) getState(mode runTxMode) *state {
	if mode == runTxModeCheck || mode == runTxModeSimulate {
		return app.checkState
	}

	return app.deliverState
}

// cacheTxContext returns a new context based off of the provided context with
// a cache wrapped multi-store.
func (app *BaseApp) cacheTxContext(ctx sdk.Context, txBytes []byte) (
	sdk.Context, sdk.CacheMultiStore) {

	ms := ctx.MultiStore()
	// TODO: https://my-cosmos/cosmos-sdk/issues/2824
	msCache := ms.CacheMultiStore()
	if msCache.TracingEnabled() {
		msCache = msCache.SetTracingContext(
			sdk.TraceContext(
				map[string]interface{}{
					"txHash": fmt.Sprintf("%X", tmhash.Sum(txBytes)),
				},
			),
		).(sdk.CacheMultiStore)
	}

	return ctx.WithMultiStore(msCache), msCache
}

// runTx processes a transaction. The transactions is proccessed via an
// anteHandler. The provided txBytes may be nil in some cases, eg. in tests. For
// further details on transaction execution, reference the BaseApp SDK
// documentation.
/*
TODO 执行交易的总入口
根据不同的交易类型，执行不同的交易
*/
func (app *BaseApp) runTx(mode runTxMode, txBytes []byte, tx sdk.Tx) (result sdk.Result) {
	// NOTE: GasWanted should be returned by the AnteHandler. GasUsed is
	// determined by the GasMeter. We need access to the context to get the gas
	// meter so we initialize upfront.
	var gasWanted uint64

	ctx := app.getContextForTx(mode, txBytes)
	ms := ctx.MultiStore()

	// only run the tx if there is block gas remaining
	if mode == runTxModeDeliver && ctx.BlockGasMeter().IsOutOfGas() {
		result = sdk.ErrOutOfGas("no block gas left to run tx").Result()
		return
	}

	var startingGas uint64
	if mode == runTxModeDeliver {
		startingGas = ctx.BlockGasMeter().GasConsumed()
	}

	defer func() {
		if r := recover(); r != nil {
			switch rType := r.(type) {
			case sdk.ErrorOutOfGas:
				log := fmt.Sprintf(
					"out of gas in location: %v; gasWanted: %d, gasUsed: %d",
					rType.Descriptor, gasWanted, ctx.GasMeter().GasConsumed(),
				)
				result = sdk.ErrOutOfGas(log).Result()
			default:
				log := fmt.Sprintf("recovered: %v\nstack:\n%v", r, string(debug.Stack()))
				result = sdk.ErrInternal(log).Result()
			}
		}

		result.GasWanted = gasWanted
		result.GasUsed = ctx.GasMeter().GasConsumed()
	}()

	// If BlockGasMeter() panics it will be caught by the above recover and will
	// return an error - in any case BlockGasMeter will consume gas past the limit.
	//
	// NOTE: This must exist in a separate defer function for the above recovery
	// to recover from this one.
	defer func() {
		if mode == runTxModeDeliver {
			ctx.BlockGasMeter().ConsumeGas(
				ctx.GasMeter().GasConsumedToLimit(),
				"block gas meter",
			)

			if ctx.BlockGasMeter().GasConsumed() < startingGas {
				panic(sdk.ErrorGasOverflow{Descriptor: "tx gas summation"})
			}
		}
	}()

	var msgs = tx.GetMsgs()
	if err := validateBasicTxMsgs(msgs); err != nil {
		return err.Result()
	}

	if app.anteHandler != nil {
		var anteCtx sdk.Context
		var msCache sdk.CacheMultiStore

		// Cache wrap context before anteHandler call in case it aborts.
		// This is required for both CheckTx and DeliverTx.
		// Ref: https://github.com/cosmos-sdk/issues/2772
		//
		// NOTE: Alternatively, we could require that anteHandler ensures that
		// writes do not happen if aborted/failed.  This may have some
		// performance benefits, but it'll be more difficult to get right.
		anteCtx, msCache = app.cacheTxContext(ctx, txBytes)

		/*
		这里才是真正调用执行 tx
		一般是指： func NewAnteHandler(ak AccountKeeper, fck FeeCollectionKeeper) sdk.AnteHandler 的返回 回调函数
		sdk.AnteHandler
		*/
		newCtx, result, abort := app.anteHandler(anteCtx, tx, (mode == runTxModeSimulate))
		if !newCtx.IsZero() {
			// At this point, newCtx.MultiStore() is cache-wrapped, or something else
			// replaced by the ante handler. We want the original multistore, not one
			// which was cache-wrapped for the ante handler.
			//
			// Also, in the case of the tx aborting, we need to track gas consumed via
			// the instantiated gas meter in the ante handler, so we update the context
			// prior to returning.
			ctx = newCtx.WithMultiStore(ms)
		}

		gasWanted = result.GasWanted

		if abort {
			return result
		}

		msCache.Write()
	}

	if mode == runTxModeCheck {
		return
	}

	// Create a new context based off of the existing context with a cache wrapped
	// multi-store in case message processing fails.
	runMsgCtx, msCache := app.cacheTxContext(ctx, txBytes)
	result = app.runMsgs(runMsgCtx, msgs, mode)
	result.GasWanted = gasWanted

	if mode == runTxModeSimulate {
		return
	}

	// only update state if all messages pass
	if result.IsOK() {
		msCache.Write()
	}

	return
}

// EndBlock implements the ABCI interface.
/**
TODO 这个函数最终会由 底层的 tendermint 发起 rpc 调用，来向cosmos 获取最新变更的 验证人列表

最终是调到了  func (app *GaiaApp) EndBlocker(ctx sdk.Context, req abci.RequestEndBlock) abci.ResponseEndBlock
types.pb.go
 */
func (app *BaseApp) EndBlock(req abci.RequestEndBlock) (res abci.ResponseEndBlock) {
	if app.deliverState.ms.TracingEnabled() {
		app.deliverState.ms = app.deliverState.ms.SetTracingContext(nil).(sdk.CacheMultiStore)
	}

	if app.endBlocker != nil {

		/**
		拉取变化的验证人列表
		TODO  调用 每个区块处理最后一般的操作 (cosmos-sdk <-> tendermint 交互)
		 在这个方法里面会有 更新验证人列表的 逻辑 (最终调到了 GaiaApp的函数)
		 */
		res = app.endBlocker(app.deliverState.ctx, req)
	}

	return
}

// Commit implements the ABCI interface.
/**
TODO 由底层 tendermint 调用
TODO 注意因为GaiaApp 继承了 BaseApp 的哦，所以GaiaApp 可以直接调用这个哦
当处理完成交易后，应该把完成的交易从内存持久化到硬盘上，
并以上为根据创建返回被下一个Tendermint区块需要的默克尔树的Root哈希值。
这个哈希值 的作用在区块链中基本是一样的，用来验证合法性
 */
func (app *BaseApp) Commit() (res abci.ResponseCommit) {
	header := app.deliverState.ctx.BlockHeader()

	// write the Deliver state and commit the MultiStore
	// 编写Deliver状态并提交MultiStore
	app.deliverState.ms.Write()

	//
	commitID := app.cms.Commit()
	app.logger.Debug("Commit synced", "commit", fmt.Sprintf("%X", commitID))

	// Reset the Check state to the latest committed.
	//
	// NOTE: safe because Tendermint holds a lock on the mempool for Commit.
	// Use the header from this latest block.
	app.setCheckState(header)

	// empty/reset the deliver state
	app.deliverState = nil

	return abci.ResponseCommit{
		Data: commitID.Hash,
	}
}

// ----------------------------------------------------------------------------
// State

type state struct {
	ms  sdk.CacheMultiStore
	ctx sdk.Context
}

func (st *state) CacheMultiStore() sdk.CacheMultiStore {
	return st.ms.CacheMultiStore()
}

func (st *state) Context() sdk.Context {
	return st.ctx
}
