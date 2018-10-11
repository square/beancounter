package backend

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"sync"

	"github.com/square/beancounter/deriver"
	"github.com/square/beancounter/reporter"
)

// RecorderBackend wraps Btcd node and its API to provide a simple
// balance and transaction history information for a given address.
// RecorderBackend implements Backend interface.
type RecorderBackend struct {
	backend      Backend
	addrIndexMu  sync.Mutex
	addrIndex    map[string]AddrResponse
	txIndexMu    sync.Mutex
	txIndex      map[string]TxResponse
	blockIndexMu sync.Mutex
	blockIndex   map[uint32]BlockResponse

	// channels used to communicate with the Accounter
	addrResponses chan *AddrResponse
	txResponses   chan *TxResponse

	// channels used to communicate with the Blockfinder
	blockResponses chan *BlockResponse

	// internal channels
	doneCh chan bool

	outputFilepath string
}

// NewRecorderBackend returns a new RecorderBackend structs or errors.
// RecorderBackend passes requests to another backend and ten records
// address and transaction responses to a file. The file can later be used by a
// FixtureBackend to reply those responses.
func NewRecorderBackend(b Backend, filepath string) (*RecorderBackend, error) {
	rb := &RecorderBackend{
		backend:        b,
		addrResponses:  make(chan *AddrResponse, addrRequestsChanSize),
		txResponses:    make(chan *TxResponse, 2*maxTxsPerAddr),
		blockResponses: make(chan *BlockResponse, blockRequestChanSize),
		addrIndex:      make(map[string]AddrResponse),
		txIndex:        make(map[string]TxResponse),
		blockIndex:     make(map[uint32]BlockResponse),
		doneCh:         make(chan bool),
		outputFilepath: filepath,
	}

	go rb.processRequests()
	return rb, nil
}

// AddrRequest schedules a request to the backend to lookup information related
// to the given address.
func (rb *RecorderBackend) AddrRequest(addr *deriver.Address) {
	rb.backend.AddrRequest(addr)
}

// AddrResponses exposes a channel that allows to consume backend's responses to
// address requests created with AddrRequest()
func (rb *RecorderBackend) AddrResponses() <-chan *AddrResponse {
	return rb.addrResponses
}

// TxRequest schedules a request to the backend to lookup information related
// to the given transaction hash.
func (rb *RecorderBackend) TxRequest(txHash string) {
	rb.backend.TxRequest(txHash)
}

// TxResponses exposes a channel that allows to consume backend's responses to
// address requests created with addrrequest().
// if an address has any transactions then they will be sent to this channel by the
// backend.
func (rb *RecorderBackend) TxResponses() <-chan *TxResponse {
	return rb.txResponses
}

func (rb *RecorderBackend) BlockRequest(height uint32) {
	rb.backend.BlockRequest(height)
}

func (rb *RecorderBackend) BlockResponses() <-chan *BlockResponse {
	return rb.blockResponses
}

// Finish informs the backend to stop doing its work.
func (rb *RecorderBackend) Finish() {
	rb.backend.Finish()
	close(rb.doneCh)

	if err := rb.writeToFile(); err != nil {
		fmt.Println(err)
	}
}

func (rb *RecorderBackend) ChainHeight() uint32 {
	return rb.backend.ChainHeight()
}

func (rb *RecorderBackend) processRequests() {
	backendAddrResponses := rb.backend.AddrResponses()
	backendTxResponses := rb.backend.TxResponses()
	backendBlockResponses := rb.backend.BlockResponses()

	for {
		select {
		case addrResp, ok := <-backendAddrResponses:
			if !ok {
				backendAddrResponses = nil
				continue
			}
			rb.addrIndexMu.Lock()
			rb.addrIndex[addrResp.Address.String()] = *addrResp
			rb.addrIndexMu.Unlock()
			rb.addrResponses <- addrResp
		case txResp, ok := <-backendTxResponses:
			if !ok {
				backendTxResponses = nil
				continue
			}
			rb.txIndexMu.Lock()
			rb.txIndex[txResp.Hash] = *txResp
			rb.txIndexMu.Unlock()
			rb.txResponses <- txResp
		case block, ok := <-backendBlockResponses:
			if !ok {
				backendBlockResponses = nil
				continue
			}
			rb.blockIndexMu.Lock()
			rb.blockIndex[block.Height] = *block
			rb.blockIndexMu.Unlock()
			rb.blockResponses <- block
		case <-rb.doneCh:
			return
		}
	}
}

func (rb *RecorderBackend) writeToFile() error {
	cachedData := index{
		Metadata: metadata{}, Addresses: []address{}, Transactions: []transaction{},
		Blocks: []block{},
	}

	reporter.GetInstance().Logf("writing data to %s\n ...", rb.outputFilepath)
	f, err := os.Create(rb.outputFilepath)
	if err != nil {
		return err
	}
	defer f.Close()

	cachedData.Metadata.Height = rb.ChainHeight()

	for addr, addrResp := range rb.addrIndex {
		a := address{
			Address:      addr,
			Path:         addrResp.Address.Path(),
			Network:      addrResp.Address.Network(),
			Change:       addrResp.Address.Change(),
			AddressIndex: addrResp.Address.Index(),
			TxHashes:     addrResp.TxHashes,
		}
		cachedData.Addresses = append(cachedData.Addresses, a)
	}

	sort.Sort(byAddress(cachedData.Addresses))

	for _, txResp := range rb.txIndex {
		tx := transaction{
			Hash:   txResp.Hash,
			Height: txResp.Height,
			Hex:    txResp.Hex,
		}
		cachedData.Transactions = append(cachedData.Transactions, tx)
	}
	sort.Sort(byTransactionID(cachedData.Transactions))

	for _, b := range rb.blockIndex {
		cachedData.Blocks = append(cachedData.Blocks, block{
			Height:    b.Height,
			Timestamp: b.Timestamp,
		})
	}

	cachedDataJSON, err := json.MarshalIndent(cachedData, "", "    ")
	if err != nil {
		return err
	}

	_, err = f.Write(cachedDataJSON)
	if err != nil {
		return err
	}

	return nil
}
