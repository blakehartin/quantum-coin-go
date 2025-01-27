// Copyright 2016 The go-ethereum Authors
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

package bind

import (
	"context"
	"errors"
	"fmt"
	"github.com/QuantumCoinProject/qc/crypto/cryptobase"
	"github.com/QuantumCoinProject/qc/crypto/signaturealgorithm"
	"io"
	"io/ioutil"
	"math/big"

	"github.com/QuantumCoinProject/qc/accounts"
	"github.com/QuantumCoinProject/qc/accounts/keystore"
	"github.com/QuantumCoinProject/qc/common"
	"github.com/QuantumCoinProject/qc/core/types"
	"github.com/QuantumCoinProject/qc/log"
)

// ErrNoChainID is returned whenever the user failed to specify a chain id.
var ErrNoChainID = errors.New("no chain id specified")

// ErrNotAuthorized is returned when an account is not properly unlocked.
var ErrNotAuthorized = errors.New("not authorized to sign this account")

// NewTransactor is a utility method to easily create a transaction signer from
// an encrypted json key stream and the associated passphrase.
//
// Deprecated: Use NewTransactorWithChainID instead.
func NewTransactor(keyin io.Reader, passphrase string) (*TransactOpts, error) {
	log.Warn("WARNING: NewTransactor has been deprecated in favour of NewTransactorWithChainID")
	json, err := ioutil.ReadAll(keyin)
	if err != nil {
		return nil, err
	}
	key, err := keystore.DecryptKey(json, passphrase)
	if err != nil {
		return nil, err
	}
	return NewKeyedTransactor(key.PrivateKey), nil
}

// NewKeyStoreTransactor is a utility method to easily create a transaction signer from
// an decrypted key from a keystore.
//
// Deprecated: Use NewKeyStoreTransactorWithChainID instead.
func NewKeyStoreTransactor(keystore *keystore.KeyStore, account accounts.Account) (*TransactOpts, error) {
	log.Warn("WARNING: NewKeyStoreTransactor has been deprecated in favour of NewTransactorWithChainID")
	signer := types.NewLondonSigner(big.NewInt(types.DEFAULT_CHAIN_ID))
	return &TransactOpts{
		From: account.Address,
		Signer: func(address common.Address, tx *types.Transaction) (*types.Transaction, error) {
			if address != account.Address {
				return nil, ErrNotAuthorized
			}
			digest, err := signer.Hash(tx)
			if err != nil {
				return nil, err
			}
			signature, err := keystore.SignHash(account, digest.Bytes())
			if err != nil {
				return nil, err
			}
			return tx.WithSignature(signer, signature)
		},
		Context: context.Background(),
	}, nil
}

// NewKeyedTransactor is a utility method to easily create a transaction signer
// from a single private key.
//
// Deprecated: Use NewKeyedTransactorWithChainID instead.
func NewKeyedTransactor(key *signaturealgorithm.PrivateKey) *TransactOpts {
	log.Warn("WARNING: NewKeyedTransactor has been deprecated in favour of NewKeyedTransactorWithChainID")
	keyAddr, err := cryptobase.SigAlg.PublicKeyToAddress(&key.PublicKey)
	if err != nil {
		fmt.Errorf("error in PubkeyToAddress")
		return nil
	}
	signer := types.NewLondonSigner(big.NewInt(types.DEFAULT_CHAIN_ID))
	return &TransactOpts{
		From: keyAddr,
		Signer: func(address common.Address, tx *types.Transaction) (*types.Transaction, error) {
			if address != keyAddr {
				return nil, ErrNotAuthorized
			}
			digestHash, err := signer.Hash(tx)

			if err != nil {
				return nil, err
			}
			digestBytes := digestHash.Bytes()

			signature, err := cryptobase.SigAlg.Sign(digestBytes, key)
			if err != nil {
				return nil, err
			}
			return tx.WithSignature(signer, signature)
		},
		Context: context.Background(),
	}
}

// NewTransactorWithChainID is a utility method to easily create a transaction signer from
// an encrypted json key stream and the associated passphrase.
func NewTransactorWithChainID(keyin io.Reader, passphrase string, chainID *big.Int) (*TransactOpts, error) {
	json, err := ioutil.ReadAll(keyin)
	if err != nil {
		return nil, err
	}
	key, err := keystore.DecryptKey(json, passphrase)
	if err != nil {
		return nil, err
	}
	return NewKeyedTransactorWithChainID(key.PrivateKey, chainID)
}

// NewKeyStoreTransactorWithChainID is a utility method to easily create a transaction signer from
// an decrypted key from a keystore.
func NewKeyStoreTransactorWithChainID(keystore *keystore.KeyStore, account accounts.Account, chainID *big.Int) (*TransactOpts, error) {
	if chainID == nil {
		return nil, ErrNoChainID
	}
	signer := types.LatestSignerForChainID(chainID)
	return &TransactOpts{
		From: account.Address,
		Signer: func(address common.Address, tx *types.Transaction) (*types.Transaction, error) {
			if address != account.Address {
				return nil, ErrNotAuthorized
			}
			digest, err := signer.Hash(tx)
			if err != nil {
				return nil, err
			}
			signature, err := keystore.SignHash(account, digest.Bytes())
			if err != nil {
				return nil, err
			}
			return tx.WithSignature(signer, signature)
		},
		Context: context.Background(),
	}, nil
}

// NewKeyedTransactorWithChainID is a utility method to easily create a transaction signer
// from a single private key.
func NewKeyedTransactorWithChainID(key *signaturealgorithm.PrivateKey, chainID *big.Int) (*TransactOpts, error) {
	keyAddr, err := cryptobase.SigAlg.PublicKeyToAddress(&key.PublicKey)
	if err != nil {
		return nil, err
	}
	if chainID == nil {
		return nil, ErrNoChainID
	}
	signer := types.LatestSignerForChainID(chainID)
	return &TransactOpts{
		From: keyAddr,
		Signer: func(address common.Address, tx *types.Transaction) (*types.Transaction, error) {
			if address != keyAddr {
				return nil, ErrNotAuthorized
			}
			digest, err := signer.Hash(tx)
			if err != nil {
				return nil, err
			}

			signature, err := cryptobase.SigAlg.Sign(digest.Bytes(), key)
			if err != nil {
				return nil, err
			}
			return tx.WithSignature(signer, signature)
		},
		Context:  context.Background(),
		GasPrice: big.NewInt(100000),
	}, nil
}
