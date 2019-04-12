package params

import (
	"testing"

	sdk "my-cosmos/cosmos-sdk/types"

	"my-cosmos/cosmos-sdk/x/params/subspace"
)

// re-export types from subspace
// 从子空间重新导出类型 ?
type (
	// 参数仓库？
	Subspace         = subspace.Subspace

	// 只读仓库？
	ReadOnlySubspace = subspace.ReadOnlySubspace

	//
	ParamSet         = subspace.ParamSet
	ParamSetPairs    = subspace.ParamSetPairs
	KeyTable         = subspace.KeyTable
)

// nolint - re-export functions from subspace
func NewKeyTable(keytypes ...interface{}) KeyTable {
	return subspace.NewKeyTable(keytypes...)
}
func DefaultTestComponents(t *testing.T) (sdk.Context, Subspace, func() sdk.CommitID) {
	return subspace.DefaultTestComponents(t)
}
