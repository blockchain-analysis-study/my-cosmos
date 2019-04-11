package simulation

import (
	sdk "my-cosmos/cosmos-sdk/types"
)

// TODO Any invariants to check here?
// AllInvariants tests all slashing invariants
func AllInvariants() sdk.Invariant {
	return func(_ sdk.Context) error {
		return nil
	}
}
