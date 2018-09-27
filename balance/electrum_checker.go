package balance

import (
	"crypto/tls"

	"github.com/d4l3k/go-electrum/electrum"
)

// ElectrumChecker wraps Electrum node and its API to provide a simple
// balance and transaction history information for a given address.
// ElectrumChecker implements Checker interface.
type ElectrumChecker struct {
	node *electrum.Node
}

// NewElectrumChecker returns a new ElectrumChecker structs or errors.
func NewElectrumChecker(addr string) (*ElectrumChecker, error) {
	node := electrum.NewNode()
	conf := &tls.Config{
		InsecureSkipVerify: true,
	}
	if err := node.ConnectSSL(addr, conf); err != nil {
		return nil, err
	}
	return &ElectrumChecker{node: node}, nil
}

// Fetch queries connected node for address balance and transaction history and
// returns Response.
func (e *ElectrumChecker) Fetch(addr string) (*Response, error) {
	b, err := e.node.BlockchainAddressGetBalance(addr)
	if err != nil {
		return nil, err
	}

	txs, err := e.node.BlockchainAddressGetHistory(addr)
	if err != nil {
		return nil, err
	}

	var transactions []Transaction
	for _, tx := range txs {
		t := Transaction{}
		t.Hash = tx.Hash
		if tx.Value < 0 {
			panic("Value cannot be negative")
		}
		t.Value = uint64(tx.Value)
		transactions = append(transactions, t)
	}

	resp := &Response{
		Balance:      uint64(b.Confirmed),
		Transactions: transactions,
	}
	return resp, nil
}
