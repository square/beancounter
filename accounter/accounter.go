package accounter

import (
	"encoding/hex"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/btcsuite/btcutil"
	"github.com/square/beancounter/reporter"

	"github.com/square/beancounter/backend"
	"github.com/square/beancounter/deriver"
	. "github.com/square/beancounter/utils"
)

// Accounter is the main struct that can tally the balance for a given wallet.
// The main elements of Accounter are backend and deriver. Deriver is used to
// derive new addresses for a given config, and backend fetches transactions for each address.
//
// Note:
// - We don't track fees. I.e. we don't answer the question: how much have we spent in fees. It
//   shouldn't be hard to answer that question.
type Accounter struct {
	account     string
	net         Network
	xpubs       []string
	blockHeight uint32 // height at which we want to compute the balance

	addresses    map[string]address     // map of address script => (Address, txHashes)
	transactions map[string]transaction // map of txhash => transaction

	backend   backend.Backend
	deriver   *deriver.AddressDeriver
	lookahead uint32

	countMu            sync.Mutex // protects lastAddresses, derivedAddrCount and processedAddrCount
	lastAddresses      [2]uint32
	derivedAddrCount   uint32
	processedAddrCount uint32
	seenTxCount        uint32
	processedTxCount   uint32

	addrResponses <-chan *backend.AddrResponse
	txResponses   <-chan *backend.TxResponse
}

type address struct {
	path     *deriver.Address
	txHashes []string
}

type transaction struct {
	height int64
	hex    string
	vin    []vin
	vout   []vout
}

type vin struct {
	prevHash string // hash of previous transaction
	index    uint32 // offset. 0-indexed.
}

type vout struct {
	value   int64 // in Satoshi. We use signed int64 so we don't have to worry about underflow.
	address string
	ours    bool
	spentBy *string // txhash of spending transaction; nil for unspent transactions.
}

// New instantiates a new Accounter.
// TODO: find a better way to pass options to the NewCounter. Maybe thru a config or functional option params?
func New(b backend.Backend, addressDeriver *deriver.AddressDeriver, lookahead uint32, blockHeight uint32) *Accounter {
	a := &Accounter{
		blockHeight:   blockHeight,
		backend:       b,
		deriver:       addressDeriver,
		lookahead:     lookahead,
		lastAddresses: [2]uint32{lookahead, lookahead},
	}
	a.addresses = make(map[string]address)
	a.transactions = make(map[string]transaction)
	a.addrResponses = b.AddrResponses()
	a.txResponses = b.TxResponses()
	return a
}

func (a *Accounter) ComputeBalance() uint64 {
	// Fetch all the transactions
	a.fetchTransactions()

	// Process the data
	a.processTransactions()

	// Compute the balance
	return a.balance()
}

// Fetch all the transactions related to our wallet. We tally the balance after we have fetched
// all the transactions so that we don't need to worry about receiving transactions out-of-order.
func (a *Accounter) fetchTransactions() {
	// send work runs forever
	go a.sendWork()

	a.recvWork()

	reporter.GetInstance().Log("done fetching addresses; waiting to finish...")
	a.backend.Finish()
	reporter.GetInstance().Log("done fetching transactions")
}

func (a *Accounter) processTransactions() {
	for hash, tx := range a.transactions {
		// remove transactions which are too recent
		if tx.height > int64(a.blockHeight) {
			reporter.GetInstance().Logf("transaction %s has height %d > BLOCK HEIGHT (%d)", hash, tx.height, a.blockHeight)
			delete(a.transactions, hash)
		}
		// remove transactions which haven't been mined
		if tx.height <= 0 {
			reporter.GetInstance().Logf("transaction %s has not been mined, yet (height=%d)", hash, tx.height)
			delete(a.transactions, hash)
		}
	}
	reporter.GetInstance().SetTxAfterFilter(int32(len(a.transactions)))
	reporter.GetInstance().Log("done filtering")

	// TODO: we could check that scheduled == fetched in the metrics we track in reporter.

	// parse the transaction hex
	for hash, tx := range a.transactions {
		b, err := hex.DecodeString(tx.hex)
		if err != nil {
			fmt.Printf("failed to unhex transaction %s: %s", hash, tx.hex)
		}
		parsedTx, err := btcutil.NewTxFromBytes(b)
		if err != nil {
			fmt.Printf("failed to parse transaction %s: %s", hash, tx.hex)
			continue
		}
		for _, txin := range parsedTx.MsgTx().TxIn {
			tx.vin = append(tx.vin, vin{
				prevHash: txin.PreviousOutPoint.Hash.String(),
				index:    txin.PreviousOutPoint.Index,
			})
		}

		for _, txout := range parsedTx.MsgTx().TxOut {
			addr := hex.EncodeToString(txout.PkScript)
			_, exists := a.addresses[addr]
			tx.vout = append(tx.vout, vout{
				value:   txout.Value,
				address: addr,
				ours:    exists,
				spentBy: nil,
			})
		}

		// ugly...
		a.transactions[hash] = tx
	}
}

func (a *Accounter) balance() uint64 {
	balance := int64(0)

	// TODO: we could check that every transaction either has an input which belongs to us or an
	// output. Otherwise, it would not have appeared in the list. It's also a good check, given
	// that we filter some transactions out.

	// compute all credits
	for _, tx := range a.transactions {
		for _, txout := range tx.vout {
			if txout.ours {
				balance += txout.value
			}
		}
	}

	// TODO: log a warning if an address is being used multiple times. Ideally, a given address
	// should only have one incoming and one outgoing transaction.

	// TODO: log a warning if a receive address is getting change.

	// TODO: log a warning if a change address is receiving funds from an address we don't own.

	// compute all debits
	for hash, tx := range a.transactions {
		for _, txin := range tx.vin {
			prev, exists := a.transactions[txin.prevHash]
			if !exists {
				continue
			}
			if int(txin.index) >= len(prev.vout) {
				panic("prev index > vouts")
			}
			if prev.vout[txin.index].ours {
				balance -= prev.vout[txin.index].value
				if prev.vout[txin.index].spentBy != nil {
					// sanity check: an output can only be spent by one transaction.
					log.Panicf("%s and %s, both spending %s", hash, *prev.vout[txin.index].spentBy, txin.prevHash)
				}
				prev.vout[txin.index].spentBy = &hash
			}
		}
	}

	if balance < 0 {
		panic("balance is negative")
	}
	return uint64(balance)
}

// sendWork starts the send loop that derives new addresses and sends them to a
// a backend.
// Addresses are derived in batches (up to a `lookahead` index) and the range can
// be extended if a transaction for a given address is found. E.g.:
// only addresses 0-99 are initially checked, but there was a transaction at
// index 43, so now all addresses up to 142 are checked.
func (a *Accounter) sendWork() {
	indexes := []uint32{0, 0}
	for {
		for _, change := range []uint32{0, 1} {
			lastAddr := a.getLastAddress(change)
			for indexes[change] < lastAddr {
				// increment the number of addresses which have been derived
				addr := a.deriver.Derive(change, indexes[change])
				a.countMu.Lock()
				a.derivedAddrCount++
				a.countMu.Unlock()
				a.backend.AddrRequest(addr)
				indexes[change]++
			}
		}
		// apparently no more work for us, so we can sleep a bit
		time.Sleep(time.Millisecond * 100)
	}
}

func (a *Accounter) recvWork() {
	addrResponses := a.addrResponses
	txResponses := a.txResponses
	for {
		select {
		case resp, ok := <-addrResponses:
			// channel is closed now, so ignore this case by blocking forever
			if !ok {
				addrResponses = nil
				continue
			}
			reporter.GetInstance().IncAddressesFetched()

			a.countMu.Lock()
			a.processedAddrCount++
			a.countMu.Unlock()

			a.addresses[resp.Address.Script()] = address{
				path:     resp.Address,
				txHashes: resp.TxHashes,
			}

			a.countMu.Lock()
			for _, txHash := range resp.TxHashes {
				if _, exists := a.transactions[txHash]; !exists {
					a.backend.TxRequest(txHash)
					a.seenTxCount++
				}
			}
			a.countMu.Unlock()

			reporter.GetInstance().Logf("address %s has %d transactions", resp.Address, len(resp.TxHashes))

			if resp.HasTransactions() {
				a.countMu.Lock()
				a.lastAddresses[resp.Address.Change()] = Max(a.lastAddresses[resp.Address.Change()], resp.Address.Index()+a.lookahead)
				a.countMu.Unlock()
			}
		case resp, ok := <-txResponses:
			// channel is closed now, so ignore this case by blocking forever
			if !ok {
				txResponses = nil
				continue
			}

			reporter.GetInstance().IncTxFetched()

			a.countMu.Lock()
			a.processedTxCount++
			a.countMu.Unlock()

			tx := transaction{
				height: resp.Height,
				hex:    resp.Hex,
				vin:    []vin{},
				vout:   []vout{},
			}
			a.transactions[resp.Hash] = tx
		case <-time.Tick(1 * time.Second):
			if a.complete() {
				return
			}
		}
	}
}

// getLastAddress synchronizes access to lastAddresses array
func (a *Accounter) getLastAddress(change uint32) uint32 {
	a.countMu.Lock()
	defer a.countMu.Unlock()

	return a.lastAddresses[change]
}

// complete checks if all addresses have been derived and checked.
// Since most of the work happens asynchronuously, there needs to be a termination
// condition.
func (a *Accounter) complete() bool {
	a.countMu.Lock()
	defer a.countMu.Unlock()

	// We are done when the right number of addresses were scheduled, fetched and processed
	// *and* all the transactions that were seen have been scheduled, fetched and processed.
	indexes := a.lastAddresses[0] + a.lastAddresses[1]
	addrsDone := a.derivedAddrCount == indexes && a.processedAddrCount == indexes
	txsDone := a.seenTxCount == a.processedTxCount

	return addrsDone && txsDone
}
