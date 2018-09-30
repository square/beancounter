// Copyright (c) 2013-2017 The btcsuite developers
// Copyright (c) 2018 The bcext developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package cashutil

import (
	"encoding/hex"
	"errors"

	"github.com/bcext/cashutil/base58"
	"github.com/bcext/gcash/btcec"
	"github.com/bcext/gcash/chaincfg"
	"golang.org/x/crypto/ripemd160"
)

var (
	// ErrChecksumMismatch describes an error where decoding failed due
	// to a bad checksum.
	ErrChecksumMismatch = errors.New("checksum mismatch")

	// ErrUnknownAddressType describes an error where an address can not
	// decoded as a specific address type due to the string encoding
	// begining with an identifier byte unknown to any standard or
	// registered (via chaincfg.Register) network.
	ErrUnknownAddressType = errors.New("unknown address type")

	// ErrAddressCollision describes an error where an address can not
	// be uniquely determined as either a pay-to-pubkey-hash or
	// pay-to-script-hash address since the leading identifier is used for
	// describing both address kinds, but for different networks.  Rather
	// than assuming or defaulting to one or the other, this error is
	// returned and the caller must decide how to decode the address.
	ErrAddressCollision = errors.New("address collision")
)

// encodeAddress returns a human-readable payment address given a ripemd160 hash
// and netID which encodes the bitcoin network and address type.  It is used
// in both pay-to-pubkey-hash (P2PKH) and pay-to-script-hash (P2SH) address
// encoding.
func encodeAddress(hash160 []byte, netID byte) string {
	// Format is 1 byte for a network and address class (i.e. P2PKH vs
	// P2SH), 20 bytes for a RIPEMD160 hash, and 4 bytes of checksum.
	return base58.CheckEncode(hash160[:ripemd160.Size], netID)
}

// Address is an interface type for any type of destination a transaction
// output may spend to.  This includes pay-to-pubkey (P2PK), pay-to-pubkey-hash
// (P2PKH), and pay-to-script-hash (P2SH).  Address is designed to be generic
// enough that other kinds of addresses may be added in the future without
// changing the decoding and encoding API.
type Address interface {
	// String returns the string encoding of the transaction output
	// destination.
	//
	// Please note that String differs subtly from EncodeAddress: String
	// will return the value as a string without any conversion, while
	// EncodeAddress may convert destination types (for example,
	// converting pubkeys to P2PKH addresses) before encoding as a
	// payment address string.
	String() string

	// EncodeAddress returns the string encoding of the payment address
	// associated with the Address value.  See the comment on String
	// for how this method differs from String.
	// Support base58 and base32 encoding according to the parameter.
	EncodeAddress(cashaddr bool) string

	// ScriptAddress returns the raw bytes of the address to be used
	// when inserting the address into a txout's script.
	ScriptAddress() []byte

	// IsForNet returns whether or not the address is associated with the
	// passed bitcoin network.
	IsForNet(*chaincfg.Params) bool
}

// DecodeAddress decodes the string encoding of an address and returns
// the Address if addr is a valid encoding for a known address type.
//
// The bitcoin network the address is associated with is extracted if possible.
// When the address does not encode the network, such as in the case of a raw
// public key, the address will be associated with the passed defaultNet.
func DecodeAddress(addr string, defaultNet *chaincfg.Params) (Address, error) {
	// bitcoin cash base32-encoded address:
	// 1. mainnet:  bitcoincash: + 42 bytes = 12 + 42
	// 2. testnet3: bchtest:     + 42 bytes = 8  + 42
	// 3. regtest:  bchreg:      + 42 bytes = 7  + 42
	// if the address does not have the prefix: length = 42
	// if the address have the prefix: max length = 42 + 12
	if len(addr) >= 42 && len(addr) <= 42+12 {
		return decodeCashAddr(addr, defaultNet)
	}

	// Serialized public keys are either 65 bytes (130 hex chars) if
	// uncompressed/hybrid or 33 bytes (66 hex chars) if compressed.
	if len(addr) == 130 || len(addr) == 66 {
		serializedPubKey, err := hex.DecodeString(addr)
		if err != nil {
			return nil, err
		}
		return NewAddressPubKey(serializedPubKey, defaultNet)
	}

	// Switch on decoded length to determine the type.
	decoded, netID, err := base58.CheckDecode(addr)
	if err != nil {
		if err == base58.ErrChecksum {
			return nil, ErrChecksumMismatch
		}
		return nil, errors.New("decoded address is of unknown format")
	}
	switch len(decoded) {
	case ripemd160.Size: // P2PKH or P2SH
		isP2PKH := chaincfg.IsPubKeyHashAddrID(netID)
		isP2SH := chaincfg.IsScriptHashAddrID(netID)

		errWrongNet := errors.New("wrong address from different chainparam")
		switch hash160 := decoded; {
		case isP2PKH && isP2SH:
			return nil, ErrAddressCollision
		case isP2PKH:
			if netID != defaultNet.PubKeyHashAddrID {
				return nil, errWrongNet
			}
			return newAddressPubKeyHash(hash160, defaultNet)
		case isP2SH:
			if netID != defaultNet.ScriptHashAddrID {
				return nil, errWrongNet
			}
			return newAddressScriptHashFromHash(hash160, defaultNet)
		default:
			return nil, ErrUnknownAddressType
		}

	default:
		return nil, errors.New("decoded address is of unknown size")
	}
}

// AddressPubKeyHash is an Address for a pay-to-pubkey-hash (P2PKH)
// transaction.
type AddressPubKeyHash struct {
	hash [ripemd160.Size]byte
	net  *chaincfg.Params
}

// NewAddressPubKeyHash returns a new AddressPubKeyHash.  pkHash must be 20
// bytes.
func NewAddressPubKeyHash(pkHash []byte, param *chaincfg.Params) (*AddressPubKeyHash, error) {
	return newAddressPubKeyHash(pkHash, param)
}

// newAddressPubKeyHash is the internal API to create a pubkey hash address
// with a known leading identifier byte for a network, rather than looking
// it up through its parameters.  This is useful when creating a new address
// structure from a string encoding where the identifier byte is already
// known.
func newAddressPubKeyHash(pkHash []byte, param *chaincfg.Params) (*AddressPubKeyHash, error) {
	// Check for a valid pubkey hash length.
	if len(pkHash) != ripemd160.Size {
		return nil, errors.New("pkHash must be 20 bytes")
	}

	addr := &AddressPubKeyHash{net: param}
	copy(addr.hash[:], pkHash)
	return addr, nil
}

// EncodeAddress returns the string encoding of a pay-to-pubkey-hash
// address.  Part of the Address interface.
func (a *AddressPubKeyHash) EncodeAddress(cashaddr bool) string {
	if cashaddr {
		return encodeCashAddr(a)
	}

	return encodeAddress(a.hash[:], a.net.PubKeyHashAddrID)
}

// ScriptAddress returns the bytes to be included in a txout script to pay
// to a pubkey hash.  Part of the Address interface.
func (a *AddressPubKeyHash) ScriptAddress() []byte {
	return a.hash[:]
}

// IsForNet returns whether or not the pay-to-pubkey-hash address is associated
// with the passed bitcoin network.
func (a *AddressPubKeyHash) IsForNet(net *chaincfg.Params) bool {
	return a.net.PubKeyHashAddrID == net.PubKeyHashAddrID
}

// String returns a human-readable string for the pay-to-pubkey-hash address.
// This is equivalent to calling EncodeAddress, but is provided so the type can
// be used as a fmt.Stringer.
// Use cashaddr encoding default.
func (a *AddressPubKeyHash) String() string {
	return a.EncodeAddress(true)
}

// Hash160 returns the underlying array of the pubkey hash.  This can be useful
// when an array is more appropiate than a slice (for example, when used as map
// keys).
func (a *AddressPubKeyHash) Hash160() *[ripemd160.Size]byte {
	return &a.hash
}

// AddressScriptHash is an Address for a pay-to-script-hash (P2SH)
// transaction.
type AddressScriptHash struct {
	hash [ripemd160.Size]byte
	net  *chaincfg.Params
}

// NewAddressScriptHash returns a new AddressScriptHash.
func NewAddressScriptHash(serializedScript []byte, param *chaincfg.Params) (*AddressScriptHash, error) {
	scriptHash := Hash160(serializedScript)
	return newAddressScriptHashFromHash(scriptHash, param)
}

// NewAddressScriptHashFromHash returns a new AddressScriptHash.  scriptHash
// must be 20 bytes.
func NewAddressScriptHashFromHash(scriptHash []byte, param *chaincfg.Params) (*AddressScriptHash, error) {
	return newAddressScriptHashFromHash(scriptHash, param)
}

// newAddressScriptHashFromHash is the internal API to create a script hash
// address with a known leading identifier byte for a network, rather than
// looking it up through its parameters.  This is useful when creating a new
// address structure from a string encoding where the identifer byte is already
// known.
func newAddressScriptHashFromHash(scriptHash []byte, param *chaincfg.Params) (*AddressScriptHash, error) {
	// Check for a valid script hash length.
	if len(scriptHash) != ripemd160.Size {
		return nil, errors.New("scriptHash must be 20 bytes")
	}

	addr := &AddressScriptHash{net: param}
	copy(addr.hash[:], scriptHash)
	return addr, nil
}

// EncodeAddress returns the string encoding of a pay-to-script-hash
// address.  Part of the Address interface.
func (a *AddressScriptHash) EncodeAddress(cashaddr bool) string {
	if cashaddr {
		return encodeCashAddr(a)
	}

	return encodeAddress(a.hash[:], a.net.ScriptHashAddrID)
}

// ScriptAddress returns the bytes to be included in a txout script to pay
// to a script hash.  Part of the Address interface.
func (a *AddressScriptHash) ScriptAddress() []byte {
	return a.hash[:]
}

// IsForNet returns whether or not the pay-to-script-hash address is associated
// with the passed bitcoin network.
func (a *AddressScriptHash) IsForNet(net *chaincfg.Params) bool {
	return a.net.ScriptHashAddrID == net.ScriptHashAddrID
}

// String returns a human-readable string for the pay-to-script-hash address.
// This is equivalent to calling EncodeAddress, but is provided so the type can
// be used as a fmt.Stringer.
// User cashaddr encoding default.
func (a *AddressScriptHash) String() string {
	return a.EncodeAddress(true)
}

// Hash160 returns the underlying array of the script hash.  This can be useful
// when an array is more appropiate than a slice (for example, when used as map
// keys).
func (a *AddressScriptHash) Hash160() *[ripemd160.Size]byte {
	return &a.hash
}

// PubKeyFormat describes what format to use for a pay-to-pubkey address.
type PubKeyFormat int

const (
	// PKFUncompressed indicates the pay-to-pubkey address format is an
	// uncompressed public key.
	PKFUncompressed PubKeyFormat = iota

	// PKFCompressed indicates the pay-to-pubkey address format is a
	// compressed public key.
	PKFCompressed

	// PKFHybrid indicates the pay-to-pubkey address format is a hybrid
	// public key.
	PKFHybrid
)

// AddressPubKey is an Address for a pay-to-pubkey transaction.
type AddressPubKey struct {
	pubKeyFormat PubKeyFormat
	pubKey       *btcec.PublicKey
	net          *chaincfg.Params
}

// NewAddressPubKey returns a new AddressPubKey which represents a pay-to-pubkey
// address.  The serializedPubKey parameter must be a valid pubkey and can be
// uncompressed, compressed, or hybrid.
func NewAddressPubKey(serializedPubKey []byte, param *chaincfg.Params) (*AddressPubKey, error) {
	pubKey, err := btcec.ParsePubKey(serializedPubKey, btcec.S256())
	if err != nil {
		return nil, err
	}

	// Set the format of the pubkey.  This probably should be returned
	// from btcec, but do it here to avoid API churn.  We already know the
	// pubkey is valid since it parsed above, so it's safe to simply examine
	// the leading byte to get the format.
	pkFormat := PKFUncompressed
	switch serializedPubKey[0] {
	case 0x02, 0x03:
		pkFormat = PKFCompressed
	case 0x06, 0x07:
		pkFormat = PKFHybrid
	}

	return &AddressPubKey{
		pubKeyFormat: pkFormat,
		pubKey:       pubKey,
		net:          param,
	}, nil
}

// serialize returns the serialization of the public key according to the
// format associated with the address.
func (a *AddressPubKey) serialize() []byte {
	switch a.pubKeyFormat {
	default:
		fallthrough
	case PKFUncompressed:
		return a.pubKey.SerializeUncompressed()

	case PKFCompressed:
		return a.pubKey.SerializeCompressed()

	case PKFHybrid:
		return a.pubKey.SerializeHybrid()
	}
}

// EncodeAddress returns the string encoding of the public key as a
// pay-to-pubkey-hash.  Note that the public key format (uncompressed,
// compressed, etc) will change the resulting address.  This is expected since
// pay-to-pubkey-hash is a hash of the serialized public key which obviously
// differs with the format.  At the time of this writing, most Bitcoin addresses
// are pay-to-pubkey-hash constructed from the uncompressed public key.
//
// Part of the Address interface.
func (a *AddressPubKey) EncodeAddress(cashaddr bool) string {
	if cashaddr {
		p2pkh, err := newAddressPubKeyHash(Hash160(a.serialize()), a.net)
		if err != nil {
			return ""
		}
		return encodeCashAddr(p2pkh)
	}

	return encodeAddress(Hash160(a.serialize()), a.net.PubKeyHashAddrID)
}

// ScriptAddress returns the bytes to be included in a txout script to pay
// to a public key.  Setting the public key format will affect the output of
// this function accordingly.  Part of the Address interface.
func (a *AddressPubKey) ScriptAddress() []byte {
	return a.serialize()
}

// IsForNet returns whether or not the pay-to-pubkey address is associated
// with the passed bitcoin network.
func (a *AddressPubKey) IsForNet(net *chaincfg.Params) bool {
	return a.net.PubKeyHashAddrID == net.PubKeyHashAddrID
}

// String returns the hex-encoded human-readable string for the pay-to-pubkey
// address.  This is not the same as calling EncodeAddress.
func (a *AddressPubKey) String() string {
	return hex.EncodeToString(a.serialize())
}

// Format returns the format (uncompressed, compressed, etc) of the
// pay-to-pubkey address.
func (a *AddressPubKey) Format() PubKeyFormat {
	return a.pubKeyFormat
}

// SetFormat sets the format (uncompressed, compressed, etc) of the
// pay-to-pubkey address.
func (a *AddressPubKey) SetFormat(pkFormat PubKeyFormat) {
	a.pubKeyFormat = pkFormat
}

// AddressPubKeyHash returns the pay-to-pubkey address converted to a
// pay-to-pubkey-hash address.  Note that the public key format (uncompressed,
// compressed, etc) will change the resulting address.  This is expected since
// pay-to-pubkey-hash is a hash of the serialized public key which obviously
// differs with the format.  At the time of this writing, most Bitcoin addresses
// are pay-to-pubkey-hash constructed from the uncompressed public key.
func (a *AddressPubKey) AddressPubKeyHash() *AddressPubKeyHash {
	addr := &AddressPubKeyHash{net: a.net}
	copy(addr.hash[:], Hash160(a.serialize()))
	return addr
}

// PubKey returns the underlying public key for the address.
func (a *AddressPubKey) PubKey() *btcec.PublicKey {
	return a.pubKey
}
