package utils

import (
	"fmt"
	"net"

	"github.com/btcsuite/btcd/chaincfg"
)

// PanicOnError panics if err is not nil
func PanicOnError(err error) {
	if err != nil {
		panic(err)
	}
}

func Max(num uint32, nums ...uint32) uint32 {
	r := num
	for _, v := range nums {
		if v > r {
			r = v
		}
	}
	return r
}

type Network string
type BackendName string

const (
	Mainnet  Network     = "mainnet"
	Testnet  Network     = "testnet"
	Electrum BackendName = "electrum"
	Btcd     BackendName = "btcd"
)

// ChainConfig returns a given chaincfg.Params for a given Network
func (n Network) ChainConfig() *chaincfg.Params {
	switch n {
	case Mainnet:
		return &chaincfg.MainNetParams
	case Testnet:
		return &chaincfg.TestNet3Params
	default:
		panic("unreachable")
	}
}

// prefixes come from BIP32
// https://github.com/bitcoin/bips/blob/master/bip-0032.mediawiki#serialization-format
func XpubToNetwork(xpub string) Network {
	prefix := xpub[0:4]
	switch prefix {
	case "xpub":
		return Mainnet
	case "tpub":
		return Testnet
	default:
		panic(fmt.Sprintf("unknown prefix: %s", xpub))
	}
}

func AddressToNetwork(addr string) Network {
	switch addr[0] {
	case 'm':
		return Testnet // pubkey hash
	case 'n':
		return Testnet // pubkey hash
	case '2':
		return Testnet //script hash
	case '1':
		return Mainnet // pubkey hash
	case '3':
		return Mainnet // script hash
	default:
		panic(fmt.Sprintf("unknown prefix: %s", addr))
	}
}

func GenesisBlock(network Network) string {
	switch network {
	case Mainnet:
		return "000000000019d6689c085ae165831e934ff763ae46a2a6c172b3f1b60a8ce26f"
	case Testnet:
		return "000000000933ea01ad0ee984209779baaec3ced90fa3f408719526f8d77f4943"
	default:
		panic("unreachable")
	}
}

func VerifyMandN(m int, n int) error {
	if m <= 0 {
		return fmt.Errorf("m has to be positive (got %d)", m)
	}

	if m > n {
		return fmt.Errorf("m cannot be larger than n (%d > %d)", m, n)
	}

	if n > 20 {
		return fmt.Errorf("n cannot be greater than 20 (got %d)", n)
	}
	return nil
}

// Picks a default server for electrum or localhost for btcd
// Returns a pair of hostname:port (or pseudo-port for electrum)
func GetDefaultServer(network Network, backend BackendName, addr string) (string, string) {
	if addr != "" {
		host, port, err := net.SplitHostPort(addr)
		PanicOnError(err)
		return host, port
	}
	switch backend {
	case Electrum:
		switch network {
		case "mainnet":
			return "electrum.petrkr.net", "s50002"
		case "testnet":
			return "electrum_testnet_unlimited.criptolayer.net", "s50102"
		default:
			panic("unreachable")
		}
	case Btcd:
		switch network {
		case "mainnet":
			return "localhost", "8334"
		case "testnet":
			return "localhost", "18334"
		default:
			panic("unreachable")
		}
	default:
		panic("unreachable")
	}
}
