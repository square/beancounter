package balance

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

type BtcdChecker struct {
	client            *rpcclient.Client
	net               *chaincfg.Params
	maxBlockHeight    int64
	blockHeightLookup map[string]uint64
}

// NewBtcdChecker returns a new BtcdChecker structs or errors.
func NewBtcdChecker(maxBlockHeight int64, hostPort, user, pass string, net *chaincfg.Params) (*BtcdChecker, error) {
	connCfg := &rpcclient.ConnConfig{
		Host:         hostPort,
		User:         user,
		Pass:         pass,
		HTTPPostMode: true, // Bitcoin core only supports HTTP POST mode
		DisableTLS:   true, // Bitcoin core does not provide TLS by default
	}
	client, err := rpcclient.New(connCfg, nil)
	if err != nil {
		return nil, err
	}

	actualMaxHeight, err := client.GetBlockCount()
	if err != nil {
		return nil, err
	}
	if maxBlockHeight == 0 {
		maxBlockHeight = actualMaxHeight
	}
	if actualMaxHeight < maxBlockHeight {
		return nil, fmt.Errorf("wanted max block height: %d, block chain has %d", maxBlockHeight, actualMaxHeight)
	}

	bc := &BtcdChecker{client: client, net: net, maxBlockHeight: maxBlockHeight}
	return bc, nil
}

func (bc *BtcdChecker) Fetch(addr string) *Response {
	address, err := btcutil.DecodeAddress(addr, bc.net)
	if err != nil {
		return &Response{Error: err}
	}

	resp, err := bc.client.SearchRawTransactionsVerbose(address, 0, 1000, true, false, nil)
	if err != nil {
		return &Response{Error: err}
	}

	balance := uint64(0)
	rcvdTxs := make(map[string]uint64)

	var txs []Transaction
	fmt.Printf("\t %s: Found # transactions %d\n", addr, len(resp))
	for _, tx := range resp {
		txs = append(txs, Transaction{Hash: tx.Txid})
		height, err := bc.getBlockHeight(tx.BlockHash)
		if err != nil {
			return &Response{Error: errors.Wrap(err, "error getting block height for hash=%s")}
		}

		if height > bc.maxBlockHeight {
			continue
		}

		fmt.Printf("\t %s: Found transaction %s, block: %d\n", addr, tx.Txid, height)
		txBalance := uint64(0)
		for _, vout := range tx.Vout {
			if containsAddr(vout.ScriptPubKey.Addresses, addr) {
				txBalance += uint64(vout.Value * math.Pow10(8))
				key := fmt.Sprintf("%s:%d", tx.Txid, vout.N)
				rcvdTxs[key] = txBalance
				fmt.Printf("\t\t value: %d\n", txBalance)
			}
		}
		balance += txBalance
	}

	for _, tx := range resp {
		for _, vin := range tx.Vin {
			key := fmt.Sprintf("%s:%d", vin.Txid, vin.Vout)
			if bal, exists := rcvdTxs[key]; exists {
				fmt.Printf("\t %s: transaction %s was send back to the adddr. Subtracting!\n", addr, tx.Txid)
				balance -= bal
			}
		}
	}

	return &Response{Balance: balance, Transactions: txs}
}

func (bc *BtcdChecker) getBlockHeight(hash string) (int64, error) {
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
//       details of each checker and therefore could be abstracted into a separate
//       struct/interface (e.g. there could be a StreamingChecker interface that
//       implements Subcribe method).
func (bc *BtcdChecker) Subscribe(addrCh <-chan *deriver.Address) <-chan *Response {
	respCh := make(chan *Response, 100)
	go func() {
		var wg sync.WaitGroup
		for addr := range addrCh {
			wg.Add(1)
			// do not block on each Fetch API call
			bc.processFetch(addr, respCh, &wg)
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
func (bc *BtcdChecker) processFetch(addr *deriver.Address, out chan<- *Response, wg *sync.WaitGroup) {
	resp := bc.Fetch(addr.String())
	resp.Address = addr
	out <- resp
	wg.Done()
}
