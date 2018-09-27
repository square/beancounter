package main

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"gopkg.in/alecthomas/kingpin.v2"

	"github.com/square/beancounter/balance"
	"github.com/square/beancounter/deriver"
	. "github.com/square/beancounter/utils"

	"github.com/olekukonko/tablewriter"
)

var (
	m         = kingpin.Flag("m", "number of signatures (quorum)").Short('m').Required().Int()
	n         = kingpin.Flag("n", "number of public keys").Short('n').Required().Int()
	account   = kingpin.Flag("account", "account number").Required().Uint32()
	network   = kingpin.Flag("network", "'mainnet' or 'testnet'").Default("mainnet").Enum("mainnet", "testnet")
	lookahead = kingpin.Flag("lookahead", "lookahead size").Default("100").Uint32()
	sleep     = kingpin.Flag("sleep", "sleep between requests to avoid API rate-limit").Default("1s").Duration()
	addr      = kingpin.Flag("addr", "Electrum server").PlaceHolder("HOST:PORT").TCP()
	debug     = kingpin.Flag("debug", "debug output").Default("false").Bool()
)

const (
	mainnetDefaultServer = "electrum.petrkr.net:50002"
	testnetDefaultServer = "electrum_testnet_unlimited.criptolayer.net:50102"
)

type totalBalance struct {
	account       string
	net           Network
	xpubs         []string
	lastAddresses [2]uint32
	totalBalance  uint64
	transactions  []transaction
	balances      []addrBalance
	// NOTE: maybe track unconfirmed balance and fees. We might want to also track each transaction's amount and whether
	// it's a credit or debit.
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

func (tb *totalBalance) writeTransactions() {
	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"Path", "Address", "Transaction Hash"})

	for _, tb := range tb.transactions {
		table.Append(tb.toArray())
	}
	table.Render()
	fmt.Printf("\n")
}

func (tb *totalBalance) writeSummary() {
	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"Total Balance", "Last Receive Index", "Last Change Index", "Report Time"})

	table.Append([]string{
		strconv.FormatUint(tb.totalBalance, 10),
		strconv.FormatUint(uint64(tb.lastAddresses[0]-1), 10),
		strconv.FormatUint(uint64(tb.lastAddresses[1]-1), 10),
		time.Now().Format(time.RFC822)})
	table.Render()
	fmt.Printf("\n")
}

func (tb *totalBalance) writeBalances() {
	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"Path", "Address", "Balance"})

	for _, tb := range tb.balances {
		table.Append(tb.toArray())
	}
	table.Render()
	fmt.Printf("\n")
}

func main() {
	kingpin.Version("0.0.2")
	kingpin.Parse()

	if !*debug {
		log.SetOutput(ioutil.Discard)
	}

	if *m <= 0 {
		panic(fmt.Sprintf("m has to be positive (got %d)", *m))
	}

	if *m > *n {
		panic(fmt.Sprintf("m cannot be larger than n (got %d)", *m))
	}

	if *n > 20 {
		panic(fmt.Sprintf("n cannot be greater han 20 (got %d)", *n))
	}

	xpubs := make([]string, 0, *n)

	reader := bufio.NewReader(os.Stdin)
	for i := 0; i < *n; i++ {
		fmt.Printf("Enter %s #%d out of #%d:\n", pubKeyPrefix(), i+1, *n)
		xpub, _ := reader.ReadString('\n')
		xpubs = append(xpubs, strings.TrimSpace(xpub))
	}

	net := Network(*network)
	deriver := deriver.NewAddressDeriver(net, xpubs, *m, *account)

	// NOTE: maybe allow to query various services like BlockCypher etc. based on
	//       CLI options
	b, err := balance.NewElectrumChecker(getServer())
	PanicOnError(err)

	tb := &totalBalance{}

	for _, change := range []uint32{0, 1} {
		tb.lastAddresses[change] = *lookahead
		for i := uint32(0); i < tb.lastAddresses[change]; i++ {
			addr := deriver.Derive(change, i)
			p := fmt.Sprintf("m/%s/%d/%d/%d", coinType(net), *account, change, i)

			fmt.Printf("Checking balance for %s %s ... ", p, addr)
			resp, err := b.Fetch(addr)
			PanicOnError(err)

			tb.addBalance(resp, p, addr, i+*lookahead, change)
			if resp.HasTransactions() {
				fmt.Printf("%d %d\n", resp.Balance, tb.totalBalance)
			} else {
				fmt.Printf("∅\n")
			}
			time.Sleep(*sleep)
		}
	}

	fmt.Printf("\n")
	tb.writeTransactions()
	tb.writeBalances()
	tb.writeSummary()
	PanicOnError(err)
}

func (tb *totalBalance) addBalance(r *balance.Response, path, addr string, lastAddrIndex, change uint32) {
	tb.totalBalance += r.Balance
	if r.HasTransactions() {
		tb.lastAddresses[change] = lastAddrIndex
		tb.balances = append(tb.balances, addrBalance{path: path, addr: addr, balance: r.Balance})

		for _, tx := range r.Transactions {
			tb.transactions = append(tb.transactions, transaction{path: path, addr: addr, hash: tx.Hash})
		}
	}
}

// as per SLIP-0044 https://github.com/satoshilabs/slips/blob/master/slip-0044.md
func coinType(n Network) string {
	switch n {
	case Mainnet:
		return "0'"
	case Testnet:
		return "1'"
	default:
		panic("unreachable")
	}
}

// prefixes come from BIP32
// https://github.com/bitcoin/bips/blob/master/bip-0032.mediawiki#serialization-format
func pubKeyPrefix() string {
	switch *network {
	case "mainnet":
		return "xpub"
	case "testnet":
		return "tpub"
	default:
		panic("unreachable")
	}
}

// pick a default server for each network if none provided
func getServer() string {
	if *addr != nil {
		return (*addr).String()
	}
	switch *network {
	case "mainnet":
		return mainnetDefaultServer
	case "testnet":
		return testnetDefaultServer
	default:
		panic("unreachable")
	}
}
