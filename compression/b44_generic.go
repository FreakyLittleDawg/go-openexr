//go:build !amd64 && !arm64

package compression

// shiftRoundSIMD computes d[i] = shiftAndRound(tMax - t[i], shift) for 16 values.
// Generic fallback implementation using scalar code. Unrolled for better performance.
//
//go:nosplit
func shiftRoundSIMD(d *[16]uint16, t *[16]uint16, tMax uint16, shift uint) {
	a := (1 << shift) - 1
	shiftP1 := shift + 1
	tMaxInt := int(tMax)

	// Unrolled loop for better performance
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
// Generic fallback implementation.
func toOrderedSIMD(dst, src *[16]uint16) {
	for i := 0; i < 16; i++ {
		s := src[i]
		// NaN/Inf check
		if (s & 0x7c00) == 0x7c00 {
			dst[i] = 0x8000
		} else if (s & 0x8000) != 0 {
			// Negative: bitwise NOT
			dst[i] = ^s
		} else {
			// Positive: set high bit
			dst[i] = s | 0x8000
		}
	}
}

// findMaxSIMD finds the maximum value among 16 uint16 values.
// Generic fallback implementation.
func findMaxSIMD(src *[16]uint16) uint16 {
	max := src[0]
	for i := 1; i < 16; i++ {
		if src[i] > max {
			max = src[i]
		}
	}
	return max
}

// fromOrderedSIMD converts 16 values from ordered back to sign-magnitude representation.
// Generic fallback implementation.
func fromOrderedSIMD(dst, src *[16]uint16) {
	for i := 0; i < 16; i++ {
		s := src[i]
		if (s & 0x8000) != 0 {
			// High bit set = was positive, clear it
			dst[i] = s & 0x7fff
		} else {
			// High bit clear = was negative, NOT it
			dst[i] = ^s
		}
	}
}
