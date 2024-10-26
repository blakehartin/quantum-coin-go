// Copyright 2015 The go-ethereum Authors
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

package core

import (
	"fmt"
	"github.com/QuantumCoinProject/qc/consensus/mockconsensus"
	"github.com/QuantumCoinProject/qc/crypto/cryptobase"
	"math/big"

	"github.com/QuantumCoinProject/qc/core/rawdb"
	"github.com/QuantumCoinProject/qc/core/types"
	"github.com/QuantumCoinProject/qc/core/vm"
	"github.com/QuantumCoinProject/qc/params"
)

func ExampleGenerateChain() {
	var (
		privtestkey1, _ = cryptobase.SigAlg.GenerateKey()
		hextestkey1, _  = cryptobase.SigAlg.PrivateKeyToHex(privtestkey1)
		privtestkey2, _ = cryptobase.SigAlg.GenerateKey()
		hextestkey2, _  = cryptobase.SigAlg.PrivateKeyToHex(privtestkey2)
		privtestkey3, _ = cryptobase.SigAlg.GenerateKey()
		hextestkey3, _  = cryptobase.SigAlg.PrivateKeyToHex(privtestkey3)

		key1, _ = cryptobase.SigAlg.HexToPrivateKey(hextestkey1)
		key2, _ = cryptobase.SigAlg.HexToPrivateKey(hextestkey2)
		key3, _ = cryptobase.SigAlg.HexToPrivateKey(hextestkey3)
		addr1   = cryptobase.SigAlg.PublicKeyToAddressNoError(&key1.PublicKey)
		addr2   = cryptobase.SigAlg.PublicKeyToAddressNoError(&key2.PublicKey)
		addr3   = cryptobase.SigAlg.PublicKeyToAddressNoError(&key3.PublicKey)
		db      = rawdb.NewMemoryDatabase()
	)

	// Ensure that key1 has some funds in the genesis block.
	gspec := &Genesis{
		Config: &params.ChainConfig{HomesteadBlock: new(big.Int)},
		Alloc:  GenesisAlloc{addr1: {Balance: big.NewInt(1000000)}},
	}
	genesis := gspec.MustCommit(db)

	// This call generates a chain of 5 blocks. The function runs for
	// each block and adds different features to gen based on the
	// block index.
	signer := types.NewLondonSignerDefaultChain()
	chain, _ := GenerateChain(gspec.Config, genesis, mockconsensus.NewMockConsensus(), db, 5, func(i int, gen *BlockGen) {
		switch i {
		case 0:
			// In block 1, addr1 sends addr2 some ether.
			tx, _ := types.SignTx(types.NewTransaction(gen.TxNonce(addr1), addr2, big.NewInt(10000), params.TxGas, nil, nil), signer, key1)
			gen.AddTx(tx)
		case 1:
			// In block 2, addr1 sends some more ether to addr2.
			// addr2 passes it on to addr3.
			tx1, _ := types.SignTx(types.NewTransaction(gen.TxNonce(addr1), addr2, big.NewInt(1000), params.TxGas, nil, nil), signer, key1)
			tx2, _ := types.SignTx(types.NewTransaction(gen.TxNonce(addr2), addr3, big.NewInt(1000), params.TxGas, nil, nil), signer, key2)
			gen.AddTx(tx1)
			gen.AddTx(tx2)
		case 2:
			// Block 3 is empty but was mined by addr3.
			gen.SetCoinbase(addr3)
			gen.SetExtra([]byte("yeehaw"))
		case 3:
			// Block 4 includes blocks 2 and 3 as uncle headers (with modified extra data).
			b2 := gen.PrevBlock(1).Header()
			b2.Extra = []byte("foo")
			b3 := gen.PrevBlock(2).Header()
			b3.Extra = []byte("foo")
		}
	})

	// Import the chain. This runs all block validation rules.
	blockchain, _ := NewBlockChain(db, nil, gspec.Config, mockconsensus.NewMockConsensus(), vm.Config{}, nil, nil)
	defer blockchain.Stop()

	if i, err := blockchain.InsertChain(chain); err != nil {
		fmt.Printf("insert error (block %d): %v\n", chain[i].NumberU64(), err)
		return
	}

	state, _ := blockchain.State()
	fmt.Printf("last block: #%d\n", blockchain.CurrentBlock().Number())
	fmt.Println("balance of addr1:", state.GetBalance(addr1))
	fmt.Println("balance of addr2:", state.GetBalance(addr2))
	fmt.Println("balance of addr3:", state.GetBalance(addr3))
	// Output:
	// last block: #5
	// balance of addr1: 989000
	// balance of addr2: 10000
	// balance of addr3: 19687500000000001000
}
