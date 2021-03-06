package keeper

import (
	"encoding/binary"

	sdk "my-cosmos/cosmos-sdk/types"
)

const (
	// default paramspace for params keeper
	DefaultParamspace = "distr"
)

// keys
// 一些超级有用的 key
var (

	// 社区奖励池的 key前缀
	FeePoolKey                        = []byte{0x00} // key for global distribution state
	// 提议人地址的key前缀 (最新区块的提议人)
	ProposerKey                       = []byte{0x01} // key for the proposer operator address
	// 验证人获得的出块奖励的key前缀
	ValidatorOutstandingRewardsPrefix = []byte{0x02} // key for outstanding rewards

	// 委托人获得的出块良奖励的key前缀 ？？？？？
	DelegatorWithdrawAddrPrefix          = []byte{0x03} // key for delegator withdraw address

	// 委托人的起始委托信息的key前缀
	DelegatorStartingInfoPrefix          = []byte{0x04} // key for delegator starting info
	// 验证人的历史出块奖励的key前缀
	ValidatorHistoricalRewardsPrefix     = []byte{0x05} // key for historical validators rewards / stake
	// 验证人的当前出块奖励的可以前缀
	ValidatorCurrentRewardsPrefix        = []byte{0x06} // key for current validator rewards
	// 记录 验证人所积累的佣金 key 前缀
	ValidatorAccumulatedCommissionPrefix = []byte{0x07} // key for accumulated validator commission
	//
	ValidatorSlashEventPrefix            = []byte{0x08} // key for validator slash fraction

	ParamStoreKeyCommunityTax        = []byte("communitytax")

	// 这个是 出块的 基础奖励
	ParamStoreKeyBaseProposerReward  = []byte("baseproposerreward")
	// 这个是奖金?
	ParamStoreKeyBonusProposerReward = []byte("bonusproposerreward")

	// 奖励是否可用标识 key?
	ParamStoreKeyWithdrawAddrEnabled = []byte("withdrawaddrenabled")
)

// gets an address from a validator's outstanding rewards key
func GetValidatorOutstandingRewardsAddress(key []byte) (valAddr sdk.ValAddress) {
	addr := key[1:]
	if len(addr) != sdk.AddrLen {
		panic("unexpected key length")
	}
	return sdk.ValAddress(addr)
}

// gets an address from a delegator's withdraw info key
func GetDelegatorWithdrawInfoAddress(key []byte) (delAddr sdk.AccAddress) {
	addr := key[1:]
	if len(addr) != sdk.AddrLen {
		panic("unexpected key length")
	}
	return sdk.AccAddress(addr)
}

// gets the addresses from a delegator starting info key
func GetDelegatorStartingInfoAddresses(key []byte) (valAddr sdk.ValAddress, delAddr sdk.AccAddress) {
	addr := key[1 : 1+sdk.AddrLen]
	if len(addr) != sdk.AddrLen {
		panic("unexpected key length")
	}
	valAddr = sdk.ValAddress(addr)
	addr = key[1+sdk.AddrLen:]
	if len(addr) != sdk.AddrLen {
		panic("unexpected key length")
	}
	delAddr = sdk.AccAddress(addr)
	return
}

// gets the address & period from a validator's historical rewards key
func GetValidatorHistoricalRewardsAddressPeriod(key []byte) (valAddr sdk.ValAddress, period uint64) {
	addr := key[1 : 1+sdk.AddrLen]
	if len(addr) != sdk.AddrLen {
		panic("unexpected key length")
	}
	valAddr = sdk.ValAddress(addr)
	b := key[1+sdk.AddrLen:]
	if len(b) != 8 {
		panic("unexpected key length")
	}
	period = binary.LittleEndian.Uint64(b)
	return
}

// gets the address from a validator's current rewards key
func GetValidatorCurrentRewardsAddress(key []byte) (valAddr sdk.ValAddress) {
	addr := key[1:]
	if len(addr) != sdk.AddrLen {
		panic("unexpected key length")
	}
	return sdk.ValAddress(addr)
}

// gets the address from a validator's accumulated commission key
func GetValidatorAccumulatedCommissionAddress(key []byte) (valAddr sdk.ValAddress) {
	addr := key[1:]
	if len(addr) != sdk.AddrLen {
		panic("unexpected key length")
	}
	return sdk.ValAddress(addr)
}

// gets the height from a validator's slash event key
func GetValidatorSlashEventAddressHeight(key []byte) (valAddr sdk.ValAddress, height uint64) {
	addr := key[1 : 1+sdk.AddrLen]
	if len(addr) != sdk.AddrLen {
		panic("unexpected key length")
	}
	valAddr = sdk.ValAddress(addr)
	b := key[1+sdk.AddrLen:]
	if len(b) != 8 {
		panic("unexpected key length")
	}
	height = binary.BigEndian.Uint64(b)
	return
}

// gets the outstanding rewards key for a validator
func GetValidatorOutstandingRewardsKey(valAddr sdk.ValAddress) []byte {
	return append(ValidatorOutstandingRewardsPrefix, valAddr.Bytes()...)
}

// gets the key for a delegator's withdraw addr
func GetDelegatorWithdrawAddrKey(delAddr sdk.AccAddress) []byte {
	return append(DelegatorWithdrawAddrPrefix, delAddr.Bytes()...)
}

// gets the key for a delegator's starting info
func GetDelegatorStartingInfoKey(v sdk.ValAddress, d sdk.AccAddress) []byte {
	return append(append(DelegatorStartingInfoPrefix, v.Bytes()...), d.Bytes()...)
}

// gets the prefix key for a validator's historical rewards
func GetValidatorHistoricalRewardsPrefix(v sdk.ValAddress) []byte {
	return append(ValidatorHistoricalRewardsPrefix, v.Bytes()...)
}

// gets the key for a validator's historical rewards
func GetValidatorHistoricalRewardsKey(v sdk.ValAddress, k uint64) []byte {
	b := make([]byte, 8)
	binary.LittleEndian.PutUint64(b, k)
	return append(append(ValidatorHistoricalRewardsPrefix, v.Bytes()...), b...)
}

// gets the key for a validator's current rewards
func GetValidatorCurrentRewardsKey(v sdk.ValAddress) []byte {
	return append(ValidatorCurrentRewardsPrefix, v.Bytes()...)
}

// gets the key for a validator's current commission
func GetValidatorAccumulatedCommissionKey(v sdk.ValAddress) []byte {
	return append(ValidatorAccumulatedCommissionPrefix, v.Bytes()...)
}

// gets the prefix key for a validator's slash fractions
func GetValidatorSlashEventPrefix(v sdk.ValAddress) []byte {
	return append(ValidatorSlashEventPrefix, v.Bytes()...)
}

// gets the key for a validator's slash fraction
func GetValidatorSlashEventKey(v sdk.ValAddress, height uint64) []byte {
	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, height)
	return append(append(ValidatorSlashEventPrefix, v.Bytes()...), b...)
}
