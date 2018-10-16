package backend

import (
	"github.com/square/beancounter/utils"
)

// index, address and transaction and helper structs used by recorder and fixture
// backends marshal/unmarshal address and transaction data

type index struct {
	Metadata     metadata      `json:"metadata"`
	Addresses    []address     `json:"addresses"`
	Transactions []transaction `json:"transactions"`
}

type metadata struct {
	Height uint32 `json:"height"`
}

type address struct {
	Address      string        `json:"address"`
	Path         string        `json:"path"`
	Network      utils.Network `json:"network"`
	Change       uint32        `json:"change"`
	AddressIndex uint32        `json:"addr_index"`
	TxHashes     []string      `json:"tx_hashes"`
}

type byAddress []address

func (a byAddress) Len() int           { return len(a) }
func (a byAddress) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a byAddress) Less(i, j int) bool { return a[i].Address < a[j].Address }

type transaction struct {
	Hash   string `json:"hash"`
	Height int64  `json:"height"`
	Hex    string `json:"hex"`
}

type byTransactionID []transaction

func (a byTransactionID) Len() int           { return len(a) }
func (a byTransactionID) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a byTransactionID) Less(i, j int) bool { return a[i].Hash < a[j].Hash }
