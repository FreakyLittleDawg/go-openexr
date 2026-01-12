// Package interleave implements byte interleaving for OpenEXR compression.
//
// OpenEXR compressors interleave bytes from different positions in the
// data stream to group similar bytes together, improving compression ratio.
// For example, with half-precision (2 bytes per value) data, all the high
// bytes are grouped together, followed by all the low bytes.
//
// The interleaving pattern collects every Nth byte:
//
//	Input:  [A0, A1, B0, B1, C0, C1, D0, D1]  (4 half values)
//	Output: [A0, B0, C0, D0, A1, B1, C1, D1]  (high bytes, then low bytes)
package interleave

// Interleave reorders bytes in place to group similar bytes together.
// The data is treated as an array of stride-byte elements, and all bytes
// at the same offset within each element are grouped together.
//
// For example, with stride=2 (half-precision):
//
//	[a0,a1, b0,b1, c0,c1] -> [a0,b0,c0, a1,b1,c1]
//
// The output buffer must be the same size as data.
// If out is nil, a new buffer is allocated.
func Interleave(data []byte, stride int, out []byte) []byte {
	if len(data) == 0 || stride <= 1 {
		if out == nil {
			out = make([]byte, len(data))
		}
		copy(out, data)
		return out
	}

	numElements := len(data) / stride
	remainder := len(data) % stride

	if out == nil {
		out = make([]byte, len(data))
	}

	// Interleave the main data
	for offset := 0; offset < stride; offset++ {
		dstBase := offset * numElements
		for elem := 0; elem < numElements; elem++ {
			out[dstBase+elem] = data[elem*stride+offset]
		}
	}

	// Copy any remaining bytes that don't fit a full element
	if remainder > 0 {
		copy(out[stride*numElements:], data[stride*numElements:])
	}

	return out
}

// Deinterleave reverses the interleaving operation.
// This restores the original byte order from grouped bytes.
//
// For example, with stride=2:
//
//	[a0,b0,c0, a1,b1,c1] -> [a0,a1, b0,b1, c0,c1]
//
// The output buffer must be the same size as data.
// If out is nil, a new buffer is allocated.
func Deinterleave(data []byte, stride int, out []byte) []byte {
	if len(data) == 0 || stride <= 1 {
		if out == nil {
			out = make([]byte, len(data))
		}
		copy(out, data)
		return out
	}

	numElements := len(data) / stride
	remainder := len(data) % stride

	if out == nil {
		out = make([]byte, len(data))
	}

	// Deinterleave the main data
	for offset := 0; offset < stride; offset++ {
		srcBase := offset * numElements
		for elem := 0; elem < numElements; elem++ {
			out[elem*stride+offset] = data[srcBase+elem]
		}
	}

	// Copy any remaining bytes
	if remainder > 0 {
		copy(out[stride*numElements:], data[stride*numElements:])
	}

	return out
}

// InterleaveInPlace interleaves data in place using a temporary buffer.
func InterleaveInPlace(data []byte, stride int) {
	if len(data) == 0 || stride <= 1 {
		return
	}

	tmp := make([]byte, len(data))
	Interleave(data, stride, tmp)
	copy(data, tmp)
}

// DeinterleaveInPlace deinterleaves data in place using a temporary buffer.
func DeinterleaveInPlace(data []byte, stride int) {
	if len(data) == 0 || stride <= 1 {
		return
	}

	tmp := make([]byte, len(data))
	Deinterleave(data, stride, tmp)
	copy(data, tmp)
}
