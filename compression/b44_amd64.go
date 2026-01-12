//go:build amd64

package compression

// toOrderedSIMD converts 16 half-float values from sign-magnitude to ordered representation.
// Uses SSE2 SIMD instructions to process 8 values at a time.
// The ordered representation makes comparison operations work correctly:
// - NaN/Inf (exponent all 1s) -> 0x8000
// - Negative values -> bitwise NOT of original
// - Positive values -> original with high bit set
//
//go:noescape
func toOrderedSIMD(dst, src *[16]uint16)

// findMaxSIMD finds the maximum value among 16 uint16 values.
// Uses SSE2 PMAXUW for horizontal reduction.
//
//go:noescape
func findMaxSIMD(src *[16]uint16) uint16

// fromOrderedSIMD converts 16 values from ordered back to sign-magnitude representation.
// This is the inverse of toOrderedSIMD.
//
//go:noescape
func fromOrderedSIMD(dst, src *[16]uint16)

// shiftRoundSIMD computes d[i] = shiftAndRound(tMax - t[i], shift) for 16 values.
// Uses SSE2 SIMD with uniform shift from register (PSRLW xmm, xmm).
//
//go:noescape
func shiftRoundSIMD(d *[16]uint16, t *[16]uint16, tMax uint16, shift uint)
