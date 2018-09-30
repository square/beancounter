// Copyright (c) 2018 The bcext developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package cashutil

import (
	"bytes"
	"strings"
)

// The cashaddr character set for encoding.
const charset = "qpzry9x8gf2tvdw0s3jn54khce6mua7l"

// The cashaddr character set for decoding.
var charsetDecoder = [128]int{-1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1,
	-1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1,
	-1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1,
	-1, 15, -1, 10, 17, 21, 20, 26, 30, 7, 5, -1, -1, -1, -1, -1, -1, -1, 29,
	-1, 24, 13, 25, 9, 8, 23, -1, 18, 22, 31, 27, 19, -1, 1, 0, 3, 16, 11, 28,
	12, 14, 6, 4, 2, -1, -1, -1, -1, -1, -1, 29, -1, 24, 13, 25, 9, 8, 23, -1,
	18, 22, 31, 27, 19, -1, 1, 0, 3, 16, 11, 28, 12, 14, 6, 4, 2, -1, -1, -1,
	-1, -1}

// encode a cashaddr string.
func encode(prefix string, payload []byte) string {
	checkSum := createChecksum(prefix, payload)
	combined := cat(payload, checkSum)

	buf := bytes.NewBuffer(make([]byte, 0, len(prefix)+len(combined)+1))
	buf.WriteString(prefix + ":")
	for _, char := range combined {
		buf.WriteString(string(charset[char]))
	}
	return buf.String()
}

// decode a cashaddr string.
func decode(str, defaultPrefix string) (string, []byte) {
	// Go over the string and do some sanity checks.
	var lower, upper, hasNumber bool
	prefixSize := 0
	for pos, char := range str {
		if char >= 'a' && char <= 'z' {
			lower = true
			continue
		}

		if char >= 'A' && char <= 'Z' {
			upper = true
			continue
		}

		if char >= '0' && char <= '9' {
			// We cannot have numbers in the prefix.
			hasNumber = true
			continue
		}

		if char == ':' {
			// The separator cannot be the first character, cannot have number
			// and there must not be 2 separators.
			if hasNumber || pos == 0 || prefixSize != 0 {
				return "", nil
			}

			prefixSize = pos
			continue
		}

		// We have an unexpected character.
		return "", nil
	}

	// We can't have both upper case and lowercase.
	if upper && lower {
		return "", nil
	}

	// Get the prefix
	var prefix string
	if prefixSize == 0 {
		prefix = defaultPrefix
	} else {
		buf := bytes.NewBuffer(make([]byte, 0, prefixSize))
		for i := 0; i < prefixSize; i++ {
			buf.WriteString(strings.ToLower(string(str[i])))
		}
		prefix = buf.String()

		// Now add the ":" in the size
		prefixSize++
	}

	//Decode values
	valueSize := len(str) - prefixSize
	values := make([]byte, valueSize)
	for i := 0; i < valueSize; i++ {
		c := str[i+prefixSize]
		// We have an invalid char in there.
		if c > 127 || charsetDecoder[c] == -1 {
			return "", nil
		}

		values[i] = byte(charsetDecoder[c])
	}

	// Verify the checksum.
	if !verifyChecksum(prefix, values) {
		return "", nil
	}

	return prefix, values[:len(values)-8]
}

// CreateChecksum Create a checksum.
func createChecksum(prefix string, payload []byte) []byte {
	enc := cat(expandPrefix(prefix), payload)
	// Append 8 zeroes
	enc = append(enc, []byte{0, 0, 0, 0, 0, 0, 0, 0}...)
	// Determine what to XOR into those 8 zeroes.
	mod := polyMod(enc)

	ret := make([]byte, 8)
	for i := 0; i < 8; i++ {
		// Convert the 5-bit groups in mod to checksum values.
		ret[i] = byte((mod >> (5 * (7 - uint(i)))) & 0x1f)
	}

	return ret
}

// Concatenate two byte arrays.
func cat(x, y []byte) []byte {
	buf := bytes.NewBuffer(make([]byte, 0, len(x)+len(y)))
	buf.Write(x)
	buf.Write(y)
	return buf.Bytes()
}

func expandPrefix(prefix string) []byte {
	ret := make([]byte, len(prefix)+1)
	for pos, char := range prefix {
		ret[pos] = byte(char) & 0x1f
	}

	ret[len(ret)-1] = 0
	return ret
}

//This function will compute what 8 5-bit values to XOR into the last 8 input
//values, in order to make the checksum 0. These 8 values are packed together
//in a single 40-bit integer. The higher bits correspond to earlier values.
func polyMod(v []byte) uint64 {
	// The input is interpreted as a list of coefficients of a polynomial over F
	// = GF(32), with an implicit 1 in front. If the input is [v0,v1,v2,v3,v4],
	// that polynomial is v(x) = 1*x^5 + v0*x^4 + v1*x^3 + v2*x^2 + v3*x + v4.
	// 	The implicit 1 guarantees that [v0,v1,v2,...] has a distinct checksum
	// from [0,v0,v1,v2,...].
	// The output is a 40-bit integer whose 5-bit groups are the coefficients of
	// the remainder of v(x) mod g(x), where g(x) is the cashaddr generator, x^8
	// + {19}*x^7 + {3}*x^6 + {25}*x^5 + {11}*x^4 + {25}*x^3 + {3}*x^2 + {19}*x
	// + {1}. g(x) is chosen in such a way that the resulting code is a BCH
	// code, guaranteeing detection of up to 4 errors within a window of 1025
	// characters. Among the various possible BCH codes, one was selected to in
	// fact guarantee detection of up to 5 errors within a window of 160
	// characters and 6 erros within a window of 126 characters. In addition,
	// 	the code guarantee the detection of a burst of up to 8 errors.
	// 	Note that the coefficients are elements of GF(32), here represented as
	// decimal numbers between {}. In this finite field, addition is just XOR of
	// the corresponding numbers. For example, {27} + {13} = {27 ^ 13} = {22}.
	// Multiplication is more complicated, and requires treating the bits of
	// values themselves as coefficients of a polynomial over a smaller field,
	// 	GF(2), and multiplying those polynomials mod a^5 + a^3 + 1. For example,
	// {5} * {26} = (a^2 + 1) * (a^4 + a^3 + a) = (a^4 + a^3 + a) * a^2 + (a^4 +
	// 	a^3 + a) = a^6 + a^5 + a^4 + a = a^3 + 1 (mod a^5 + a^3 + 1) = {9}.
	// During the course of the loop below, `c` contains the bitpacked
	// coefficients of the polynomial constructed from just the values of v that
	// were processed so far, mod g(x). In the above example, `c` initially
	// corresponds to 1 mod (x), and after processing 2 inputs of v, it
	// corresponds to x^2 + v0*x + v1 mod g(x). As 1 mod g(x) = 1, that is the
	// starting value for `c`.
	c := uint64(1)
	for _, char := range v {
		// We want to update `c` to correspond to a polynomial with one extra
		// term. If the initial value of `c` consists of the coefficients of
		// c(x) = f(x) mod g(x), we modify it to correspond to
		// c'(x) = (f(x) * x + d) mod g(x), where d is the next input to
		// process.
		// 	Simplifying:
		// c'(x) = (f(x) * x + d) mod g(x)
		// ((f(x) mod g(x)) * x + d) mod g(x)
		// (c(x) * x + d) mod g(x)
		// If c(x) = c0*x^5 + c1*x^4 + c2*x^3 + c3*x^2 + c4*x + c5, we want to
		// compute
		// c'(x) = (c0*x^5 + c1*x^4 + c2*x^3 + c3*x^2 + c4*x + c5) * x + d
		// mod g(x)
		// = c0*x^6 + c1*x^5 + c2*x^4 + c3*x^3 + c4*x^2 + c5*x + d
		// mod g(x)
		// = c0*(x^6 mod g(x)) + c1*x^5 + c2*x^4 + c3*x^3 + c4*x^2 +
		// 	c5*x + d
		// If we call (x^6 mod g(x)) = k(x), this can be written as
		// c'(x) = (c1*x^5 + c2*x^4 + c3*x^3 + c4*x^2 + c5*x + d) + c0*k(x)

		// First, determine the value of c0:
		c0 := byte(c >> 35)

		// Then compute c1*x^5 + c2*x^4 + c3*x^3 + c4*x^2 + c5*x + d:
		c = ((c & 0x07ffffffff) << 5) ^ uint64(char)

		// Finally, for each set bit n in c0, conditionally add {2^n}k(x):
		if c0&0x01 != 0 {
			// k(x) = {19}*x^7 + {3}*x^6 + {25}*x^5 + {11}*x^4 + {25}*x^3 +
			//        {3}*x^2 + {19}*x + {1}
			c ^= 0x98f2bc8e61
		}

		if c0&0x2 != 0 {
			// {2}k(x) = {15}*x^7 + {6}*x^6 + {27}*x^5 + {22}*x^4 + {27}*x^3 +
			//           {6}*x^2 + {15}*x + {2}
			c ^= 0x79b76d99e2
		}

		if c0&0x04 != 0 {
			// {4}k(x) = {30}*x^7 + {12}*x^6 + {31}*x^5 + {5}*x^4 + {31}*x^3 +
			//           {12}*x^2 + {30}*x + {4}
			c ^= 0xf33e5fb3c4
		}

		if c0&0x08 != 0 {
			// {8}k(x) = {21}*x^7 + {24}*x^6 + {23}*x^5 + {10}*x^4 + {23}*x^3 +
			//           {24}*x^2 + {21}*x + {8}
			c ^= 0xae2eabe2a8
		}

		if c0&0x10 != 0 {
			// {16}k(x) = {3}*x^7 + {25}*x^6 + {7}*x^5 + {20}*x^4 + {7}*x^3 +
			//            {25}*x^2 + {3}*x + {16}
			c ^= 0x1e4f43e470
		}
	}

	// PolyMod computes what value to xor into the final values to make the
	// checksum 0. However, if we required that the checksum was 0, it would be
	// the case that appending a 0 to a valid list of values would result in a
	// new valid list. For that reason, cashaddr requires the resulting checksum
	// to be 1 instead.
	return c ^ 1
}

func verifyChecksum(prefix string, payload []byte) bool {
	return polyMod(cat(expandPrefix(prefix), payload)) == 0
}
