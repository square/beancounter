package backend

import (
	"github.com/square/beancounter/deriver"
)

// Backend is an interface which abstracts different types of backends.
//
// The Backends are responsible for fetching all the transactions related to an address.
// For each transaction, the Backend must also grab:
// - the height
// - the raw transaction bytes
//
// The Backend doesn't keep much state. It pushes the data in the AddrResponses and TxResponses
// channels, which the Accounter reads from. A given transaction can (and often does) involve
// multiple addresses from the same wallet. The Backend maintains a small map to dedup fetches.
//
// There are a few differences between Electrum and Btcd (and potentially any other Backend we
// decide to add in the future). For instance, Electrum returns  the block height when fetching all
// the transactions for a given address, but Btcd doesn't. On  the other hand, Btcd returns the raw
// transaction information right away but Electrum requires additional requests.
//
// Because of these differences, the Backend exposes a Finish() method. This method allows the
// Accounter to wait until the Backend is done with any additional requests. In theory, we could
// forgo the Finish() method and have the Accounter read from the TxResponses channel until it has
// all the data it needs. This would require the Accounter to maintain its own set of transactions.

type Backend interface {
	AddrRequest(addr *deriver.Address)
	AddrResponses() <-chan *AddrResponse
	TxResponses() <-chan *TxResponse

	// Dec is super hacky. Need to find another wait to do this...
	Dec()
	Finish()
}

// AddrResponse lists transaction hashes for a given address
type AddrResponse struct {
	Address  *deriver.Address
	TxHashes []string
}

type TxResponse struct {
	Hash   string
	Height int64
	Hex    string
}

// HasTransactions returns true if the Response contains any transactions
func (r *AddrResponse) HasTransactions() bool {
	return len(r.TxHashes) > 0
}
