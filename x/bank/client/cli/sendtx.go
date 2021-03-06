package cli

import (
	"fmt"

	"my-cosmos/cosmos-sdk/client"
	"my-cosmos/cosmos-sdk/client/context"
	"my-cosmos/cosmos-sdk/client/utils"
	"my-cosmos/cosmos-sdk/codec"
	sdk "my-cosmos/cosmos-sdk/types"
	authtxb "my-cosmos/cosmos-sdk/x/auth/client/txbuilder"
	"my-cosmos/cosmos-sdk/x/bank"

	"github.com/spf13/cobra"
)

const (
	flagTo     = "to"
	flagAmount = "amount"
)

// SendTxCmd will create a send tx and sign it with the given key.
// SendTxCmd将创建一个send tx并使用给定的密钥对其进行签名
/**
发起一个普通交易
 */
func SendTxCmd(cdc *codec.Codec) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "send [to_address] [amount]",
		Short: "Create and sign a send tx",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			txBldr := authtxb.NewTxBuilderFromCLI().WithTxEncoder(utils.GetTxEncoder(cdc))
			cliCtx := context.NewCLIContext().
				WithCodec(cdc).
				WithAccountDecoder(cdc)

			if err := cliCtx.EnsureAccountExists(); err != nil {
				return err
			}

			to, err := sdk.AccAddressFromBech32(args[0])
			if err != nil {
				return err
			}

			// parse coins trying to be sent
			coins, err := sdk.ParseCoins(args[1])
			if err != nil {
				return err
			}

			from := cliCtx.GetFromAddress()
			account, err := cliCtx.GetAccount(from)
			if err != nil {
				return err
			}

			// ensure account has enough coins
			if !account.GetCoins().IsAllGTE(coins) {
				return fmt.Errorf("address %s doesn't have enough coins to pay for this transaction", from)
			}

			// build and sign the transaction, then broadcast to Tendermint
			msg := bank.NewMsgSend(from, to, coins)
			return utils.GenerateOrBroadcastMsgs(cliCtx, txBldr, []sdk.Msg{msg}, false)
		},
	}
	return client.PostCommands(cmd)[0]
}
