package main

import (
	"bufio"
	"fmt"
	"github.com/btcsuite/btcutil/hdkeychain"
	"github.com/square/beancounter/blockfinder"
	"log"
	"math"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/square/beancounter/accounter"
	"github.com/square/beancounter/backend"
	"github.com/square/beancounter/backend/electrum"
	"github.com/square/beancounter/deriver"
	. "github.com/square/beancounter/utils"
	"gopkg.in/alecthomas/kingpin.v2"
)

var (
	app   = kingpin.New("beancounter", "A command-line Bitcoin wallet balance audit tool.")
	debug = app.Flag("debug", "Enable debug output.").Default("false").Bool()

	keytree    = app.Command("keytree", "Performs one or more child key derivations.")
	keytreeArg = keytree.Arg("i", "(repeated) Values for path.").Required().Uint32List()
	keytreeN   = keytree.Flag("n", "number of public keys").Short('n').Default("1").Int()

	findAddr    = app.Command("find-address", "Finds the change/index values for a given address.")
	findAddrArg = findAddr.Arg("address", "Address to look for.").Required().String()
	findAddrM   = findAddr.Flag("m", "number of signatures (quorum)").Short('m').Default("1").Int()
	findAddrN   = findAddr.Flag("n", "number of public keys").Short('n').Default("1").Int()

	findBlock            = app.Command("find-block", "Finds the block height for a given date/time.")
	findBlockTimestamp   = findBlock.Arg("timestamp", "Date/time to resolve. E.g. \"2006-01-02 15:04:05 MST\"").Required().String()
	findBlockBackend     = findBlock.Flag("backend", "electrum | btcd | electrum-recorder | btcd-recorder | fixture").Default("electrum").Enum("electrum", "btcd", "electrum-recorder", "btcd-recorder", "fixture")
	findBlockAddr        = findBlock.Flag("addr", "Backend to connect to initially. Defaults to a hardcoded node for Electrum and localhost for Btcd.").PlaceHolder("HOST:PORT").TCP()
	findBlockRpcUser     = findBlock.Flag("rpcuser", "RPC username").PlaceHolder("USER").String()
	findBlockRpcPass     = findBlock.Flag("rpcpass", "RPC password").PlaceHolder("PASSWORD").String()
	findBlockFixtureFile = findBlock.Flag("fixture-file", "Fixture file to use for recording or replaying data.").PlaceHolder("FILEPATH").String()

	computeBalance            = app.Command("compute-balance", "Computes balance for a given watch wallet.")
	computeBalanceBlockHeight = computeBalance.Flag("block-height", "Compute balance at given block height. Defaults to current chain height - 6.").Default("0").Uint32()
	computeBalanceType        = computeBalance.Flag("type", "multisig | single-address").Required().Enum("multisig", "single-address")
	computeBalanceM           = computeBalance.Flag("m", "number of signatures (quorum)").Short('m').Default("1").Int()
	computeBalanceN           = computeBalance.Flag("n", "number of public keys").Short('n').Default("1").Int()
	computeBalanceBackend     = computeBalance.Flag("backend", "electrum | btcd | electrum-recorder | btcd-recorder | fixture").Default("electrum").Enum("electrum", "btcd", "electrum-recorder", "btcd-recorder", "fixture")
	computeBalanceAddr        = computeBalance.Flag("addr", "Backend to connect to initially. Defaults to a hardcoded node for Electrum and localhost for Btcd.").PlaceHolder("HOST:PORT").TCP()
	computeBalanceRpcUser     = computeBalance.Flag("rpcuser", "RPC username").PlaceHolder("USER").String()
	computeBalanceRpcPass     = computeBalance.Flag("rpcpass", "RPC password").PlaceHolder("PASSWORD").String()
	computeBalanceFixtureFile = computeBalance.Flag("fixture-file", "Fixture file to use for recording or replaying data.").PlaceHolder("FILEPATH").String()
	computeBalanceLookahead   = computeBalance.Flag("lookahead", "lookahead size").Default("100").Uint32()
)

const (
	// number of confirmations required so we don't have to worry about orphaned blocks.
	minConfirmations = 6
)

func main() {
	app.Version("0.0.3")
	switch kingpin.MustParse(app.Parse(os.Args[1:])) {
	case keytree.FullCommand():
		doKeytree()
	case findAddr.FullCommand():
		doFindAddr()
	case findBlock.FullCommand():
		doFindBlock()
	case computeBalance.FullCommand():
		doComputeBalance()
	default:
		panic("unreachable")
	}
}

func doKeytree() {
	if !*debug {
		// Disallow piping to prevent leaking addresses in bash history, etc.
		stat, err := os.Stdin.Stat()
		PanicOnError(err)
		if (stat.Mode() & os.ModeCharDevice) == 0 {
			fmt.Println("Piping stdin forbidden.")
			return
		}
	}

	xpubs := make([]string, 0, *keytreeN)
	reader := bufio.NewReader(os.Stdin)
	for i := 0; i < *keytreeN; i++ {
		fmt.Printf("Enter pubkey #%d out of #%d:\n", i+1, *keytreeN)
		xpub, _ := reader.ReadString('\n')
		xpubs = append(xpubs, strings.TrimSpace(xpub))
	}

	// Check that all the addresses have the same prefix
	for i := 1; i < *keytreeN; i++ {
		if xpubs[0][0:4] != xpubs[i][0:4] {
			fmt.Printf("Prefixes must match: %s %s\n", xpubs[0], xpubs[i])
			return
		}
	}

	for _, path := range *keytreeArg {
		for i, xpub := range xpubs {
			key, err := hdkeychain.NewKeyFromString(xpub)
			PanicOnError(err)
			key, err = key.Child(path)
			PanicOnError(err)
			xpubs[i] = key.String()
		}
	}

	for i, xpub := range xpubs {
		fmt.Printf("Child pubkey #%d: %s\n", i+1, xpub)
	}
}

func doFindAddr() {
	err := VerifyMandN(*findAddrM, *findAddrN)
	if err != nil {
		panic(err)
	}

	if !*debug {
		// Disallow piping to prevent leaking addresses in bash history, etc.
		stat, err := os.Stdin.Stat()
		PanicOnError(err)
		if (stat.Mode() & os.ModeCharDevice) == 0 {
			fmt.Println("Piping stdin forbidden.")
			return
		}
	}

	xpubs := make([]string, 0, *findAddrN)
	reader := bufio.NewReader(os.Stdin)
	for i := 0; i < *findAddrN; i++ {
		fmt.Printf("Enter pubkey #%d out of #%d:\n", i+1, *findAddrN)
		xpub, _ := reader.ReadString('\n')
		xpubs = append(xpubs, strings.TrimSpace(xpub))
	}

	// Check that all the addresses have the same prefix
	for i := 1; i < *findAddrN; i++ {
		if xpubs[0][0:4] != xpubs[i][0:4] {
			fmt.Printf("Prefixes must match: %s %s\n", xpubs[0], xpubs[i])
			return
		}
	}
	network := XpubToNetwork(xpubs[0])
	deriver := deriver.NewAddressDeriver(network, xpubs, *findAddrM, "")

	fmt.Printf("Searching for %s\n", *findAddrArg)
	for i := uint32(0); i < math.MaxUint32; i++ {
		for _, change := range []uint32{0, 1} {
			addr := deriver.Derive(change, i)
			if addr.String() == *findAddrArg {
				fmt.Printf("found: %s %s\n", addr.Path(), addr)
				return
			}
			if i%1000 == 0 {
				fmt.Printf("reached: %s %s\n", addr.Path(), addr)
			}
		}
	}
	fmt.Printf("not found\n")
}

func doFindBlock() {
	t, err := time.Parse("2006-01-02 15:04:05 MST", *findBlockTimestamp)
	PanicOnError(err)

	backend, err := findBlockBuildBackend(Mainnet)
	PanicOnError(err)
	bf := blockfinder.New(backend)
	block, median, timestamp := bf.Search(t)
	fmt.Printf("Closest block to '%s' is block #%d with a median time of '%s'\n",
		t.String(), block, median.String())
	if *debug {
		fmt.Printf("timestamp: '%s'\n", timestamp.String())
	}
}

func doComputeBalance() {
	err := VerifyMandN(*computeBalanceM, *computeBalanceN)
	if err != nil {
		panic(err)
	}

	if *debug {
		electrum.DebugMode = true
	} else {
		// Disallow piping to prevent leaking addresses in bash history, etc.
		stat, err := os.Stdin.Stat()
		PanicOnError(err)
		if (stat.Mode() & os.ModeCharDevice) == 0 {
			fmt.Println("Piping stdin forbidden.")
			return
		}
	}

	xpubs := make([]string, 0, *computeBalanceN)
	var network Network
	reader := bufio.NewReader(os.Stdin)
	singleAddress := ""
	if *computeBalanceType == "single-address" {
		fmt.Printf("Enter single address:\n")
		singleAddress, _ = reader.ReadString('\n')
		network = AddressToNetwork(singleAddress)
	} else {
		for i := 0; i < *computeBalanceN; i++ {
			fmt.Printf("Enter pubkey #%d out of #%d:\n", i+1, *computeBalanceN)
			xpub, _ := reader.ReadString('\n')
			xpubs = append(xpubs, strings.TrimSpace(xpub))
		}

		// Check that all the addresses have the same prefix
		for i := 1; i < *computeBalanceN; i++ {
			if xpubs[0][0:4] != xpubs[i][0:4] {
				fmt.Printf("Prefixes must match: %s %s\n", xpubs[0], xpubs[i])
				return
			}
		}
		network = XpubToNetwork(xpubs[0])
	}
	deriver := deriver.NewAddressDeriver(network, xpubs, *computeBalanceM, singleAddress)

	backend, err := computeBalanceBuildBackend(network)
	PanicOnError(err)

	// If blockHeight is 0, we default to current height - 6.
	if *computeBalanceBlockHeight == 0 {
		*computeBalanceBlockHeight = backend.ChainHeight() - minConfirmations
	}
	if *computeBalanceBlockHeight > backend.ChainHeight()-minConfirmations {
		log.Panicf("blockHeight %d is too high (> %d - %d)", *computeBalanceBlockHeight, backend.ChainHeight(), minConfirmations)
	}
	fmt.Printf("Going to compute balance at %d\n", *computeBalanceBlockHeight)

	tb := accounter.New(backend, deriver, *computeBalanceLookahead, *computeBalanceBlockHeight)

	balance := tb.ComputeBalance()

	fmt.Printf("Balance: %d\n", balance)
}

// TODO: copy-pasta
func findBlockBuildBackend(network Network) (backend.Backend, error) {
	var b backend.Backend
	var err error
	switch *findBlockBackend {
	case "electrum":
		addr, port := getServer(network, *findBlockAddr)
		b, err = backend.NewElectrumBackend(addr, port, network)
		if err != nil {
			return nil, err
		}
	case "btcd":
		b, err = backend.NewBtcdBackend((**findBlockAddr).String(), *findBlockRpcUser,
			*findBlockRpcPass, network)
		if err != nil {
			return nil, err
		}
	case "electrum-recorder":
		if *findBlockFixtureFile == "" {
			panic("electrum-recorder backend requires output --fixture-file.")
		}
		addr, port := getServer(network, *findBlockAddr)
		b, err = backend.NewElectrumBackend(addr, port, network)
		if err != nil {
			return nil, err
		}
		b, err = backend.NewRecorderBackend(b, *findBlockFixtureFile)
	case "btcd-recorder":
		if *findBlockFixtureFile == "" {
			panic("btcd-recorder backend requires output --fixture-file.")
		}
		b, err = backend.NewBtcdBackend((*findBlockAddr).String(), *findBlockRpcUser,
			*findBlockRpcPass, network)
		if err != nil {
			return nil, err
		}
		b, err = backend.NewRecorderBackend(b, *findBlockFixtureFile)
	case "fixture":
		if *findBlockFixtureFile == "" {
			panic("fixture backend requires input --fixture-file.")
		}
		b, err = backend.NewFixtureBackend(*findBlockFixtureFile)
		if err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("unreachable")
	}
	return b, err
}

// TODO: return *backend.Backend, error instead?
func computeBalanceBuildBackend(network Network) (backend.Backend, error) {
	var b backend.Backend
	var err error
	switch *computeBalanceBackend {
	case "electrum":
		addr, port := getServer(network, *computeBalanceAddr)
		b, err = backend.NewElectrumBackend(addr, port, network)
		if err != nil {
			return nil, err
		}
	case "btcd":
		b, err = backend.NewBtcdBackend((*computeBalanceAddr).String(), *computeBalanceRpcUser,
			*computeBalanceRpcPass, network)
		if err != nil {
			return nil, err
		}
	case "electrum-recorder":
		if *computeBalanceFixtureFile == "" {
			panic("electrum-recorder backend requires output --fixture-file.")
		}
		addr, port := getServer(network, *computeBalanceAddr)
		b, err = backend.NewElectrumBackend(addr, port, network)
		if err != nil {
			return nil, err
		}
		b, err = backend.NewRecorderBackend(b, *computeBalanceFixtureFile)
	case "btcd-recorder":
		if *computeBalanceFixtureFile == "" {
			panic("btcd-recorder backend requires output --fixture-file.")
		}
		b, err = backend.NewBtcdBackend((*computeBalanceAddr).String(), *computeBalanceRpcUser,
			*computeBalanceRpcPass, network)
		if err != nil {
			return nil, err
		}
		b, err = backend.NewRecorderBackend(b, *computeBalanceFixtureFile)
	case "fixture":
		if *computeBalanceFixtureFile == "" {
			panic("fixture backend requires input --fixture-file.")
		}
		b, err = backend.NewFixtureBackend(*computeBalanceFixtureFile)
		if err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("unreachable")
	}
	return b, err
}

// pick a default server for each network if none provided
// TODO: default server should be localhost for Btcd backend.
func getServer(network Network, addr *net.TCPAddr) (string, string) {
	if addr != nil {
		return addr.IP.String(), strconv.Itoa(addr.Port)
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
