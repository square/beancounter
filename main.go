package main

import (
	"bufio"
	"fmt"
	"math"
	"os"
	"strings"

	"gopkg.in/alecthomas/kingpin.v2"

	"github.com/qshuai/go-electrum/electrum"
	"github.com/square/beancounter/balance"
	"github.com/square/beancounter/beancounter"
	"github.com/square/beancounter/deriver"
	. "github.com/square/beancounter/utils"
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
	find_addr = kingpin.Flag("find", "finds the offset of an address").String()
)

const (
	mainnetDefaultServer = "electrum.petrkr.net:50002"
	testnetDefaultServer = "electrum_testnet_unlimited.criptolayer.net:50102"
)

func main() {
	kingpin.Version("0.0.2")
	kingpin.Parse()

	if *debug {
		electrum.DebugMode = true
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

	if *find_addr != "" {
		fmt.Printf("Searching for %s\n", *find_addr)
		for i := uint32(0); i < math.MaxUint32; i++ {
			for _, change := range []uint32{0, 1} {
				addr := deriver.Derive(change, i)
				if addr.String() == *find_addr {
					fmt.Printf("found: %s %s\n", addr.Path(), addr)
					return
				}
				if i%1000 == 0 {
					fmt.Printf("reached: %s %s\n", addr.Path(), addr)
				}
			}
		}
		fmt.Printf("not found\n")
		return
	}

	// NOTE: maybe allow to query various services like BlockCypher etc. based on
	//       CLI options
	checker, err := balance.NewElectrumChecker(getServer())
	PanicOnError(err)

	tb := beancounter.NewCounter(checker, deriver, *lookahead, *sleep)

	tb.Count()

	fmt.Printf("\n")
	tb.WriteTransactions()
	tb.WriteBalances()
	tb.WriteSummary()
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
