// Copyright (c) 2013, 2014 The btcsuite developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package cashutil

import (
	"errors"
	"math"
	"strconv"
)

// AmountUnit describes a method of converting an Amount to something
// other than the base unit of a bitcoin.  The value of the AmountUnit
// is the exponent component of the decadic multiple to convert from
// an amount in bitcoin to an amount counted in units.
type AmountUnit int

// These constants define various units used when describing a bitcoin
// monetary amount.
const (
	AmountMegaBCH  AmountUnit = 6
	AmountKiloBCH  AmountUnit = 3
	AmountBCH      AmountUnit = 0
	AmountMilliBCH AmountUnit = -3
	AmountMicroBCH AmountUnit = -6
	AmountSatoshi  AmountUnit = -8
)

// String returns the unit as a string.  For recognized units, the SI
// prefix is used, or "Satoshi" for the base unit.  For all unrecognized
// units, "1eN BCH" is returned, where N is the AmountUnit.
func (u AmountUnit) String() string {
	switch u {
	case AmountMegaBCH:
		return "MBCH"
	case AmountKiloBCH:
		return "kBCH"
	case AmountBCH:
		return "BCH"
	case AmountMilliBCH:
		return "mBCH"
	case AmountMicroBCH:
		return "μBCH"
	case AmountSatoshi:
		return "Satoshi"
	default:
		return "1e" + strconv.FormatInt(int64(u), 10) + " BCH"
	}
}

// Amount represents the base bitcoin monetary unit (colloquially referred
// to as a `Satoshi').  A single Amount is equal to 1e-8 of a bitcoin.
type Amount int64

// round converts a floating point number, which may or may not be representable
// as an integer, to the Amount integer type by rounding to the nearest integer.
// This is performed by adding or subtracting 0.5 depending on the sign, and
// relying on integer truncation to round the value to the nearest Amount.
func round(f float64) Amount {
	if f < 0 {
		return Amount(f - 0.5)
	}
	return Amount(f + 0.5)
}

// NewAmount creates an Amount from a floating point value representing
// some value in bitcoin.  NewAmount errors if f is NaN or +-Infinity, but
// does not check that the amount is within the total amount of bitcoin
// producible as f may not refer to an amount at a single moment in time.
//
// NewAmount is for specifically for converting BCH to Satoshi.
// For creating a new Amount with an int64 value which denotes a quantity of Satoshi,
// do a simple type conversion from type int64 to Amount.
// See GoDoc for example: http://godoc.org/github.com/bcext/cashutil#example-Amount
func NewAmount(f float64) (Amount, error) {
	// The amount is only considered invalid if it cannot be represented
	// as an integer type.  This may happen if f is NaN or +-Infinity.
	switch {
	case math.IsNaN(f):
		fallthrough
	case math.IsInf(f, 1):
		fallthrough
	case math.IsInf(f, -1):
		return 0, errors.New("invalid bitcoin amount")
	}

	return round(f * SatoshiPerBitcoin), nil
}

// ToUnit converts a monetary amount counted in bitcoin base units to a
// floating point value representing an amount of bitcoin.
func (a Amount) ToUnit(u AmountUnit) float64 {
	return float64(a) / math.Pow10(int(u+8))
}

// ToBCH is the equivalent of calling ToUnit with AmountBCH.
func (a Amount) ToBCH() float64 {
	return a.ToUnit(AmountBCH)
}

// Format formats a monetary amount counted in bitcoin base units as a
// string for a given unit.  The conversion will succeed for any unit,
// however, known units will be formated with an appended label describing
// the units with SI notation, or "Satoshi" for the base unit.
func (a Amount) Format(u AmountUnit) string {
	units := " " + u.String()
	return strconv.FormatFloat(a.ToUnit(u), 'f', -int(u+8), 64) + units
}

// String is the equivalent of calling Format with AmountBCH.
func (a Amount) String() string {
	return a.Format(AmountBCH)
}

// MulF64 multiplies an Amount by a floating point value.  While this is not
// an operation that must typically be done by a full node or wallet, it is
// useful for services that build on top of bitcoin (for example, calculating
// a fee by multiplying by a percentage).
func (a Amount) MulF64(f float64) Amount {
	return round(float64(a) * f)
}
