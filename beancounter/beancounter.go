package beancounter

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/olekukonko/tablewriter"
	"github.com/square/beancounter/backend"
	"github.com/square/beancounter/deriver"
	. "github.com/square/beancounter/utils"
)

// Beancounter is the main struct that can count the balance for a given wallet.
// The main elements of Beancounter are backend and deriver. Deriver is used to
// derive new addresses for a given config, and backend checks the balances and
// transactions for each address.
// Beancounter takes balances and transaction histories and tally them up.
type Beancounter struct {
	account string
	net     Network
	xpubs   []string

	totalBalance uint64
	transactions []transaction
	balances     []addrBalance
	// NOTE: maybe track unconfirmed balance and fees. We might want to also track each transaction's amount and whether
	// it's a credit or debit.

	backend   backend.Backend
	deriver   *deriver.AddressDeriver
	lookahead uint32
	start     uint32
	sleep     time.Duration
	wg        sync.WaitGroup

	countMu       sync.Mutex // protects lastAddresses, derivedCount and checkedCount
	lastAddresses [2]uint32
	derivedCount  uint32
	checkedCount  uint32

	checkerCh  chan *deriver.Address
	receivedCh <-chan *backend.Response
}

// NewCounter instantiates the Beancounter
// TODO: find a better way to pass options to the NewCounter. Maybe thru a config or functional option params?
func NewCounter(backend backend.Backend, drvr *deriver.AddressDeriver, lookahead, start uint32, sleep time.Duration) *Beancounter {
	b := &Beancounter{
		backend:       backend,
		deriver:       drvr,
		lookahead:     lookahead,
		start:         start,
		sleep:         sleep,
		lastAddresses: [2]uint32{start + lookahead, start + lookahead},
		checkerCh:     make(chan *deriver.Address, 100),
	}
	b.receivedCh = b.backend.Subscribe(b.checkerCh)
	return b
}

// Count is Beancounters main function that derives the addresses and feeds them
// into the backend.
// The address derivation, address checking for balance and transactions, and the final
// tally are all happening asynchronuously
// NOTE: maybe add a reset step so that Beancounter struct can be reused
//       or Count can be called multiple time?
//       The other option is for Count to return a result struct instead of mutating
//       Beancounter struct.
func (b *Beancounter) Count() {
	b.wg.Add(1)
	go b.sendWork()
	go b.receiveWork()
	b.wg.Wait()
}

// sendWork starts the send loop that derives new addresses and sends them to a
// a backend.
// Addresses are derived in batches (up to a `lookahead` index) and the range can
// be extended if a transaction for a given address is found. E.g.:
// only addresses 0-99 are supposed to be checked, but there was a transaction at
// index 43, so now the last address to be checked should be 142.
func (b *Beancounter) sendWork() {
	indexes := []uint32{b.start, b.start}
	for {
		for _, change := range []uint32{0, 1} {
			lastAddr := b.getLastAddress(change)
			for i := indexes[change]; i < lastAddr; i++ {
				//go func(change, i uint32) {
				// schedule work for backend
				b.countMu.Lock()
				b.derivedCount++
				b.countMu.Unlock()
				b.checkerCh <- b.deriver.Derive(change, i)
				//}(change, i)

				indexes[change] = i
			}
			indexes[change]++
		}
		// apparently no more work for us, so we can sleep a bit
		time.Sleep(time.Millisecond * 100)
	}
}

// getLastAddress synchronizes access to lastAddresses array
func (b *Beancounter) getLastAddress(change uint32) uint32 {
	b.countMu.Lock()
	defer b.countMu.Unlock()

	return b.lastAddresses[change]
}

// receiveWork starts a receive work loop and then waits for others parts of
// Beancounter to finish
func (b *Beancounter) receiveWork() {
	b.receiveWorkLoop()
	b.wg.Done()
}

// receiveWorkLoop encapsulates the receive loop that continues to processing
// responses until complete() returns true.
func (b *Beancounter) receiveWorkLoop() {
	for {
		select {
		case resp := <-b.receivedCh:
			b.countMu.Lock()
			b.checkedCount++
			b.countMu.Unlock()

			if resp != nil && resp.Error == nil {
				b.addBalance(resp)

				fmt.Printf("Checking balance for %s %s ... ", resp.Address.Path(), resp.Address.String())
				if resp.HasTransactions() {
					fmt.Printf("%d %d\n", resp.Balance, b.totalBalance)
				} else {
					fmt.Printf("∅\n")
				}
			} else if resp != nil {
				log.Printf("[RESP ERROR]: %s:  %s\n", resp.Address.String(), resp.Error.Error())
			} else {
				log.Printf("resp is nil\n")
			}
		default:
			// no work check if we're done
			if b.complete() {
				return
			}

			// TODO: the select should probably be removed so that the receive is blocking. We will then not need the sleep
			// to avoid looping around b.complete() while waiting for network responses.
			time.Sleep(100 * time.Millisecond)
		}
	}
}

// complete checks if all addresses have been derived and checked.
// Since most of the work happens asynchronuously, there needs to be a termination
// condition.
func (b *Beancounter) complete() bool {
	b.countMu.Lock()
	defer b.countMu.Unlock()

	// We are done when the right number of addresses were scheduled, fetched and processed
	indexes := (b.lastAddresses[0] - b.start) + (b.lastAddresses[1] - b.start)
	return b.derivedCount == indexes && b.checkedCount == indexes
}

type addrBalance struct {
	path    string
	addr    string
	balance uint64
}

func (b *addrBalance) toCSV() string {
	return b.path + "," + b.addr + "," + strconv.FormatUint(b.balance, 10)
}

func (b *addrBalance) toArray() []string {
	return []string{b.path, b.addr, strconv.FormatUint(b.balance, 10)}
}

type transaction struct {
	path string
	addr string
	hash string
}

func (t *transaction) toCSV() string {
	return t.path + "," + t.addr + "," + t.hash
}

func (t *transaction) toArray() []string {
	return []string{t.path, t.addr, t.hash}
}

// WriteTransactions prints to STDOUT every transaction for each address scanned.
// TODO: Move it to some output formatter/writer. Beancounter shouldn't care what
//       happens with data after it has been computed.
func (b *Beancounter) WriteTransactions() {
	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"Path", "Address", "Transaction Hash"})

	for _, b := range b.transactions {
		table.Append(b.toArray())
	}
	table.Render()
	fmt.Printf("\n")
}

// WriteSummary prints a summary table with total balance and the range of
// addresses scanned to the STDOUT.
// TODO: Move it to some output formatter/writer. Beancounter shouldn't care what
//       happens with data after it has been computed.
func (b *Beancounter) WriteSummary() {
	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"Total Balance", "Last Receive Index", "Last Change Index", "Report Time"})

	table.Append([]string{
		strconv.FormatUint(b.totalBalance, 10),
		strconv.FormatUint(uint64(b.lastAddresses[0]-1), 10),
		strconv.FormatUint(uint64(b.lastAddresses[1]-1), 10),
		time.Now().Format(time.RFC822)})
	table.Render()
	fmt.Printf("\n")
}

// WriteBalances prints to STDOUT every non-zero balance for each address scanned.
// TODO: Move it to some output formatter/writer. Beancounter shouldn't care what
//       happens with data after it has been computed.
func (b *Beancounter) WriteBalances() {
	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"Path", "Address", "Balance"})

	for _, b := range b.balances {
		table.Append(b.toArray())
	}
	table.Render()
	fmt.Printf("\n")
}

// addBalance update the total balance and list of transactions for each Response
// from the backend.
func (b *Beancounter) addBalance(r *backend.Response) {
	b.totalBalance += r.Balance
	if r.HasTransactions() {
		// move lookahead since we found a transaction
		b.countMu.Lock()
		b.lastAddresses[r.Address.Change()] = Max(b.lastAddresses[r.Address.Change()], r.Address.Index()+b.lookahead)
		b.countMu.Unlock()
		b.balances = append(b.balances, addrBalance{path: r.Address.Path(), addr: r.Address.String(), balance: r.Balance})

		for _, tx := range r.Transactions {
			b.transactions = append(b.transactions, transaction{path: r.Address.Path(), addr: r.Address.String(), hash: tx.Hash})
		}
	}
}
