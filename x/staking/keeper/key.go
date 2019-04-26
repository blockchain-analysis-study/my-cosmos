package keeper

import (
	"encoding/binary"
	"time"

	sdk "my-cosmos/cosmos-sdk/types"
	"my-cosmos/cosmos-sdk/x/staking/types"
)

// TODO remove some of these prefixes once have working multistore

//nolint
/**
##############
存储在DB中的Key的前缀 定义
#############
 */
var (
	// Keys for store prefixes
	PoolKey = []byte{0x00} // key for the staking pools

	// Last* values are constant during a block.
	/**
	最新的验证人的权重信息 key前缀
	 */
	LastValidatorPowerKey = []byte{0x11} // prefix for each key to a validator index, for bonded validators
	/**
	最新的验证人 总权重信息 key
	 */
	LastTotalPowerKey     = []byte{0x12} // prefix for the total power

	/**
	验证人的key前缀
	 */
	ValidatorsKey             = []byte{0x21} // prefix for each key to a validator
	/**
	验证人的公钥索引做的key前缀
	 */
	ValidatorsByConsAddrKey   = []byte{0x22} // prefix for each key to a validator index, by pubkey

	/**
	TODO 重要的 key
	验证人的权重key前缀
	 */
	ValidatorsByPowerIndexKey = []byte{0x23} // prefix for each key to a validator index, sorted by power


	/**
	当前节点上所有委托人的信息的
	 */
	DelegationKey                    = []byte{0x31} // key for a delegation
	UnbondingDelegationKey           = []byte{0x32} // key for an unbonding-delegation
	UnbondingDelegationByValIndexKey = []byte{0x33} // prefix for each key for an unbonding-delegation, by validator operator
	RedelegationKey                  = []byte{0x34} // key for a redelegation
	RedelegationByValSrcIndexKey     = []byte{0x35} // prefix for each key for an redelegation, by source validator operator
	RedelegationByValDstIndexKey     = []byte{0x36} // prefix for each key for an redelegation, by destination validator operator

	/**
	unbonding队列中时间戳的前缀
	 */
	UnbondingQueueKey    = []byte{0x41} // prefix for the timestamps in unbonding queue

	/**
	重新授权委托队列中时间戳的前缀
	 */
	RedelegationQueueKey = []byte{0x42} // prefix for the timestamps in redelegations queue

	/**
	验证器队列中时间戳的前缀
	 */
	ValidatorQueueKey    = []byte{0x43} // prefix for the timestamps in validator queue
)

// gets the key for the validator with address
// VALUE: staking/types.Validator
func GetValidatorKey(operatorAddr sdk.ValAddress) []byte {
	return append(ValidatorsKey, operatorAddr.Bytes()...)
}

// gets the key for the validator with pubkey
// VALUE: validator operator address ([]byte)
func GetValidatorByConsAddrKey(addr sdk.ConsAddress) []byte {
	return append(ValidatorsByConsAddrKey, addr.Bytes()...)
}

// Get the validator operator address from LastValidatorPowerKey
func AddressFromLastValidatorPowerKey(key []byte) []byte {
	return key[1:] // remove prefix bytes
}

// get the validator by power index.
// Power index is the key used in the power-store, and represents the relative
// power ranking of the validator.
// VALUE: validator operator address ([]byte)
/**
通过幂指数获得验证人。
权重索引就是在db中的key的位置?，并且表示验证人的相对权重等级。
VALUE：验证人的地址（[] byte）
 */
func GetValidatorsByPowerIndexKey(validator types.Validator) []byte {
	// NOTE the address doesn't need to be stored because counter bytes must always be different
	// 注意:不需要存储地址，因为计数器字节必须始终不同 （真的不明白什么含义？）
	return getValidatorPowerRank(validator)
}

// get the bonded validator index key for an operator address
func GetLastValidatorPowerKey(operator sdk.ValAddress) []byte {
	return append(LastValidatorPowerKey, operator...)
}

// get the power ranking of a validator
// NOTE the larger values are of higher value
// nolint: unparam
/**
TODO ############ $$$$$$$$$$$
TODO 非常重要的一步 计算权重
获得验证人的权重排名
注意，较大的值具有较高的值
nolint：unparam
 */
func getValidatorPowerRank(validator types.Validator) []byte {

	/**
	根据当前验证人身上背负的 token数目作为权重参数
	 */
	potentialPower := validator.Tokens

	// todo: deal with cases above 2**64, ref https://my-cosmos/cosmos-sdk/issues/2439#issuecomment-427167556
	// 最多可处理 2 ** 64 长度? 参考https：// my-cosmos / cosmos-sdk / issues / 2439＃issuecomment-427167556
	tendermintPower := potentialPower.Int64()

	// 创建8字节 32bit == 4byte   64bit == 8byte
	// 所以起 8长度的 []byte
	tendermintPowerBytes := make([]byte, 8)

	// 使用大字端里存储 token的权重值
	binary.BigEndian.PutUint64(tendermintPowerBytes[:], uint64(tendermintPower))

	// 我擦，这写得我不想说话
	powerBytes := tendermintPowerBytes
	powerBytesLen := len(powerBytes) // 8

	// key is of format prefix || powerbytes || addrBytes
	// 定义一个 key，用于返回采集到的key值
	// 规则为:
	// 1 byte的 prefix + 8 byte的权重值 + addr
	key := make([]byte, 1+powerBytesLen+sdk.AddrLen)

	// 先填充固定的前缀
	// ValidatorsByPowerIndexKey 只有一个元素
	key[0] = ValidatorsByPowerIndexKey[0]

	// copy填充权重值
	copy(key[1:powerBytesLen+1], powerBytes)

	// 这里看不明白，为什么要去 ^值替换自己的原先的值？
	operAddrInvr := cp(validator.OperatorAddress)
	for i, b := range operAddrInvr {
		operAddrInvr[i] = ^b
	}

	// 填充最后一部分
	copy(key[powerBytesLen+1:], operAddrInvr)

	return key
}

func parseValidatorPowerRankKey(key []byte) (operAddr []byte) {
	powerBytesLen := 8
	if len(key) != 1+powerBytesLen+sdk.AddrLen {
		panic("Invalid validator power rank key length")
	}
	operAddr = cp(key[powerBytesLen+1:])
	for i, b := range operAddr {
		operAddr[i] = ^b
	}
	return operAddr
}

// gets the prefix for all unbonding delegations from a delegator
func GetValidatorQueueTimeKey(timestamp time.Time) []byte {
	bz := sdk.FormatTimeBytes(timestamp)
	return append(ValidatorQueueKey, bz...)
}

//______________________________________________________________________________

// gets the key for delegator bond with validator
// VALUE: staking/types.Delegation
func GetDelegationKey(delAddr sdk.AccAddress, valAddr sdk.ValAddress) []byte {
	return append(GetDelegationsKey(delAddr), valAddr.Bytes()...)
}

// gets the prefix for a delegator for all validators
func GetDelegationsKey(delAddr sdk.AccAddress) []byte {
	return append(DelegationKey, delAddr.Bytes()...)
}

//______________________________________________________________________________

// gets the key for an unbonding delegation by delegator and validator addr
// VALUE: staking/types.UnbondingDelegation
func GetUBDKey(delAddr sdk.AccAddress, valAddr sdk.ValAddress) []byte {
	return append(
		GetUBDsKey(delAddr.Bytes()),
		valAddr.Bytes()...)
}

// gets the index-key for an unbonding delegation, stored by validator-index
// VALUE: none (key rearrangement used)
func GetUBDByValIndexKey(delAddr sdk.AccAddress, valAddr sdk.ValAddress) []byte {
	return append(GetUBDsByValIndexKey(valAddr), delAddr.Bytes()...)
}

// rearranges the ValIndexKey to get the UBDKey
func GetUBDKeyFromValIndexKey(IndexKey []byte) []byte {
	addrs := IndexKey[1:] // remove prefix bytes
	if len(addrs) != 2*sdk.AddrLen {
		panic("unexpected key length")
	}
	valAddr := addrs[:sdk.AddrLen]
	delAddr := addrs[sdk.AddrLen:]
	return GetUBDKey(delAddr, valAddr)
}

//______________

// gets the prefix for all unbonding delegations from a delegator
func GetUBDsKey(delAddr sdk.AccAddress) []byte {
	return append(UnbondingDelegationKey, delAddr.Bytes()...)
}

// gets the prefix keyspace for the indexes of unbonding delegations for a validator
func GetUBDsByValIndexKey(valAddr sdk.ValAddress) []byte {
	return append(UnbondingDelegationByValIndexKey, valAddr.Bytes()...)
}

// gets the prefix for all unbonding delegations from a delegator
func GetUnbondingDelegationTimeKey(timestamp time.Time) []byte {
	bz := sdk.FormatTimeBytes(timestamp)
	return append(UnbondingQueueKey, bz...)
}

//________________________________________________________________________________

// gets the key for a redelegation
// VALUE: staking/types.RedelegationKey
func GetREDKey(delAddr sdk.AccAddress, valSrcAddr, valDstAddr sdk.ValAddress) []byte {
	key := make([]byte, 1+sdk.AddrLen*3)

	copy(key[0:sdk.AddrLen+1], GetREDsKey(delAddr.Bytes()))
	copy(key[sdk.AddrLen+1:2*sdk.AddrLen+1], valSrcAddr.Bytes())
	copy(key[2*sdk.AddrLen+1:3*sdk.AddrLen+1], valDstAddr.Bytes())

	return key
}

// gets the index-key for a redelegation, stored by source-validator-index
// VALUE: none (key rearrangement used)
func GetREDByValSrcIndexKey(delAddr sdk.AccAddress, valSrcAddr, valDstAddr sdk.ValAddress) []byte {
	REDSFromValsSrcKey := GetREDsFromValSrcIndexKey(valSrcAddr)
	offset := len(REDSFromValsSrcKey)

	// key is of the form REDSFromValsSrcKey || delAddr || valDstAddr
	key := make([]byte, len(REDSFromValsSrcKey)+2*sdk.AddrLen)
	copy(key[0:offset], REDSFromValsSrcKey)
	copy(key[offset:offset+sdk.AddrLen], delAddr.Bytes())
	copy(key[offset+sdk.AddrLen:offset+2*sdk.AddrLen], valDstAddr.Bytes())
	return key
}

// gets the index-key for a redelegation, stored by destination-validator-index
// VALUE: none (key rearrangement used)
func GetREDByValDstIndexKey(delAddr sdk.AccAddress, valSrcAddr, valDstAddr sdk.ValAddress) []byte {
	REDSToValsDstKey := GetREDsToValDstIndexKey(valDstAddr)
	offset := len(REDSToValsDstKey)

	// key is of the form REDSToValsDstKey || delAddr || valSrcAddr
	key := make([]byte, len(REDSToValsDstKey)+2*sdk.AddrLen)
	copy(key[0:offset], REDSToValsDstKey)
	copy(key[offset:offset+sdk.AddrLen], delAddr.Bytes())
	copy(key[offset+sdk.AddrLen:offset+2*sdk.AddrLen], valSrcAddr.Bytes())

	return key
}

// GetREDKeyFromValSrcIndexKey rearranges the ValSrcIndexKey to get the REDKey
func GetREDKeyFromValSrcIndexKey(indexKey []byte) []byte {
	// note that first byte is prefix byte
	if len(indexKey) != 3*sdk.AddrLen+1 {
		panic("unexpected key length")
	}
	valSrcAddr := indexKey[1 : sdk.AddrLen+1]
	delAddr := indexKey[sdk.AddrLen+1 : 2*sdk.AddrLen+1]
	valDstAddr := indexKey[2*sdk.AddrLen+1 : 3*sdk.AddrLen+1]

	return GetREDKey(delAddr, valSrcAddr, valDstAddr)
}

// GetREDKeyFromValDstIndexKey rearranges the ValDstIndexKey to get the REDKey
func GetREDKeyFromValDstIndexKey(indexKey []byte) []byte {
	// note that first byte is prefix byte
	if len(indexKey) != 3*sdk.AddrLen+1 {
		panic("unexpected key length")
	}
	valDstAddr := indexKey[1 : sdk.AddrLen+1]
	delAddr := indexKey[sdk.AddrLen+1 : 2*sdk.AddrLen+1]
	valSrcAddr := indexKey[2*sdk.AddrLen+1 : 3*sdk.AddrLen+1]
	return GetREDKey(delAddr, valSrcAddr, valDstAddr)
}

// gets the prefix for all unbonding delegations from a delegator
func GetRedelegationTimeKey(timestamp time.Time) []byte {
	bz := sdk.FormatTimeBytes(timestamp)
	return append(RedelegationQueueKey, bz...)
}

//______________

// gets the prefix keyspace for redelegations from a delegator
func GetREDsKey(delAddr sdk.AccAddress) []byte {
	return append(RedelegationKey, delAddr.Bytes()...)
}

// gets the prefix keyspace for all redelegations redelegating away from a source validator
func GetREDsFromValSrcIndexKey(valSrcAddr sdk.ValAddress) []byte {
	return append(RedelegationByValSrcIndexKey, valSrcAddr.Bytes()...)
}

// gets the prefix keyspace for all redelegations redelegating towards a destination validator
func GetREDsToValDstIndexKey(valDstAddr sdk.ValAddress) []byte {
	return append(RedelegationByValDstIndexKey, valDstAddr.Bytes()...)
}

// gets the prefix keyspace for all redelegations redelegating towards a destination validator
// from a particular delegator
func GetREDsByDelToValDstIndexKey(delAddr sdk.AccAddress, valDstAddr sdk.ValAddress) []byte {
	return append(
		GetREDsToValDstIndexKey(valDstAddr),
		delAddr.Bytes()...)
}

//-------------------------------------------------

func cp(bz []byte) (ret []byte) {
	if bz == nil {
		return nil
	}
	ret = make([]byte, len(bz))
	copy(ret, bz)
	return ret
}
