package server

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/tendermint/tendermint/abci/server"

	tcmd "github.com/tendermint/tendermint/cmd/tendermint/commands"
	cmn "github.com/tendermint/tendermint/libs/common"
	"github.com/tendermint/tendermint/node"
	"github.com/tendermint/tendermint/p2p"
	pvm "github.com/tendermint/tendermint/privval"
	"github.com/tendermint/tendermint/proxy"
)

// Tendermint full-node start flags
const (
	flagWithTendermint = "with-tendermint"
	flagAddress        = "address"
	flagTraceStore     = "trace-store"
	flagPruning        = "pruning"
	FlagMinGasPrices   = "minimum-gas-prices"
)

// StartCmd runs the service passed in, either stand-alone or in-process with
// Tendermint.
// StartCmd运行传入的 server，可以是独立的，也可以在Tendermint中进行。
// 其实这个函数就是创建node实例，并且把它加入 cmd中
func StartCmd(ctx *Context, appCreator AppCreator) *cobra.Command {

	// 实例化一个命令行实例 (注意： cobra 起的命令行实例是可以以层级的关系存在的，类似与 tree
	cmd := &cobra.Command{
		/**
		注意：
			以下都是一些回调函数
		 */

		Use:   "start",
		Short: "Run the full node",
		RunE: func(cmd *cobra.Command, args []string) error {
			// 启动一个没有 tendermint 共识的进程 (可能是一个单节点的私有链)
			if !viper.GetBool(flagWithTendermint) {
				ctx.Logger.Info("Starting ABCI without Tendermint")

				// appCreator 其实这个就是  newApp()
				return startStandAlone(ctx, appCreator)
			}

			ctx.Logger.Info("Starting ABCI with Tendermint")

			// 启动一个内部进程(一个tendermint共识的进程)
			// appCreator 其实这个就是  newApp()
			_, err := startInProcess(ctx, appCreator)
			return err
		},
	}

	// core flags for the ABCI application
	// ABCI应用程序的核心标志
	cmd.Flags().Bool(flagWithTendermint, true, "Run abci app embedded in-process with tendermint")
	cmd.Flags().String(flagAddress, "tcp://0.0.0.0:26658", "Listen address")
	cmd.Flags().String(flagTraceStore, "", "Enable KVStore tracing to an output file")
	cmd.Flags().String(flagPruning, "syncable", "Pruning strategy: syncable, nothing, everything")
	cmd.Flags().String(
		FlagMinGasPrices, "",
		"Minimum gas prices to accept for transactions; Any fee in a tx must meet this minimum (e.g. 0.01photino;0.0001stake)",
	)

	// add support for all Tendermint-specific command line options
	// 添加对所有特定于Tendermint的命令行选项的支持
	tcmd.AddNodeFlags(cmd)
	return cmd
}


/**
创建一个 无 tendermint 共识的节点实例
 */
func startStandAlone(ctx *Context, appCreator AppCreator) error {
	addr := viper.GetString(flagAddress)
	home := viper.GetString("home")
	traceWriterFile := viper.GetString(flagTraceStore)

	/**
	打开levelDB实例， 底层用的是 tendermint 的 db库哦 (tendermint 那边用的是 leveldb的)
	 */
	db, err := openDB(home)
	if err != nil {
		return err
	}
	traceWriter, err := openTraceWriter(traceWriterFile)
	if err != nil {
		return err
	}

	app := appCreator(ctx.Logger, db, traceWriter)

	svr, err := server.NewServer(addr, "socket", app)
	if err != nil {
		return fmt.Errorf("error creating listener: %v", err)
	}

	svr.SetLogger(ctx.Logger.With("module", "abci-server"))

	err = svr.Start()
	if err != nil {
		cmn.Exit(err.Error())
	}

	// wait forever
	cmn.TrapSignal(ctx.Logger, func() {
		// cleanup
		err = svr.Stop()
		if err != nil {
			cmn.Exit(err.Error())
		}
	})
	return nil
}


/**
创建一个具备了 tendermint 共识的node实例
注意： db等都是在这添加进去的
 */
func startInProcess(ctx *Context, appCreator AppCreator) (*node.Node, error) {
	cfg := ctx.Config
	home := cfg.RootDir
	traceWriterFile := viper.GetString(flagTraceStore)

	db, err := openDB(home)
	if err != nil {
		return nil, err
	}
	traceWriter, err := openTraceWriter(traceWriterFile)
	if err != nil {
		return nil, err
	}

	// appCreator 其实这个就是  newApp()
	// 所以这里是创建了一个 GaiaApp 实例
	// 这里面就是 实例化各种 keeper 和 注册各种 模块插件的 路由了
	// TODO 注意： cosmos-sdk 有自己的 存储
	app := appCreator(ctx.Logger, db, traceWriter)

	/**
	获取当前 节点的 nodeKey
	 */
	nodeKey, err := p2p.LoadOrGenNodeKey(cfg.NodeKeyFile())
	if err != nil {
		return nil, err
	}

	UpgradeOldPrivValFile(cfg)
	// create & start tendermint node
	/**
	#################
	#################
	TODO 超级重要

	TODO 注意： 这里是实例化了一个 tendermint 节点， 直接用了tendermint的包的哦

	TODO tendermint 节点有自己的存储
	#################
	#################
	 */
	tmNode, err := node.NewNode(
		cfg,
		// 返回的FilePV 其实是一个 tendermint 的types.PrivValidator接口的实例
		pvm.LoadOrGenFilePV(cfg.PrivValidatorKeyFile(), cfg.PrivValidatorStateFile()),
		nodeKey,

		// 使用 GaiaApp 创建一个本地的 client 【这部门查看对应的 tendermint 源码】
		proxy.NewLocalClientCreator(app),
		node.DefaultGenesisDocProviderFunc(cfg),
		node.DefaultDBProvider,
		node.DefaultMetricsProvider(cfg.Instrumentation),
		ctx.Logger.With("module", "node"),
	)
	if err != nil {
		return nil, err
	}

	err = tmNode.Start()
	if err != nil {
		return nil, err
	}

	TrapSignal(func() {
		if tmNode.IsRunning() {
			_ = tmNode.Stop()
		}
	})

	// run forever (the node will not be returned)
	select {}
}
