package backend

import (
	"github.com/square/beancounter/deriver"
	time "time"
)

// Backend is an interface which abstracts different types of backends.
//
// The Backends are responsible for fetching all the transactions related to an address.
// For each transaction, the Backend must grab:
// - the height
// - the raw transaction bytes
//
// In addition, the backend must know the chain height. The backend is allowed to fetch this value
// once (at startup) and cache it.
//
// In general, we tried to keep the backends minimal and move as much (common) logic as possible
// into the accounter.
//
// There are a few differences between Electrum and Btcd (and potentially any other Backend we
// decide to add in the future). For instance, Electrum returns the block height when fetching all
// the transactions for a given address, but Btcd doesn't. On the other hand, Btcd returns the raw
// transaction information right away but Electrum requires additional requests.
//
// Because of these differences, the Backend exposes a Finish() method. This method allows the
// Accounter to wait until the Backend is done with any additional requests. In theory, we could
// forgo the Finish() method and have the Accounter read from the TxResponses channel until it has
// all the data it needs. This would require the Accounter to maintain its own set of transactions.
type Backend interface {
	// Returns chain height. Possibly connects + disconnects from first node.
	ChainHeight() uint32

	// Gets backend ready to serve requests
	Start(blockHeight uint32) error

	// Request-response channels
	AddrRequest(addr *deriver.Address)
	AddrResponses() <-chan *AddrResponse
	TxRequest(txHash string)
	TxResponses() <-chan *TxResponse
	BlockRequest(height uint32)
	BlockResponses() <-chan *BlockResponse

	// Call this to disconnect from nodes and cleanup
	Finish()
}

// AddrResponse lists transaction hashes for a given address
type AddrResponse struct {
	Address  *deriver.Address
	TxHashes []string
}

// TxResponse contains raw transaction, transaction hash and a block height in which
// it was confirmed.
type TxResponse struct {
	Hash   string
	Height int64
	Hex    string
}

type BlockResponse struct {
	Height    uint32
	Timestamp time.Time
}

// HasTransactions returns true if the Response contains any transactions
func (r *AddrResponse) HasTransactions() bool {
	return len(r.TxHashes) > 0
}
