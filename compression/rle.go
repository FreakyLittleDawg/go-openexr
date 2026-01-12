// Package compression provides compression algorithms for OpenEXR files.
package compression

import (
	"errors"
)

// RLE compression errors
var (
	ErrRLECorrupted = errors.New("compression: corrupted RLE data")
	ErrRLEOverflow  = errors.New("compression: RLE decompressed size overflow")
)

// RLE constants
const (
	// MinRunLength is the minimum run length that triggers encoding
	rleMinRunLength = 3
	// MaxRunLength is the maximum run length that can be encoded
	rleMaxRunLength = 127
)

// RLECompress compresses data using OpenEXR's RLE encoding.
//
// The RLE format uses signed bytes to indicate run types:
//   - Negative count (-n): The next byte is repeated (n+1) times (run)
//   - Positive count (+n): The next (n+1) bytes are copied literally
//
// For example:
//
//	[A, A, A, A, B, C, D] -> [-3, A, 2, B, C, D]
//	(4 copies of A, then 3 literal bytes B, C, D)
func RLECompress(src []byte) []byte {
	if len(src) == 0 {
		return nil
	}

	// Worst case: each byte becomes 2 bytes (literal byte + count)
	dst := make([]byte, 0, len(src)+len(src)/2)

	i := 0
	for i < len(src) {
		// Look for a run of identical bytes
		val := src[i]
		runEnd := i + 1
		for runEnd < len(src) && src[runEnd] == val && runEnd-i < rleMaxRunLength {
			runEnd++
		}
		runLength := runEnd - i

		if runLength >= rleMinRunLength {
			// Encode as a run: negative count, then the byte value
			dst = append(dst, byte(-(runLength - 1)), val)
			i = runEnd
			continue
		}

		// Start a literal sequence
		literalStart := i

		for i < len(src) && i-literalStart < rleMaxRunLength {
			// Check if a run starts here (only if we have enough bytes left)
			if i+rleMinRunLength <= len(src) {
				val := src[i]
				if src[i+1] == val && src[i+2] == val {
					break // Found start of a run
				}
			}
			i++
		}

		literalLength := i - literalStart
		if literalLength > 0 {
			// Encode as literals: positive count, then the bytes
			dst = append(dst, byte(literalLength-1))
			dst = append(dst, src[literalStart:i]...)
		}
	}

	return dst
}

// RLEDecompressTo decompresses RLE-encoded data into a pre-allocated buffer.
// This avoids allocation when called repeatedly.
func RLEDecompressTo(src []byte, dst []byte) error {
	if len(src) == 0 {
		return nil
	}

	dstPos := 0
	expectedSize := len(dst)

	i := 0
	for i < len(src) {
		count := int(int8(src[i]))
		i++

		if count < 0 {
			// Run: repeat the next byte (-count + 1) times
			runLength := -count + 1
			if i >= len(src) {
				return ErrRLECorrupted
			}
			if dstPos+runLength > expectedSize {
				return ErrRLEOverflow
			}
			val := src[i]
			i++
			for end := dstPos + runLength; dstPos < end; dstPos++ {
				dst[dstPos] = val
			}
		} else {
			// Literal: copy the next (count + 1) bytes
			literalLength := count + 1
			if i+literalLength > len(src) {
				return ErrRLECorrupted
			}
			if dstPos+literalLength > expectedSize {
				return ErrRLEOverflow
			}
			copy(dst[dstPos:], src[i:i+literalLength])
			dstPos += literalLength
			i += literalLength
		}
	}

	if dstPos != expectedSize {
		return ErrRLECorrupted
	}

	return nil
}

// RLEDecompress decompresses RLE-encoded data.
// The expectedSize parameter is the expected decompressed size,
// which is used to preallocate the output buffer and validate the result.
func RLEDecompress(src []byte, expectedSize int) ([]byte, error) {
	if len(src) == 0 {
		if expectedSize != 0 {
			return nil, ErrRLECorrupted
		}
		return nil, nil
	}

	// Pre-allocate exact size and write directly
	dst := make([]byte, expectedSize)
	dstPos := 0

	i := 0
	for i < len(src) {
		count := int(int8(src[i]))
		i++

		if count < 0 {
			// Run: repeat the next byte (-count + 1) times
			runLength := -count + 1
			if i >= len(src) {
				return nil, ErrRLECorrupted
			}
			if dstPos+runLength > expectedSize {
				return nil, ErrRLEOverflow
			}
			val := src[i]
			i++
			// Fill using direct indexing (faster than append)
			for end := dstPos + runLength; dstPos < end; dstPos++ {
				dst[dstPos] = val
			}
		} else {
			// Literal: copy the next (count + 1) bytes
			literalLength := count + 1
			if i+literalLength > len(src) {
				return nil, ErrRLECorrupted
			}
			if dstPos+literalLength > expectedSize {
				return nil, ErrRLEOverflow
			}
			copy(dst[dstPos:], src[i:i+literalLength])
			dstPos += literalLength
			i += literalLength
		}
	}

	if dstPos != expectedSize {
		return nil, ErrRLECorrupted
	}

	return dst, nil
}
