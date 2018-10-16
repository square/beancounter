package backend

import (
	"encoding/json"
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
	txRequests    chan string
	txResponses   chan *TxResponse

	transactionsMu sync.Mutex // mutex to guard read/writes to transactions map
	transactions   map[string]int64

	// internal channels
	doneCh chan bool

	readOnly bool

	height uint32
}

// NewFixtureBackend returns a new FixtureBackend structs or errors.
func NewFixtureBackend(filepath string) (*FixtureBackend, error) {
	cb := &FixtureBackend{
		addrRequests:  make(chan *deriver.Address, 10),
		addrResponses: make(chan *AddrResponse, 10),
		txRequests:    make(chan string, 1000),
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

// AddrRequest schedules a request to the backend to lookup information related
// to the given address.
func (b *FixtureBackend) AddrRequest(addr *deriver.Address) {
	reporter.GetInstance().IncAddressesScheduled()
	reporter.GetInstance().Logf("[fixture] scheduling address: %s", addr)
	b.addrRequests <- addr
}

// TxRequest schedules a request to the backend to lookup information related
// to the given transaction hash.
func (b *FixtureBackend) TxRequest(txHash string) {
	reporter.GetInstance().IncTxScheduled()
	reporter.GetInstance().Logf("[fixture] scheduling tx: %s", txHash)
	b.txRequests <- txHash
}

// AddrResponses exposes a channel that allows to consume backend's responses to
// address requests created with AddrRequest()
func (b *FixtureBackend) AddrResponses() <-chan *AddrResponse {
	return b.addrResponses
}

// TxResponses exposes a channel that allows to consume backend's responses to
// address requests created with addrrequest().
// if an address has any transactions then they will be sent to this channel by the
// backend.
func (b *FixtureBackend) TxResponses() <-chan *TxResponse {
	return b.txResponses
}

// Finish informs the backend to stop doing its work.
func (b *FixtureBackend) Finish() {
	close(b.doneCh)
}

func (b *FixtureBackend) ChainHeight() uint32 {
	return b.height
}

func (b *FixtureBackend) processRequests() {
	for {
		select {
		case addr := <-b.addrRequests:
			b.processAddrRequest(addr)
		case tx := <-b.txRequests:
			b.processTxRequest(tx)
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

func (b *FixtureBackend) processAddrRequest(addr *deriver.Address) {
	b.addrIndexMu.Lock()
	resp, exists := b.addrIndex[addr.String()]
	b.addrIndexMu.Unlock()

	if exists {
		b.addrResponses <- &resp
		return
	}

	// assuming that address has not been used
	b.addrResponses <- &AddrResponse{
		Address: addr,
	}
}

func (b *FixtureBackend) processTxRequest(txHash string) {
	b.txIndexMu.Lock()
	resp, exists := b.txIndex[txHash]
	b.txIndexMu.Unlock()

	if exists {
		b.txResponses <- &resp
		return
	}

	// assuming that transaction does not exist in the fixture file
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

	b.height = cachedData.Metadata.Height

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

		b.transactions[tx.Hash] = tx.Height
	}

	return nil
}
