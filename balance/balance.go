package balance

import (
	"time"
)

// Checker is an interface wraps the Fetch method.
// Checker fetches the balance information for an address.
type Checker interface {
	Fetch(addr string) (*Response, error)
}

// Response wraps the balance and transaction information
type Response struct {
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
