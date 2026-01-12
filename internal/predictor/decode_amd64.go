//go:build amd64

package predictor

// decodeASM performs predictor decode using SSE2/SSE4.1 SIMD instructions.
// It processes 16 bytes at a time using parallel prefix sum.
// The data slice is modified in place.
//
//go:noescape
func decodeASM(data []byte)

// hasSSE41 returns true if the CPU supports SSE4.1 instructions.
//
//go:noescape
func hasSSE41() bool
