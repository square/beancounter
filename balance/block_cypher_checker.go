package balance

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/square/beancounter/utils"
)

const (
	apiURL = "https://api.blockcypher.com/v1/btc/"
)

// BlockCypherChecker wraps calls to BlockCypher servers and their API
// to provide a simple balance and transaction history information for a given address.
// BlockCypherChecker implements Checker interface.
type BlockCypherChecker struct {
	network utils.Network
}

// NewBlockCypherChecker returns a new BlockCypherChecker struct.
func NewBlockCypherChecker(network utils.Network) *BlockCypherChecker {
	return &BlockCypherChecker{
		network: network,
	}
}

// Fetch queries connected node for address balance and transaction history and
// returns Response.
func (b *BlockCypherChecker) Fetch(addr string) (*Response, error) {
	url := apiURL + b.chain() + "addrs/" + addr + "?limit=0"
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("bad response: %+v", resp)
	}

	return decode(resp.Body)
}

// decode attempts to read data from the reader and decode it a BalanceResponse.
func decode(resp io.Reader) (*Response, error) {
	dec := json.NewDecoder(resp)
	var r Response
	err := dec.Decode(&r)
	if err != nil {
		return nil, err
	}
	return &r, nil
}

// chain maps a Network type into a chain name used by BlockCypher in their API
func (b *BlockCypherChecker) chain() string {
	switch b.network {
	case utils.Mainnet:
		return "main/"
	case utils.Testnet:
		return "test3/"
	default:
		panic("unreachable")
	}
}
