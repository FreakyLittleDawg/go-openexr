// Package half provides IEEE 754 binary16 half-precision floating-point numbers.
//
// Half-precision floats use 16 bits with the following layout:
//   - 1 bit sign
//   - 5 bits exponent (bias of 15)
//   - 10 bits mantissa (implicit leading 1 for normalized values)
//
// This format is used extensively in OpenEXR files for storing HDR pixel data,
// offering a good balance between dynamic range and storage efficiency.
package half

import (
	"math"
)

// Half represents an IEEE 754 binary16 half-precision floating-point number.
// The underlying storage is a uint16.
type Half uint16

// Constants for half-precision floating-point.
const (
	// Bit layout constants
	signBit      = 0x8000
	exponentMask = 0x7C00
	mantissaMask = 0x03FF

	// Exponent values
	exponentBias = 15
	maxExponent  = 31

	// Special values
	posInf = Half(0x7C00) // +Infinity
	negInf = Half(0xFC00) // -Infinity
	nan    = Half(0x7E00) // Quiet NaN (one of many valid NaN representations)

	// Zero values
	posZero = Half(0x0000)
	negZero = Half(0x8000)

	// Limits
	maxHalf = Half(0x7BFF) // Largest positive finite value (~65504)
	minHalf = Half(0xFBFF) // Most negative finite value (~-65504)

	// Smallest positive values
	minPosNormal    = Half(0x0400) // Smallest positive normalized value (~6.1e-5)
	minPosSubnormal = Half(0x0001) // Smallest positive subnormal value (~5.96e-8)
)

// Common constant values
var (
	// Inf is positive infinity.
	Inf = posInf
	// NegInf is negative infinity.
	NegInf = negInf
	// NaN is a quiet NaN value.
	NaN = nan
	// Zero is positive zero.
	Zero = posZero
	// NegZero is negative zero.
	NegZero = negZero
	// Max is the largest finite positive half-precision value (~65504).
	Max = maxHalf
	// Min is the most negative finite half-precision value (~-65504).
	Min = minHalf
	// SmallestNormal is the smallest positive normalized value (~6.1e-5).
	SmallestNormal = minPosNormal
	// SmallestSubnormal is the smallest positive subnormal value (~5.96e-8).
	SmallestSubnormal = minPosSubnormal
)

// FromFloat32 converts a float32 to a Half using round-to-nearest-even.
func FromFloat32(f float32) Half {
	bits := math.Float32bits(f)
	return fromFloat32Bits(bits)
}

// fromFloat32Bits converts float32 bits to Half.
func fromFloat32Bits(bits uint32) Half {
	sign := uint16((bits >> 16) & signBit)
	exp := int((bits >> 23) & 0xFF)
	mantissa := bits & 0x007FFFFF

	// Handle special cases
	switch {
	case exp == 0xFF: // Inf or NaN
		if mantissa == 0 {
			return Half(sign | uint16(exponentMask))
		}
		// NaN - preserve some mantissa bits
		return Half(sign | uint16(exponentMask) | uint16(mantissa>>13))

	case exp == 0: // Zero or subnormal float32
		// These become zero in half (float32 subnormals are way smaller than half can represent)
		return Half(sign)
	}

	// Adjust exponent from float32 bias (127) to half bias (15)
	exp = exp - 127 + exponentBias

	// Check for overflow
	if exp >= maxExponent {
		return Half(sign | uint16(exponentMask)) // Infinity
	}

	// Check for underflow to zero
	if exp < -10 {
		return Half(sign) // Underflow to zero
	}

	// Check for subnormal half
	if exp <= 0 {
		// Subnormal: add implicit leading bit and shift right
		mantissa = mantissa | 0x00800000
		shift := uint(14 - exp)
		if shift > 24 {
			return Half(sign)
		}

		// Round to nearest even
		halfMantissa := mantissa >> shift
		round := mantissa >> (shift - 1) & 1
		sticky := mantissa & ((1 << (shift - 1)) - 1)

		if round != 0 && (sticky != 0 || (halfMantissa&1) != 0) {
			halfMantissa++
		}

		return Half(sign | uint16(halfMantissa&mantissaMask))
	}

	// Normalized value
	// Round mantissa from 23 bits to 10 bits using round-to-nearest-even
	halfMantissa := mantissa >> 13
	round := (mantissa >> 12) & 1
	sticky := mantissa & 0x0FFF

	if round != 0 && (sticky != 0 || (halfMantissa&1) != 0) {
		halfMantissa++
		// Check for mantissa overflow
		if halfMantissa > mantissaMask {
			halfMantissa = 0
			exp++
			if exp >= maxExponent {
				return Half(sign | uint16(exponentMask)) // Overflow to infinity
			}
		}
	}

	return Half(sign | uint16(exp<<10) | uint16(halfMantissa&mantissaMask))
}

// Float32 converts a Half to a float32.
func (h Half) Float32() float32 {
	bits := h.float32Bits()
	return math.Float32frombits(bits)
}

// float32Bits converts Half to float32 bit representation.
func (h Half) float32Bits() uint32 {
	sign := uint32(h&signBit) << 16
	exp := int((h >> 10) & 0x1F)
	mantissa := uint32(h & mantissaMask)

	switch {
	case exp == 0: // Zero or subnormal
		if mantissa == 0 {
			return sign // Preserve sign of zero
		}
		// Subnormal half to normalized float32
		// Find the leading 1 bit
		for mantissa&0x0400 == 0 {
			mantissa <<= 1
			exp--
		}
		exp++
		mantissa &= mantissaMask
		exp = exp - exponentBias + 127
		return sign | uint32(exp<<23) | (mantissa << 13)

	case exp == maxExponent: // Inf or NaN
		if mantissa == 0 {
			return sign | 0x7F800000 // Infinity
		}
		// NaN - set quiet bit and preserve some mantissa
		return sign | 0x7F800000 | (mantissa << 13) | 0x00400000

	default: // Normalized
		exp = exp - exponentBias + 127
		return sign | uint32(exp<<23) | (mantissa << 13)
	}
}

// FromFloat64 converts a float64 to a Half using round-to-nearest-even.
func FromFloat64(f float64) Half {
	// Convert to float32 first, then to half
	// This is correct because float64 -> float32 -> half rounds correctly
	return FromFloat32(float32(f))
}

// Float64 converts a Half to a float64.
func (h Half) Float64() float64 {
	return float64(h.Float32())
}

// IsNaN returns true if h is a NaN value.
func (h Half) IsNaN() bool {
	return h&exponentMask == exponentMask && h&mantissaMask != 0
}

// IsInf returns true if h is positive or negative infinity.
func (h Half) IsInf() bool {
	return h&0x7FFF == exponentMask
}

// IsPosInf returns true if h is positive infinity.
func (h Half) IsPosInf() bool {
	return h == posInf
}

// IsNegInf returns true if h is negative infinity.
func (h Half) IsNegInf() bool {
	return h == negInf
}

// IsZero returns true if h is positive or negative zero.
func (h Half) IsZero() bool {
	return h&0x7FFF == 0
}

// IsNormal returns true if h is a normalized non-zero finite value.
func (h Half) IsNormal() bool {
	exp := h & exponentMask
	return exp != 0 && exp != exponentMask
}

// IsSubnormal returns true if h is a subnormal (denormalized) non-zero value.
func (h Half) IsSubnormal() bool {
	return h&exponentMask == 0 && h&mantissaMask != 0
}

// IsFinite returns true if h is not Inf or NaN.
func (h Half) IsFinite() bool {
	return h&exponentMask != exponentMask
}

// Sign returns the sign of h: -1 for negative, 0 for zero, 1 for positive.
// NaN returns 0.
func (h Half) Sign() int {
	if h.IsNaN() {
		return 0
	}
	if h.IsZero() {
		return 0
	}
	if h&signBit != 0 {
		return -1
	}
	return 1
}

// Neg returns the negation of h.
func (h Half) Neg() Half {
	return h ^ signBit
}

// Abs returns the absolute value of h.
func (h Half) Abs() Half {
	return h &^ signBit
}

// Bits returns the IEEE 754 binary16 representation of h.
func (h Half) Bits() uint16 {
	return uint16(h)
}

// FromBits creates a Half from its IEEE 754 binary16 bit representation.
func FromBits(bits uint16) Half {
	return Half(bits)
}

// String returns a string representation of h.
func (h Half) String() string {
	switch {
	case h.IsNaN():
		return "NaN"
	case h.IsPosInf():
		return "+Inf"
	case h.IsNegInf():
		return "-Inf"
	default:
		// Use float32 formatting
		return ""
	}
}

// Less returns true if h < other.
// NaN comparisons always return false.
func (h Half) Less(other Half) bool {
	if h.IsNaN() || other.IsNaN() {
		return false
	}
	// Handle zeros (positive and negative zero are equal)
	if h.IsZero() && other.IsZero() {
		return false
	}

	hSign := h & signBit
	otherSign := other & signBit

	// Different signs
	if hSign != otherSign {
		// Negative is less than positive (unless both are zero, handled above)
		return hSign != 0
	}

	// Same sign - compare magnitudes
	hMag := h &^ signBit
	otherMag := other &^ signBit

	if hSign != 0 {
		// Both negative - larger magnitude is smaller
		return hMag > otherMag
	}
	// Both positive - smaller magnitude is smaller
	return hMag < otherMag
}

// LessOrEqual returns true if h <= other.
// NaN comparisons always return false.
func (h Half) LessOrEqual(other Half) bool {
	if h.IsNaN() || other.IsNaN() {
		return false
	}
	return h == other || h.Less(other)
}

// Greater returns true if h > other.
// NaN comparisons always return false.
func (h Half) Greater(other Half) bool {
	if h.IsNaN() || other.IsNaN() {
		return false
	}
	return other.Less(h)
}

// GreaterOrEqual returns true if h >= other.
// NaN comparisons always return false.
func (h Half) GreaterOrEqual(other Half) bool {
	if h.IsNaN() || other.IsNaN() {
		return false
	}
	return h == other || other.Less(h)
}

// Equal returns true if h == other.
// Note: NaN != NaN and +0 == -0.
func (h Half) Equal(other Half) bool {
	if h.IsNaN() || other.IsNaN() {
		return false
	}
	if h.IsZero() && other.IsZero() {
		return true
	}
	return h == other
}

// ConvertSlice32 converts a slice of float32 values to Half values.
// The destination slice must be the same length as the source.
func ConvertSlice32(dst []Half, src []float32) {
	if len(dst) != len(src) {
		panic("half: destination and source slices must have the same length")
	}
	for i, f := range src {
		dst[i] = FromFloat32(f)
	}
}

// ConvertSliceToFloat32 converts a slice of Half values to float32 values.
// The destination slice must be the same length as the source.
func ConvertSliceToFloat32(dst []float32, src []Half) {
	if len(dst) != len(src) {
		panic("half: destination and source slices must have the same length")
	}
	for i, h := range src {
		dst[i] = h.Float32()
	}
}

// MakeSlice32 creates a new slice of Half values from float32 values.
func MakeSlice32(src []float32) []Half {
	dst := make([]Half, len(src))
	ConvertSlice32(dst, src)
	return dst
}

// ToFloat32Slice creates a new slice of float32 values from Half values.
func ToFloat32Slice(src []Half) []float32 {
	dst := make([]float32, len(src))
	ConvertSliceToFloat32(dst, src)
	return dst
}
