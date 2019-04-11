package rest

import (
	"github.com/gorilla/mux"

	"my-cosmos/cosmos-sdk/client/context"
	"my-cosmos/cosmos-sdk/codec"
)

// RegisterRoutes register distribution REST routes.
func RegisterRoutes(cliCtx context.CLIContext, r *mux.Router, cdc *codec.Codec, queryRoute string) {
	registerQueryRoutes(cliCtx, r, cdc, queryRoute)
	registerTxRoutes(cliCtx, r, cdc, queryRoute)
}
