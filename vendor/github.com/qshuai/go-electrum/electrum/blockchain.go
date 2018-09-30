package electrum

import (
	"encoding/json"
	"log"

	"github.com/bcext/cashutil"
)

type BlockchainHeader struct {
	Nonce         uint32 `json:"nonce"`
	PrevBlockHash string `json:"prev_block_hash"`
	Timestamp     int64  `json:"timestamp"`
	MerkleRoot    string `json:"merkle_root"`
	BlockHeight   int32  `json:"block_height"`
	UtxoRoot      string `json:"utxo_root"`
	Version       int32  `json:"version"`
	Bits          int64  `json:"bits"`
}

// TODO implement
// BlockchainBlockGetChunk returns a chunk of block headers as a hexadecimal string.
// method: "blockchain.block.get_chunk"
//
// document not updated
func (n *Node) BlockchainBlockGetChunk(index int32) (string, error) {
	return "", ErrNotImplemented
}

// TODO implement
// BlockchainBlockGetHeader returns the deserialized header at a given height.
// method "blockchain.block.get_header"
//
// document not updated
func (n *Node) BlockchainBlockGetHeader(height int32) error {
	return ErrNotImplemented
}

// BlockchainEstimateFee returns the estimated transaction fee per kilobyte for a transaction
// to be confirmed within a certain number of blocks.
//
// https://electrumx.readthedocs.io/en/latest/protocol-methods.html#blockchain-estimatefee
func (n *Node) BlockchainEstimateFee(block int) (float64, error) {
	resp := &struct {
		Result float64 `json:"result"`
	}{}
	err := n.request("blockchain.estimatefee", []interface{}{block}, resp)
	return resp.Result, err
}

// TODO implement
// BlockchainRelayfee returns the minimum fee a low-priority tx must pay in order to be
// accepted to the daemon's memory pool.
// method: "blockchain.relayfee"
//
// https://electrumx.readthedocs.io/en/latest/protocol-methods.html#blockchain-relayfee
func (n *Node) BlockchainRelayfee() error {
	return ErrNotImplemented
}

type Balance struct {
	// Address field is unnecessary for Electrumx server protocol,
	// but is required for user of this library.
	Address string `json:"address"`

	Confirmed   cashutil.Amount `json:"confirmed"`
	Unconfirmed cashutil.Amount `json:"unconfirmed"`
}

// TODO implement
// BlockchainScripthashGetBalance returns the confirmed and unconfirmed balance of a scripthash.
// method: "blockchain.scripthash.get_balance"
//
// https://electrumx.readthedocs.io/en/latest/protocol-methods.html#blockchain-scripthash-get-balance
func (n *Node) BlockchainScripthashGetBalance(scriptHash string) (*Balance, error) {
	return nil, ErrNotImplemented
}

// BlockchainGetBalance returns the balance of an address.
// Available(version < 1.3)
//
// http://docs.electrum.org/en/latest/protocol.html#blockchain-address-get-balance
func (n *Node) BlockchainAddressGetBalance(address string) (*Balance, error) {
	resp := &struct {
		Result *Balance `json:"result"`
	}{}
	err := n.request("blockchain.address.get_balance", []interface{}{address}, resp)
	resp.Result.Address = address
	return resp.Result, err
}

type Transaction struct {
	Hash   string `json:"tx_hash"`
	Height int32  `json:"height"`
	Value  int64  `json:"value"`
	Pos    uint32 `json:"tx_pos"`
}

// TODO implement
// BlockchainScripthashGetHistory returns the confirmed and unconfirmed history of a scripthash.
// method: "blockchain.scripthash.get_history"
//
// https://electrumx.readthedocs.io/en/latest/protocol-methods.html#blockchain-scripthash-get-history
func (n *Node) BlockchainScripthashGetHistory(scriptHash string) ([]*Transaction, error) {
	return nil, ErrNotImplemented
}

// BlockchainAddressGetHistory returns the history of an address.
// Available(version < 1.3)
//
// http://docs.electrum.org/en/latest/protocol.html#blockchain-address-get-history
func (n *Node) BlockchainAddressGetHistory(address string) ([]*Transaction, error) {
	resp := &struct {
		Result []*Transaction `json:"result"`
	}{}
	err := n.request("blockchain.address.get_history", []interface{}{address}, resp)
	return resp.Result, err
}

// TODO implement
// BlockchainScripthashGetMempool returns the mempool transactions touching a scripthash.
// method: "blockchain.scripthash.get_mempool"
//
// https://electrumx.readthedocs.io/en/latest/protocol-methods.html#blockchain-scripthash-get-mempool
func (n *Node) BlockchainScripthashGetMempool(scriptHash string) error {
	return ErrNotImplemented
}

// TODO implement
//  Available(version < 1.3)
//
// http://docs.electrum.org/en/latest/protocol.html#blockchain-address-get-mempool
func (n *Node) BlockchainAddressGetMempool() error {
	return ErrNotImplemented
}

// TODO implement
// BlockchainScripthashListUnspent returns the list of UTXOs of a scripthash.
// method: "blockchain.scripthash.listunspent"
//
// https://electrumx.readthedocs.io/en/latest/protocol-methods.html#blockchain-scripthash-listunspent
func (n *Node) BlockchainScripthashListUnspent(scriptHash string) ([]*Transaction, error) {
	return nil, ErrNotImplemented
}

// BlockchainAddressListUnspent lists the unspent transactions for the given address.
// Available(version < 1.3)
//
// http://docs.electrum.org/en/latest/protocol.html#blockchain-address-listunspent
func (n *Node) BlockchainAddressListUnspent(address string) ([]*Transaction, error) {
	resp := &struct {
		Result []*Transaction `json:"result"`
	}{}
	err := n.request("blockchain.address.listunspent", []interface{}{address}, resp)
	return resp.Result, err
}

// TODO implement
// BlockchainScripthashSubscribe subscribes to a script hash.
// method: "blockchain.scripthash.subscribe"
//
// https://electrumx.readthedocs.io/en/latest/protocol-methods.html#blockchain-scripthash-subscribe
func (n *Node) BlockchainScripthashSubscribe(scriptHash string) (<-chan string, error) {
	return nil, ErrNotImplemented
}

// BlockchainAddressSubscribe subscribes to transactions on an address and
// returns the hash of the transaction history.
// Available(version < 1.3)
//
// http://docs.electrum.org/en/latest/protocol.html#blockchain-address-subscribe
func (n *Node) BlockchainAddressSubscribe(address string) (<-chan string, error) {
	resp := &basicResp{}
	err := n.request("blockchain.address.subscribe", []interface{}{address}, resp)
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
				// TODO handle error. Notify the error for caller about that electrum server
				// will not track the balance change for the param address
				return
			}
			if len(resp.Params) != 2 {
				continue
			}

			if resp.Params[0] == address {
				addressChan <- resp.Params[1]
			}
		}
	}()

	return addressChan, err
}

// TODO implement
// Broadcast a transaction to the network.
//
// https://electrumx.readthedocs.io/en/latest/protocol-methods.html#blockchain-transaction-broadcast
func (n *Node) BlockchainTransactionBroadcast(tx []byte) (interface{}, error) {
	resp := &struct {
		Result interface{} `json:"result"`
	}{}
	err := n.request("blockchain.transaction.broadcast", []interface{}{string(tx)}, resp)
	return resp.Result, err
}

type GetTransaction struct {
	Hex           string `json:"hex"`
	Txid          string `json:"txid"`
	Version       int32  `json:"version"`
	Locktime      uint32 `json:"locktime"`
	Vin           []Vin  `json:"vin"`
	Vout          []Vout `json:"vout"`
	BlockHash     string `json:"blockhash"`
	Confirmations int32  `json:"confirmations"`
	Time          int64  `json:"time"`
	Blocktime     int64  `json:"blocktime"`
}

// Vin models parts of the tx data.
type Vin struct {
	Coinbase  string     `json:"coinbase"`
	Txid      string     `json:"txid"`
	Vout      uint32     `json:"vout"`
	ScriptSig *ScriptSig `json:"scriptSig"`
	Sequence  uint32     `json:"sequence"`
}

// ScriptSig models a signature script.  It is defined separately since it only
// applies to non-coinbase.  Therefore the field in the Vin structure needs
// to be a pointer.
type ScriptSig struct {
	Asm string `json:"asm"`
	Hex string `json:"hex"`
}

// IsCoinBase returns a bool to show if a Vin is a Coinbase one or not.
func (v *Vin) IsCoinBase() bool {
	return len(v.Coinbase) > 0
}

// MarshalJSON provides a custom Marshal method for Vin.
func (v *Vin) MarshalJSON() ([]byte, error) {
	if v.IsCoinBase() {
		coinbaseStruct := struct {
			Coinbase string `json:"coinbase"`
			Sequence uint32 `json:"sequence"`
		}{
			Coinbase: v.Coinbase,
			Sequence: v.Sequence,
		}
		return json.Marshal(coinbaseStruct)
	}

	txStruct := struct {
		Txid      string     `json:"txid"`
		Vout      uint32     `json:"vout"`
		ScriptSig *ScriptSig `json:"scriptSig"`
		Sequence  uint32     `json:"sequence"`
	}{
		Txid:      v.Txid,
		Vout:      v.Vout,
		ScriptSig: v.ScriptSig,
		Sequence:  v.Sequence,
	}
	return json.Marshal(txStruct)
}

// ScriptPubKeyResult models the scriptPubKey data of a tx script.  It is
// defined separately since it is used by multiple commands.
type ScriptPubKeyResult struct {
	Asm       string   `json:"asm"`
	Hex       string   `json:"hex,omitempty"`
	ReqSigs   int32    `json:"reqSigs,omitempty"`
	Type      string   `json:"type"`
	Addresses []string `json:"addresses,omitempty"`
}

// Vout models parts of the tx data.
type Vout struct {
	Value        float64            `json:"value"`
	N            uint32             `json:"n"`
	ScriptPubKey ScriptPubKeyResult `json:"scriptPubKey"`
}

// BlockchainTransactionGet returns a raw transaction.
//
// https://electrumx.readthedocs.io/en/latest/protocol-methods.html#blockchain-transaction-get
func (n *Node) BlockchainTransactionGet(txid string, verbose bool) (*GetTransaction, error) {
	resp := &struct {
		Result GetTransaction `json:"result"`
	}{}
	err := n.request("blockchain.transaction.get", []interface{}{txid, verbose}, resp)
	return &resp.Result, err
}

// TODO implement
// Return the merkle branch to a confirmed transaction given its hash and height.
//
// https://electrumx.readthedocs.io/en/latest/protocol-methods.html#blockchain-transaction-get-merkle
func (n *Node) BlockchainTransactionGetMerkle() error {
	return ErrNotImplemented
}

// BlockchainHeadersSubscribe subscribes to get raw headers of new blocks.
//
// https://electrumx.readthedocs.io/en/latest/protocol-methods.html#blockchain-headers-subscribe
func (n *Node) BlockchainHeadersSubscribe() (<-chan *BlockchainHeader, error) {
	resp := &struct {
		Result *BlockchainHeader `json:"result"`
	}{}
	if err := n.request("blockchain.headers.subscribe", []interface{}{}, resp); err != nil {
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
				// TODO: deal with error
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
