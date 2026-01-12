// Package compression provides compression algorithms for OpenEXR files.
package compression

// Interleave reorders bytes by separating odd and even positions.
// This groups similar bytes together, improving compression for
// multi-byte pixel values where bytes at the same position
// within a value tend to be similar.
//
// For example, for 16-bit values stored as [A0, A1, B0, B1, C0, C1]:
// After interleaving: [A0, B0, C0, A1, B1, C1]
//
// This is used by ZIP and PIZ compression in OpenEXR.
func Interleave(src []byte) []byte {
	n := len(src)
	if n == 0 {
		return nil
	}

	dst := make([]byte, n)

	// Separate into two halves: even indices, then odd indices
	half := (n + 1) / 2
	evenIdx := 0
	oddIdx := half

	for i := 0; i < n; i++ {
		if i%2 == 0 {
			dst[evenIdx] = src[i]
			evenIdx++
		} else {
			dst[oddIdx] = src[i]
			oddIdx++
		}
	}

	return dst
}

// Deinterleave reverses the interleaving operation.
// It restores the original byte order from the interleaved format.
func Deinterleave(src []byte) []byte {
	n := len(src)
	if n == 0 {
		return nil
	}

	dst := make([]byte, n)

	// First half contains even-indexed bytes, second half contains odd
	half := (n + 1) / 2

	for i := 0; i < half; i++ {
		dst[i*2] = src[i]
	}
	for i := 0; i < n-half; i++ {
		dst[i*2+1] = src[half+i]
	}

	return dst
}
