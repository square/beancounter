package backend

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"sync"

	"github.com/square/beancounter/deriver"
	"github.com/square/beancounter/reporter"
	. "github.com/square/beancounter/utils"
)

// RecorderBackend wraps Btcd node and its API to provide a simple
// balance and transaction history information for a given address.
// RecorderBackend implements Backend interface.
type RecorderBackend struct {
	backend     Backend
	addrIndexMu sync.Mutex
	addrIndex   map[string]AddrResponse
	txIndexMu   sync.Mutex
	txIndex     map[string]TxResponse

	// channels used to communicate with the Accounter
	addrResponses chan *AddrResponse
	txResponses   chan *TxResponse

	transactionsMu sync.Mutex // mutex to guard read/writes to transactions map
	transactions   map[string]int64

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
		addrIndex:      make(map[string]AddrResponse),
		txIndex:        make(map[string]TxResponse),
		transactions:   make(map[string]int64),
		doneCh:         make(chan bool),
		outputFilepath: filepath,
	}

	go rb.processRequests()
	return rb, nil
}

func (rb *RecorderBackend) AddrRequest(addr *deriver.Address) {
	rb.backend.AddrRequest(addr)
}

func (rb *RecorderBackend) AddrResponses() <-chan *AddrResponse {
	return rb.addrResponses
}

func (rb *RecorderBackend) TxResponses() <-chan *TxResponse {
	return rb.txResponses
}

func (rb *RecorderBackend) Finish() {
	rb.backend.Finish()
	close(rb.doneCh)

	if err := rb.writeToFile(); err != nil {
		fmt.Println(err)
	}
}

func (rb *RecorderBackend) processRequests() {
	backendAddrResponses := rb.backend.AddrResponses()
	backendTxResponses := rb.backend.TxResponses()

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
		case <-rb.doneCh:
			return
		}
	}
}

func (rb *RecorderBackend) writeToFile() error {
	cachedData := index{Addresses: []address{}, Transactions: []transaction{}}

	reporter.GetInstance().Log(fmt.Sprintf("writing data to %s\n ...", rb.outputFilepath))
	f, err := os.Create(rb.outputFilepath)
	if err != nil {
		return err
	}
	defer f.Close()

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
