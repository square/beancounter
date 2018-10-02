package deriver

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"log"
	"sort"

	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcutil"
	"github.com/btcsuite/btcutil/hdkeychain"

	. "github.com/square/beancounter/utils"
)

// AddressDeriver is a struct that contains necessary information to derive
// an address from a given extended public key (or list of public keys).
// It follows the conventions as written in BIP32
// // https://github.com/bitcoin/bips/blob/master/bip-0032.mediawiki#serialization-format
type AddressDeriver struct {
	network Network
	xpubs   []string
	m       int
	account uint32
}

// Address wraps a simple wallet address.
// It contains information such as network type (e.g. mainnet or testnet), derivation
// path (e.g. m/0/0/123/50), change value and address index.
type Address struct {
	path      string
	addr      string
	net       Network
	change    uint32
	addrIndex uint32
}

// Path returns derivation path
func (a *Address) Path() string {
	return a.path
}

// String returns the address as string
func (a *Address) String() string {
	return a.addr
}

// Change returns the change value (so 0 or 1)
func (a *Address) Change() uint32 {
	return a.change
}

// Index returns the address index
func (a *Address) Index() uint32 {
	return a.addrIndex
}

// NewAddressDeriver returns a new instance of AddressDeriver
func NewAddressDeriver(network Network, xpubs []string, m int, account uint32) *AddressDeriver {
	return &AddressDeriver{
		network: network,
		xpubs:   xpubs,
		m:       m,
		account: account,
	}
}

// Derive dervives an address for given change and address index.
// It supports derivation using single extended public key and multisig + segwit.
func (d *AddressDeriver) Derive(change uint32, addressIndex uint32) *Address {

	path := fmt.Sprintf("m/%s/%d/%d/%d", coinType(d.network), d.account, change, addressIndex)
	addr := &Address{path: path, net: d.network, change: change, addrIndex: addressIndex}
	if len(d.xpubs) == 1 {
		addr.addr = d.singleDerive(change, addressIndex)
		return addr
	}
	addr.addr = d.multiSigSegwitDerive(change, addressIndex)
	return addr
}

// singleDerive performs a derivation using a single extended public key
func (d *AddressDeriver) singleDerive(change uint32, addressIndex uint32) string {
	key, err := hdkeychain.NewKeyFromString(d.xpubs[0])
	PanicOnError(err)

	if d.account != 4294967295 {
		key, err = key.Child(d.account)
		PanicOnError(err)
	}

	key, err = key.Child(change)
	PanicOnError(err)

	key, err = key.Child(addressIndex)
	PanicOnError(err)

	pubKey, err := key.Address(d.network.ChainConfig())
	PanicOnError(err)

	return pubKey.String()
}

// multiSigSegwitDerive performs a multisig + segwit derivation.
func (d *AddressDeriver) multiSigSegwitDerive(change uint32, addressIndex uint32) string {
	pubKeysBytes := make([][]byte, 0, len(d.xpubs))
	pubKeys := make([]*btcutil.AddressPubKey, 0, len(d.xpubs))

	for _, xpub := range d.xpubs {
		key, err := hdkeychain.NewKeyFromString(xpub)
		PanicOnError(err)

		key, err = key.Child(d.account)
		PanicOnError(err)

		key, err = key.Child(change)
		PanicOnError(err)

		key, err = key.Child(addressIndex)
		PanicOnError(err)

		pubKey, err := key.ECPubKey()
		PanicOnError(err)

		pubKeyBytes := pubKey.SerializeCompressed()
		if len(pubKeyBytes) != 33 {
			panic(fmt.Sprintf("expected pubkey length 33, got %d", len(pubKeyBytes)))
		}

		pubKeysBytes = append(pubKeysBytes, pubKeyBytes)

		sortByteArrays(pubKeysBytes)
	}

	for _, pubKeyBytes := range pubKeysBytes {
		key, err := btcutil.NewAddressPubKey(pubKeyBytes, d.network.ChainConfig())
		PanicOnError(err)
		pubKeys = append(pubKeys, key)
	}

	multiSigScript, err := txscript.MultiSigScript(pubKeys, d.m)
	PanicOnError(err)

	sha := sha256.Sum256(multiSigScript)

	segWitScriptBuilder := txscript.NewScriptBuilder()
	segWitScriptBuilder.AddOp(txscript.OP_0)
	segWitScriptBuilder.AddData(sha[:])
	segWitScript, err := segWitScriptBuilder.Script()
	PanicOnError(err)

	addrScriptHash, err := btcutil.NewAddressScriptHash(segWitScript, d.network.ChainConfig())
	PanicOnError(err)

	return addrScriptHash.EncodeAddress()
}

// implement `Interface` in sort package.
type sortedByteArrays [][]byte

func (b sortedByteArrays) Len() int {
	return len(b)
}

func (b sortedByteArrays) Less(i, j int) bool {
	// bytes package already implements Comparable for []byte.
	switch bytes.Compare(b[i], b[j]) {
	case -1:
		return true
	case 0, 1:
		return false
	default:
		log.Panic("not fail-able with `bytes.Comparable` bounded [-1, 1].")
		return false
	}
}

func (b sortedByteArrays) Swap(i, j int) {
	b[j], b[i] = b[i], b[j]
}

func sortByteArrays(src [][]byte) [][]byte {
	sorted := sortedByteArrays(src)
	sort.Sort(sorted)
	return sorted
}

// as per SLIP-0044 https://github.com/satoshilabs/slips/blob/master/slip-0044.md
func coinType(n Network) string {
	switch n {
	case Mainnet:
		return "0'"
	case Testnet:
		return "1'"
	default:
		panic("unreachable")
	}
}
