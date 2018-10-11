package backendtest

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"sync"

	pkgerr "github.com/pkg/errors"
	"github.com/square/beancounter/backend"
	"github.com/square/beancounter/deriver"
	"github.com/square/beancounter/reporter"
	. "github.com/square/beancounter/utils"
)

// FixtureBackend wraps Btcd node and its API to provide a simple
// balance and transaction history information for a given address.
// FixtureBackend implements Backend interface.
type FixtureBackend struct {
	backend     backend.Backend
	addrIndexMu sync.Mutex
	addrIndex   map[string]backend.AddrResponse
	txIndexMu   sync.Mutex
	txIndex     map[string]backend.TxResponse

	// channels used to communicate with the Accounter
	addrRequests  chan *deriver.Address
	addrResponses chan *backend.AddrResponse
	txResponses   chan *backend.TxResponse

	transactionsMu sync.Mutex // mutex to guard read/writes to transactions map
	transactions   map[string]int64

	// internal channels
	doneCh chan bool

	readOnly bool
}

// NewFixtureBackend returns a new FixtureBackend structs or errors.
// FixtureBackend takes into account maxBlockHeight and ignores any transactions that belong to higher blocks.
// If 0 is passed, then the block chain is queried for max block height and minConfirmations is subtracted
// (to avoid querying blocks that might potentially be orphaned).
//
// NOTE: FixtureBackend is assumed to be connecting to a personal node, hence it disables TLS for now
func NewFixture(filepath string) (*FixtureBackend, error) {
	cb := &FixtureBackend{
		addrRequests:  make(chan *deriver.Address, 10),
		addrResponses: make(chan *backend.AddrResponse, 10),
		txResponses:   make(chan *backend.TxResponse, 1000),
		addrIndex:     make(map[string]backend.AddrResponse),
		txIndex:       make(map[string]backend.TxResponse),
		transactions:  make(map[string]int64),
		doneCh:        make(chan bool),
	}

	f, err := os.Open(filepath)
	if err != nil {
		return nil, pkgerr.Wrap(err, "cannot open a fixture file")
	}
	defer f.Close()

	if err := cb.loadFromFile(f); err != nil {
		return nil, pkgerr.Wrap(err, "cannot load data from a fixture file")
	}

	go cb.processRequests()
	return cb, nil
}

func (b *FixtureBackend) AddrRequest(addr *deriver.Address) {
	b.addrRequests <- addr
}

func (b *FixtureBackend) AddrResponses() <-chan *backend.AddrResponse {
	return b.addrResponses
}

func (b *FixtureBackend) TxResponses() <-chan *backend.TxResponse {
	return b.txResponses
}

func (b *FixtureBackend) Finish() {
	close(b.doneCh)
}

func (b *FixtureBackend) processRequests() {
	for {
		select {
		case addr := <-b.addrRequests:
			b.processAddrRequest(addr)
		case addrResp, ok := <-b.addrResponses:
			if !ok {
				b.addrResponses = nil
				continue
			}
			b.addrResponses <- addrResp
		case txResp, ok := <-b.txResponses:
			if !ok {
				b.txResponses = nil
				continue
			}
			b.txResponses <- txResp
		case <-b.doneCh:
			return
		}
	}
}

func (b *FixtureBackend) processAddrRequest(address *deriver.Address) {
	b.addrIndexMu.Lock()
	resp, exists := b.addrIndex[address.String()]
	b.addrIndexMu.Unlock()

	reporter.GetInstance().IncAddressesScheduled()
	reporter.GetInstance().Log(fmt.Sprintf("[fixture] scheduling address: %s", address))

	if exists {
		b.addrResponses <- &resp
		go b.scheduleTx(resp.TxHashes)
		return
	}

	// assuming that address has not been used
	b.addrResponses <- &backend.AddrResponse{
		Address: address,
	}
}

func (b *FixtureBackend) scheduleTx(txIDs []string) {
	for _, txid := range txIDs {
		b.transactionsMu.Lock()
		_, exists := b.transactions[txid]
		b.transactionsMu.Unlock()

		if exists {
			return
		}

		b.txIndexMu.Lock()
		tx, exists := b.txIndex[txid]
		b.txIndexMu.Unlock()

		// if cached address lists a transaction that doesn't exist in cache,
		// then something is wrong.
		if !exists {
			panic(fmt.Sprintf("inconsistent cache: %s", txid))
		}
		reporter.GetInstance().IncTxScheduled()
		reporter.GetInstance().Log(fmt.Sprintf("[fixture] scheduling tx: %s", txid))

		b.txResponses <- &tx
	}
}

type index struct {
	Addresses    []address     `json:"addresses"`
	Transactions []transaction `json:"transactions"`
}

type address struct {
	Address      string   `json:"address"`
	Path         string   `json:"path"`
	Network      Network  `json:"network"`
	Change       uint32   `json:"change"`
	AddressIndex uint32   `json:"addr_index"`
	TxHashes     []string `json:"tx_hashes"`
}

type byAddress []address

func (a byAddress) Len() int           { return len(a) }
func (a byAddress) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a byAddress) Less(i, j int) bool { return a[i].Address < a[j].Address }

type transaction struct {
	Hash   string `json:"hash"`
	Height int64  `json:"height"`
	Hex    string `json:"hex"`
}

type byTransactionID []transaction

func (a byTransactionID) Len() int           { return len(a) }
func (a byTransactionID) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a byTransactionID) Less(i, j int) bool { return a[i].Hash < a[j].Hash }

func (b *FixtureBackend) loadFromFile(f *os.File) error {
	var cachedData index

	byteValue, err := ioutil.ReadAll(f)
	if err != nil {
		return err
	}

	err = json.Unmarshal(byteValue, &cachedData)
	if err != nil {
		return err
	}

	for _, addr := range cachedData.Addresses {
		a := backend.AddrResponse{
			Address:  deriver.NewAddress(addr.Path, addr.Address, addr.Network, addr.Change, addr.AddressIndex),
			TxHashes: addr.TxHashes,
		}
		b.addrIndex[addr.Address] = a
	}

	for _, tx := range cachedData.Transactions {
		b.txIndex[tx.Hash] = backend.TxResponse{
			Hash:   tx.Hash,
			Height: tx.Height,
			Hex:    tx.Hex,
		}
	}

	return nil
}
