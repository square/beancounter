package backend

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"sort"
	"sync"
	"time"

	"github.com/square/beancounter/deriver"
	"github.com/square/beancounter/reporter"
	. "github.com/square/beancounter/utils"
)

// CacheBackend wraps Btcd node and its API to provide a simple
// balance and transaction history information for a given address.
// CacheBackend implements Backend interface.
type CacheBackend struct {
	backend     Backend
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

// NewCacheBackend returns a new CacheBackend structs or errors.
// CacheBackend takes into account maxBlockHeight and ignores any transactions that belong to higher blocks.
// If 0 is passed, then the block chain is queried for max block height and minConfirmations is subtracted
// (to avoid querying blocks that might potentially be orphaned).
//
// NOTE: CacheBackend is assumed to be connecting to a personal node, hence it disables TLS for now
func NewCacheBackend(b Backend, storage *os.File) (*CacheBackend, error) {
	cb := &CacheBackend{backend: b,
		addrRequests:  make(chan *deriver.Address, addrRequestsChanSize),
		addrResponses: make(chan *AddrResponse, addrRequestsChanSize),
		txResponses:   make(chan *TxResponse, 2*maxTxsPerAddr),
		addrIndex:     make(map[string]AddrResponse),
		txIndex:       make(map[string]TxResponse),
		transactions:  make(map[string]int64),
		doneCh:        make(chan bool),
	}

	if storage != nil {
		if err := cb.loadFromFile(storage); err != nil {
			return nil, err
		}
		cb.readOnly = true
	}

	go cb.processRequests()
	return cb, nil
}

func (b *CacheBackend) AddrRequest(addr *deriver.Address) {
	b.addrRequests <- addr
}

func (b *CacheBackend) AddrResponses() <-chan *AddrResponse {
	return b.addrResponses
}

func (b *CacheBackend) TxResponses() <-chan *TxResponse {
	return b.txResponses
}

func (b *CacheBackend) Dec() {
	// NOOP
}

func (b *CacheBackend) Finish() {
	b.backend.Finish()
	close(b.doneCh)

	if !b.readOnly {
		if err := b.writeToFile(); err != nil {
			fmt.Println(err)
		}
	}
}

func (b *CacheBackend) processRequests() {
	backendAddrResponses := b.backend.AddrResponses()
	backendTxResponses := b.backend.TxResponses()

	for {
		select {
		case addr := <-b.addrRequests:
			b.processAddrRequest(addr)
		case addrResp, ok := <-backendAddrResponses:
			if !ok {
				backendAddrResponses = nil
				continue
			}
			b.addrIndexMu.Lock()
			b.addrIndex[addrResp.Address.String()] = *addrResp
			b.addrIndexMu.Unlock()
			b.addrResponses <- addrResp
		case txResp, ok := <-backendTxResponses:
			if !ok {
				backendTxResponses = nil
				continue
			}
			b.txIndexMu.Lock()
			b.txIndex[txResp.Hash] = *txResp
			b.txIndexMu.Unlock()
			b.txResponses <- txResp
		case <-b.doneCh:
			return
		}
	}
}

func (b *CacheBackend) processAddrRequest(address *deriver.Address) {
	b.addrIndexMu.Lock()
	resp, exists := b.addrIndex[address.String()]
	b.addrIndexMu.Unlock()

	if exists {
		reporter.GetInstance().IncAddressesScheduled()
		reporter.GetInstance().Log(fmt.Sprintf("[cache] scheduling address: %s", address))

		b.addrResponses <- &resp
		go b.scheduleTx(resp.TxHashes)
		return
	}

	// cache miss
	b.backend.AddrRequest(address)
}

func (b *CacheBackend) scheduleTx(txIDs []string) {
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
		reporter.GetInstance().Log(fmt.Sprintf("[cache] scheduling tx: %s", txid))

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

func (b *CacheBackend) writeToFile() error {
	cachedData := index{Addresses: []address{}, Transactions: []transaction{}}

	filename := "cached_data_" + time.Now().Format(time.RFC3339)
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	filepath := path.Join(cwd, filename)

	reporter.GetInstance().Log(fmt.Sprintf("writing data to %s\n ...", filepath))
	f, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer f.Close()

	for addr, addrResp := range b.addrIndex {
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

	for _, txResp := range b.txIndex {
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

func (b *CacheBackend) loadFromFile(f *os.File) error {
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
