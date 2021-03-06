package client

import (
	"github.com/spf13/cobra"
	amino "github.com/tendermint/go-amino"

	"my-cosmos/cosmos-sdk/client"
	"my-cosmos/cosmos-sdk/x/staking/client/cli"
	"my-cosmos/cosmos-sdk/x/staking/types"
)

// ModuleClient exports all client functionality from this module
type ModuleClient struct {
	storeKey string
	cdc      *amino.Codec
}

func NewModuleClient(storeKey string, cdc *amino.Codec) ModuleClient {
	return ModuleClient{storeKey, cdc}
}

// GetQueryCmd returns the cli query commands for this module
func (mc ModuleClient) GetQueryCmd() *cobra.Command {
	stakingQueryCmd := &cobra.Command{
		Use:   types.ModuleName,
		Short: "Querying commands for the staking module",
	}
	stakingQueryCmd.AddCommand(client.GetCommands(
		cli.GetCmdQueryDelegation(mc.storeKey, mc.cdc),
		cli.GetCmdQueryDelegations(mc.storeKey, mc.cdc),
		cli.GetCmdQueryUnbondingDelegation(mc.storeKey, mc.cdc),
		cli.GetCmdQueryUnbondingDelegations(mc.storeKey, mc.cdc),
		cli.GetCmdQueryRedelegation(mc.storeKey, mc.cdc),
		cli.GetCmdQueryRedelegations(mc.storeKey, mc.cdc),
		cli.GetCmdQueryValidator(mc.storeKey, mc.cdc),
		cli.GetCmdQueryValidators(mc.storeKey, mc.cdc),
		cli.GetCmdQueryValidatorDelegations(mc.storeKey, mc.cdc),
		cli.GetCmdQueryValidatorUnbondingDelegations(mc.storeKey, mc.cdc),
		cli.GetCmdQueryValidatorRedelegations(mc.storeKey, mc.cdc),
		cli.GetCmdQueryParams(mc.storeKey, mc.cdc),
		cli.GetCmdQueryPool(mc.storeKey, mc.cdc))...)

	return stakingQueryCmd

}

// GetTxCmd returns the transaction commands for this module
// todo 返回所有 Staking 相关tx的 Cmd模块
func (mc ModuleClient) GetTxCmd() *cobra.Command {
	stakingTxCmd := &cobra.Command{
		Use:   types.ModuleName,
		Short: "Staking transaction subcommands",
	}

	stakingTxCmd.AddCommand(client.PostCommands(
		// 创建 验证人
		cli.GetCmdCreateValidator(mc.cdc),
		// 修改验证人
		cli.GetCmdEditValidator(mc.cdc),
		// 委托
		cli.GetCmdDelegate(mc.cdc),
		// 重置委托
		cli.GetCmdRedelegate(mc.storeKey, mc.cdc),
		// 解除委托
		cli.GetCmdUnbond(mc.storeKey, mc.cdc),
	)...)

	return stakingTxCmd
}
