package gov

import (
	"time"

	codec "my-cosmos/cosmos-sdk/codec"
	sdk "my-cosmos/cosmos-sdk/types"
	"my-cosmos/cosmos-sdk/x/params"

	"github.com/tendermint/tendermint/crypto"
)

const (
	// ModuleKey is the name of the module
	ModuleName = "gov"

	// StoreKey is the store key string for gov
	StoreKey = ModuleName

	// RouterKey is the message route for gov
	RouterKey = ModuleName

	// QuerierRoute is the querier route for gov
	QuerierRoute = ModuleName

	// Parameter store default namestore
	DefaultParamspace = ModuleName
)

// Parameter store key
var (

	// 治理 质押的key
	ParamStoreKeyDepositParams = []byte("depositparams")
	// 治理 投票的key
	ParamStoreKeyVotingParams  = []byte("votingparams")
	ParamStoreKeyTallyParams   = []byte("tallyparams")

	// TODO: Find another way to implement this without using accounts, or find a cleaner way to implement it using accounts.
	// TODO：找到另一种方法来实现这一点而不使用帐户，或者找到一种更简洁的方法来使用帐户实现它。
	DepositedCoinsAccAddr     = sdk.AccAddress(crypto.AddressHash([]byte("govDepositedCoins")))
	BurnedDepositCoinsAccAddr = sdk.AccAddress(crypto.AddressHash([]byte("govBurnedDepositCoins")))
)

// Key declaration for parameters
func ParamKeyTable() params.KeyTable {
	return params.NewKeyTable(
		ParamStoreKeyDepositParams, DepositParams{},
		ParamStoreKeyVotingParams, VotingParams{},
		ParamStoreKeyTallyParams, TallyParams{},
	)
}

// Governance Keeper
type Keeper struct {
	// The reference to the Param Keeper to get and set Global Params
	paramsKeeper params.Keeper

	// The reference to the Paramstore to get and set gov specific params
	paramSpace params.Subspace

	// The reference to the CoinKeeper to modify balances
	ck BankKeeper

	// The ValidatorSet to get information about validators
	vs sdk.ValidatorSet

	// The reference to the DelegationSet to get information about delegators
	ds sdk.DelegationSet

	// The (unexposed) keys used to access the stores from the Context.
	storeKey sdk.StoreKey

	// The codec codec for binary encoding/decoding.
	cdc *codec.Codec

	// Reserved codespace
	codespace sdk.CodespaceType
}

// NewKeeper returns a governance keeper. It handles:
// - submitting governance proposals
// - depositing funds into proposals, and activating upon sufficient funds being deposited
// - users voting on proposals, with weight proportional to stake in the system
// - and tallying the result of the vote.
func NewKeeper(cdc *codec.Codec, key sdk.StoreKey, paramsKeeper params.Keeper,
	paramSpace params.Subspace, ck BankKeeper, ds sdk.DelegationSet, codespace sdk.CodespaceType) Keeper {

	return Keeper{
		storeKey:     key,
		paramsKeeper: paramsKeeper,
		paramSpace:   paramSpace.WithKeyTable(ParamKeyTable()),
		ck:           ck,
		ds:           ds,
		vs:           ds.GetValidatorSet(),
		cdc:          cdc,
		codespace:    codespace,
	}
}

// Proposals

// Creates a NewProposal
/**
创建 一个治理相关的 提案 (文本提案？？)
 */
func (keeper Keeper) NewTextProposal(ctx sdk.Context, title string, description string, proposalType ProposalKind) Proposal {
	// 生成一个新的提案ID
	proposalID, err := keeper.getNewProposalID(ctx)
	if err != nil {
		return nil
	}

	// 创建一个文本提案
	var proposal Proposal = &TextProposal{
		ProposalID:       proposalID,
		Title:            title,
		Description:      description,
		ProposalType:     proposalType,
		Status:           StatusDepositPeriod,
		FinalTallyResult: EmptyTallyResult(),
		TotalDeposit:     sdk.Coins{},
		SubmitTime:       ctx.BlockHeader().Time,
	}

	// 获取治理质押的起始周期
	depositPeriod := keeper.GetDepositParams(ctx).MaxDepositPeriod
	proposal.SetDepositEndTime(proposal.GetSubmitTime().Add(depositPeriod))

	// 保存新提案信息
	keeper.SetProposal(ctx, proposal)
	// 将提案ID往 非激活提案队列中追加
	keeper.InsertInactiveProposalQueue(ctx, proposal.GetDepositEndTime(), proposalID)
	return proposal
}

// Get Proposal from store by ProposalID
func (keeper Keeper) GetProposal(ctx sdk.Context, proposalID uint64) Proposal {
	store := ctx.KVStore(keeper.storeKey)
	bz := store.Get(KeyProposal(proposalID))
	if bz == nil {
		return nil
	}
	var proposal Proposal
	keeper.cdc.MustUnmarshalBinaryLengthPrefixed(bz, &proposal)
	return proposal
}

// Implements sdk.AccountKeeper.
func (keeper Keeper) SetProposal(ctx sdk.Context, proposal Proposal) {
	store := ctx.KVStore(keeper.storeKey)
	bz := keeper.cdc.MustMarshalBinaryLengthPrefixed(proposal)
	store.Set(KeyProposal(proposal.GetProposalID()), bz)
}

// Implements sdk.AccountKeeper.
// 清除某提案相关信息
func (keeper Keeper) DeleteProposal(ctx sdk.Context, proposalID uint64) {
	store := ctx.KVStore(keeper.storeKey)
	proposal := keeper.GetProposal(ctx, proposalID)
	// 非激活队列中清除 提案ID
	keeper.RemoveFromInactiveProposalQueue(ctx, proposal.GetDepositEndTime(), proposalID)
	// 激活队列中删除提案ID (因为做成通用的，所有 非激活和激活都需要删一遍)
	keeper.RemoveFromActiveProposalQueue(ctx, proposal.GetVotingEndTime(), proposalID)
	// 删掉提案信息
	store.Delete(KeyProposal(proposalID))
}

// Get Proposal from store by ProposalID
// voterAddr will filter proposals by whether or not that address has voted on them
// depositorAddr will filter proposals by whether or not that address has deposited to them
// status will filter proposals by status
// numLatest will fetch a specified number of the most recent proposals, or 0 for all proposals
func (keeper Keeper) GetProposalsFiltered(ctx sdk.Context, voterAddr sdk.AccAddress, depositorAddr sdk.AccAddress, status ProposalStatus, numLatest uint64) []Proposal {

	maxProposalID, err := keeper.peekCurrentProposalID(ctx)
	if err != nil {
		return nil
	}

	matchingProposals := []Proposal{}

	if numLatest == 0 {
		numLatest = maxProposalID
	}

	for proposalID := maxProposalID - numLatest; proposalID < maxProposalID; proposalID++ {
		if voterAddr != nil && len(voterAddr) != 0 {
			_, found := keeper.GetVote(ctx, proposalID, voterAddr)
			if !found {
				continue
			}
		}

		if depositorAddr != nil && len(depositorAddr) != 0 {
			_, found := keeper.GetDeposit(ctx, proposalID, depositorAddr)
			if !found {
				continue
			}
		}

		proposal := keeper.GetProposal(ctx, proposalID)
		if proposal == nil {
			continue
		}

		if validProposalStatus(status) {
			if proposal.GetStatus() != status {
				continue
			}
		}

		matchingProposals = append(matchingProposals, proposal)
	}
	return matchingProposals
}

// Set the initial proposal ID
func (keeper Keeper) setInitialProposalID(ctx sdk.Context, proposalID uint64) sdk.Error {
	store := ctx.KVStore(keeper.storeKey)
	bz := store.Get(KeyNextProposalID)
	if bz != nil {
		return ErrInvalidGenesis(keeper.codespace, "Initial ProposalID already set")
	}
	bz = keeper.cdc.MustMarshalBinaryLengthPrefixed(proposalID)
	store.Set(KeyNextProposalID, bz)
	return nil
}

// Get the last used proposal ID
func (keeper Keeper) GetLastProposalID(ctx sdk.Context) (proposalID uint64) {
	proposalID, err := keeper.peekCurrentProposalID(ctx)
	if err != nil {
		return 0
	}
	proposalID--
	return
}

// Gets the next available ProposalID and increments it
// 生成一个新的提案ID
func (keeper Keeper) getNewProposalID(ctx sdk.Context) (proposalID uint64, err sdk.Error) {
	store := ctx.KVStore(keeper.storeKey)
	bz := store.Get(KeyNextProposalID)
	if bz == nil {
		return 0, ErrInvalidGenesis(keeper.codespace, "InitialProposalID never set")
	}
	// 解码出 最新的这个提案的ID
	keeper.cdc.MustUnmarshalBinaryLengthPrefixed(bz, &proposalID)
	// 对最新的这个提案Id + 1 得到当前新的提案Id
	bz = keeper.cdc.MustMarshalBinaryLengthPrefixed(proposalID + 1)
	// 把当前提案Id设置成最新的提案Id
	store.Set(KeyNextProposalID, bz)
	// 返回当前提案Id
	return proposalID, nil
}

// Peeks the next available ProposalID without incrementing it
func (keeper Keeper) peekCurrentProposalID(ctx sdk.Context) (proposalID uint64, err sdk.Error) {
	store := ctx.KVStore(keeper.storeKey)
	bz := store.Get(KeyNextProposalID)
	if bz == nil {
		return 0, ErrInvalidGenesis(keeper.codespace, "InitialProposalID never set")
	}
	keeper.cdc.MustUnmarshalBinaryLengthPrefixed(bz, &proposalID)
	return proposalID, nil
}

// 开始激活提案投票周期
func (keeper Keeper) activateVotingPeriod(ctx sdk.Context, proposal Proposal) {
	// 获取当前block的时间戳
	proposal.SetVotingStartTime(ctx.BlockHeader().Time)
	// 获取上下文中的 最长投票周期限制
	votingPeriod := keeper.GetVotingParams(ctx).VotingPeriod
	// 根据当前block的时间戳和 最长投票期限先算出该提案投票终止时间
	proposal.SetVotingEndTime(proposal.GetVotingStartTime().Add(votingPeriod))
	// 更改提案状态为 投票周期
	proposal.SetStatus(StatusVotingPeriod)
	// 保存提案变更信息
	keeper.SetProposal(ctx, proposal)

	// 从非活动(非激活)提案队列中删除提案ID
	keeper.RemoveFromInactiveProposalQueue(ctx, proposal.GetDepositEndTime(), proposal.GetProposalID())
	// 追加到 激活提案队列
	keeper.InsertActiveProposalQueue(ctx, proposal.GetVotingEndTime(), proposal.GetProposalID())
}

// Params

// Returns the current DepositParams from the global param store
func (keeper Keeper) GetDepositParams(ctx sdk.Context) DepositParams {
	var depositParams DepositParams
	keeper.paramSpace.Get(ctx, ParamStoreKeyDepositParams, &depositParams)
	return depositParams
}

// Returns the current VotingParams from the global param store
func (keeper Keeper) GetVotingParams(ctx sdk.Context) VotingParams {
	var votingParams VotingParams
	keeper.paramSpace.Get(ctx, ParamStoreKeyVotingParams, &votingParams)
	return votingParams
}

// Returns the current TallyParam from the global param store
func (keeper Keeper) GetTallyParams(ctx sdk.Context) TallyParams {
	var tallyParams TallyParams
	keeper.paramSpace.Get(ctx, ParamStoreKeyTallyParams, &tallyParams)
	return tallyParams
}

func (keeper Keeper) setDepositParams(ctx sdk.Context, depositParams DepositParams) {
	keeper.paramSpace.Set(ctx, ParamStoreKeyDepositParams, &depositParams)
}

func (keeper Keeper) setVotingParams(ctx sdk.Context, votingParams VotingParams) {
	keeper.paramSpace.Set(ctx, ParamStoreKeyVotingParams, &votingParams)
}

func (keeper Keeper) setTallyParams(ctx sdk.Context, tallyParams TallyParams) {
	keeper.paramSpace.Set(ctx, ParamStoreKeyTallyParams, &tallyParams)
}

// Votes

// Adds a vote on a specific proposal
// 添加对特定提案的投票
func (keeper Keeper) AddVote(ctx sdk.Context, proposalID uint64, voterAddr sdk.AccAddress, option VoteOption) sdk.Error {
	// 先获取提案，非空判断一波
	proposal := keeper.GetProposal(ctx, proposalID)
	if proposal == nil {
		return ErrUnknownProposal(keeper.codespace, proposalID)
	}
	// 是否为投票轮状态
	if proposal.GetStatus() != StatusVotingPeriod {
		return ErrInactiveProposal(keeper.codespace, proposalID)
	}

	if !validVoteOption(option) {
		return ErrInvalidVote(keeper.codespace, option)
	}

	/**
	实例化一个提案的 投票
	 */
	vote := Vote{
		ProposalID: proposalID,
		Voter:      voterAddr,
		Option:     option,
	}
	// 开始对提案进行投票
	keeper.setVote(ctx, proposalID, voterAddr, vote)

	return nil
}

// Gets the vote of a specific voter on a specific proposal
func (keeper Keeper) GetVote(ctx sdk.Context, proposalID uint64, voterAddr sdk.AccAddress) (Vote, bool) {
	store := ctx.KVStore(keeper.storeKey)
	bz := store.Get(KeyVote(proposalID, voterAddr))
	if bz == nil {
		return Vote{}, false
	}
	var vote Vote
	keeper.cdc.MustUnmarshalBinaryLengthPrefixed(bz, &vote)
	return vote, true
}

// 保存提案投票信息
func (keeper Keeper) setVote(ctx sdk.Context, proposalID uint64, voterAddr sdk.AccAddress, vote Vote) {
	store := ctx.KVStore(keeper.storeKey)
	bz := keeper.cdc.MustMarshalBinaryLengthPrefixed(vote)
	store.Set(KeyVote(proposalID, voterAddr), bz)
}

// Gets all the votes on a specific proposal
func (keeper Keeper) GetVotes(ctx sdk.Context, proposalID uint64) sdk.Iterator {
	store := ctx.KVStore(keeper.storeKey)
	return sdk.KVStorePrefixIterator(store, KeyVotesSubspace(proposalID))
}

func (keeper Keeper) deleteVote(ctx sdk.Context, proposalID uint64, voterAddr sdk.AccAddress) {
	store := ctx.KVStore(keeper.storeKey)
	store.Delete(KeyVote(proposalID, voterAddr))
}

// Deposits

// Gets the deposit of a specific depositor on a specific proposal
func (keeper Keeper) GetDeposit(ctx sdk.Context, proposalID uint64, depositorAddr sdk.AccAddress) (Deposit, bool) {
	store := ctx.KVStore(keeper.storeKey)
	bz := store.Get(KeyDeposit(proposalID, depositorAddr))
	if bz == nil {
		return Deposit{}, false
	}
	var deposit Deposit
	keeper.cdc.MustUnmarshalBinaryLengthPrefixed(bz, &deposit)
	return deposit, true
}

func (keeper Keeper) setDeposit(ctx sdk.Context, proposalID uint64, depositorAddr sdk.AccAddress, deposit Deposit) {
	store := ctx.KVStore(keeper.storeKey)
	bz := keeper.cdc.MustMarshalBinaryLengthPrefixed(deposit)
	store.Set(KeyDeposit(proposalID, depositorAddr), bz)
}

// Adds or updates a deposit of a specific depositor on a specific proposal
// Activates voting period when appropriate
/**
添加提案上的 质押
在特定提案上添加或更新特定存款人的存款在适当时激活投票期
 */
func (keeper Keeper) AddDeposit(ctx sdk.Context, proposalID uint64, depositorAddr sdk.AccAddress, depositAmount sdk.Coins) (sdk.Error, bool) {
	// Checks to see if proposal exists
	// 根据提案ID检查提案是否存在
	proposal := keeper.GetProposal(ctx, proposalID)
	if proposal == nil {
		return ErrUnknownProposal(keeper.codespace, proposalID), false
	}

	// Check if proposal is still depositable
	// 检查提案是否仍可存款(质押)
	// 如果当前提案的状态不属于 质押轮 也不属于 投票轮
	if (proposal.GetStatus() != StatusDepositPeriod) && (proposal.GetStatus() != StatusVotingPeriod) {
		return ErrAlreadyFinishedProposal(keeper.codespace, proposalID), false
	}

	// Send coins from depositor's account to DepositedCoinsAccAddr account
	// TODO: Don't use an account for this purpose; it's clumsy and prone to misuse.
	/**
	将存款人账户中的硬币发送到 DepositedCoinsAccAddr账户
	TODO：不要为此目的使用帐户; 它很笨拙，容易被滥用。
	 */
	_, err := keeper.ck.SendCoins(ctx, depositorAddr, DepositedCoinsAccAddr, depositAmount)
	if err != nil {
		return err, false
	}

	// Update proposal
	// 更新提案的质押金额信息
	proposal.SetTotalDeposit(proposal.GetTotalDeposit().Add(depositAmount))
	keeper.SetProposal(ctx, proposal)

	// Check if deposit has provided sufficient total funds to transition the proposal into the voting period
	// TODO
	//
	// 检查存款是否提供足够的总资金以将提案转换为投票期
	// 可以看出来，是属于每次增持质押时主动触发  提案状态的变更
	activatedVotingPeriod := false
	// 如果现在属于 质押状态 且 所质押的钱满足 最小 提案投票质押金门槛
	if proposal.GetStatus() == StatusDepositPeriod && proposal.GetTotalDeposit().IsAllGTE(keeper.GetDepositParams(ctx).MinDeposit) {
		// 开始激活提案投票周期
		keeper.activateVotingPeriod(ctx, proposal)
		activatedVotingPeriod = true
	}

	// Add or update deposit object
	//
	currDeposit, found := keeper.GetDeposit(ctx, proposalID, depositorAddr)
	if !found {
		newDeposit := Deposit{depositorAddr, proposalID, depositAmount}
		keeper.setDeposit(ctx, proposalID, depositorAddr, newDeposit)
	} else {
		currDeposit.Amount = currDeposit.Amount.Add(depositAmount)
		keeper.setDeposit(ctx, proposalID, depositorAddr, currDeposit)
	}

	return nil, activatedVotingPeriod
}

// Gets all the deposits on a specific proposal as an sdk.Iterator
func (keeper Keeper) GetDeposits(ctx sdk.Context, proposalID uint64) sdk.Iterator {
	store := ctx.KVStore(keeper.storeKey)
	return sdk.KVStorePrefixIterator(store, KeyDepositsSubspace(proposalID))
}

// Refunds and deletes all the deposits on a specific proposal
func (keeper Keeper) RefundDeposits(ctx sdk.Context, proposalID uint64) {
	store := ctx.KVStore(keeper.storeKey)
	depositsIterator := keeper.GetDeposits(ctx, proposalID)
	defer depositsIterator.Close()
	for ; depositsIterator.Valid(); depositsIterator.Next() {
		deposit := &Deposit{}
		keeper.cdc.MustUnmarshalBinaryLengthPrefixed(depositsIterator.Value(), deposit)

		_, err := keeper.ck.SendCoins(ctx, DepositedCoinsAccAddr, deposit.Depositor, deposit.Amount)
		if err != nil {
			panic("should not happen")
		}

		store.Delete(depositsIterator.Key())
	}
}

// Deletes all the deposits on a specific proposal without refunding them
func (keeper Keeper) DeleteDeposits(ctx sdk.Context, proposalID uint64) {
	store := ctx.KVStore(keeper.storeKey)
	// 根据提案ID 获取该提案上的所有质押
	depositsIterator := keeper.GetDeposits(ctx, proposalID)
	defer depositsIterator.Close()
	// 遍历它们
	for ; depositsIterator.Valid(); depositsIterator.Next() {
		deposit := &Deposit{}
		keeper.cdc.MustUnmarshalBinaryLengthPrefixed(depositsIterator.Value(), deposit)

		// TODO: Find a way to do this without using accounts.
		// 找到一种更好的方法，不能用addr 来做
		// 将对应的钱从 提案质押账户 转到 销毁账户 (注意：销毁账户这里其实只是为了做一个记录用的，不上链)
		_, err := keeper.ck.SendCoins(ctx, DepositedCoinsAccAddr, BurnedDepositCoinsAccAddr, deposit.Amount)
		if err != nil {
			panic("should not happen")
		}

		// 删除掉这些质押信息
		store.Delete(depositsIterator.Key())
	}
}

// ProposalQueues

// Returns an iterator for all the proposals in the Active Queue that expire by endTime
func (keeper Keeper) ActiveProposalQueueIterator(ctx sdk.Context, endTime time.Time) sdk.Iterator {
	store := ctx.KVStore(keeper.storeKey)
	return store.Iterator(PrefixActiveProposalQueue, sdk.PrefixEndBytes(PrefixActiveProposalQueueTime(endTime)))
}

// Inserts a ProposalID into the active proposal queue at endTime
func (keeper Keeper) InsertActiveProposalQueue(ctx sdk.Context, endTime time.Time, proposalID uint64) {
	store := ctx.KVStore(keeper.storeKey)
	bz := keeper.cdc.MustMarshalBinaryLengthPrefixed(proposalID)
	store.Set(KeyActiveProposalQueueProposal(endTime, proposalID), bz)
}

// removes a proposalID from the Active Proposal Queue
func (keeper Keeper) RemoveFromActiveProposalQueue(ctx sdk.Context, endTime time.Time, proposalID uint64) {
	store := ctx.KVStore(keeper.storeKey)
	store.Delete(KeyActiveProposalQueueProposal(endTime, proposalID))
}

// Returns an iterator for all the proposals in the Inactive Queue that expire by endTime
func (keeper Keeper) InactiveProposalQueueIterator(ctx sdk.Context, endTime time.Time) sdk.Iterator {
	store := ctx.KVStore(keeper.storeKey)
	return store.Iterator(PrefixInactiveProposalQueue, sdk.PrefixEndBytes(PrefixInactiveProposalQueueTime(endTime)))
}

// Inserts a ProposalID into the inactive proposal queue at endTime
func (keeper Keeper) InsertInactiveProposalQueue(ctx sdk.Context, endTime time.Time, proposalID uint64) {
	store := ctx.KVStore(keeper.storeKey)
	bz := keeper.cdc.MustMarshalBinaryLengthPrefixed(proposalID)
	store.Set(KeyInactiveProposalQueueProposal(endTime, proposalID), bz)
}

// removes a proposalID from the Inactive Proposal Queue
// 从非活动(非激活)提案队列中删除提案ID
func (keeper Keeper) RemoveFromInactiveProposalQueue(ctx sdk.Context, endTime time.Time, proposalID uint64) {
	store := ctx.KVStore(keeper.storeKey)
	store.Delete(KeyInactiveProposalQueueProposal(endTime, proposalID))
}
