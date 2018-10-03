package utils

import (
	"fmt"
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

const (
	Mainnet Network = "mainnet"
	Testnet Network = "testnet"
)
