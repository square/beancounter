package backend

import (
	. "github.com/square/beancounter/utils"
)

func genesisBlock(network Network) string {
	switch network {
	case Mainnet:
		return "000000000019d6689c085ae165831e934ff763ae46a2a6c172b3f1b60a8ce26f"
	case Testnet:
		return "000000000933ea01ad0ee984209779baaec3ced90fa3f408719526f8d77f4943"
	default:
		panic("unreachable")
	}
}
