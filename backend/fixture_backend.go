package backend

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"os"
	"sync"

	pkgerr "github.com/pkg/errors"
	"github.com/square/beancounter/deriver"
	"github.com/square/beancounter/reporter"
)

// FixtureBackend loads data from a file that was previously recorded by
// RecorderBackend
type FixtureBackend struct {
	addrIndexMu  sync.Mutex
	addrIndex    map[string]AddrResponse
	txIndexMu    sync.Mutex
	txIndex      map[string]TxResponse
	blockIndexMu sync.Mutex
	blockIndex   map[uint32]BlockResponse

	// channels used to communicate with the Accounter
	addrRequests  chan *deriver.Address
	addrResponses chan *AddrResponse
	txRequests    chan string
	txResponses   chan *TxResponse

	// channels used to communicate with the Blockfinder
	blockRequests  chan uint32
	blockResponses chan *BlockResponse

	transactionsMu sync.Mutex // mutex to guard read/writes to transactions map
	transactions   map[string]int64

	// internal channels
	doneCh chan bool

	readOnly bool

	height uint32
}

// NewFixtureBackend returns a new FixtureBackend structs or errors.
func NewFixtureBackend(filepath string) (*FixtureBackend, error) {
	fb := &FixtureBackend{
		addrRequests:   make(chan *deriver.Address, 10),
		addrResponses:  make(chan *AddrResponse, 10),
		txRequests:     make(chan string, 1000),
		txResponses:    make(chan *TxResponse, 1000),
		blockRequests:  make(chan uint32, 10),
		blockResponses: make(chan *BlockResponse, 10),
		addrIndex:      make(map[string]AddrResponse),
		txIndex:        make(map[string]TxResponse),
		blockIndex:     make(map[uint32]BlockResponse),
		transactions:   make(map[string]int64),
		doneCh:         make(chan bool),
	}

	f, err := os.Open(filepath)
	if err != nil {
		return nil, pkgerr.Wrap(err, "cannot open a fixture file")
	}
	defer f.Close()

	if err := fb.loadFromFile(f); err != nil {
		return nil, pkgerr.Wrap(err, "cannot load data from a fixture file")
	}

	return fb, nil
}

func (fb *FixtureBackend) ChainHeight() uint32 {
	return fb.height
}

func (fb *FixtureBackend) Start(blockHeight uint32) error {
	if fb.height < blockHeight {
		log.Panicf("recorded height %d < %d", fb.height, blockHeight)
	}
	go fb.processRequests()
	return nil
}

// AddrRequest schedules a request to the backend to lookup information related
// to the given address.
func (fb *FixtureBackend) AddrRequest(addr *deriver.Address) {
	reporter.GetInstance().IncAddressesScheduled()
	reporter.GetInstance().Logf("[fixture] scheduling address: %s", addr)
	fb.addrRequests <- addr
}

// TxRequest schedules a request to the backend to lookup information related
// to the given transaction hash.
func (fb *FixtureBackend) TxRequest(txHash string) {
	reporter.GetInstance().IncTxScheduled()
	reporter.GetInstance().Logf("[fixture] scheduling tx: %s", txHash)
	fb.txRequests <- txHash
}

func (fb *FixtureBackend) BlockRequest(height uint32) {
	fb.blockRequests <- height
}

// AddrResponses exposes a channel that allows to consume backend's responses to
// address requests created with AddrRequest()
func (fb *FixtureBackend) AddrResponses() <-chan *AddrResponse {
	return fb.addrResponses
}

// TxResponses exposes a channel that allows to consume backend's responses to
// address requests created with addrrequest().
// if an address has any transactions then they will be sent to this channel by the
// backend.
func (fb *FixtureBackend) TxResponses() <-chan *TxResponse {
	return fb.txResponses
}

func (fb *FixtureBackend) BlockResponses() <-chan *BlockResponse {
	return fb.blockResponses
}

// Finish informs the backend to stop doing its work.
func (fb *FixtureBackend) Finish() {
	close(fb.doneCh)
}

func (fb *FixtureBackend) processRequests() {
	for {
		select {
		case addr := <-fb.addrRequests:
			fb.processAddrRequest(addr)
		case tx := <-fb.txRequests:
			fb.processTxRequest(tx)
		case addrResp, ok := <-fb.addrResponses:
			if !ok {
				fb.addrResponses = nil
				continue
			}
			fb.addrResponses <- addrResp
		case txResp, ok := <-fb.txResponses:
			if !ok {
				fb.txResponses = nil
				continue
			}
			fb.txResponses <- txResp
		case block := <-fb.blockRequests:
			fb.processBlockRequest(block)
		case <-fb.doneCh:
			return
		}
	}
}

func (fb *FixtureBackend) processAddrRequest(addr *deriver.Address) {
	fb.addrIndexMu.Lock()
	resp, exists := fb.addrIndex[addr.String()]
	fb.addrIndexMu.Unlock()

	if exists {
		fb.addrResponses <- &resp
		return
	}

	// assuming that address has not been used
	fb.addrResponses <- &AddrResponse{
		Address: addr,
	}
}

func (fb *FixtureBackend) processTxRequest(txHash string) {
	fb.txIndexMu.Lock()
	resp, exists := fb.txIndex[txHash]
	fb.txIndexMu.Unlock()

	if exists {
		fb.txResponses <- &resp
		return
	}

	// assuming that transaction does not exist in the fixture file
}

func (fb *FixtureBackend) processBlockRequest(height uint32) {
	fb.blockIndexMu.Lock()
	resp, exists := fb.blockIndex[height]
	fb.blockIndexMu.Unlock()

	if exists {
		fb.blockResponses <- &resp
		return
	}
	log.Panicf("fixture doesn't contain block %d", height)
}

func (fb *FixtureBackend) loadFromFile(f *os.File) error {
	var cachedData index

	byteValue, err := ioutil.ReadAll(f)
	if err != nil {
		return err
	}

	err = json.Unmarshal(byteValue, &cachedData)
	if err != nil {
		return err
	}

	fb.height = cachedData.Metadata.Height

	for _, addr := range cachedData.Addresses {
		a := AddrResponse{
			Address:  deriver.NewAddress(addr.Path, addr.Address, addr.Network, addr.Change, addr.AddressIndex),
			TxHashes: addr.TxHashes,
		}
		fb.addrIndex[addr.Address] = a
	}

	for _, tx := range cachedData.Transactions {
		fb.txIndex[tx.Hash] = TxResponse{
			Hash:   tx.Hash,
			Height: tx.Height,
			Hex:    tx.Hex,
		}

		fb.transactions[tx.Hash] = tx.Height
	}

	for _, b := range cachedData.Blocks {
		fb.blockIndex[b.Height] = BlockResponse{
			Height:    b.Height,
			Timestamp: b.Timestamp,
		}
	}

	return nil
}
