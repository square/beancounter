package electrum

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync/atomic"
	"time"

	"github.com/bcext/cashutil"
	"github.com/square/beancounter/utils"
)

const (
	sleep = 200 * time.Millisecond // Be nice to the Electrum nodes
)

type Node struct {
	// Ident is a an identifier of the form 127.0.0.1|s1234 or ::1|t5432.
	Ident   string
	Network utils.Network

	transport Transport

	// Next ID for request. Store/load this via sync/atomic.
	nextId uint64
}

type Feature struct {
	Prunning string `json:"prunning"`
	Protocol string `json:"protocol_max"`
	Genesis  string `json:"genesis_hash"`
}

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

type Balance struct {
	// Address field is unnecessary for Electrumx server protocol,
	// but is required for user of this library.
	Address string `json:"address"`

	Confirmed   cashutil.Amount `json:"confirmed"`
	Unconfirmed cashutil.Amount `json:"unconfirmed"`
}

type Transaction struct {
	Hash   string `json:"tx_hash"`
	Height uint32 `json:"height"`
	Value  int64  `json:"value"`
	Pos    uint32 `json:"tx_pos"`
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

// ScriptSig models a signature script.  It is defined separately since it only
// applies to non-coinbase.  Therefore the field in the Vin structure needs
// to be a pointer.
type ScriptSig struct {
	Asm string `json:"asm"`
	Hex string `json:"hex"`
}

type Peer struct {
	IP       string
	Host     string
	Version  string
	Features []string
}

type Header struct {
	Height uint32 `json:"height"`
	Hex    string `json:"hex"`
}

type Block struct {
	Count uint   `json:"count"`
	Hex   string `json:"hex"`
	Max   uint   `json:"max"`
}

func NewNode(addr, port string, network utils.Network) (*Node, error) {
	n := &Node{}
	var a string
	var t Transport
	var err error

	defaultTCP, defaultSSL := defaultPorts(network)

	if strings.Contains(addr, ":") {
		a = fmt.Sprintf("[%s]", addr)
	} else {
		a = addr
	}

	if port[0] == 't' {
		// TCP
		var p string
		if len(port) == 1 {
			p = defaultTCP
		} else {
			p = port[1:]
		}
		t, err = NewTCPTransport(fmt.Sprintf("%s:%s", a, p))
	} else if port[0] == 's' {
		// TLS
		var p string
		if len(port) == 1 {
			p = defaultSSL
		} else {
			p = port[1:]
		}
		t, err = NewSSLTransport(fmt.Sprintf("%s:%s", a, p))
	} else {
		panic(fmt.Sprintf("port (%s) must start with t or s", port))
	}

	if err != nil {
		return nil, err
	}

	n.transport = t
	n.Network = network
	n.Ident = NodeIdent(addr, port)
	return n, nil
}

func (n *Node) Disconnect() error {
	return n.transport.Shutdown()
}

func NodeIdent(addr, port string) string {
	return fmt.Sprintf("%s|%s", addr, port)
}

// IsCoinBase returns a bool to show if a Vin is a Coinbase one or not.
func (v *Vin) IsCoinBase() bool {
	return len(v.Coinbase) > 0
}

// ServerFeatures returns the server features dictionary.
// method: "server.features"
//
// version 1.1
// https://electrumx.readthedocs.io/en/latest/protocol-methods.html#server-features
func (n *Node) ServerFeatures() (*Feature, error) {
	var result Feature
	err := n.request("server.features", []interface{}{}, &result)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// ServerVersion allows negotiating a min protocol version. This is required, as various methods
// appeared (or were removed) in various versions.
//
// version 1.1
// https://electrumx.readthedocs.io/en/latest/protocol-methods.html#server-version
func (n *Node) ServerVersion(ver string) error {
	var ignored []string
	return n.request("server.version", []interface{}{"beancounter", ver}, &ignored)
}

// BlockchainAddressGetHistory returns the history of an address.
//
// version 1.1 and version 1.2 only
// https://electrumx.readthedocs.io/en/latest/protocol-methods.html#blockchain-scripthash-get_history
func (n *Node) BlockchainAddressGetHistory(address string) ([]*Transaction, error) {
	var result []*Transaction
	err := n.request("blockchain.address.get_history", []interface{}{address}, &result)
	if err != nil {
		return nil, err
	}
	return result, nil
}

// BlockchainTransactionGet returns a raw transaction.
//
// https://electrumx.readthedocs.io/en/latest/protocol-methods.html#blockchain-transaction-get
func (n *Node) BlockchainTransactionGet(txid string) (string, error) {
	var hex string
	err := n.request("blockchain.transaction.get", []interface{}{txid, false}, &hex)
	return hex, err
}

// Subscribe to receive block headers when a new block is found.
//
// Note: there's no way to unsubscribe, and the rest of this code doesn't know how to deal with
// notifications. It is advisable to only call this method once and disconnect/reconnect after
// getting the block height.
//
// https://electrumx.readthedocs.io/en/latest/protocol-methods.html#blockchain-headers-subscribe
func (n *Node) BlockchainHeadersSubscribe() (*Header, error) {
	var header Header
	err := n.request("blockchain.headers.subscribe", []interface{}{true}, &header)
	return &header, err
}

// ServerPeersSubscribe requests peers from a server.
//
// https://electrumx.readthedocs.io/en/latest/protocol-methods.html#server-peers-subscribe
func (n *Node) ServerPeersSubscribe() ([]Peer, error) {
	var peers [][]interface{}
	err := n.request("server.peers.subscribe", []interface{}{}, &peers)
	if err != nil {
		return nil, err
	}

	out := []Peer{}
	for _, peer := range peers {
		features := make([]string, 0, len(peer[2].([]interface{})))
		for _, feature := range peer[2].([]interface{}) {
			features = append(features, feature.(string))
		}

		p := Peer{
			IP:       peer[0].(string),
			Host:     peer[1].(string),
			Version:  features[0],
			Features: features[1:],
		}
		out = append(out, p)
	}

	return out, nil
}

// BlockchainBlockHeaders returns a block header (160 hex).
func (n *Node) BlockchainBlockHeaders(height uint32, count uint) (Block, error) {
	var block Block
	err := n.request("blockchain.block.headers", []interface{}{height, count}, &block)
	return block, err
}

func (n *Node) request(method string, params []interface{}, result interface{}) error {
	msg := RequestMessage{
		Id:     atomic.AddUint64(&n.nextId, 1),
		Method: method,
		Params: params,
	}

	resp, err := n.transport.SendMessage(msg)
	if err != nil {
		return err
	}

	r, err := json.Marshal(resp.Result)
	if err != nil {
		return err
	}
	json.Unmarshal(r, result)
	time.Sleep(sleep)
	return nil
}

func defaultPorts(network utils.Network) (string, string) {
	switch network {
	case utils.Mainnet:
		return "50001", "50002"
	case utils.Testnet:
		return "50101", "50102"
	default:
		panic("unreachable")
	}
}
