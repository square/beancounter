package main

import (
	"bufio"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"

	"github.com/square/beancounter/accounter"
	"github.com/square/beancounter/backend"
	"github.com/square/beancounter/backend/electrum"
	"github.com/square/beancounter/deriver"
	. "github.com/square/beancounter/utils"
	"gopkg.in/alecthomas/kingpin.v2"
	//	_ "net/http/pprof"
)

// TODO: create sub commands:
// - findAddr
// - findBlock
// - watchMultiSig
// - watch
// - singleAddress
var (
	m             = kingpin.Flag("m", "number of signatures (quorum)").Short('m').Required().Int()
	n             = kingpin.Flag("n", "number of public keys").Short('n').Required().Int()
	account       = kingpin.Flag("account", "account number").Required().Uint32()
	backendName   = kingpin.Flag("backend", "electrum | btcd | electrum-recorder | btcd-recorder | fixture").Default("electrum").Enum("electrum", "btcd", "electrum-recorder", "btcd-recorder", "fixture")
	lookahead     = kingpin.Flag("lookahead", "lookahead size").Default("100").Uint32()
	addr          = kingpin.Flag("addr", "backend to connect to initially").PlaceHolder("HOST:PORT").TCP()
	rpcUser       = kingpin.Flag("rpcuser", "RPC username").PlaceHolder("USER").String()
	rpcPass       = kingpin.Flag("rpcpass", "RPC password").PlaceHolder("PASSWORD").String()
	debug         = kingpin.Flag("debug", "debug output").Default("false").Bool()
	findAddr      = kingpin.Flag("find-addr", "finds the offset of an address").String()
	blockHeight   = kingpin.Flag("block-height", "compute balance at given block height").Default("0").Int64()
	singleAddress = kingpin.Flag("single-address", "for debugging purpose").String()
	fixtureFile   = kingpin.Flag("fixture-file", "fixture file to use for recording data or reading from").PlaceHolder("FILEPATH").String()
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
		panic(fmt.Sprintf("n cannot be greater than 20 (got %d)", *n))
	}

	xpubs := make([]string, 0, *n)
	var network Network
	if *singleAddress == "" {
		reader := bufio.NewReader(os.Stdin)
		for i := 0; i < *n; i++ {
			fmt.Printf("Enter pubkey #%d out of #%d:\n", i+1, *n)
			xpub, _ := reader.ReadString('\n')
			xpubs = append(xpubs, strings.TrimSpace(xpub))
		}

		// Check that all the addresses have the same prefix
		for i := 1; i < *n; i++ {
			if xpubs[0][0:4] != xpubs[i][0:4] {
				fmt.Printf("Prefixes must match: %s %s\n", xpubs[0], xpubs[i])
				return
			}
		}
		network = XpubToNetwork(xpubs[0])
	} else {
		network = AddressToNetwork(*singleAddress)
	}
	deriver := deriver.NewAddressDeriver(network, xpubs, *m, *account, *singleAddress)

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

	backend, err := buildBackend(network)
	PanicOnError(err)

	// TODO: if blockHeight is 0, we should default to current height - 6.
	if *blockHeight == 0 {
		panic("blockHeight not set")
	}
	tb := accounter.New(backend, deriver, *lookahead, *blockHeight)

	balance := tb.ComputeBalance()

	fmt.Printf("Balance: %d\n", balance)
}

// TODO: return *backend.Backend, error instead?
func buildBackend(network Network) (backend.Backend, error) {
	//net := Network(*network)
	var b backend.Backend
	var err error
	switch *backendName {
	case "electrum":
		addr, port := getServer(network)
		b, err = backend.NewElectrumBackend(addr, port, network)
		if err != nil {
			return nil, err
		}
	case "btcd":
		b, err = backend.NewBtcdBackend(*blockHeight, (*addr).String(), *rpcUser, *rpcPass, network)
		if err != nil {
			return nil, err
		}
	case "electrum-recorder":
		if *fixtureFile == "" {
			panic("electrum-recorder backend requires a --fixture-file to be specified, so data can be recorded.")
		}
		addr, port := getServer(network)
		b, err = backend.NewElectrumBackend(addr, port, network)
		if err != nil {
			return nil, err
		}
		b, err = backend.NewRecorderBackend(b, *fixtureFile)
	case "btcd-recorder":
		if *fixtureFile == "" {
			panic("btcd-recorder backend requires a --fixture-file to be specified, so data can be recorded.")
		}
		b, err = backend.NewBtcdBackend(*blockHeight, (*addr).String(), *rpcUser, *rpcPass, network)
		if err != nil {
			return nil, err
		}
		b, err = backend.NewRecorderBackend(b, *fixtureFile)
	case "fixture":
		if *fixtureFile == "" {
			panic("fixture backend requires a file to load data from")
		}
		b, err = backend.NewFixtureBackend(*fixtureFile)
		if err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("unreachable")
	}
	return b, err
}

// pick a default server for each network if none provided
func getServer(network Network) (string, string) {
	if *addr != nil {
		return (*addr).IP.String(), strconv.Itoa((*addr).Port)
	}
	switch network {
	case "mainnet":
		return "electrum.petrkr.net", "s50002"
	case "testnet":
		return "electrum_testnet_unlimited.criptolayer.net", "s50102"
	default:
		panic("unreachable")
	}
}
