package types

import (
	"fmt"

	sdk "my-cosmos/cosmos-sdk/types"
)

// global fee pool for distribution
// 全局的佣金分配池？
type FeePool struct {
	// 尚未花费的社区资金池
	CommunityPool sdk.DecCoins `json:"community_pool"` // pool for community funds yet to be spent
}

// zero fee pool
func InitialFeePool() FeePool {
	return FeePool{
		CommunityPool: sdk.DecCoins{},
	}
}

// ValidateGenesis validates the fee pool for a genesis state
func (f FeePool) ValidateGenesis() error {
	if f.CommunityPool.IsAnyNegative() {
		return fmt.Errorf("negative CommunityPool in distribution fee pool, is %v",
			f.CommunityPool)
	}

	return nil
}
