package backend

import (
	"time"

	"github.com/square/beancounter/deriver"
)

// Backend is an interface which abstracts different types of backends.
// Enables fetching transaction information (for a given address) as well as block information.
type Backend interface {
	Fetch(addr string) *Response
	Subscribe(addrCh <-chan *deriver.Address) <-chan *Response
}

// Response wraps the balance and transaction information
type Response struct {
	Error        error
	Address      *deriver.Address
	Balance      uint64        `json:"balance"` // in Satoshi
	Transactions []Transaction `json:"txrefs,omitempty"`
}

// HasTransactions returns true if the Response contains any transactions
func (r *Response) HasTransactions() bool {
	return len(r.Transactions) > 0
}

// Transaction struct hold basic information about the transaction
type Transaction struct {
	Timestamp time.Time `json:"confirmed,omitempty"`
	Hash      string    `json:"tx_hash"`
	TXInputN  int64     `json:"tx_input_n"`
	TXOutputN int64     `json:"tx_output_n"`
	Value     uint64    `json:"value"` // in Satoshi
}
