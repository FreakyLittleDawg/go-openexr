//go:build arm64

package compression

// shiftRoundSIMD computes d[i] = shiftAndRound(tMax - t[i], shift) for 16 values.
// Pure Go implementation for ARM64 - Go's assembler doesn't expose NEON USHL with register shifts.
// Unrolled to help compiler inlining.
//
//go:nosplit
func shiftRoundSIMD(d *[16]uint16, t *[16]uint16, tMax uint16, shift uint) {
	a := (1 << shift) - 1
	shiftP1 := shift + 1
	tMaxInt := int(tMax)

	// Unrolled loop for better inlining
	var x int
	x = (tMaxInt - int(t[0])) << 1
	d[0] = uint16((x + a + ((x >> shiftP1) & 1)) >> shiftP1)
	x = (tMaxInt - int(t[1])) << 1
	d[1] = uint16((x + a + ((x >> shiftP1) & 1)) >> shiftP1)
	x = (tMaxInt - int(t[2])) << 1
	d[2] = uint16((x + a + ((x >> shiftP1) & 1)) >> shiftP1)
	x = (tMaxInt - int(t[3])) << 1
	d[3] = uint16((x + a + ((x >> shiftP1) & 1)) >> shiftP1)
	x = (tMaxInt - int(t[4])) << 1
	d[4] = uint16((x + a + ((x >> shiftP1) & 1)) >> shiftP1)
	x = (tMaxInt - int(t[5])) << 1
	d[5] = uint16((x + a + ((x >> shiftP1) & 1)) >> shiftP1)
	x = (tMaxInt - int(t[6])) << 1
	d[6] = uint16((x + a + ((x >> shiftP1) & 1)) >> shiftP1)
	x = (tMaxInt - int(t[7])) << 1
	d[7] = uint16((x + a + ((x >> shiftP1) & 1)) >> shiftP1)
	x = (tMaxInt - int(t[8])) << 1
	d[8] = uint16((x + a + ((x >> shiftP1) & 1)) >> shiftP1)
	x = (tMaxInt - int(t[9])) << 1
	d[9] = uint16((x + a + ((x >> shiftP1) & 1)) >> shiftP1)
	x = (tMaxInt - int(t[10])) << 1
	d[10] = uint16((x + a + ((x >> shiftP1) & 1)) >> shiftP1)
	x = (tMaxInt - int(t[11])) << 1
	d[11] = uint16((x + a + ((x >> shiftP1) & 1)) >> shiftP1)
	x = (tMaxInt - int(t[12])) << 1
	d[12] = uint16((x + a + ((x >> shiftP1) & 1)) >> shiftP1)
	x = (tMaxInt - int(t[13])) << 1
	d[13] = uint16((x + a + ((x >> shiftP1) & 1)) >> shiftP1)
	x = (tMaxInt - int(t[14])) << 1
	d[14] = uint16((x + a + ((x >> shiftP1) & 1)) >> shiftP1)
	x = (tMaxInt - int(t[15])) << 1
	d[15] = uint16((x + a + ((x >> shiftP1) & 1)) >> shiftP1)
}

// toOrderedSIMD converts 16 half-float values from sign-magnitude to ordered representation.
// Uses ARM NEON SIMD instructions to process 8 values at a time.
// The ordered representation makes comparison operations work correctly:
// - NaN/Inf (exponent all 1s) -> 0x8000
// - Negative values -> bitwise NOT of original
// - Positive values -> original with high bit set
//
//go:noescape
func toOrderedSIMD(dst, src *[16]uint16)

// findMaxSIMD finds the maximum value among 16 uint16 values.
// Uses ARM NEON UMAX for parallel unsigned comparison and horizontal reduction.
//
//go:noescape
func findMaxSIMD(src *[16]uint16) uint16

// fromOrderedSIMD converts 16 values from ordered back to sign-magnitude representation.
// This is the inverse of toOrderedSIMD:
// - High bit set (ordered positive) -> clear high bit
// - High bit clear (ordered negative) -> NOT
//
//go:noescape
func fromOrderedSIMD(dst, src *[16]uint16)
