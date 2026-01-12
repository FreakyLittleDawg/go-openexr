//go:build arm64

package compression

// deinterleaveASM performs byte deinterleaving using ARM NEON SIMD instructions.
// Input is split format: [even bytes | odd bytes]
// Output is interleaved: [e0,o0,e1,o1,e2,o2,...]
//
//go:noescape
func deinterleaveASM(dst, src []byte)

// interleaveASM performs byte interleaving using ARM NEON SIMD instructions.
// Input is interleaved: [e0,o0,e1,o1,e2,o2,...]
// Output is split format: [even bytes | odd bytes]
//
//go:noescape
func interleaveASM(dst, src []byte)
