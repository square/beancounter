package balance

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"strings"
	"sync"
	"time"

	"github.com/mbyczkowski/go-electrum/electrum"
	"github.com/square/beancounter/deriver"
	. "github.com/square/beancounter/utils"
)

// Fetches balance and transaction information from Electrum servers.
// A few things to keep in mind:
// - Servers might prune data. It should not matter for computing balances, but
//   it implies that we might fail to list some transactions. A crazy edge case
//   could be that we fail to lookahead far enough.
// - We don't check if the servers we are connected to are using the right
//   genesis block, right protocol versions, etc.
// - We don't handle servers disconnecting. Once connected, we assume the connection
//   is stable.
//
// In the long term, it might make more sense to change our tool to only talk to a
// local bitcoind node.
//
// Electrum protocol docs: https://electrumx.readthedocs.io/en/latest/protocol.html

// ElectrumChecker wraps Electrum node and its API to provide a simple
// balance and transaction history information for a given address.
// ElectrumChecker implements Checker interface.
type ElectrumChecker struct {
	nodeMu           sync.RWMutex // mutex to guard reads/writes to nodes map
	nodes            map[string]*electrum.Node
	blacklistedNodes map[string]struct{}
}

const (
	maxPeers = 100
)

type serverPeersSubscribe struct {
	ip       string
	host     string
	features []string
}

// NewElectrumChecker returns a new ElectrumChecker structs or errors.
// Initially connects to 1 node. A background job handles connecting to
// additional peers. The background job fails if there are no peers left.
func NewElectrumChecker(addr string) (*ElectrumChecker, error) {
	ec := &ElectrumChecker{
		nodes:            make(map[string]*electrum.Node),
		blacklistedNodes: make(map[string]struct{}),
	}
	if err := ec.addNode(addr); err != nil {
		return nil, err
	}

	// goroutine to continuously fetch additional peers
	go func() {
		for {
			ec.fetchPeers()
			time.Sleep(1 * time.Second)
		}
	}()

	return ec, nil
}

// Fetch queries connected node for address balance and transaction history and
// returns Response.
func (e *ElectrumChecker) Fetch(addr string) *Response {
	n := e.randomNode()
	b, err := n.BlockchainAddressGetBalance(addr)
	if err != nil {
		// node is not healthy any more
		e.removeNode(n.Address())
		return &Response{Error: err}
	}

	txs, err := n.BlockchainAddressGetHistory(addr)
	if err != nil {
		// node is not healthy any more
		e.removeNode(n.Address())
		return &Response{Error: err}
	}

	var transactions []Transaction
	for _, tx := range txs {
		t := Transaction{}
		t.Hash = tx.Hash
		if tx.Value < 0 {
			panic("Value cannot be negative")
		}
		t.Value = uint64(tx.Value)
		transactions = append(transactions, t)
	}

	resp := &Response{
		Balance:      uint64(b.Confirmed),
		Transactions: transactions,
	}
	return resp
}

// Subscribe provides a bidirectional streaming of checks for addresses.
// It takes a channel of addresses and returns a channel of responses, to which
// it is writing asynchronuously.
// TODO: Looks like Subscribe implementation is separate from implementation
//       details of each checker and therefore could be abstracted into a separate
//       struct/interface (e.g. there could be a StreamingChecker interface that
//       implements Subcribe method).
func (e *ElectrumChecker) Subscribe(addrCh <-chan *deriver.Address) <-chan *Response {
	respCh := make(chan *Response, 100)
	go func() {
		var wg sync.WaitGroup
		for addr := range addrCh {
			wg.Add(1)
			// do not block on each Fetch API call
			go e.processFetch(addr, respCh, &wg)
		}
		// ensure that all addresses are processed and written to the output channel
		// before closing it.
		wg.Wait()
		close(respCh)
	}()

	return respCh
}

// processFetch fetches the data for an address, sends the response to the outgoing
// channel and marks itself as done in the shared WorkGroup
func (e *ElectrumChecker) processFetch(addr *deriver.Address, out chan<- *Response, wg *sync.WaitGroup) {
	resp := e.Fetch(addr.String())
	resp.Address = addr
	out <- resp
	wg.Done()
}

// add a node to the map of nodes.
func (e *ElectrumChecker) addNode(addr string) error {
	e.nodeMu.RLock()
	_, existsGood := e.nodes[addr]
	_, existsBad := e.blacklistedNodes[addr]
	e.nodeMu.RUnlock()
	if existsGood {
		return fmt.Errorf("already connected to %s", addr)
	}

	if existsBad {
		return fmt.Errorf("%s is known to be unreachable", addr)
	}

	log.Printf("connecting to %s", addr)
	node := electrum.NewNode()
	conf := &tls.Config{
		InsecureSkipVerify: true,
	}
	if err := node.ConnectSSL(addr, conf); err != nil {
		e.nodeMu.Lock()
		e.blacklistedNodes[addr] = struct{}{}
		e.nodeMu.Unlock()
		return err
	}

	e.nodeMu.Lock()
	e.nodes[addr] = node
	e.nodeMu.Unlock()

	return nil
}

// remove a node from the map of nodes.
func (e *ElectrumChecker) removeNode(addr string) {
	e.nodeMu.Lock()
	defer e.nodeMu.Unlock()

	delete(e.nodes, addr)
}

func (e *ElectrumChecker) fetchPeers() {
	e.nodeMu.Lock()
	numNodes := len(e.nodes)
	e.nodeMu.Unlock()

	if numNodes >= maxPeers {
		return
	}
	if numNodes == 0 {
		panic("No more peers.")
	}
	log.Printf("We have %d peers. Attempting to fetch additional ones.\n", numNodes)

	node := e.randomNode()
	responses, err := node.ServerPeersSubscribe()
	if err != nil {
		log.Printf("ServerPeersSubscribe failed: %+v", err)
		return
	}
	for _, response := range responses {
		var newNode serverPeersSubscribe
		bridgeInterface := []interface{}{&newNode.ip, &newNode.host, &newNode.features}
		t, err := json.Marshal(response)
		if err != nil {
			log.Printf("re-marshal failed: %+v", err)
			continue
		}
		json.Unmarshal(t, &bridgeInterface)

		// randomize the order of peers
		ShuffleStrings(newNode.features)

		for _, feature := range newNode.features {
			if strings.HasPrefix(feature, "s") {
				addr := newNode.ip + ":" + strings.TrimPrefix(feature, "s")
				go func(addr string) {
					if err := e.addNode(addr); err != nil {
						//log.Printf("addNode failed, skipping: %+v\n", err)
					}
					//log.Printf("successfully added %s", addr)
				}(addr)
			}
		}
	}
}

func (e *ElectrumChecker) randomNode() *electrum.Node {
	e.nodeMu.RLock()
	defer e.nodeMu.RUnlock()

	i := rand.Intn(len(e.nodes))
	for _, v := range e.nodes {
		if i == 0 {
			return v
		}
		i--
	}
	panic("unreachable")
}
