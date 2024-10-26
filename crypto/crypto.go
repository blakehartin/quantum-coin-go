// Copyright 2014 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

package crypto

import (
	"github.com/QuantumCoinProject/qc/common"
	"github.com/QuantumCoinProject/qc/rlp"
	"golang.org/x/crypto/sha3"
)

const DILITHIUM_ED25519_SPHINCS_COMPACT_ID = 1

const DILITHIUM_ED25519_SPHINCS_FULL_ID = 2

func Sha256(data ...[]byte) []byte {
	h1 := sha3.NewLegacyKeccak256()
	for _, b := range data {
		h1.Write(b)
	}
	return h1.Sum(nil)
}

// Keccak256 calculates and returns the Keccak256 hash of the input data.
func Keccak256(data ...[]byte) []byte {
	//Round 1
	h1 := sha3.NewLegacyKeccak256()
	for _, b := range data {
		h1.Write(b)
	}
	return h1.Sum(nil)

}

// Keccak256Hash calculates and returns the Keccak256 hash of the input data,
// converting it to an internal Hash data structure.
func Keccak256Hash(data ...[]byte) (h common.Hash) {
	h.SetBytes(Keccak256(data...))
	return h
}

// Keccak512 calculates and returns the Keccak512 hash of the input data.
func Keccak512(data ...[]byte) []byte {
	d := sha3.NewLegacyKeccak512()
	for _, b := range data {
		d.Write(b)
	}
	return d.Sum(nil)
}

// CreateAddress creates an ethereum address given the bytes and the nonce
func CreateAddress(b common.Address, nonce uint64) common.Address {
	data, _ := rlp.EncodeToBytes([]interface{}{b, nonce})
	return common.BytesToAddress(Keccak256(data)[:])
}

// CreateAddress2 creates an ethereum address given the address bytes, initial
// contract code hash and a salt.
func CreateAddress2(b common.Address, salt [common.HashLength]byte, inithash []byte) common.Address {
	return common.BytesToAddress(Keccak256([]byte{0xff}, b.Bytes(), salt[:], inithash)[:])
}

func PublicKeyBytesToAddress(pubKey []byte) common.Address {
	var a common.Address
	b := Keccak256(pubKey[:])[common.AddressTruncateBytes:]
	a.SetBytes(b)
	return a
}
