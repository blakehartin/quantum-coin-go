// Copyright 2018 The go-ethereum Authors
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

// This file contains a miner stress test based on the consensus engine.
package main

import (
	"bytes"
	"github.com/QuantumCoinProject/qc/crypto/cryptobase"
	"github.com/QuantumCoinProject/qc/crypto/signaturealgorithm"
	"io/ioutil"
	"math/big"
	"math/rand"
	"os"
	"time"

	"github.com/QuantumCoinProject/qc/accounts/keystore"
	"github.com/QuantumCoinProject/qc/common"
	"github.com/QuantumCoinProject/qc/common/fdlimit"
	"github.com/QuantumCoinProject/qc/core"
	"github.com/QuantumCoinProject/qc/core/types"
	"github.com/QuantumCoinProject/qc/eth"
	"github.com/QuantumCoinProject/qc/eth/downloader"
	"github.com/QuantumCoinProject/qc/eth/ethconfig"
	"github.com/QuantumCoinProject/qc/log"
	"github.com/QuantumCoinProject/qc/miner"
	"github.com/QuantumCoinProject/qc/node"
	"github.com/QuantumCoinProject/qc/p2p"
	"github.com/QuantumCoinProject/qc/p2p/enode"
	"github.com/QuantumCoinProject/qc/params"
)

func main() {
	log.Root().SetHandler(log.LvlFilterHandler(log.LvlInfo, log.StreamHandler(os.Stderr, log.TerminalFormat(true))))
	fdlimit.Raise(2048)

	// Generate a batch of accounts to seal and fund with
	faucets := make([]*signaturealgorithm.PrivateKey, 128)
	for i := 0; i < len(faucets); i++ {
		faucets[i], _ = cryptobase.SigAlg.GenerateKey()
	}
	sealers := make([]*signaturealgorithm.PrivateKey, 4)
	for i := 0; i < len(sealers); i++ {
		sealers[i], _ = cryptobase.SigAlg.GenerateKey()
	}
	// Create a Clique network based off of the Rinkeby config
	genesis := makeGenesis(faucets, sealers)

	var (
		nodes  []*eth.Ethereum
		enodes []*enode.Node
	)

	for _, sealer := range sealers {
		// Start the node and wait until it's up
		stack, ethBackend, err := makeSealer(genesis)
		if err != nil {
			panic(err)
		}
		defer stack.Close()

		for stack.Server().NodeInfo().Ports.Listener == 0 {
			time.Sleep(250 * time.Millisecond)
		}
		// Connect the node to all the previous ones
		for _, n := range enodes {
			stack.Server().AddPeer(n)
		}
		// Start tracking the node and its enode
		nodes = append(nodes, ethBackend)
		enodes = append(enodes, stack.Server().Self())

		// Inject the signer key and start sealing with it
		store := stack.AccountManager().Backends(keystore.KeyStoreType)[0].(*keystore.KeyStore)
		signer, err := store.ImportKey(sealer, "")
		if err != nil {
			panic(err)
		}
		if err := store.Unlock(signer, ""); err != nil {
			panic(err)
		}
	}

	// Iterate over all the nodes and start signing on them
	time.Sleep(3 * time.Second)
	for _, node := range nodes {
		if err := node.StartMining(1); err != nil {
			panic(err)
		}
	}
	time.Sleep(3 * time.Second)

	// Start injecting transactions from the faucet like crazy
	nonces := make([]uint64, len(faucets))
	for {
		// Pick a random signer node
		index := rand.Intn(len(faucets))
		backend := nodes[index%len(nodes)]

		// Create a self transaction and inject into the pool
		pubKeyAddress, err := cryptobase.SigAlg.PublicKeyToAddress(&faucets[index].PublicKey)
		if err != nil {
			panic(err)
		}
		tx, err := types.SignTx(types.NewTransaction(nonces[index], pubKeyAddress, new(big.Int), 21000, big.NewInt(100000000000), nil), types.NewLondonSigner(big.NewInt(types.DEFAULT_CHAIN_ID)), faucets[index])
		if err != nil {
			panic(err)
		}
		if err := backend.TxPool().AddLocal(tx); err != nil {
			panic(err)
		}
		nonces[index]++

		// Wait if we're too saturated
		if pend, _ := backend.TxPool().Stats(); pend > 2048 {
			time.Sleep(100 * time.Millisecond)
		}
	}
}

// makeGenesis creates a custom Clique genesis block based on some pre-defined
// signer and faucet accounts.
func makeGenesis(faucets []*signaturealgorithm.PrivateKey, sealers []*signaturealgorithm.PrivateKey) *core.Genesis {
	// Create a Clique network based off of the Rinkeby config
	genesis := core.DefaultRinkebyGenesisBlock()
	genesis.GasLimit = 25000000

	genesis.Config.ChainID = big.NewInt(18)
	genesis.Config.ProofOfStake.Period = 1
	genesis.Config.EIP150Hash = common.Hash{}

	genesis.Alloc = core.GenesisAlloc{}
	for _, faucet := range faucets {
		pubKeyAddress, err := cryptobase.SigAlg.PublicKeyToAddress(&faucet.PublicKey)
		if err != nil {
			panic(err)
		}
		genesis.Alloc[pubKeyAddress] = core.GenesisAccount{
			Balance: new(big.Int).Exp(big.NewInt(2), big.NewInt(128), nil),
		}
	}
	// Sort the signers and embed into the extra-data section
	signers := make([]common.Address, len(sealers))
	for i, sealer := range sealers {
		pubKeyAddr, err := cryptobase.SigAlg.PublicKeyToAddress(&sealer.PublicKey)
		if err != nil {
			panic(err)
		}
		signers[i] = pubKeyAddr
	}
	for i := 0; i < len(signers); i++ {
		for j := i + 1; j < len(signers); j++ {
			if bytes.Compare(signers[i][:], signers[j][:]) > 0 {
				signers[i], signers[j] = signers[j], signers[i]
			}
		}
	}
	genesis.ExtraData = make([]byte, 32+len(signers)*common.AddressLength+65)
	for i, signer := range signers {
		copy(genesis.ExtraData[32+i*common.AddressLength:], signer[:])
	}
	// Return the genesis block for initialization
	return genesis
}

func makeSealer(genesis *core.Genesis) (*node.Node, *eth.Ethereum, error) {
	// Define the basic configurations for the Ethereum node
	datadir, _ := ioutil.TempDir("", "")

	config := &node.Config{
		Name:    "geth",
		Version: params.Version,
		DataDir: datadir,
		P2P: p2p.Config{
			ListenAddr:  "0.0.0.0:0",
			NoDiscovery: true,
			MaxPeers:    128,
		},
	}
	// Start the node and configure a full Ethereum node on it
	stack, err := node.New(config)
	if err != nil {
		return nil, nil, err
	}
	// Create and register the backend
	ethBackend, err := eth.New(stack, &ethconfig.Config{
		Genesis:         genesis,
		NetworkId:       genesis.Config.ChainID.Uint64(),
		SyncMode:        downloader.FullSync,
		DatabaseCache:   256,
		DatabaseHandles: 256,
		TxPool:          core.DefaultTxPoolConfig,
		GPO:             ethconfig.Defaults.GPO,
		Miner: miner.Config{
			GasFloor: genesis.GasLimit * 9 / 10,
			GasCeil:  genesis.GasLimit * 11 / 10,
			GasPrice: big.NewInt(1),
			Recommit: time.Second,
		},
	})
	if err != nil {
		return nil, nil, err
	}

	err = stack.Start()
	return stack, ethBackend, err
}
