package deriver

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"log"
	"sort"

	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcutil"
	"github.com/btcsuite/btcutil/hdkeychain"

	. "github.com/square/beancounter/utils"
)

// AddressDeriver ...
type AddressDeriver struct {
	network Network
	xpubs   []string
	m       int
	account uint32
}

// NewAddressDeriver ...
func NewAddressDeriver(network Network, xpubs []string, m int, account uint32) *AddressDeriver {
	return &AddressDeriver{
		network: network,
		xpubs:   xpubs,
		m:       m,
		account: account,
	}
}

// Derive ...
func (d *AddressDeriver) Derive(change uint32, addressIndex uint32) string {
	if len(d.xpubs) == 1 {
		return d.singleDerive(change, addressIndex)
	}
	return d.multiSigSegwitDerive(change, addressIndex)
}

func (d *AddressDeriver) singleDerive(change uint32, addressIndex uint32) string {
	key, err := hdkeychain.NewKeyFromString(d.xpubs[0])
	PanicOnError(err)

	key, err = key.Child(d.account)
	PanicOnError(err)

	key, err = key.Child(change)
	PanicOnError(err)

	key, err = key.Child(addressIndex)
	PanicOnError(err)

	pubKey, err := key.Address(d.chainConfig())
	PanicOnError(err)

	return pubKey.String()
}

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
		key, err := btcutil.NewAddressPubKey(pubKeyBytes, d.chainConfig())
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

	addrScriptHash, err := btcutil.NewAddressScriptHash(segWitScript, d.chainConfig())
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

func (d *AddressDeriver) chainConfig() *chaincfg.Params {
	switch d.network {
	case Mainnet:
		return &chaincfg.MainNetParams
	case Testnet:
		return &chaincfg.TestNet3Params
	default:
		panic("unreachable")
	}
}
