package electrum

import (
	"encoding/json"
	"log"
	"strconv"

	"github.com/btcsuite/btcutil"
)

// BlockchainNumBlocksSubscribe returns the current number of blocks.
// http://docs.electrum.org/en/latest/protocol.html#blockchain-numblocks-subscribe
func (n *Node) BlockchainNumBlocksSubscribe() (int, error) {
	resp := &struct {
		Result int `json:"result"`
	}{}
	err := n.request("blockchain.numblocks.subscribe", nil, resp)
	return resp.Result, err
}

type BlockchainHeader struct {
	Nonce         uint64 `json:"nonce"`
	PrevBlockHash string `json:"prev_block_hash"`
	Timestamp     uint64 `json:"timestamp"`
	MerkleRoot    string `json:"merkle_root"`
	BlockHeight   uint64 `json:"block_height"`
	UtxoRoot      string `json:"utxo_root"`
	Version       int    `json:"version"`
	Bits          uint64 `json:"bits"`
}

// BlockchainHeadersSubscribe request client notifications about new blocks in
// form of parsed blockheaders and returns the current block header.
// http://docs.electrum.org/en/latest/protocol.html#blockchain-headers-subscribe
func (n *Node) BlockchainHeadersSubscribe() (<-chan *BlockchainHeader, error) {
	resp := &struct {
		Result *BlockchainHeader `json:"result"`
	}{}
	if err := n.request("blockchain.headers.subscribe", nil, resp); err != nil {
		return nil, err
	}
	headerChan := make(chan *BlockchainHeader, 1)
	headerChan <- resp.Result
	go func() {
		for msg := range n.listenPush("blockchain.headers.subscribe") {
			resp := &struct {
				Params []*BlockchainHeader `json:"params"`
			}{}
			if err := json.Unmarshal(msg, resp); err != nil {
				log.Printf("ERR %s", err)
				return
			}
			for _, param := range resp.Params {
				headerChan <- param
			}
		}
	}()
	return headerChan, nil
}

// BlockchainAddressSubscribe subscribes to transactions on an address and
// returns the hash of the transaction history.
// http://docs.electrum.org/en/latest/protocol.html#blockchain-address-subscribe
func (n *Node) BlockchainAddressSubscribe(address string) (<-chan string, error) {
	resp := &basicResp{}
	err := n.request("blockchain.address.subscribe", []string{address}, resp)
	if err != nil {
		return nil, err
	}
	addressChan := make(chan string, 1)
	if len(resp.Result) > 0 {
		addressChan <- resp.Result
	}
	go func() {
		for msg := range n.listenPush("blockchain.address.subscribe") {
			resp := &struct {
				Params []string `json:"params"`
			}{}
			if err := json.Unmarshal(msg, resp); err != nil {
				log.Printf("ERR %s", err)
				return
			}
			if len(resp.Params) != 2 {
				log.Printf("address subscription params len != 2 %+v", resp.Params)
				continue
			}
			if resp.Params[0] == address {
				addressChan <- resp.Params[1]
			}
		}
	}()
	return addressChan, err
}

type Transaction struct {
	Hash   string `json:"tx_hash"`
	Height int    `json:"height"`
	Value  int    `json:"value"`
	Pos    int    `json:"tx_pos"`
}

// BlockchainAddressGetHistory returns the history of an address.
// http://docs.electrum.org/en/latest/protocol.html#blockchain-address-get-history
func (n *Node) BlockchainAddressGetHistory(address string) ([]*Transaction, error) {
	resp := &struct {
		Result []*Transaction `json:"result"`
	}{}
	err := n.request("blockchain.address.get_history", []string{address}, resp)
	return resp.Result, err
}

// TODO(d4l3k) implement
// http://docs.electrum.org/en/latest/protocol.html#blockchain-address-get-mempool
func (n *Node) BlockchainAddressGetMempool() error { return ErrNotImplemented }

type Balance struct {
	Confirmed   btcutil.Amount `json:"confirmed"`
	Unconfirmed btcutil.Amount `json:"unconfirmed"`
}

// BlockchainAddressGetBalance returns the balance of an address.
// TODO (d4l3k) investigate `error from server: "'Node' object has no attribute '__getitem__'"`
// http://docs.electrum.org/en/latest/protocol.html#blockchain-address-get-balance
func (n *Node) BlockchainAddressGetBalance(address string) (*Balance, error) {
	resp := &struct {
		Result *Balance `json:"result"`
	}{}
	err := n.request("blockchain.address.get_balance", []string{address}, resp)
	return resp.Result, err
}

// TODO(d4l3k) implement
// http://docs.electrum.org/en/latest/protocol.html#blockchain-address-get-proof
func (n *Node) BlockchainAddressGetProof() error { return ErrNotImplemented }

// BlockchainAddressListUnspent lists the unspent transactions for the given address.
// http://docs.electrum.org/en/latest/protocol.html#blockchain-address-listunspent
func (n *Node) BlockchainAddressListUnspent(address string) ([]*Transaction, error) {
	resp := &struct {
		Result []*Transaction `json:"result"`
	}{}
	err := n.request("blockchain.address.listunspent", []string{address}, resp)
	return resp.Result, err
}

// TODO(d4l3k) implement
// http://docs.electrum.org/en/latest/protocol.html#blockchain-utxo-get-address
func (n *Node) BlockchainUtxoGetAddress() error { return ErrNotImplemented }

// TODO(d4l3k) implement
// http://docs.electrum.org/en/latest/protocol.html#blockchain-block-get-header
func (n *Node) BlockchainBlockGetHeader() error { return ErrNotImplemented }

// TODO(d4l3k) implement
// http://docs.electrum.org/en/latest/protocol.html#blockchain-block-get-chunk
func (n *Node) BlockchainBlockGetChunk() error { return ErrNotImplemented }

// BlockchainTransactionBroadcast sends a raw transaction.
// TODO(d4l3k) implement
// http://docs.electrum.org/en/latest/protocol.html#blockchain-transaction-broadcast
func (n *Node) BlockchainTransactionBroadcast(tx []byte) (interface{}, error) {
	resp := &struct {
		Result interface{} `json:"result"`
	}{}
	err := n.request("blockchain.transaction.broadcast", []string{string(tx)}, resp)
	return resp.Result, err
}

// TODO(d4l3k) implement
// http://docs.electrum.org/en/latest/protocol.html#blockchain-transaction-get-merkle
func (n *Node) BlockchainTransactionGetMerkle() error { return ErrNotImplemented }

// BlockchainTransactionGet returns the raw transaction (hex-encoded) for the given txid. If transaction doesn't exist, an error is returned.
// http://docs.electrum.org/en/latest/protocol.html#blockchain-transaction-get
func (n *Node) BlockchainTransactionGet(txid string) (string, error) {
	resp := &basicResp{}
	err := n.request("blockchain.transaction.get", []string{txid}, resp)
	return resp.Result, err
}

// http://docs.electrum.org/en/latest/protocol.html#blockchain-estimatefee
// BlockchainEstimateFee estimates the transaction fee per kilobyte that needs to be paid for a transaction to be included within a certain number of blocks.
func (n *Node) BlockchainEstimateFee(block int) (float64, error) {
	resp := &struct {
		Result float64 `json:"result"`
	}{}
	err := n.request("blockchain.estimatefee", []string{strconv.Itoa(block)}, resp)
	return resp.Result, err
}
