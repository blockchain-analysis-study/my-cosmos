package bank

import (
	"fmt"
	"time"

	sdk "my-cosmos/cosmos-sdk/types"
	"my-cosmos/cosmos-sdk/x/auth"
	"my-cosmos/cosmos-sdk/x/params"
)

var _ Keeper = (*BaseKeeper)(nil)

// Keeper defines a module interface that facilitates the transfer of coins
// between accounts.
type Keeper interface {
	SendKeeper

	SetCoins(ctx sdk.Context, addr sdk.AccAddress, amt sdk.Coins) sdk.Error
	SubtractCoins(ctx sdk.Context, addr sdk.AccAddress, amt sdk.Coins) (sdk.Coins, sdk.Tags, sdk.Error)
	AddCoins(ctx sdk.Context, addr sdk.AccAddress, amt sdk.Coins) (sdk.Coins, sdk.Tags, sdk.Error)
	InputOutputCoins(ctx sdk.Context, inputs []Input, outputs []Output) (sdk.Tags, sdk.Error)

	DelegateCoins(ctx sdk.Context, addr sdk.AccAddress, amt sdk.Coins) (sdk.Tags, sdk.Error)
	UndelegateCoins(ctx sdk.Context, addr sdk.AccAddress, amt sdk.Coins) (sdk.Tags, sdk.Error)
}

// BaseKeeper manages transfers between accounts. It implements the Keeper interface.
// BaseKeeper管理帐户之间的转移。 它实现了Keeper接口。
type BaseKeeper struct {
	BaseSendKeeper

	ak         auth.AccountKeeper
	paramSpace params.Subspace
}

// NewBaseKeeper returns a new BaseKeeper
func NewBaseKeeper(ak auth.AccountKeeper,
	paramSpace params.Subspace,
	codespace sdk.CodespaceType) BaseKeeper {

	ps := paramSpace.WithKeyTable(ParamKeyTable())
	return BaseKeeper{
		BaseSendKeeper: NewBaseSendKeeper(ak, ps, codespace),
		ak:             ak,
		paramSpace:     ps,
	}
}

// SetCoins sets the coins at the addr.
func (keeper BaseKeeper) SetCoins(
	ctx sdk.Context, addr sdk.AccAddress, amt sdk.Coins,
) sdk.Error {

	if !amt.IsValid() {
		return sdk.ErrInvalidCoins(amt.String())
	}
	return setCoins(ctx, keeper.ak, addr, amt)
}

// SubtractCoins subtracts amt from the coins at the addr.
func (keeper BaseKeeper) SubtractCoins(
	ctx sdk.Context, addr sdk.AccAddress, amt sdk.Coins,
) (sdk.Coins, sdk.Tags, sdk.Error) {

	if !amt.IsValid() {
		return nil, nil, sdk.ErrInvalidCoins(amt.String())
	}
	return subtractCoins(ctx, keeper.ak, addr, amt)
}

// AddCoins adds amt to the coins at the addr.
// AddCoins 将a追加的 coins追加到对应的addr的coins字段上。
func (keeper BaseKeeper) AddCoins(
	ctx sdk.Context, addr sdk.AccAddress, amt sdk.Coins,
) (sdk.Coins, sdk.Tags, sdk.Error) {

	if !amt.IsValid() {
		return nil, nil, sdk.ErrInvalidCoins(amt.String())
	}

	// AddCoins 将a追加的 coins追加到对应的addr的coins字段上。
	return addCoins(ctx, keeper.ak, addr, amt)
}

// InputOutputCoins handles a list of inputs and outputs
func (keeper BaseKeeper) InputOutputCoins(
	ctx sdk.Context, inputs []Input, outputs []Output,
) (sdk.Tags, sdk.Error) {

	return inputOutputCoins(ctx, keeper.ak, inputs, outputs)
}

// DelegateCoins performs delegation by deducting amt coins from an account with
// address addr. For vesting accounts, delegations amounts are tracked for both
// vesting and vested coins.
/**
DelegateCoins通过从地址为addr的帐户中扣除 目标硬币来执行委托。
对于授权账户，所委托的数量是被可通过授权及授权的代币追溯的
 */
func (keeper BaseKeeper) DelegateCoins(
	ctx sdk.Context, addr sdk.AccAddress, amt sdk.Coins,
) (sdk.Tags, sdk.Error) {


	// 先判断入参的 coins是否可用
	if !amt.IsValid() {
		return nil, sdk.ErrInvalidCoins(amt.String())
	}

	// 更新发起委托的账户信息
	return delegateCoins(ctx, keeper.ak, addr, amt)
}

// UndelegateCoins performs undelegation by crediting amt coins to an account with
// address addr. For vesting accounts, undelegation amounts are tracked for both
// vesting and vested coins.
// If any of the undelegation amounts are negative, an error is returned.
/**
UndelegateCoins通过将amt硬币存入账户来执行解除授权
地址地址。 对于归属账户，可以跟踪两个账户的未抵押金额
归属和归属硬币。
如果任何未撤销金额为负数，则返回错误。
 */
func (keeper BaseKeeper) UndelegateCoins(
	ctx sdk.Context, addr sdk.AccAddress, amt sdk.Coins,
) (sdk.Tags, sdk.Error) {

	if !amt.IsValid() {
		return nil, sdk.ErrInvalidCoins(amt.String())
	}
	return undelegateCoins(ctx, keeper.ak, addr, amt)
}

// SendKeeper defines a module interface that facilitates the transfer of coins
// between accounts without the possibility of creating coins.
type SendKeeper interface {
	ViewKeeper

	SendCoins(ctx sdk.Context, fromAddr sdk.AccAddress, toAddr sdk.AccAddress, amt sdk.Coins) (sdk.Tags, sdk.Error)

	GetSendEnabled(ctx sdk.Context) bool
	SetSendEnabled(ctx sdk.Context, enabled bool)
}

var _ SendKeeper = (*BaseSendKeeper)(nil)

// BaseSendKeeper only allows transfers between accounts without the possibility of
// creating coins. It implements the SendKeeper interface.
// BaseSendKeeper仅允许在帐户之间进行转帐，而无法创建硬币。 它实现了SendKeeper接口。
type BaseSendKeeper struct {
	BaseViewKeeper

	ak         auth.AccountKeeper
	paramSpace params.Subspace
}

// NewBaseSendKeeper returns a new BaseSendKeeper.
func NewBaseSendKeeper(ak auth.AccountKeeper,
	paramSpace params.Subspace, codespace sdk.CodespaceType) BaseSendKeeper {

	return BaseSendKeeper{
		BaseViewKeeper: NewBaseViewKeeper(ak, codespace),
		ak:             ak,
		paramSpace:     paramSpace,
	}
}

// SendCoins moves coins from one account to another
func (keeper BaseSendKeeper) SendCoins(
	ctx sdk.Context, fromAddr sdk.AccAddress, toAddr sdk.AccAddress, amt sdk.Coins,
) (sdk.Tags, sdk.Error) {

	if !amt.IsValid() {
		return nil, sdk.ErrInvalidCoins(amt.String())
	}
	return sendCoins(ctx, keeper.ak, fromAddr, toAddr, amt)
}

// GetSendEnabled returns the current SendEnabled
// nolint: errcheck
func (keeper BaseSendKeeper) GetSendEnabled(ctx sdk.Context) bool {
	var enabled bool
	keeper.paramSpace.Get(ctx, ParamStoreKeySendEnabled, &enabled)
	return enabled
}

// SetSendEnabled sets the send enabled
func (keeper BaseSendKeeper) SetSendEnabled(ctx sdk.Context, enabled bool) {
	keeper.paramSpace.Set(ctx, ParamStoreKeySendEnabled, &enabled)
}

var _ ViewKeeper = (*BaseViewKeeper)(nil)

// ViewKeeper defines a module interface that facilitates read only access to
// account balances.
type ViewKeeper interface {
	GetCoins(ctx sdk.Context, addr sdk.AccAddress) sdk.Coins
	HasCoins(ctx sdk.Context, addr sdk.AccAddress, amt sdk.Coins) bool

	Codespace() sdk.CodespaceType
}

// BaseViewKeeper implements a read only keeper implementation of ViewKeeper.
// BaseViewKeeper 实现了ViewKeeper的只读 keeper 的实现。
type BaseViewKeeper struct {
	ak        auth.AccountKeeper
	codespace sdk.CodespaceType
}

// NewBaseViewKeeper returns a new BaseViewKeeper.
func NewBaseViewKeeper(ak auth.AccountKeeper, codespace sdk.CodespaceType) BaseViewKeeper {
	return BaseViewKeeper{ak: ak, codespace: codespace}
}

// GetCoins returns the coins at the addr.
func (keeper BaseViewKeeper) GetCoins(ctx sdk.Context, addr sdk.AccAddress) sdk.Coins {
	return getCoins(ctx, keeper.ak, addr)
}

// HasCoins returns whether or not an account has at least amt coins.
func (keeper BaseViewKeeper) HasCoins(ctx sdk.Context, addr sdk.AccAddress, amt sdk.Coins) bool {
	return hasCoins(ctx, keeper.ak, addr, amt)
}

// Codespace returns the keeper's codespace.
func (keeper BaseViewKeeper) Codespace() sdk.CodespaceType {
	return keeper.codespace
}

func getCoins(ctx sdk.Context, am auth.AccountKeeper, addr sdk.AccAddress) sdk.Coins {
	acc := am.GetAccount(ctx, addr)
	if acc == nil {
		return sdk.Coins{}
	}
	return acc.GetCoins()
}

func setCoins(ctx sdk.Context, am auth.AccountKeeper, addr sdk.AccAddress, amt sdk.Coins) sdk.Error {
	if !amt.IsValid() {
		return sdk.ErrInvalidCoins(amt.String())
	}
	acc := am.GetAccount(ctx, addr)
	if acc == nil {
		acc = am.NewAccountWithAddress(ctx, addr)
	}
	err := acc.SetCoins(amt)
	if err != nil {
		// Handle w/ #870
		panic(err)
	}
	am.SetAccount(ctx, acc)
	return nil
}

// HasCoins returns whether or not an account has at least amt coins.
func hasCoins(ctx sdk.Context, am auth.AccountKeeper, addr sdk.AccAddress, amt sdk.Coins) bool {
	return getCoins(ctx, am, addr).IsAllGTE(amt)
}

func getAccount(ctx sdk.Context, ak auth.AccountKeeper, addr sdk.AccAddress) auth.Account {
	return ak.GetAccount(ctx, addr)
}

func setAccount(ctx sdk.Context, ak auth.AccountKeeper, acc auth.Account) {
	ak.SetAccount(ctx, acc)
}

// subtractCoins subtracts amt coins from an account with the given address addr.
//
// CONTRACT: If the account is a vesting account, the amount has to be spendable.
func subtractCoins(ctx sdk.Context, ak auth.AccountKeeper, addr sdk.AccAddress, amt sdk.Coins) (sdk.Coins, sdk.Tags, sdk.Error) {

	if !amt.IsValid() {
		return nil, nil, sdk.ErrInvalidCoins(amt.String())
	}

	oldCoins, spendableCoins := sdk.Coins{}, sdk.Coins{}

	acc := getAccount(ctx, ak, addr)
	if acc != nil {
		oldCoins = acc.GetCoins()
		spendableCoins = acc.SpendableCoins(ctx.BlockHeader().Time)
	}

	// For non-vesting accounts, spendable coins will simply be the original coins.
	// So the check here is sufficient instead of subtracting from oldCoins.
	_, hasNeg := spendableCoins.SafeSub(amt)
	if hasNeg {
		return amt, nil, sdk.ErrInsufficientCoins(
			fmt.Sprintf("insufficient account funds; %s < %s", spendableCoins, amt),
		)
	}

	newCoins := oldCoins.Sub(amt) // should not panic as spendable coins was already checked
	err := setCoins(ctx, ak, addr, newCoins)
	tags := sdk.NewTags(TagKeySender, addr.String())

	return newCoins, tags, err
}

// AddCoins adds amt to the coins at the addr.
// AddCoins 将a追加的 coins追加到对应的addr的coins字段上。
func addCoins(ctx sdk.Context, am auth.AccountKeeper, addr sdk.AccAddress, amt sdk.Coins) (sdk.Coins, sdk.Tags, sdk.Error) {

	if !amt.IsValid() {
		return nil, nil, sdk.ErrInvalidCoins(amt.String())
	}

	// 从存储中获取该addr的coins
	oldCoins := getCoins(ctx, am, addr)
	// 追加coins
	newCoins := oldCoins.Add(amt)

	// 如果新的 coins中有一个有负数的话, 则异常
	if newCoins.IsAnyNegative() {
		return amt, nil, sdk.ErrInsufficientCoins(
			fmt.Sprintf("insufficient account funds; %s < %s", oldCoins, amt),
		)
	}

	// 重新设置 某个账户的 coins, 写入存储
	err := setCoins(ctx, am, addr, newCoins)
	tags := sdk.NewTags(TagKeyRecipient, addr.String())

	return newCoins, tags, err
}

// SendCoins moves coins from one account to another
// Returns ErrInvalidCoins if amt is invalid.
func sendCoins(ctx sdk.Context, am auth.AccountKeeper, fromAddr sdk.AccAddress, toAddr sdk.AccAddress, amt sdk.Coins) (sdk.Tags, sdk.Error) {
	// Safety check ensuring that when sending coins the keeper must maintain the
	if !amt.IsValid() {
		return nil, sdk.ErrInvalidCoins(amt.String())
	}

	_, subTags, err := subtractCoins(ctx, am, fromAddr, amt)
	if err != nil {
		return nil, err
	}

	_, addTags, err := addCoins(ctx, am, toAddr, amt)
	if err != nil {
		return nil, err
	}

	return subTags.AppendTags(addTags), nil
}

// InputOutputCoins handles a list of inputs and outputs
// NOTE: Make sure to revert state changes from tx on error
func inputOutputCoins(ctx sdk.Context, am auth.AccountKeeper, inputs []Input, outputs []Output) (sdk.Tags, sdk.Error) {
	// Safety check ensuring that when sending coins the keeper must maintain the
	// Check supply invariant and validity of Coins.
	if err := ValidateInputsOutputs(inputs, outputs); err != nil {
		return nil, err
	}

	allTags := sdk.EmptyTags()

	for _, in := range inputs {
		_, tags, err := subtractCoins(ctx, am, in.Address, in.Coins)
		if err != nil {
			return nil, err
		}
		allTags = allTags.AppendTags(tags)
	}

	for _, out := range outputs {
		_, tags, err := addCoins(ctx, am, out.Address, out.Coins)
		if err != nil {
			return nil, err
		}
		allTags = allTags.AppendTags(tags)
	}

	return allTags, nil
}

func delegateCoins(
	ctx sdk.Context, ak auth.AccountKeeper, addr sdk.AccAddress, amt sdk.Coins,
) (sdk.Tags, sdk.Error) {


	// 判断coins是否可用？
	if !amt.IsValid() {
		return nil, sdk.ErrInvalidCoins(amt.String())
	}

	// 根据入参的地址(一般就是发起委托的地址), 获取该账户的信息
	// acc 是一个指针的接口引用
	acc := getAccount(ctx, ak, addr)
	if acc == nil {
		return nil, sdk.ErrUnknownAddress(fmt.Sprintf("account %s does not exist", addr))
	}

	// 获取该账户上原来的钱
	oldCoins := acc.GetCoins()

	// 先做一步相减判断
	_, hasNeg := oldCoins.SafeSub(amt)
	if hasNeg {
		return nil, sdk.ErrInsufficientCoins(
			fmt.Sprintf("insufficient account funds; %s < %s", oldCoins, amt),
		)
	}

	// 记录下旧信息,使之变为可追溯的
	if err := trackDelegation(acc, ctx.BlockHeader().Time, amt); err != nil {
		return nil, sdk.ErrInternal(fmt.Sprintf("failed to track delegation: %v", err))
	}

	// 将该账户存储起来
	setAccount(ctx, ak, acc)

	return sdk.NewTags(
		sdk.TagAction, TagActionDelegateCoins,
		sdk.TagDelegator, []byte(addr.String()),
	), nil
}

// 解锁委托的钱，并原路返回到 委托人账户中
func undelegateCoins(
	ctx sdk.Context, ak auth.AccountKeeper, addr sdk.AccAddress, amt sdk.Coins,
) (sdk.Tags, sdk.Error) {

	if !amt.IsValid() {
		return nil, sdk.ErrInvalidCoins(amt.String())
	}

	acc := getAccount(ctx, ak, addr)
	if acc == nil {
		return nil, sdk.ErrUnknownAddress(fmt.Sprintf("account %s does not exist", addr))
	}

	// 这里面，把钱加回账户上
	if err := trackUndelegation(acc, amt); err != nil {
		return nil, sdk.ErrInternal(fmt.Sprintf("failed to track undelegation: %v", err))
	}

	setAccount(ctx, ak, acc)

	return sdk.NewTags(
		sdk.TagAction, TagActionUndelegateCoins,
		sdk.TagDelegator, []byte(addr.String()),
	), nil
}

// CONTRACT: assumes that amt is valid.
// 合约：假设amt有效。
func trackDelegation(acc auth.Account, blockTime time.Time, amt sdk.Coins) error {
	vacc, ok := acc.(auth.VestingAccount)
	if ok {
		// 授权和取消授权账户处理，返回产生的基础硬币金额。
		vacc.TrackDelegation(blockTime, amt)
		return nil
	}

	// 真正的去扣减该账户上的钱, 并设置到账户实体中
	return acc.SetCoins(acc.GetCoins().Sub(amt))
}

// CONTRACT: assumes that amt is valid.
func trackUndelegation(acc auth.Account, amt sdk.Coins) error {
	vacc, ok := acc.(auth.VestingAccount)
	if ok {
		vacc.TrackUndelegation(amt)
		return nil
	}

	// 把钱加回账户上
	return acc.SetCoins(acc.GetCoins().Add(amt))
}
