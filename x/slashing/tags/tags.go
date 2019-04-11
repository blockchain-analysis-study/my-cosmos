package tags

import (
	sdk "my-cosmos/cosmos-sdk/types"
)

// Slashing tags
var (
	ActionValidatorUnjailed = "validator-unjailed"

	Action    = sdk.TagAction
	Validator = "validator"
)
