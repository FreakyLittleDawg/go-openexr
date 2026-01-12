package compression

import (
	"unsafe"
)

// DeinterleaveFast performs optimized byte deinterleaving using SIMD when available.
// Uses SSE2/NEON assembly on amd64/arm64, falls back to 64-bit operations on other platforms.
//
// Input layout (split format):  [A0, B0, C0, D0, E0, F0, G0, H0 | A1, B1, C1, D1, E1, F1, G1, H1]
// Output layout (interleaved): [A0, A1, B0, B1, C0, C1, D0, D1, E0, E1, F0, F1, G0, G1, H0, H1]
func DeinterleaveFast(src []byte) []byte {
	n := len(src)
	if n == 0 {
		return nil
	}
	if n < 32 {
		return Deinterleave(src)
	}

	dst := make([]byte, n)
	// Use SIMD assembly implementation
	deinterleaveASM(dst, src)
	return dst
}

// DeinterleaveFastPureGo is the pure Go implementation for testing/benchmarking.
func DeinterleaveFastPureGo(src []byte) []byte {
	n := len(src)
	if n == 0 {
		return nil
	}
	if n < 32 {
		return Deinterleave(src)
	}

	dst := make([]byte, n)
	half := (n + 1) / 2

	// Process 8 bytes at a time from each half -> 16 bytes output
	chunks := half / 8
	for i := 0; i < chunks; i++ {
		srcOffset1 := i * 8
		srcOffset2 := half + i*8
		dstOffset := i * 16

		v1 := *(*uint64)(unsafe.Pointer(&src[srcOffset1]))
		v2 := *(*uint64)(unsafe.Pointer(&src[srcOffset2]))

		lo := interleaveBytes4(uint32(v1), uint32(v2))
		hi := interleaveBytes4(uint32(v1>>32), uint32(v2>>32))

		*(*uint64)(unsafe.Pointer(&dst[dstOffset])) = lo
		*(*uint64)(unsafe.Pointer(&dst[dstOffset+8])) = hi
	}

	for i := chunks * 8; i < half; i++ {
		dst[i*2] = src[i]
		if i < n-half {
			dst[i*2+1] = src[half+i]
		}
	}
	if n%2 == 1 {
		dst[n-1] = src[half-1]
	}

	return dst
}

// interleaveBytes4 interleaves 4 bytes from each of two 32-bit values.
// Input:  a = [a0, a1, a2, a3], b = [b0, b1, b2, b3] (little endian)
// Output: [a0, b0, a1, b1, a2, b2, a3, b3] as uint64
func interleaveBytes4(a, b uint32) uint64 {
	// Extract bytes
	a0 := uint64(a & 0xFF)
	a1 := uint64((a >> 8) & 0xFF)
	a2 := uint64((a >> 16) & 0xFF)
	a3 := uint64((a >> 24) & 0xFF)
	b0 := uint64(b & 0xFF)
	b1 := uint64((b >> 8) & 0xFF)
	b2 := uint64((b >> 16) & 0xFF)
	b3 := uint64((b >> 24) & 0xFF)

	// Combine interleaved (little endian)
	return a0 | (b0 << 8) | (a1 << 16) | (b1 << 24) |
		(a2 << 32) | (b2 << 40) | (a3 << 48) | (b3 << 56)
}

// InterleaveFast performs optimized byte interleaving using SIMD when available.
// Separates even and odd bytes into two halves.
func InterleaveFast(src []byte) []byte {
	n := len(src)
	if n == 0 {
		return nil
	}
	if n < 32 {
		return Interleave(src)
	}

	dst := make([]byte, n)
	// Use SIMD assembly implementation
	interleaveASM(dst, src)
	return dst
}

// InterleaveFastPureGo is the pure Go implementation for testing/benchmarking.
func InterleaveFastPureGo(src []byte) []byte {
	n := len(src)
	if n == 0 {
		return nil
	}
	if n < 32 {
		return Interleave(src)
	}

	dst := make([]byte, n)
	half := (n + 1) / 2

	chunks := n / 16
	for i := 0; i < chunks; i++ {
		srcOffset := i * 16
		dstOffset1 := i * 8
		dstOffset2 := half + i*8

		lo := *(*uint64)(unsafe.Pointer(&src[srcOffset]))
		hi := *(*uint64)(unsafe.Pointer(&src[srcOffset+8]))

		even, odd := deinterleaveBytes8(lo, hi)

		*(*uint64)(unsafe.Pointer(&dst[dstOffset1])) = even
		*(*uint64)(unsafe.Pointer(&dst[dstOffset2])) = odd
	}

	for i := chunks * 16; i < n; i++ {
		if i%2 == 0 {
			dst[i/2] = src[i]
		} else {
			dst[half+i/2] = src[i]
		}
	}

	return dst
}

// deinterleaveBytes8 separates even and odd bytes from 16 bytes of input.
// Input: lo = [a0,a1,b0,b1,c0,c1,d0,d1], hi = [e0,e1,f0,f1,g0,g1,h0,h1]
// Output: even = [a0,b0,c0,d0,e0,f0,g0,h0], odd = [a1,b1,c1,d1,e1,f1,g1,h1]
func deinterleaveBytes8(lo, hi uint64) (even, odd uint64) {
	// Extract even bytes (indices 0, 2, 4, 6) from each 64-bit value
	e0 := lo & 0xFF
	e1 := (lo >> 16) & 0xFF
	e2 := (lo >> 32) & 0xFF
	e3 := (lo >> 48) & 0xFF
	e4 := hi & 0xFF
	e5 := (hi >> 16) & 0xFF
	e6 := (hi >> 32) & 0xFF
	e7 := (hi >> 48) & 0xFF

	// Extract odd bytes (indices 1, 3, 5, 7)
	o0 := (lo >> 8) & 0xFF
	o1 := (lo >> 24) & 0xFF
	o2 := (lo >> 40) & 0xFF
	o3 := (lo >> 56) & 0xFF
	o4 := (hi >> 8) & 0xFF
	o5 := (hi >> 24) & 0xFF
	o6 := (hi >> 40) & 0xFF
	o7 := (hi >> 56) & 0xFF

	even = e0 | (e1 << 8) | (e2 << 16) | (e3 << 24) | (e4 << 32) | (e5 << 40) | (e6 << 48) | (e7 << 56)
	odd = o0 | (o1 << 8) | (o2 << 16) | (o3 << 24) | (o4 << 32) | (o5 << 40) | (o6 << 48) | (o7 << 56)
	return
}
