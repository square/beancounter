package backend

import (
	"fmt"
	"sync"

	"github.com/btcsuite/btcd/btcjson"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/rpcclient"
	"github.com/pkg/errors"
	"github.com/square/beancounter/deriver"
	"github.com/square/beancounter/reporter"
	"github.com/square/beancounter/utils"
)

// BtcdBackend wraps Btcd node and its API to provide a simple
// balance and transaction history information for a given address.
// BtcdBackend implements Backend interface.
type BtcdBackend struct {
	chainHeight uint32

	client            *rpcclient.Client
	network           utils.Network
	blockHeightMu     sync.Mutex // mutex to guard read/writes to blockHeightLookup map
	blockHeightLookup map[string]int64

	// channels used to communicate with the Accounter
	addrRequests  chan *deriver.Address
	addrResponses chan *AddrResponse
	txRequests    chan string
	txResponses   chan *TxResponse

	// channels used to communicate with the Blockfinder
	blockRequests  chan uint32
	blockResponses chan *BlockResponse

	// internal channels
	transactionsMu     sync.Mutex // mutex to guard read/writes to transactions map
	cachedTransactions map[string]*TxResponse
	doneCh             chan bool
}

const (
	// For now assume that there cannot be more than maxTxsPerAddr.
	// Ideally, if maxTxsPerAddr is reached then we should paginate and retrieve
	// all the transactions.
	maxTxsPerAddr = 1000

	addrRequestsChanSize = 100
	blockRequestChanSize = 100

	concurrency = 100
)

// NewBtcdBackend returns a new BtcdBackend structs or errors.
//
// BtcdBackend is meants to connect to a personal Btcd node (because public nodes don't expose the
// API we need). There's no TLS support. If your node is not co-located with Beancounter, we
// recommend wrapping your connection in a ssh or other secure tunnel.
func NewBtcdBackend(host, port, user, pass string, network utils.Network) (*BtcdBackend, error) {
	connCfg := &rpcclient.ConnConfig{
		Host:         fmt.Sprintf("%s:%s", host, port),
		User:         user,
		Pass:         pass,
		HTTPPostMode: true, // Bitcoin core only supports HTTP POST mode
		DisableTLS:   true, // Since we're assuming a personal bitcoin node for now, skip TLS
	}
	client, err := rpcclient.New(connCfg, nil)
	if err != nil {
		return nil, errors.Wrap(err, "could not create a Btcd RPC client")
	}

	// Check that we are talking to the right chain
	genesis, err := client.GetBlockHash(0)
	if err != nil {
		return nil, errors.Wrap(err, "GetBlockHash(0) failed")
	}
	if genesis.String() != utils.GenesisBlock(network) {
		return nil, errors.New(fmt.Sprintf("Unexpected genesis block %s != %s", genesis.String(), utils.GenesisBlock(network)))
	}

	height, err := client.GetBlockCount()
	if err != nil {
		return nil, errors.Wrap(err, "could not connect to the Btcd server")
	}

	b := &BtcdBackend{
		client:         client,
		network:        network,
		chainHeight:    uint32(height),
		addrRequests:   make(chan *deriver.Address, addrRequestsChanSize),
		addrResponses:  make(chan *AddrResponse, addrRequestsChanSize),
		txRequests:     make(chan string, 2*maxTxsPerAddr),
		txResponses:    make(chan *TxResponse, 2*maxTxsPerAddr),
		blockRequests:  make(chan uint32, 2*blockRequestChanSize),
		blockResponses: make(chan *BlockResponse, 2*blockRequestChanSize),

		blockHeightLookup:  make(map[string]int64),
		cachedTransactions: make(map[string]*TxResponse),
		doneCh:             make(chan bool),
	}

	// launch
	for i := 0; i < concurrency; i++ {
		go b.processRequests()
	}
	return b, nil
}

// AddrRequest schedules a request to the backend to lookup information related
// to the given address.
func (b *BtcdBackend) AddrRequest(addr *deriver.Address) {
	reporter.GetInstance().IncAddressesScheduled()
	reporter.GetInstance().Logf("scheduling address: %s", addr)
	b.addrRequests <- addr
}

// AddrResponses exposes a channel that allows to consume backend's responses to
// address requests created with AddrRequest()
func (b *BtcdBackend) AddrResponses() <-chan *AddrResponse {
	return b.addrResponses
}

// TxRequest schedules a request to the backend to lookup information related
// to the given transaction hash.
func (b *BtcdBackend) TxRequest(txHash string) {
	reporter.GetInstance().IncTxScheduled()
	reporter.GetInstance().Logf("scheduling tx: %s", txHash)
	b.txRequests <- txHash
}

// TxResponses exposes a channel that allows to consume backend's responses to
// address requests created with addrrequest().
// if an address has any transactions then they will be sent to this channel by the
// backend.
func (b *BtcdBackend) TxResponses() <-chan *TxResponse {
	return b.txResponses
}

func (b *BtcdBackend) BlockRequest(height uint32) {
	b.blockRequests <- height
}

func (b *BtcdBackend) BlockResponses() <-chan *BlockResponse {
	return b.blockResponses
}

// Finish informs the backend to stop doing its work.
func (b *BtcdBackend) Finish() {
	close(b.doneCh)
	b.client.Disconnect()
}

func (b *BtcdBackend) ChainHeight() uint32 {
	return b.chainHeight
}

func (b *BtcdBackend) processRequests() {
	for {
		select {
		case addr := <-b.addrRequests:
			err := b.processAddrRequest(addr)
			if err != nil {
				panic(fmt.Sprintf("processAddrRequest failed: %+v", err))
			}
		case tx := <-b.txRequests:
			err := b.processTxRequest(tx)
			if err != nil {
				panic(fmt.Sprintf("processTxRequest failed: %+v", err))
			}
		case block := <-b.blockRequests:
			err := b.processBlockRequest(block)
			if err != nil {
				panic(fmt.Sprintf("processBlockRequest failed: %+v", err))
			}
		case <-b.doneCh:
			break
		}
	}
}

func (b *BtcdBackend) processAddrRequest(address *deriver.Address) error {
	addr := address.Script()
	txs, err := b.client.SearchRawTransactionsVerbose(address.Address(), 0, maxTxsPerAddr+1, true, false, nil)
	if err != nil {
		if jerr, ok := err.(*btcjson.RPCError); ok {
			switch jerr.Code {
			case btcjson.ErrRPCInvalidAddressOrKey:
				// the address doesn't exist in the blockchain - either because it was not used
				// or given backend doesn't have a complete blockchain
				b.addrResponses <- &AddrResponse{
					Address:  address,
					TxHashes: []string{},
				}
				return nil
			}
		}
		return errors.Wrap(err, "could not fetch transactions for "+addr)
	}

	if len(txs) > maxTxsPerAddr {
		return fmt.Errorf("address %s has more than max allowed transactions of %d", addr, maxTxsPerAddr)
	}

	txHashes := make([]string, 0, len(txs))
	for _, tx := range txs {
		txHashes = append(txHashes, tx.Txid)
	}

	b.cacheTxs(txs)

	b.addrResponses <- &AddrResponse{
		Address:  address,
		TxHashes: txHashes,
	}

	return nil
}

func (b *BtcdBackend) processTxRequest(txHash string) error {
	b.transactionsMu.Lock()
	tx, exists := b.cachedTransactions[txHash]
	b.transactionsMu.Unlock()

	if exists {
		b.txResponses <- tx

		return nil
	}

	hash, err := chainhash.NewHashFromStr(txHash)
	if err != nil {
		return err
	}
	txResp, err := b.client.GetRawTransactionVerbose(hash)
	if err != nil {
		if jerr, ok := err.(*btcjson.RPCError); ok {
			switch jerr.Code {
			case btcjson.ErrRPCInvalidAddressOrKey:
				return errors.Wrap(err, "blockchain doesn't have transaction "+txHash)
			}
		}
		return errors.Wrap(err, "could not fetch transaction "+txHash)
	}
	height, err := b.getBlockHeight(txResp.BlockHash)
	if err != nil {
		return err
	}

	b.txResponses <- &TxResponse{
		Hash:   txHash,
		Height: height,
		Hex:    txResp.Hex,
	}
	return nil
}

func (b *BtcdBackend) processBlockRequest(height uint32) error {
	hash, err := b.client.GetBlockHash(int64(height))
	if err != nil {
		if jerr, ok := err.(*btcjson.RPCError); ok {
			switch jerr.Code {
			case btcjson.ErrRPCInvalidAddressOrKey:
				return errors.Wrap(err, fmt.Sprintf("blockchain doesn't have block %d", height))
			}
		}
		return errors.Wrap(err, fmt.Sprintf("could not fetch block %d", height))
	}

	header, err := b.client.GetBlockHeader(hash)
	if err != nil {
		if jerr, ok := err.(*btcjson.RPCError); ok {
			switch jerr.Code {
			case btcjson.ErrRPCInvalidAddressOrKey:
				return errors.Wrap(err, fmt.Sprintf("blockchain doesn't have block %d", height))
			}
		}
		return errors.Wrap(err, fmt.Sprintf("could not fetch block %d", height))
	}

	b.blockResponses <- &BlockResponse{
		Height:    height,
		Timestamp: header.Timestamp,
	}
	return nil
}

func (b *BtcdBackend) cacheTxs(txs []*btcjson.SearchRawTransactionsResult) {
	for _, tx := range txs {
		b.transactionsMu.Lock()
		_, exists := b.cachedTransactions[tx.Txid]
		b.transactionsMu.Unlock()

		if exists {
			return
		}

		height, err := b.getBlockHeight(tx.BlockHash)
		if err != nil {
			panic(fmt.Sprintf("error getting block height for hash %s: %s", tx.BlockHash, err.Error()))
		}

		b.transactionsMu.Lock()
		b.cachedTransactions[tx.Txid] = &TxResponse{
			Hash:   tx.Txid,
			Height: height,
			Hex:    tx.Hex,
		}
		b.transactionsMu.Unlock()
	}
}

// getBlockHeight returns a block height for a given block hash or returns an error
func (b *BtcdBackend) getBlockHeight(hash string) (int64, error) {
	b.blockHeightMu.Lock()
	height, exists := b.blockHeightLookup[hash]
	b.blockHeightMu.Unlock()
	if exists {
		return height, nil
	}

	h, err := chainhash.NewHashFromStr(hash)
	if err != nil {
		return -1, err
	}
	resp, err := b.client.GetBlockVerbose(h)
	if err != nil {
		if jerr, ok := err.(*btcjson.RPCError); ok {
			switch jerr.Code {
			case btcjson.ErrRPCInvalidAddressOrKey:
				return -1, errors.Wrap(err, "blockchain doesn't have block "+hash)
			}
		}
		return -1, errors.Wrap(err, "could not fetch block "+hash)
	}

	b.blockHeightMu.Lock()
	b.blockHeightLookup[hash] = resp.Height
	b.blockHeightMu.Unlock()

	return resp.Height, nil
}
