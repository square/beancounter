package backend

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/btcsuite/btcd/wire"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/square/beancounter/backend/electrum"
	"github.com/square/beancounter/deriver"
	"github.com/square/beancounter/reporter"
	"github.com/square/beancounter/utils"
)

// Fetches transaction information from Electrum servers.
// Electrum protocol docs: https://electrumx.readthedocs.io/en/latest/protocol.html
//
// When we connect to an Electrum server, we:
// - ensure the server talks v1.2
// - has the right genesis block
// - has crossed the height we are interested in.
// - we then negotiate protocol v1.2
//
// A background goroutine continuously connects to peers.

// ElectrumBackend wraps Electrum node and its API to provide a simple
// balance and transaction history information for a given address.
// ElectrumBackend implements Backend interface.
type ElectrumBackend struct {
	chainHeight uint32

	// peer management
	nodeMu sync.RWMutex // mutex to guard reads/writes to nodes map
	nodes  map[string]*electrum.Node
	// todo: blacklistedNodes should be a timestamp and we should re-try after a certain amount of
	// time has elapsed.
	blacklistedNodes map[string]struct{}
	network          utils.Network

	// channels used to communicate with the Accounter
	addrRequests  chan *deriver.Address
	addrResponses chan *AddrResponse
	txResponses   chan *TxResponse
	txRequests    chan string

	// channels used to communicate with the Blockfinder
	blockRequests  chan uint32
	blockResponses chan *BlockResponse

	// internal channels
	peersRequests  chan struct{}
	transactionsMu sync.Mutex // mutex to guard read/writes to transactions map
	transactions   map[string]int64
	doneCh         chan bool
}

const (
	maxPeers          = 100
	peerFetchInterval = 30 * time.Second // How often to fetch additional peers?
)

var (
	// ErrIncorrectGenesisBlock means electrum is set-up to for wrong currency
	ErrIncorrectGenesisBlock = errors.New("Incorrect genesis block")
	// ErrIncompatibleVersion means electrum server version is not compatible with electrum client lib
	ErrIncompatibleVersion = errors.New("Incompatible version")
	// ErrFailedNegotiateVersion means electrum server doesn't support version(s) used by the electrum client lib
	ErrFailedNegotiateVersion = errors.New("Failed negotiate version")
)

// NewElectrumBackend returns a new ElectrumBackend structs or errors.
// Initially connects to 1 node. A background job handles connecting to
// additional peers. The background job fails if there are no peers left.
func NewElectrumBackend(addr, port string, network utils.Network) (*ElectrumBackend, error) {

	// TODO: should the channels have k * maxPeers buffers? Each node needs to enqueue a
	// potentially large number of transactions. If all nodes are doing that at the same time,
	// there's a deadlock risk?
	eb := &ElectrumBackend{
		nodes:            make(map[string]*electrum.Node),
		blacklistedNodes: make(map[string]struct{}),
		network:          network,
		addrRequests:     make(chan *deriver.Address, 2*maxPeers),
		addrResponses:    make(chan *AddrResponse, 2*maxPeers),
		txRequests:       make(chan string, 2*maxPeers),
		txResponses:      make(chan *TxResponse, 2*maxPeers),
		blockRequests:    make(chan uint32, 2*maxPeers),
		blockResponses:   make(chan *BlockResponse, 2*maxPeers),

		peersRequests: make(chan struct{}),
		transactions:  make(map[string]int64),
		doneCh:        make(chan bool),
	}

	// Connect to a node to fetch the height
	height, err := eb.getHeight(addr, port, network)
	if err != nil {
		return nil, err
	}
	eb.chainHeight = height

	// Connect to a node and handle requests
	if err := eb.addNode(addr, port, network); err != nil {
		fmt.Printf("failed to connect to initial node: %+v", err)
		return nil, err
	}

	// goroutine to continuously fetch additional peers
	go func() {
		eb.findPeers()
		for {
			select {
			case <-time.Tick(peerFetchInterval):
				eb.findPeers()
			case <-eb.doneCh:
				return
			}
		}
	}()

	return eb, nil
}

// AddrRequest schedules a request to the backend to lookup information related
// to the given address.
func (eb *ElectrumBackend) AddrRequest(addr *deriver.Address) {
	reporter.GetInstance().IncAddressesScheduled()
	reporter.GetInstance().Logf("scheduling address: %s", addr)
	eb.addrRequests <- addr
}

// AddrResponses exposes a channel that allows to consume backend's responses to
// address requests created with AddrRequest()
func (eb *ElectrumBackend) AddrResponses() <-chan *AddrResponse {
	return eb.addrResponses
}

// TxRequest schedules a request to the backend to lookup information related
// to the given transaction hash.
func (eb *ElectrumBackend) TxRequest(txHash string) {
	reporter.GetInstance().IncTxScheduled()
	reporter.GetInstance().Logf("scheduling tx: %s", txHash)
	eb.txRequests <- txHash
}

// TxResponses exposes a channel that allows to consume backend's responses to
// address requests created with AddrRequest().
// If an address has any transactions then they will be sent to this channel by the
// backend.
func (eb *ElectrumBackend) TxResponses() <-chan *TxResponse {
	return eb.txResponses
}

func (eb *ElectrumBackend) BlockRequest(height uint32) {
	eb.blockRequests <- height
}

func (eb *ElectrumBackend) BlockResponses() <-chan *BlockResponse {
	return eb.blockResponses
}

// Finish informs the backend to stop doing its work.
func (eb *ElectrumBackend) Finish() {
	close(eb.doneCh)
	eb.removeAllNodes()
	// TODO: we could gracefully disconnect from all the nodes. We currently don't, because the
	// program is going to terminate soon anyways.
}

func (eb *ElectrumBackend) ChainHeight() uint32 {
	return eb.chainHeight
}

// Connect to a node and add it to the map of nodes
func (eb *ElectrumBackend) addNode(addr, port string, network utils.Network) error {
	ident := electrum.NodeIdent(addr, port)

	eb.nodeMu.RLock()
	_, existsGood := eb.nodes[ident]
	_, existsBad := eb.blacklistedNodes[ident]
	eb.nodeMu.RUnlock()
	if existsGood {
		return fmt.Errorf("already connected to %s", addr)
	}
	if existsBad {
		// TODO: if we can't connect to a node over TCP, we should try the TLS port?
		return fmt.Errorf("%s is known to be unreachable", addr)
	}

	log.Printf("connecting to %s", addr)
	node, err := electrum.NewNode(addr, port, network)
	if err != nil {
		eb.nodeMu.Lock()
		eb.blacklistedNodes[ident] = struct{}{}
		eb.nodeMu.Unlock()
		return err
	}

	// Get the server's features
	feature, err := node.ServerFeatures()
	if err != nil {
		eb.nodeMu.Lock()
		eb.blacklistedNodes[ident] = struct{}{}
		eb.nodeMu.Unlock()
		return err
	}
	// Check genesis block
	if feature.Genesis != utils.GenesisBlock(network) {
		eb.nodeMu.Lock()
		eb.blacklistedNodes[ident] = struct{}{}
		eb.nodeMu.Unlock()
		return ErrIncorrectGenesisBlock
	}
	// TODO: check pruning. Currently, servers currently don't prune, so it's fine to skip for now.

	// Check version
	err = checkVersion(feature.Protocol)
	if err != nil {
		eb.nodeMu.Lock()
		eb.blacklistedNodes[ident] = struct{}{}
		eb.nodeMu.Unlock()
		return err
	}

	// Negotiate version
	err = node.ServerVersion("1.2")
	if err != nil {
		eb.nodeMu.Lock()
		eb.blacklistedNodes[ident] = struct{}{}
		eb.nodeMu.Unlock()
		return ErrFailedNegotiateVersion
	}

	// TODO: ask the server for info on the block height we care about. If the server doesn't have
	// that block, we'll automatically disconnect.

	eb.nodeMu.Lock()
	eb.nodes[ident] = node
	eb.nodeMu.Unlock()

	// We can process requests
	go eb.processRequests(node)

	return nil
}

// Connect to a node without registering it, fetch height and disconnect.
func (eb *ElectrumBackend) getHeight(addr, port string, network utils.Network) (uint32, error) {
	log.Printf("connecting to %s", addr)
	node, err := electrum.NewNode(addr, port, network)
	if err != nil {
		return 0, err
	}
	defer node.Disconnect()

	// Get the server's features
	feature, err := node.ServerFeatures()
	if err != nil {
		return 0, err
	}
	// Check genesis block
	if feature.Genesis != utils.GenesisBlock(network) {
		return 0, ErrIncorrectGenesisBlock
	}
	// TODO: check pruning. Currently, servers currently don't prune, so it's fine to skip for now.

	// Check version
	err = checkVersion(feature.Protocol)
	if err != nil {
		return 0, err
	}

	// Negotiate version
	err = node.ServerVersion("1.2")
	if err != nil {
		return 0, ErrFailedNegotiateVersion
	}

	header, err := node.BlockchainHeadersSubscribe()
	if err != nil {
		log.Printf("BlockchainHeadersSubscribe failed: %+v", err)
		return 0, err
	}

	return header.Height, nil
}

func (eb *ElectrumBackend) processRequests(node *electrum.Node) {
	for {
		select {
		case _ = <-eb.peersRequests:
			err := eb.processPeersRequest(node)
			if err != nil {
				return
			}
		case addr := <-eb.addrRequests:
			err := eb.processAddrRequest(node, addr)
			if err != nil {
				return
			}
		case tx := <-eb.txRequests:
			err := eb.processTxRequest(node, tx)
			if err != nil {
				return
			}
		case block := <-eb.blockRequests:
			err := eb.processBlockRequest(node, block)
			if err != nil {
				return
			}
		}
	}
}

func (eb *ElectrumBackend) processPeersRequest(node *electrum.Node) error {
	eb.nodeMu.Lock()
	numNodes := len(eb.nodes)
	eb.nodeMu.Unlock()

	if numNodes >= maxPeers {
		return nil
	}
	peers, err := node.ServerPeersSubscribe()
	if err != nil {
		log.Printf("ServerPeersSubscribe failed: %+v", err)
		return err
	}
	for _, peer := range peers {
		eb.addPeer(peer)
	}
	return nil
}

func (eb *ElectrumBackend) processTxRequest(node *electrum.Node, txHash string) error {
	hex, err := node.BlockchainTransactionGet(txHash)
	if err != nil {
		log.Printf("processTxRequest failed with: %s, %+v", node.Ident, err)
		eb.removeNode(node.Ident)

		// requeue request
		// TODO: we should have a retry counter and fail gracefully if a transaction fails
		//       too many times.
		eb.txRequests <- txHash
		return err
	}
	height := eb.getTxHeight(txHash)

	eb.txResponses <- &TxResponse{
		Hash:   txHash,
		Height: height,
		Hex:    hex,
	}

	return nil
}

func (eb *ElectrumBackend) getTxHeight(txHash string) int64 {
	eb.transactionsMu.Lock()
	defer eb.transactionsMu.Unlock()

	height, exists := eb.transactions[txHash]
	if !exists {
		log.Panicf("transactions cache miss for %s", txHash)
	}
	return height
}

// note: we could be more efficient and batch things up.
func (eb *ElectrumBackend) processBlockRequest(node *electrum.Node, height uint32) error {
	block, err := node.BlockchainBlockHeaders(height, 1)
	if err != nil {
		log.Printf("processBlockRequest failed with: %s, %+v", node.Ident, err)
		eb.removeNode(node.Ident)

		// requeue request
		// TODO: we should have a retry counter and fail gracefully if an address fails too
		// many times.
		eb.blockRequests <- height
		return err
	}

	// Decode hex to get Timestamp
	b, err := hex.DecodeString(block.Hex)
	if err != nil {
		fmt.Printf("failed to unhex block %d: %s\n", height, block.Hex)
		panic(err)
	}

	var blockHeader wire.BlockHeader
	err = blockHeader.Deserialize(bytes.NewReader(b))
	if err != nil {
		fmt.Printf("failed to parse block %d: %s\n", height, block.Hex)
		panic(err)
	}

	eb.blockResponses <- &BlockResponse{
		Height:    height,
		Timestamp: blockHeader.Timestamp,
	}

	return nil
}

func (eb *ElectrumBackend) processAddrRequest(node *electrum.Node, addr *deriver.Address) error {
	txs, err := node.BlockchainAddressGetHistory(addr.String())
	if err != nil {
		log.Printf("processAddrRequest failed with: %s, %+v", node.Ident, err)
		eb.removeNode(node.Ident)

		// requeue request
		// TODO: we should have a retry counter and fail gracefully if an address fails too
		// many times.
		eb.addrRequests <- addr
		return err
	}

	txHashes := make([]string, 0, len(txs))
	for _, tx := range txs {
		txHashes = append(txHashes, tx.Hash)
		// fetch additional data if needed
	}
	eb.cacheTxs(txs)

	// TODO: we assume there are no more transactions. We should check what the API returns for
	// addresses with very large number of transactions.
	eb.addrResponses <- &AddrResponse{
		Address:  addr,
		TxHashes: txHashes,
	}
	return nil
}

func (eb *ElectrumBackend) cacheTxs(txs []*electrum.Transaction) {
	eb.transactionsMu.Lock()
	defer eb.transactionsMu.Unlock()

	for _, tx := range txs {
		height, exists := eb.transactions[tx.Hash]
		if exists && (height != int64(tx.Height)) {
			log.Panicf("inconsistent cache: %s %d != %d", tx.Hash, height, tx.Height)
		}
		eb.transactions[tx.Hash] = int64(tx.Height)
	}
}

// Checks that a string such as "1.2" or "v1.3" is greater than or equal to 1.2
func checkVersion(ver string) error {
	if ver[0] == 'v' {
		ver = ver[1:]
	}
	f, err := strconv.ParseFloat(ver, 32)
	if err != nil {
		return err
	}
	if f < 1.2 {
		return ErrIncompatibleVersion
	}
	return nil
}

// remove a node from the map of nodes.
func (eb *ElectrumBackend) removeNode(ident string) {
	eb.nodeMu.Lock()
	defer eb.nodeMu.Unlock()
	node, exists := eb.nodes[ident]
	if exists {
		node.Disconnect()
		delete(eb.nodes, ident)
	}
}

func (eb *ElectrumBackend) removeAllNodes() {
	eb.nodeMu.Lock()
	defer eb.nodeMu.Unlock()

	for _, node := range eb.nodes {
		node.Disconnect()
	}

	eb.nodes = map[string]*electrum.Node{}
}

func (eb *ElectrumBackend) findPeers() {
	eb.peersRequests <- struct{}{}
	eb.nodeMu.Lock()
	reporter.GetInstance().SetPeers(int32(len(eb.nodes)))
	eb.nodeMu.Unlock()
}

func (eb *ElectrumBackend) addPeer(peer electrum.Peer) {
	if strings.HasSuffix(peer.Host, ".onion") {
		log.Printf("skipping %s because of .onion\n", peer.Host)
		return
	}
	err := checkVersion(peer.Version)
	if err != nil {
		log.Printf("skipping %s because of protocol version %s\n", peer.Host, peer.Version)
		return
	}
	for _, feature := range peer.Features {
		if strings.HasPrefix(feature, "t") {
			go func(addr, feature string, network utils.Network) {
				if err := eb.addNode(addr, feature, network); err != nil {
					log.Printf("error on addNode: %+v\n", err)
				}
			}(peer.IP, feature, eb.network)
			return
		}
	}
	for _, feature := range peer.Features {
		if strings.HasPrefix(feature, "s") {
			go func(addr, feature string, network utils.Network) {
				if err := eb.addNode(addr, feature, network); err != nil {
					log.Printf("error on addNode: %+v\n", err)
				}
			}(peer.IP, feature, eb.network)
			return
		}
	}
	log.Printf("skipping %s because of feature mismatch: %+v\n", peer, peer.Features)
}
