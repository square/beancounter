package main

import (
	"bufio"
	"fmt"
	"log"
	"math"
	"os"
	"strings"

	"gopkg.in/alecthomas/kingpin.v2"

	"github.com/mbyczkowski/go-electrum/electrum"
	"github.com/square/beancounter/balance"
	"github.com/square/beancounter/beancounter"
	"github.com/square/beancounter/deriver"
	. "github.com/square/beancounter/utils"

	"net/http"
	_ "net/http/pprof"
)

var (
	m              = kingpin.Flag("m", "number of signatures (quorum)").Short('m').Required().Int()
	n              = kingpin.Flag("n", "number of public keys").Short('n').Required().Int()
	account        = kingpin.Flag("account", "account number").Required().Uint32()
	network        = kingpin.Flag("network", "'mainnet' or 'testnet'").Default("mainnet").Enum("mainnet", "testnet")
	backend        = kingpin.Flag("backend", "Personal Btcd or public Electrum nodes").Default("electrum").Enum("electrum", "btcd")
	lookahead      = kingpin.Flag("lookahead", "lookahead size").Default("100").Uint32()
	sleep          = kingpin.Flag("sleep", "sleep between requests to avoid API rate-limit").Default("1s").Duration()
	addr           = kingpin.Flag("addr", "Electrum or btcd server").PlaceHolder("HOST:PORT").TCP()
	rpcuser        = kingpin.Flag("rpcuser", "RPC username").PlaceHolder("USER").String()
	rpcpass        = kingpin.Flag("rpcpass", "RPC password").PlaceHolder("PASSWORD").String()
	debug          = kingpin.Flag("debug", "debug output").Default("false").Bool()
	findAddr       = kingpin.Flag("find", "finds the offset of an address").String()
	maxBlockHeight = kingpin.Flag("max-block-height", "finds the offset of an address").Default("0").Int64()
)

const (
	mainnetDefaultServer = "electrum.petrkr.net:50002"
	testnetDefaultServer = "electrum_testnet_unlimited.criptolayer.net:50102"
)

func main() {
	go func() {
		log.Println(http.ListenAndServe("localhost:6060", nil))
	}()

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
		panic(fmt.Sprintf("n cannot be greater than 20 (got %d)", *n))
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

	if *findAddr != "" {
		fmt.Printf("Searching for %s\n", *findAddr)
		for i := uint32(0); i < math.MaxUint32; i++ {
			for _, change := range []uint32{0, 1} {
				addr := deriver.Derive(change, i)
				if addr.String() == *findAddr {
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

	checker, err := buildChecker()
	PanicOnError(err)

	tb := beancounter.NewCounter(checker, deriver, *lookahead, 0, *sleep)

	tb.Count()

	fmt.Printf("\n")
	tb.WriteTransactions()
	tb.WriteBalances()
	tb.WriteSummary()
}

func buildChecker() (balance.Checker, error) {
	net := Network(*network)
	switch *backend {
	case "electrum":
		return balance.NewElectrumChecker(getServer())
	case "btcd":
		return balance.NewBtcdChecker(*maxBlockHeight, (*addr).String(), *rpcuser, *rpcpass, net.ChainConfig())
	}
	return nil, fmt.Errorf("unreachable")
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
