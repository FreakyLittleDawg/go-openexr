package compression

import (
	"bytes"
	"math/rand"
	"testing"
)

func TestInterleaveFastRoundTrip(t *testing.T) {
	sizes := []int{16, 32, 33, 64, 100, 256, 1000}
	r := rand.New(rand.NewSource(42))

	for _, size := range sizes {
		t.Run("", func(t *testing.T) {
			original := make([]byte, size)
			r.Read(original)

			// Test InterleaveFast -> DeinterleaveFast round trip
			interleaved := InterleaveFast(original)
			restored := DeinterleaveFast(interleaved)

			if !bytes.Equal(original, restored) {
				t.Errorf("Round trip failed for size %d", size)
			}
		})
	}
}

func TestInterleaveFastMatchesOriginal(t *testing.T) {
	sizes := []int{8, 16, 32, 33, 64, 100}
	r := rand.New(rand.NewSource(42))

	for _, size := range sizes {
		t.Run("", func(t *testing.T) {
			original := make([]byte, size)
			r.Read(original)

			// Compare fast version with original
			expected := Interleave(original)
			got := InterleaveFast(original)

			if !bytes.Equal(expected, got) {
				t.Errorf("InterleaveFast mismatch for size %d:\nexpected: %v\ngot:      %v",
					size, expected[:min(32, len(expected))], got[:min(32, len(got))])
			}
		})
	}
}

func TestDeinterleaveFastMatchesOriginal(t *testing.T) {
	sizes := []int{8, 16, 32, 33, 64, 100}
	r := rand.New(rand.NewSource(42))

	for _, size := range sizes {
		t.Run("", func(t *testing.T) {
			original := make([]byte, size)
			r.Read(original)

			// Compare fast version with original
			expected := Deinterleave(original)
			got := DeinterleaveFast(original)

			if !bytes.Equal(expected, got) {
				t.Errorf("DeinterleaveFast mismatch for size %d:\nexpected: %v\ngot:      %v",
					size, expected[:min(32, len(expected))], got[:min(32, len(got))])
			}
		})
	}
}

func BenchmarkInterleaveFastVsOriginal(b *testing.B) {
	r := rand.New(rand.NewSource(42))
	sizes := []int{1024, 4096, 16384, 65536}
	for _, size := range sizes {
		data := make([]byte, size)
		r.Read(data)

		b.Run("Original", func(b *testing.B) {
			b.SetBytes(int64(size))
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				Interleave(data)
			}
		})

		b.Run("Fast", func(b *testing.B) {
			b.SetBytes(int64(size))
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				InterleaveFast(data)
			}
		})
	}
}

func BenchmarkDeinterleaveFastVsOriginal(b *testing.B) {
	r := rand.New(rand.NewSource(42))
	sizes := []int{1024, 4096, 16384, 65536}
	for _, size := range sizes {
		data := make([]byte, size)
		r.Read(data)

		b.Run("Original", func(b *testing.B) {
			b.SetBytes(int64(size))
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				Deinterleave(data)
			}
		})

		b.Run("Fast", func(b *testing.B) {
			b.SetBytes(int64(size))
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				DeinterleaveFast(data)
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
