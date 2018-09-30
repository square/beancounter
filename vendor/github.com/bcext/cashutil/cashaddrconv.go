// Copyright (c) 2018 The bcext developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package cashutil

import (
	"bytes"
	"errors"
	"strings"

	"github.com/bcext/gcash/chaincfg"
)

type addrType uint8

const (
	pubKeyType addrType = iota
	scriptType
)

var (
	errEmptyAddressContent = errors.New("empty address content")
	errInvalidHash160Size  = errors.New("invalid hash160 size")
	errInvalidAddressType  = errors.New("invalid address type")
)

type addrContent struct {
	t    addrType
	hash []byte
}

func encodeCashAddr(dst Address) string {
	switch addr := dst.(type) {
	case *AddressPubKeyHash:
		data := packAddrData(addr.Hash160()[:], uint8(pubKeyType))
		return encode(addr.net.CashAddrPrefix, data)
	case *AddressScriptHash:
		data := packAddrData(addr.Hash160()[:], uint8(scriptType))
		return encode(addr.net.CashAddrPrefix, data)
	default:
		return ""
	}
}

func decodeCashAddr(addr string, param *chaincfg.Params) (Address, error) {
	// handle bitcoin address prefixed with net tag
	if strings.Contains(addr, ":") {
		pos := strings.LastIndex(addr, ":")
		addr = addr[pos+1:]
	}

	content := decodeCashaddrContent(addr, param)
	if content == nil || len(content.hash) == 0 {
		return nil, errEmptyAddressContent
	}

	return decodeCashAddrDestination(content, param)
}

// Convert the data part to a 5 bit representation.
func packAddrData(id []byte, t uint8) []byte {
	version := t << 3
	size := len(id)
	var encodedSize uint8
	switch size * 8 {
	case 160:
		encodedSize = 0
	case 192:
		encodedSize = 1
	case 224:
		encodedSize = 2
	case 256:
		encodedSize = 3
	case 320:
		encodedSize = 4
	case 384:
		encodedSize = 5
	case 448:
		encodedSize = 6
	case 512:
		encodedSize = 7
	default:
		panic("Error packing cashaddr: invalid address length")
	}

	version |= encodedSize
	data := bytes.NewBuffer(make([]byte, 0, len(id)+1))
	data.WriteByte(version)
	data.Write(id)

	// Reserve the number of bytes required for a 5-bit packed version of a
	// hash, with version byte.  Add half a byte(4) so integer math provides
	// the next multiple-of-5 that would fit all the data.
	ret, _ := convertBits(8, 5, true, data.Bytes())

	return ret
}

func convertBits(frombits uint, tobits uint, pad bool, data []byte) ([]byte, bool) {
	var acc, bits int
	maxv := (1 << tobits) - 1
	maxAcc := (1 << (frombits + tobits - 1)) - 1

	ret := bytes.NewBuffer(nil)
	for _, bit := range data {
		acc = ((acc << frombits) | int(bit)) & maxAcc
		bits += int(frombits)

		for bits >= int(tobits) {
			bits -= int(tobits)
			ret.WriteByte(byte((acc >> uint(bits)) & maxv))
		}
	}

	// We have remaining bits to encode but do not pad.
	if !pad && bits != 0 {
		return ret.Bytes(), false
	}

	// We have remaining bits to encode so we do pad.
	if pad && bits != 0 {
		ret.WriteByte(byte(acc << (tobits - uint(bits)) & maxv))
	}

	return ret.Bytes(), true
}

func decodeCashaddrContent(addr string, param *chaincfg.Params) *addrContent {
	prefix, payload := decode(addr, param.CashAddrPrefix)
	if prefix != param.CashAddrPrefix {
		return nil
	}

	if len(payload) == 0 {
		return nil
	}

	// Check that the padding is zero.
	extraBits := len(payload) * 5 % 8
	if extraBits >= 5 {
		// We have more padding than allowed.
		return nil
	}

	last := payload[len(payload)-1]
	mask := 1<<uint(extraBits) - 1
	if int(last)&mask != 0 {
		// We have non zero bits as padding.
		return nil
	}

	data, _ := convertBits(5, 8, false, payload)

	// Decode type and size from the version.
	version := data[0]
	if version&0x80 != 0 {
		// First bit is reserved.
		return nil
	}

	t := addrType((version >> 3) & 0x1f)
	hashSize := 20 + 4*(version&0x03)
	if version&0x04 != 0 {
		hashSize *= 2
	}

	// Check that we decoded the exact number of bytes we expected.
	if len(data) != int(hashSize)+1 {
		return nil
	}

	// Pop the version.
	data = data[1:]

	return &addrContent{t, data}
}

func decodeCashAddrDestination(content *addrContent, params *chaincfg.Params) (Address, error) {
	if len(content.hash) != 20 {
		return nil, errInvalidHash160Size
	}

	switch content.t {
	case pubKeyType:
		addr, err := NewAddressPubKeyHash(content.hash, params)
		if err != nil {
			return nil, err
		}
		return addr, nil
	case scriptType:
		addr, err := NewAddressScriptHashFromHash(content.hash, params)
		if err != nil {
			return nil, err
		}
		return addr, nil
	default:
		return nil, errInvalidAddressType
	}
}
