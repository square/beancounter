package backend

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"sync"

	pkgerr "github.com/pkg/errors"
	"github.com/square/beancounter/deriver"
	"github.com/square/beancounter/reporter"
)

// FixtureBackend loads data from a file that was previously recorded by
// RecorderBackend
type FixtureBackend struct {
	addrIndexMu sync.Mutex
	addrIndex   map[string]AddrResponse
	txIndexMu   sync.Mutex
	txIndex     map[string]TxResponse

	// channels used to communicate with the Accounter
	addrRequests  chan *deriver.Address
	addrResponses chan *AddrResponse
	txResponses   chan *TxResponse

	transactionsMu sync.Mutex // mutex to guard read/writes to transactions map
	transactions   map[string]int64

	// internal channels
	doneCh chan bool

	readOnly bool
}

// NewFixtureBackend returns a new FixtureBackend structs or errors.
func NewFixtureBackend(filepath string) (*FixtureBackend, error) {
	cb := &FixtureBackend{
		addrRequests:  make(chan *deriver.Address, 10),
		addrResponses: make(chan *AddrResponse, 10),
		txResponses:   make(chan *TxResponse, 1000),
		addrIndex:     make(map[string]AddrResponse),
		txIndex:       make(map[string]TxResponse),
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

func (b *FixtureBackend) AddrResponses() <-chan *AddrResponse {
	return b.addrResponses
}

func (b *FixtureBackend) TxResponses() <-chan *TxResponse {
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
	b.addrResponses <- &AddrResponse{
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
		a := AddrResponse{
			Address:  deriver.NewAddress(addr.Path, addr.Address, addr.Network, addr.Change, addr.AddressIndex),
			TxHashes: addr.TxHashes,
		}
		b.addrIndex[addr.Address] = a
	}

	for _, tx := range cachedData.Transactions {
		b.txIndex[tx.Hash] = TxResponse{
			Hash:   tx.Hash,
			Height: tx.Height,
			Hex:    tx.Hex,
		}
	}

	return nil
}
