package types

import (
	"github.com/QuantumCoinProject/qc/common"
	"math/big"
)

const DEFAULT_CHAIN_ID int64 = 123123
const MAX_REMARKS_LENGTH = 64

type GasTier uint64

const (
	GAS_TIER_DEFAULT GasTier = 1
	GAS_TIER_2X      GasTier = 2
	GAS_TIER_5X      GasTier = 5
	GAS_TIER_10X     GasTier = 10
)

type AccessList []AccessTuple

// AccessTuple is the element type of an access list.
type AccessTuple struct {
	Address     common.Address `json:"address"        gencodec:"required"`
	StorageKeys []common.Hash  `json:"storageKeys"    gencodec:"required"`
}

// StorageKeys returns the total number of storage keys in the access list.
func (al AccessList) StorageKeys() int {
	sum := 0
	for _, tuple := range al {
		sum += len(tuple.StorageKeys)
	}
	return sum
}

var GAS_TIER_DEFAULT_PRICE = big.NewInt(47619047619047600) // 1000 DP / 21000 in wei (1000/21000 = 0.0476190476190476)
var GAS_TIER_2x_PRICE = common.SafeMulBigInt(GAS_TIER_DEFAULT_PRICE, big.NewInt(2))
var GAS_TIER_5x_PRICE = common.SafeMulBigInt(GAS_TIER_DEFAULT_PRICE, big.NewInt(5))
var GAS_TIER_10x_PRICE = common.SafeMulBigInt(GAS_TIER_DEFAULT_PRICE, big.NewInt(10))

type DefaultFeeTx struct {
	ChainID    *big.Int
	Nonce      uint64
	Gas        uint64
	MaxGasTier GasTier
	To         *common.Address `rlp:"nil"` // nil means contract creation
	Value      *big.Int
	Data       []byte
	Remarks    []byte
	AccessList AccessList

	// Signature values
	V *big.Int `json:"v" gencodec:"required"`
	R *big.Int `json:"r" gencodec:"required"`
	S *big.Int `json:"s" gencodec:"required"`
}

// NewTransaction creates an unsigned legacy transaction.
// Deprecated: use NewTx instead.
func NewTransaction(nonce uint64, to common.Address, amount *big.Int, gasLimit uint64, data []byte) *Transaction {
	return NewDefaultFeeTransaction(big.NewInt(DEFAULT_CHAIN_ID), nonce, &to, amount, gasLimit, GAS_TIER_DEFAULT, data)
}

func NewDefaultFeeTransaction(chainId *big.Int, nonce uint64, to *common.Address, amount *big.Int, gasLimit uint64, maxGasTier GasTier, data []byte) *Transaction {
	tx := NewTx(&DefaultFeeTx{
		ChainID:    chainId,
		Nonce:      nonce,
		To:         to,
		Value:      amount,
		Data:       data,
		Gas:        gasLimit,
		MaxGasTier: maxGasTier,
	})

	return tx
}

// copy creates a deep copy of the transaction data and initializes all fields.
func (tx *DefaultFeeTx) copy() TxData {
	cpy := &DefaultFeeTx{
		Nonce:      tx.Nonce,
		To:         tx.To, // TODO: copy pointed-to address
		Data:       common.CopyBytes(tx.Data),
		Gas:        tx.Gas,
		MaxGasTier: tx.MaxGasTier,
		// These are copied below.
		AccessList: make(AccessList, len(tx.AccessList)),
		Value:      new(big.Int),
		ChainID:    new(big.Int),
		V:          new(big.Int),
		R:          new(big.Int),
		S:          new(big.Int),
		Remarks:    common.CopyBytes(tx.Remarks),
	}
	copy(cpy.AccessList, tx.AccessList)
	if tx.Value != nil {
		cpy.Value.Set(tx.Value)
	}
	if tx.ChainID != nil {
		cpy.ChainID.Set(tx.ChainID)
	}
	if tx.V != nil {
		cpy.V.Set(tx.V)
	}
	if tx.R != nil {
		cpy.R.Set(tx.R)
	}
	if tx.S != nil {
		cpy.S.Set(tx.S)
	}
	return cpy
}

// accessors for innerTx.
func (tx *DefaultFeeTx) txType() byte           { return DefaultFeeTxType }
func (tx *DefaultFeeTx) chainID() *big.Int      { return tx.ChainID }
func (tx *DefaultFeeTx) protected() bool        { return true }
func (tx *DefaultFeeTx) accessList() AccessList { return tx.AccessList }
func (tx *DefaultFeeTx) data() []byte           { return tx.Data }
func (tx *DefaultFeeTx) gas() uint64            { return tx.Gas }
func (tx *DefaultFeeTx) gasFeeCap() *big.Int    { return GAS_TIER_DEFAULT_PRICE }
func (tx *DefaultFeeTx) gasPrice() *big.Int {
	if tx.MaxGasTier == GAS_TIER_DEFAULT {
		return GAS_TIER_DEFAULT_PRICE
	} else if tx.MaxGasTier == GAS_TIER_2X {
		return GAS_TIER_2x_PRICE
	} else if tx.MaxGasTier == GAS_TIER_5X {
		return GAS_TIER_5x_PRICE
	} else if tx.MaxGasTier == GAS_TIER_10X {
		return GAS_TIER_10x_PRICE
	}

	return GAS_TIER_DEFAULT_PRICE
}
func (tx *DefaultFeeTx) maxGasTier() GasTier { return tx.MaxGasTier }
func (tx *DefaultFeeTx) value() *big.Int     { return tx.Value }
func (tx *DefaultFeeTx) nonce() uint64       { return tx.Nonce }
func (tx *DefaultFeeTx) to() *common.Address { return tx.To }
func (tx *DefaultFeeTx) verifyFields() bool  { return len(tx.Remarks) <= MAX_REMARKS_LENGTH }

func (tx *DefaultFeeTx) rawSignatureValues() (v, r, s *big.Int) {
	return tx.V, tx.R, tx.S
}

func (tx *DefaultFeeTx) remarks() []byte {
	return tx.Remarks
}
func (tx *DefaultFeeTx) setSignatureValues(chainID, v, r, s *big.Int) {
	tx.ChainID, tx.V, tx.R, tx.S = chainID, v, r, s
}
