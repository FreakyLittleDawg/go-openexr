//go:build !amd64 && !arm64

package predictor

// decodeASM is a no-op on platforms without SIMD assembly.
// The caller should use DecodeSIMD which has a pure Go fallback.
func decodeASM(data []byte) {
	// Fallback to pure Go implementation
	decodePureGo(data)
}

func decodePureGo(data []byte) {
	n := len(data)
	if n < 2 {
		return
	}

	// Process in chunks of 8 with running sum
	i := 1
	for ; i+7 < n; i += 8 {
		data[i] += data[i-1]
		data[i+1] += data[i]
		data[i+2] += data[i+1]
		data[i+3] += data[i+2]
		data[i+4] += data[i+3]
		data[i+5] += data[i+4]
		data[i+6] += data[i+5]
		data[i+7] += data[i+6]
	}

	for ; i < n; i++ {
		data[i] += data[i-1]
	}
}
