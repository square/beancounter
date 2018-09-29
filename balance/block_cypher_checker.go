package balance

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"

	"github.com/square/beancounter/deriver"
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
func (b *BlockCypherChecker) Fetch(addr string) *Response {
	url := apiURL + b.chain() + "addrs/" + addr + "?limit=0"
	resp, err := http.Get(url)
	if err != nil {
		return &Response{Error: err}
	}

	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return &Response{Error: fmt.Errorf("bad response: %+v", resp)}
	}

	return decode(resp.Body)
}

// Subscribe provides a bidirectional streaming of checks for addresses.
// It takes a channel of addresses and returns a channel of responses, to which
// it is writing asynchronuously.
// TODO: Looks like Subscribe implementation is separate from implementation
//       details of each checker and therefore could be abstracted into a separate
//       struct/interface (e.g. there could be a StreamingChecker interface that
//       implements Subcribe method).
func (b *BlockCypherChecker) Subscribe(addrCh <-chan *deriver.Address) <-chan *Response {
	respCh := make(chan *Response, 100)
	go func() {
		var wg sync.WaitGroup
		for addr := range addrCh {
			wg.Add(1)
			// do not block on each Fetch API call
			b.processFetch(addr, respCh, &wg)
		}
		// ensure that all addresses are processed and written to the output channel
		// before closing it.
		wg.Wait()
		close(respCh)
	}()

	return respCh
}

// processFetch fetches the data for an address, sends the response to the outgoing
// channel and marks itself as done in the shared WorkGroup
func (b *BlockCypherChecker) processFetch(addr *deriver.Address, out chan<- *Response, wg *sync.WaitGroup) {
	resp := b.Fetch(addr.String())
	resp.Address = addr
	out <- resp
	wg.Done()
}

// decode attempts to read data from the reader and decode it a BalanceResponse.
func decode(resp io.Reader) *Response {
	dec := json.NewDecoder(resp)
	var r Response
	err := dec.Decode(&r)
	if err != nil {
		return &Response{Error: err}
	}
	return &r
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
