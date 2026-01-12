//go:build arm64

package predictor

// decodeASM performs predictor decode using ARM NEON SIMD instructions.
// It processes 16 bytes at a time using parallel prefix sum.
// The data slice is modified in place.
//
//go:noescape
func decodeASM(data []byte)
