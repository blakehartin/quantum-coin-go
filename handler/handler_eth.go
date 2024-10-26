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

package handler

import (
	"errors"
	"fmt"
	"math/big"
	"strings"
	"sync/atomic"
	"time"

	"github.com/QuantumCoinProject/qc/common"
	"github.com/QuantumCoinProject/qc/core"
	"github.com/QuantumCoinProject/qc/core/types"
	"github.com/QuantumCoinProject/qc/eth/protocols/eth"
	"github.com/QuantumCoinProject/qc/log"
	"github.com/QuantumCoinProject/qc/p2p/enode"
	"github.com/QuantumCoinProject/qc/trie"
	"math/rand"
)

const REBROADCAST_CLEANUP_MILLI_SECONDS = 300000
const REBROADCAST_CLEANUP_TIMER_MILLI_SECONDS = 900000
const REBROADCAST_MIN_DELAY_PACKET_HASH = 30000

// EthHandler implements the eth.Backend interface to handle the various network
// packets that are sent as replies or broadcasts.
type EthHandler P2PHandler

func (h *EthHandler) Chain() *core.BlockChain     { return h.chain }
func (h *EthHandler) StateBloom() *trie.SyncBloom { return h.stateBloom }
func (h *EthHandler) TxPool() eth.TxPool          { return h.txpool }

// RunPeer is invoked when a peer joins on the `eth` protocol.
func (h *EthHandler) RunPeer(peer *eth.Peer, hand eth.Handler) error {
	return (*P2PHandler)(h).runEthPeer(peer, hand)
}

// PeerInfo retrieves all known `eth` information about a peer.
func (h *EthHandler) PeerInfo(id enode.ID) interface{} {
	if p := h.peers.peer(id.String()); p != nil {
		return p.info()
	}
	return nil
}

// AcceptTxs retrieves whether transaction processing is enabled on the node
// or if inbound transactions should simply be dropped.
func (h *EthHandler) AcceptTxs() bool {
	return atomic.LoadUint32(&h.AcceptTxns) == 1
}

// Handle is invoked from a peer's message P2PHandler when it receives a new remote
// message that the P2PHandler couldn't consume and serve itself.
func (h *EthHandler) Handle(peer *eth.Peer, packet eth.Packet) error {
	// Consume any broadcasts and announces, forwarding the rest to the Downloader
	switch packet := packet.(type) {
	case *eth.BlockHeadersPacket:
		return h.handleHeaders(peer, *packet)

	case *eth.BlockBodiesPacket:
		txset := packet.Unpack()
		return h.handleBodies(peer, txset)

	case *eth.NodeDataPacket:
		if err := h.Downloader.DeliverNodeData(peer.ID(), *packet); err != nil {
			log.Debug("Failed to deliver node state data", "err", err)
		}
		return nil

	case *eth.ReceiptsPacket:
		if err := h.Downloader.DeliverReceipts(peer.ID(), *packet); err != nil {
			log.Debug("Failed to deliver receipts", "err", err)
		}
		return nil

	case *eth.NewBlockHashesPacket:
		hashes, numbers := packet.Unpack()
		return h.handleBlockAnnounces(peer, hashes, numbers)

	case *eth.NewBlockPacket:
		return h.handleBlockBroadcast(peer, packet.Block, packet.TD)

	case *eth.NewPooledTransactionHashesPacket:
		return h.txFetcher.Notify(peer.ID(), *packet)

	case *eth.TransactionsPacket:
		return h.txFetcher.Enqueue(peer.ID(), *packet, false)

	case *eth.PooledTransactionsPacket:
		return h.txFetcher.Enqueue(peer.ID(), *packet, true)

	case *eth.ConsensusPacket:
		if h.consensusHandler != nil {
			err := h.consensusHandler.Handler.HandleConsensusPacket(packet, peer.ID())
			if err != nil {
				log.Trace("Error in HandleConsensusPacket", "err", err, "peer", peer.ID())
			} else {
				go h.rebroadcast(peer.ID(), packet)
			}

			return err
		} else {
			return nil
		}

	case *eth.RequestConsensusDataPacket:
		if h.consensusHandler != nil {
			packets, err := h.consensusHandler.Handler.HandleRequestConsensusDataPacket(packet)
			if err != nil {
				return err
			}
			for i := 0; i < len(packets); i++ {
				peer.AsyncSendConsensusPacket(packets[i])
			}
		}
		return nil

	case *eth.RequestPeerListPacket:
		err := h.handleRequestPeerList(peer)
		if err != nil {
			return err
		}
		return nil

	case *eth.PeerListPacket:
		err := h.handlePeerList(peer, packet)
		if err != nil {
			return err
		}
		return nil

	default:
		return fmt.Errorf("unexpected eth packet type: %T", packet)
	}
}

// handleHeaders is invoked from a peer's message P2PHandler when it transmits a batch
// of headers for the local node to process.
func (h *EthHandler) handleHeaders(peer *eth.Peer, headers []*types.Header) error {
	p := h.peers.peer(peer.ID())
	if p == nil {
		return errors.New("unregistered during callback")
	}
	// If no headers were received, but we're expencting a checkpoint header, consider it that
	if len(headers) == 0 && p.syncDrop != nil {
		// Stop the timer either way, decide later to drop or not
		p.syncDrop.Stop()
		p.syncDrop = nil

		// If we're doing a fast (or snap) sync, we must enforce the checkpoint block to avoid
		// eclipse attacks. Unsynced nodes are welcome to connect after we're done
		// joining the network
		if atomic.LoadUint32(&h.fastSync) == 1 {
			peer.Log().Warn("Dropping unsynced node during sync", "addr", peer.RemoteAddr(), "type", peer.Name())
			return errors.New("unsynced node cannot serve sync")
		}
	}
	// Filter out any explicitly requested headers, deliver the rest to the Downloader
	filter := len(headers) == 1
	if filter {
		// If it's a potential sync progress check, validate the content and advertised chain weight
		if p.syncDrop != nil && headers[0].Number.Uint64() == h.checkpointNumber {
			// Disable the sync drop timer
			p.syncDrop.Stop()
			p.syncDrop = nil

			// Validate the header and either drop the peer or continue
			if headers[0].Hash() != h.checkpointHash {
				return errors.New("checkpoint hash mismatch")
			}
			return nil
		}
		// Otherwise if it's a whitelisted block, validate against the set
		if want, ok := h.whitelist[headers[0].Number.Uint64()]; ok {
			if hash := headers[0].Hash(); want != hash {
				peer.Log().Info("Whitelist mismatch, dropping peer", "number", headers[0].Number.Uint64(), "hash", hash, "want", want)
				return errors.New("whitelist block mismatch")
			}
			peer.Log().Debug("Whitelist block verified", "number", headers[0].Number.Uint64(), "hash", want)
		}
		// Irrelevant of the fork checks, send the header to the fetcher just in case
		headers = h.blockFetcher.FilterHeaders(peer.ID(), headers, time.Now())
	}
	if len(headers) > 0 || !filter {
		err := h.Downloader.DeliverHeaders(peer.ID(), headers)
		if err != nil {
			log.Debug("Failed to deliver headers", "err", err)
		}
	}
	return nil
}

// handleBodies is invoked from a peer's message P2PHandler when it transmits a batch
// of block bodies for the local node to process.
func (h *EthHandler) handleBodies(peer *eth.Peer, txs [][]*types.Transaction) error {
	// Filter out any explicitly requested bodies, deliver the rest to the Downloader
	filter := len(txs) > 0
	if filter {
		txs = h.blockFetcher.FilterBodies(peer.ID(), txs, time.Now())
	}
	if len(txs) > 0 || !filter {
		err := h.Downloader.DeliverBodies(peer.ID(), txs)
		if err != nil {
			log.Debug("Failed to deliver bodies", "err", err)
		}
	}
	return nil
}

// handleBlockAnnounces is invoked from a peer's message P2PHandler when it transmits a
// batch of block announcements for the local node to process.
func (h *EthHandler) handleBlockAnnounces(peer *eth.Peer, hashes []common.Hash, numbers []uint64) error {
	// Schedule all the unknown hashes for retrieval
	var (
		unknownHashes  = make([]common.Hash, 0, len(hashes))
		unknownNumbers = make([]uint64, 0, len(numbers))
	)
	for i := 0; i < len(hashes); i++ {
		if !h.chain.HasBlock(hashes[i], numbers[i]) {
			unknownHashes = append(unknownHashes, hashes[i])
			unknownNumbers = append(unknownNumbers, numbers[i])
		}
	}
	for i := 0; i < len(unknownHashes); i++ {
		h.blockFetcher.Notify(peer.ID(), unknownHashes[i], unknownNumbers[i], time.Now(), peer.RequestOneHeader, peer.RequestBodies)
	}
	return nil
}

// handleBlockBroadcast is invoked from a peer's message P2PHandler when it transmits a
// block broadcast for the local node to process.
func (h *EthHandler) handleBlockBroadcast(peer *eth.Peer, block *types.Block, td *big.Int) error {
	// Schedule the block for import
	h.blockFetcher.Enqueue(peer.ID(), block)

	// Assuming the block is importable by the peer, but possibly not yet done so,
	// calculate the head hash and TD that the peer truly must have.
	var (
		trueHead = block.ParentHash()
		trueTD   = new(big.Int).Sub(td, block.Difficulty())
	)
	// Update the peer's total difficulty if better than the previous
	if _, td := peer.Head(); trueTD.Cmp(td) > 0 {
		peer.SetHead(trueHead, trueTD)
		h.chainSync.handlePeerEvent(peer)
	}
	return nil
}

func (h *EthHandler) handleRequestPeerList(peer *eth.Peer) error {
	packet := &eth.PeerListPacket{
		PeerList: h.peers.PeerList(),
	}
	log.Trace("handleRequestPeerList", "peercount", len(packet.PeerList), "peer", peer.Node().IP())
	peer.AsyncSendPeerListPacket(packet)

	return nil
}

func (h *EthHandler) handlePeerList(peer *eth.Peer, packet *eth.PeerListPacket) error {
	log.Trace("handlePeerList", "peercount", len(packet.PeerList), "peer", peer.Node().IP())
	return h.handlePeerListFn(packet.PeerList)
}

func (h *EthHandler) ShouldRebroadcastIfYesSetFlag(packetHash common.Hash) bool {
	h.rebroadcastLock.Lock()
	defer h.rebroadcastLock.Unlock()

	lastRebroadCast, ok := h.rebroadcastMap[packetHash]
	if ok == false {
		h.rebroadcastMap[packetHash] = time.Now().UnixNano()

		//Lazy cleanup
		if Elapsed(h.rebroadcastLastCleanupTime) > REBROADCAST_CLEANUP_TIMER_MILLI_SECONDS {
			log.Debug("Cleaning up rebroadcast queue")
			for k, v := range h.rebroadcastMap {
				start := v / int64(time.Millisecond)
				end := time.Now().UnixNano() / int64(time.Millisecond)
				diff := end - start
				if diff > REBROADCAST_CLEANUP_MILLI_SECONDS {
					log.Debug("Cleaning up rebroadcast packet hash", "packetHash", k.Hex())
					delete(h.rebroadcastMap, k)
				}
			}
			h.rebroadcastLastCleanupTime = time.Now()
		}

		log.Trace("ShouldRebroadcastIfYesSetFlag true first time", "packet", packetHash.Hex())
		return true
	}

	start := lastRebroadCast / int64(time.Millisecond)
	end := time.Now().UnixNano() / int64(time.Millisecond)
	diff := end - start
	if diff > REBROADCAST_MIN_DELAY_PACKET_HASH {
		h.rebroadcastMap[packetHash] = time.Now().UnixNano()
		log.Trace("ShouldRebroadcastIfYesSetFlag true", "packet", packetHash.Hex())
		return true
	}

	log.Trace("ShouldRebroadcastIfYesSetFlag false", "packet", packetHash.Hex())
	return false
}

func (h *EthHandler) rebroadcast(incomingPeerId string, packet *eth.ConsensusPacket) {
	log.Trace("rebroadcast", "packet", packet.Hash().Hex())
	if h.consensusHandler.Handler.ShouldRebroadCast(packet, incomingPeerId) == false {
		return
	}
	packetHash := packet.Hash()
	shouldRebroadcast := h.ShouldRebroadcastIfYesSetFlag(packetHash)
	if shouldRebroadcast == false {
		return
	}
	peerList := h.peers.PeerIdList()
	for i := len(peerList) - 1; i > 0; i-- { //Fisher Yates shuffle. Send to a random set of peers each time
		minVal := 0
		maxVal := i
		j := rand.Intn(maxVal-minVal) + minVal //non-crypto rand is ok for this purpose
		temp := peerList[i]
		peerList[i] = peerList[j]
		peerList[j] = temp
	}

	count := 0
	for index := range peerList {
		p := h.peers.peer(peerList[index])
		if p == nil {
			continue
		}
		log.Trace("Rebroadcast peer", "peer", peerList[index])
		if strings.Compare(incomingPeerId, p.ID()) != 0 {
			log.Trace("Rebroadcast ConsensusPacket", "incoming peer", incomingPeerId, "outgoing peer", p.ID(), "parentHash", packet.ParentHash, "packetHash", packetHash.Hex())
			p.AsyncSendConsensusPacket(packet)
			count = count + 1
			if count >= h.rebroadcastCount {
				break
			}
		}
	}
}

func Elapsed(startTime time.Time) int64 {
	end := time.Now().UnixNano() / int64(time.Millisecond)
	start := startTime.UnixNano() / int64(time.Millisecond)
	diff := end - start
	return diff
}
