package utils

import "github.com/btcsuite/btcd/chaincfg"

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

const (
	Mainnet Network = "mainnet"
	Testnet Network = "testnet"
)
