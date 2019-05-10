package main

import (
	"encoding/json"
	"io"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/libs/cli"
	dbm "github.com/tendermint/tendermint/libs/db"
	"github.com/tendermint/tendermint/libs/log"
	tmtypes "github.com/tendermint/tendermint/types"

	"my-cosmos/cosmos-sdk/baseapp"
	"my-cosmos/cosmos-sdk/client"
	"my-cosmos/cosmos-sdk/cmd/gaia/app"
	gaiaInit "my-cosmos/cosmos-sdk/cmd/gaia/init"
	"my-cosmos/cosmos-sdk/server"
	"my-cosmos/cosmos-sdk/store"
	sdk "my-cosmos/cosmos-sdk/types"
)

// cosmos的主入口
//
// release： v0.34.0
func main() {

	// 开始初始化各种 (自定义的)编码解码器
	cdc := app.MakeCodec()

	// 获取默认配置项， 说白了都是些 前缀信息
	config := sdk.GetConfig()

	// 人工的设置下这些前缀，因为不一定使用默认的(可能使用了命令行的)
	config.SetBech32PrefixForAccount(sdk.Bech32PrefixAccAddr, sdk.Bech32PrefixAccPub)
	config.SetBech32PrefixForValidator(sdk.Bech32PrefixValAddr, sdk.Bech32PrefixValPub)
	config.SetBech32PrefixForConsensusNode(sdk.Bech32PrefixConsAddr, sdk.Bech32PrefixConsPub)
	config.Seal() // 设置不可变更标识位

	// 加载默认的服务上下文
	ctx := server.NewDefaultContext()

	/**
	cobra: 是一个第三方的库：
		其提供简单的接口来创建强大现代的CLI接口，类似于git或者go工具
		Docker的源码中就使用了它

	EnableCommandSorting控制命令切片的排序，默认情况下处于打开状态。
	要禁用排序，请将其设置为false。
	 */
	cobra.EnableCommandSorting = false

	rootCmd := &cobra.Command{
		Use:               "gaiad",
		Short:             "Gaia Daemon (server)",
		PersistentPreRunE: server.PersistentPreRunEFn(ctx),
	}

	// 向第三方库，添加各种命令
	rootCmd.AddCommand(gaiaInit.InitCmd(ctx, cdc))
	rootCmd.AddCommand(gaiaInit.CollectGenTxsCmd(ctx, cdc))
	rootCmd.AddCommand(gaiaInit.TestnetFilesCmd(ctx, cdc))
	// TODO 注册普通交易命令行
	rootCmd.AddCommand(gaiaInit.GenTxCmd(ctx, cdc))


	rootCmd.AddCommand(gaiaInit.AddGenesisAccountCmd(ctx, cdc))
	rootCmd.AddCommand(gaiaInit.ValidateGenesisCmd(ctx, cdc))
	rootCmd.AddCommand(client.NewCompletionCmd(rootCmd, true))


	// 向cosmos-sdk服务添加 各种必须的组件
	// 主要是向底层 tendermint 共识命令行实例注入 参数及组件
	// 其实就是向 rootCmd 中注册各种 内容
	server.AddCommands(ctx, cdc, rootCmd, newApp, exportAppStateAndTMValidators)

	// prepare and add flags
	// 这里就是真正启动 cosmos-sdk
	executor := cli.PrepareBaseCmd(rootCmd, "GA", app.DefaultNodeHome)
	/** 其实这里的执行是，执行 rootCmd */
	err := executor.Execute()
	if err != nil {
		// handle with #870
		panic(err)
	}
}

// ##################
// ##################
// 一个真正的cosmos-sdk 程序的实例创建
// ##################
// ##################
func newApp(logger log.Logger, db dbm.DB, traceStore io.Writer) abci.Application {
	// 创建一个 Gaia App 实例
	// Gaia 其实就是 cosmos hub 程序
	return app.NewGaiaApp(
		logger, db, traceStore, true,
		// Setpruning: 在与应用程序关联的多存储上设置一个修剪选项.
		baseapp.SetPruning(store.NewPruningOptionsFromString(viper.GetString("pruning"))),

		// Setmingasprices: 返回在应用程序上设置最低天然气价格的选项.
		baseapp.SetMinGasPrices(viper.GetString(server.FlagMinGasPrices)),
	)
}

//
func exportAppStateAndTMValidators(
	logger log.Logger, db dbm.DB, traceStore io.Writer, height int64, forZeroHeight bool, jailWhiteList []string,
) (json.RawMessage, []tmtypes.GenesisValidator, error) {
	if height != -1 {
		gApp := app.NewGaiaApp(logger, db, traceStore, false)
		err := gApp.LoadHeight(height)
		if err != nil {
			return nil, nil, err
		}
		return gApp.ExportAppStateAndValidators(forZeroHeight, jailWhiteList)
	}
	gApp := app.NewGaiaApp(logger, db, traceStore, true)
	return gApp.ExportAppStateAndValidators(forZeroHeight, jailWhiteList)
}
