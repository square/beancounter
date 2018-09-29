package beancounter

import (
	"fmt"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/olekukonko/tablewriter"
	"github.com/square/beancounter/balance"
	"github.com/square/beancounter/deriver"
	. "github.com/square/beancounter/utils"
)

type Beancounter struct {
	account      string
	net          Network
	xpubs        []string
	totalBalance uint64
	transactions []transaction
	balances     []addrBalance
	// NOTE: maybe track unconfirmed balance and fees. We might want to also track each transaction's amount and whether
	// it's a credit or debit.

	checker   balance.Checker
	deriver   *deriver.AddressDeriver
	lookahead uint32
	sleep     time.Duration
	wg        sync.WaitGroup

	countMu       sync.Mutex
	lastAddresses [2]uint32
	derivedCount  uint32
	checkedCount  uint32

	checkerCh  chan *deriver.Address
	receivedCh <-chan *balance.Response
}

func NewBalance(checker balance.Checker, drvr *deriver.AddressDeriver, lookahead uint32, sleep time.Duration) *Beancounter {
	b := &Beancounter{
		checker:       checker,
		deriver:       drvr,
		lookahead:     lookahead,
		sleep:         sleep,
		lastAddresses: [2]uint32{lookahead, lookahead},
		checkerCh:     make(chan *deriver.Address, 100),
	}
	b.receivedCh = b.checker.Subscribe(b.checkerCh)
	return b
}

func (b *Beancounter) Count() {
	b.wg.Add(1)
	go b.sendWork()
	go b.receiveWork()
	b.wg.Wait()
}

func (b *Beancounter) sendWork() {
	indexes := []uint32{0, 0}
	for {
		for _, change := range []uint32{0, 1} {
			//fmt.Println("[sendWork] ABOUT TO START WORK")
			lastAddr := b.getLastAddress(change)
			for i := indexes[change]; i < lastAddr; i++ {
				//fmt.Println("[sendWork] BEFORE THE LOCK")
				b.countMu.Lock()
				//fmt.Println("[sendWork] INSIDE THE LOCK")
				b.derivedCount++
				b.countMu.Unlock()
				//fmt.Println("[sendWork] AFTER THE LOCK")

				// schedule work for checker
				b.checkerCh <- b.deriver.Derive(change, i)

				indexes[change] = i
			}
			indexes[change]++
		}
		// apparently no more work for us, so we can sleep a bit
		time.Sleep(time.Millisecond * 100)
	}
}

func (b *Beancounter) getLastAddress(change uint32) uint32 {
	//fmt.Println("[getLastAddress] BEFORE THE LOCK")
	b.countMu.Lock()
	//fmt.Println("[getLastAddress] INSIDE THE LOCK")
	v := b.lastAddresses[change]
	b.countMu.Unlock()
	//fmt.Println("[getLastAddress] AFTER THE LOCK")

	return v
}

func (b *Beancounter) receiveWork() {
	b.receiveWorkLoop()
	//fmt.Println("DONE and waiting for workgroup")
	b.wg.Done()
}

func (b *Beancounter) receiveWorkLoop() {
	for {
		select {
		case resp := <-b.receivedCh:
			//fmt.Println("[receiveWorkLoop] BEFORE THE LOCK")
			b.countMu.Lock()
			//fmt.Println("[receiveWorkLoop] INSIDE THE LOCK")
			b.checkedCount++
			b.countMu.Unlock()
			//fmt.Println("[receiveWorkLoop] AFTER THE LOCK")
			if resp != nil {
				b.AddBalance(resp)

				fmt.Printf("Checking balance for %s %s ... ", resp.Address.Path(), resp.Address.String())
				if resp.HasTransactions() {
					fmt.Printf("%d %d\n", resp.Balance, b.totalBalance)
				} else {
					fmt.Printf("∅\n")
				}
			} else {
				//fmt.Println("SOMETHING IS OFF")
			}
		default:
			// no work check if we're done
			if b.Complete() {
				return
			}
		}
	}
}

func (b *Beancounter) Complete() bool {
	//fmt.Println("[Complete] BEFORE THE LOCK")
	b.countMu.Lock()
	//fmt.Println("[Complete] INSIDE THE LOCK")

	indexes := b.lastAddresses[0] + b.lastAddresses[1]
	////fmt.Printf("Derived: %d, checked: %d, lastAddresses[0]: %d, lastAddresses[1]: %d\n", b.derivedCount, b.checkedCount, b.lastAddresses[0], b.lastAddresses[1])
	check := b.derivedCount == indexes && b.checkedCount == indexes
	b.countMu.Unlock()
	//fmt.Println("[Complete] AFTER THE LOCK")
	return check
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

func (b *Beancounter) WriteTransactions() {
	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"Path", "Address", "Transaction Hash"})

	for _, b := range b.transactions {
		table.Append(b.toArray())
	}
	table.Render()
	fmt.Printf("\n")
}

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

func (b *Beancounter) WriteBalances() {
	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"Path", "Address", "Balance"})

	for _, b := range b.balances {
		table.Append(b.toArray())
	}
	table.Render()
	fmt.Printf("\n")
}

func (b *Beancounter) AddBalance(r *balance.Response) {
	b.totalBalance += r.Balance
	if r.HasTransactions() {
		// move lookahead since we found a transaction
		b.countMu.Lock()
		b.lastAddresses[r.Address.Change()] = r.Address.Index() + b.lookahead
		b.countMu.Unlock()
		b.balances = append(b.balances, addrBalance{path: r.Address.Path(), addr: r.Address.String(), balance: r.Balance})

		for _, tx := range r.Transactions {
			b.transactions = append(b.transactions, transaction{path: r.Address.Path(), addr: r.Address.String(), hash: tx.Hash})
		}
	}
}
