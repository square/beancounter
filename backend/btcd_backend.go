package backend

import (
	"fmt"
	"math"
	"sync"

	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/rpcclient"
	"github.com/btcsuite/btcutil"
	"github.com/pkg/errors"
	"github.com/square/beancounter/deriver"
)

// BtcdBackend wraps Btcd node and its API to provide a simple
// balance and transaction history information for a given address.
// BtcdBackend implements Backend interface.
type BtcdBackend struct {
	client            *rpcclient.Client
	net               *chaincfg.Params
	maxBlockHeight    int64
	blockHeightLookup map[string]uint64
}

const (
	// min number of confirmations required
	// any blocks with lower confirmation numbers will be ignored
	minConfirmations = 6
	// For now assume that there cannot be more than maxTxsPerAddr.
	// Ideally, if maxTxsPerAddr is reached then we should paginate and retrieve
	// all the transactions.
	maxTxsPerAddr = 1000
)

// NewBtcdBackend returns a new BtcdBackend structs or errors.
// BtcdBackend takes into account maxBlockHeight and ignores any transactions that belong to higher blocks.
// If 0 is passed, then the block chain is queried for max block height and minConfirmations is subtracted
// (to avoid querying blocks that might potentially be orphaned).
//
// NOTE: BtcdBackend is assumed to be connecting to a personal node, hence it disables TLS for now
func NewBtcdBackend(maxBlockHeight int64, hostPort, user, pass string, net *chaincfg.Params) (*BtcdBackend, error) {
	connCfg := &rpcclient.ConnConfig{
		Host:         hostPort,
		User:         user,
		Pass:         pass,
		HTTPPostMode: true, // Bitcoin core only supports HTTP POST mode
		DisableTLS:   true, // Since we're assuming a personal bitcoin node for now, skip TLS
	}
	client, err := rpcclient.New(connCfg, nil)
	if err != nil {
		return nil, errors.Wrap(err, "could not create a Btcd RPC client")
	}

	actualMaxHeight, err := client.GetBlockCount()
	maxAllowedHeight := actualMaxHeight - minConfirmations
	if err != nil {
		return nil, errors.Wrap(err, "could not connect to the Btcd server")
	}
	if maxBlockHeight == 0 {
		maxBlockHeight = maxAllowedHeight
	}
	if maxAllowedHeight < maxBlockHeight {
		return nil, fmt.Errorf("wanted max block height: %d, block chain has %d (with # confirmations of %d)", maxBlockHeight, maxAllowedHeight, minConfirmations)
	}

	bc := &BtcdBackend{client: client, net: net, maxBlockHeight: maxBlockHeight}
	return bc, nil
}

// Fetch queries connected node for address balance and transaction history and returns Response.
func (bc *BtcdBackend) Fetch(addr string) *Response {
	address, err := btcutil.DecodeAddress(addr, bc.net)
	if err != nil {
		return errResp(errors.Wrap(err, "could not decode address for "+addr))
	}

	resp, err := bc.client.SearchRawTransactionsVerbose(address, 0, maxTxsPerAddr+1, true, false, nil)
	if err != nil {
		return errResp(errors.Wrap(err, "could not fetch transactions for "+addr))
	}

	if len(resp) > maxTxsPerAddr {
		return errResp(fmt.Errorf("address %s has more than max allowed transactions of %d", addr, maxTxsPerAddr))
	}

	balance := uint64(0)
	rcvdTxs := make(map[string]uint64)

	var txs []Transaction
	for _, tx := range resp {
		txs = append(txs, Transaction{Hash: tx.Txid})
		height, err := bc.getBlockHeight(tx.BlockHash)
		if err != nil {
			return errResp(errors.Wrap(err, "error getting block height for hash %s"))
		}

		if height > bc.maxBlockHeight {
			continue
		}

		txBalance := uint64(0)
		for _, vout := range tx.Vout {
			if containsAddr(vout.ScriptPubKey.Addresses, addr) {
				txBalance += uint64(vout.Value * math.Pow10(8))
				key := fmt.Sprintf("%s:%d", tx.Txid, vout.N)
				rcvdTxs[key] = txBalance
			}
		}
		balance += txBalance
	}

	for _, tx := range resp {
		for _, vin := range tx.Vin {
			key := fmt.Sprintf("%s:%d", vin.Txid, vin.Vout)
			if bal, exists := rcvdTxs[key]; exists {
				balance -= bal
			}
		}
	}

	return &Response{Balance: balance, Transactions: txs}
}

// getBlockHeight returns a block height for a given block hash or returns an error
func (bc *BtcdBackend) getBlockHeight(hash string) (int64, error) {
	h, err := chainhash.NewHashFromStr(hash)
	if err != nil {
		return -1, err
	}
	resp, err := bc.client.GetBlockVerbose(h)
	if err != nil {
		return -1, err
	}

	return resp.Height, nil
}

// containsAddr returns true if an array of strings contains string s
func containsAddr(arr []string, s string) bool {
	for _, v := range arr {
		if v == s {
			return true
		}
	}
	return false
}

// Subscribe provides a bidirectional streaming of checks for addresses.
// It takes a channel of addresses and returns a channel of responses, to which
// it is writing asynchronuously.
// TODO: Looks like Subscribe implementation is separate from implementation
//       details of each backend and therefore could be abstracted into a separate
//       struct/interface (e.g. there could be a StreamingBackend interface that
//       implements Subcribe method).
func (bc *BtcdBackend) Subscribe(addrCh <-chan *deriver.Address) <-chan *Response {
	respCh := make(chan *Response, 100)
	go func() {
		var wg sync.WaitGroup
		for addr := range addrCh {
			wg.Add(1)
			// do not block on each Fetch API call
			go bc.processFetch(addr, respCh, &wg)
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
func (bc *BtcdBackend) processFetch(addr *deriver.Address, out chan<- *Response, wg *sync.WaitGroup) {
	resp := bc.Fetch(addr.String())
	resp.Address = addr
	out <- resp
	wg.Done()
}

func errResp(err error) *Response {
	return &Response{Error: err}
}
