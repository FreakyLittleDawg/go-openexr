package predictor

import (
	"bytes"
	"math/rand"
	"testing"
)

func TestDecodeSIMD(t *testing.T) {
	testCases := []struct {
		name  string
		input []byte
	}{
		{
			name:  "small",
			input: []byte{1, 2, 3, 4, 5, 6, 7, 8},
		},
		{
			name:  "16 bytes",
			input: []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16},
		},
		{
			name:  "17 bytes",
			input: []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17},
		},
		{
			name:  "zeros",
			input: make([]byte, 32),
		},
		{
			name:  "all same",
			input: bytes.Repeat([]byte{42}, 64),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Make copies for both methods
			input1 := make([]byte, len(tc.input))
			input2 := make([]byte, len(tc.input))
			copy(input1, tc.input)
			copy(input2, tc.input)

			// Apply both decode methods
			Decode(input1)
			DecodeSIMD(input2)

			// Compare results
			if !bytes.Equal(input1, input2) {
				t.Errorf("DecodeSIMD mismatch:\nwant: %v\ngot:  %v", input1, input2)
			}
		})
	}
}

func TestDecodeSIMDRandom(t *testing.T) {
	r := rand.New(rand.NewSource(42))

	sizes := []int{7, 8, 15, 16, 17, 31, 32, 33, 63, 64, 65, 100, 256, 1000}
	for _, size := range sizes {
		t.Run("", func(t *testing.T) {
			input := make([]byte, size)
			r.Read(input)

			input1 := make([]byte, len(input))
			input2 := make([]byte, len(input))
			copy(input1, input)
			copy(input2, input)

			Decode(input1)
			DecodeSIMD(input2)

			if !bytes.Equal(input1, input2) {
				t.Errorf("DecodeSIMD mismatch for size %d:\nfirst 32 bytes want: %v\nfirst 32 bytes got:  %v",
					size, input1[:min(32, len(input1))], input2[:min(32, len(input2))])
			}
		})
	}
}

func TestEncodeSIMD(t *testing.T) {
	testCases := []struct {
		name  string
		input []byte
	}{
		{
			name:  "small",
			input: []byte{1, 2, 3, 4, 5, 6, 7, 8},
		},
		{
			name:  "16 bytes",
			input: []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16},
		},
		{
			name:  "prefix sum",
			input: []byte{1, 3, 6, 10, 15, 21, 28, 36, 45, 55},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			input1 := make([]byte, len(tc.input))
			input2 := make([]byte, len(tc.input))
			copy(input1, tc.input)
			copy(input2, tc.input)

			Encode(input1)
			EncodeSIMD(input2)

			if !bytes.Equal(input1, input2) {
				t.Errorf("EncodeSIMD mismatch:\nwant: %v\ngot:  %v", input1, input2)
			}
		})
	}
}

func TestReconstructBytes(t *testing.T) {
	// Test that ReconstructBytes produces same result as deinterleave + Decode
	// This matches the OpenEXR ZIP pipeline order: deinterleave first, then predictor decode
	sizes := []int{8, 16, 32, 64, 100, 256}

	for _, size := range sizes {
		t.Run("", func(t *testing.T) {
			r := rand.New(rand.NewSource(int64(size)))
			original := make([]byte, size)
			r.Read(original)

			// Method 1: separate steps (deinterleave first, then decode)
			source1 := make([]byte, size)
			copy(source1, original)
			half := (size + 1) / 2
			out1 := make([]byte, size)
			// Deinterleave: source is [even bytes | odd bytes]
			for i := 0; i < half; i++ {
				out1[i*2] = source1[i]
				if half+i < size {
					out1[i*2+1] = source1[half+i]
				}
			}
			// Predictor decode on deinterleaved data
			Decode(out1)

			// Method 2: combined ReconstructBytes
			source2 := make([]byte, size)
			copy(source2, original)
			out2 := make([]byte, size)
			ReconstructBytes(out2, source2)

			if !bytes.Equal(out1, out2) {
				t.Errorf("ReconstructBytes mismatch for size %d:\nwant: %v\ngot:  %v",
					size, out1[:min(32, len(out1))], out2[:min(32, len(out2))])
			}
		})
	}
}

func TestDeconstructBytes(t *testing.T) {
	// Test that DeconstructBytes produces same result as Encode + manual interleave
	sizes := []int{8, 16, 32, 64, 100, 256}

	for _, size := range sizes {
		t.Run("", func(t *testing.T) {
			r := rand.New(rand.NewSource(int64(size)))
			original := make([]byte, size)
			r.Read(original)

			// Method 1: separate steps
			half := (size + 1) / 2
			scratch1 := make([]byte, size)
			// Interleave (split even/odd)
			for i := 0; i < size; i++ {
				if i%2 == 0 {
					scratch1[i/2] = original[i]
				} else {
					scratch1[half+i/2] = original[i]
				}
			}
			Encode(scratch1)

			// Method 2: combined DeconstructBytes
			scratch2 := make([]byte, size)
			DeconstructBytes(scratch2, original)

			if !bytes.Equal(scratch1, scratch2) {
				t.Errorf("DeconstructBytes mismatch for size %d:\nwant: %v\ngot:  %v",
					size, scratch1[:min(32, len(scratch1))], scratch2[:min(32, len(scratch2))])
			}
		})
	}
}

func BenchmarkDecodeSIMD(b *testing.B) {
	r := rand.New(rand.NewSource(42))
	sizes := []int{1024, 4096, 16384, 65536}
	for _, size := range sizes {
		data := make([]byte, size)
		r.Read(data)

		b.Run("Decode", func(b *testing.B) {
			buf := make([]byte, size)
			b.SetBytes(int64(size))
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				copy(buf, data)
				Decode(buf)
			}
		})

		b.Run("DecodeSIMD", func(b *testing.B) {
			buf := make([]byte, size)
			b.SetBytes(int64(size))
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				copy(buf, data)
				DecodeSIMD(buf)
			}
		})
	}
}

func BenchmarkReconstructBytes(b *testing.B) {
	r := rand.New(rand.NewSource(42))
	sizes := []int{1024, 4096, 16384, 65536}
	for _, size := range sizes {
		data := make([]byte, size)
		r.Read(data)

		b.Run("Separate", func(b *testing.B) {
			source := make([]byte, size)
			out := make([]byte, size)
			b.SetBytes(int64(size))
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				copy(source, data)
				// Simulate separate steps
				Decode(source)
				half := (size + 1) / 2
				for j := 0; j < half; j++ {
					out[j*2] = source[j]
					if half+j < size {
						out[j*2+1] = source[half+j]
					}
				}
			}
		})

		b.Run("Combined", func(b *testing.B) {
			source := make([]byte, size)
			out := make([]byte, size)
			b.SetBytes(int64(size))
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				copy(source, data)
				ReconstructBytes(out, source)
			}
		})
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
