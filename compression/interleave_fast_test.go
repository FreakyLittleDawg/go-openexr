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

// TestDeinterleaveFastPureGo tests the pure Go implementation directly
func TestDeinterleaveFastPureGo(t *testing.T) {
	sizes := []int{32, 33, 64, 100, 256}
	r := rand.New(rand.NewSource(42))

	for _, size := range sizes {
		t.Run("", func(t *testing.T) {
			original := make([]byte, size)
			r.Read(original)

			// Compare PureGo version with original Deinterleave
			expected := Deinterleave(original)
			got := DeinterleaveFastPureGo(original)

			if !bytes.Equal(expected, got) {
				t.Errorf("DeinterleaveFastPureGo mismatch for size %d", size)
			}
		})
	}
}

// TestDeinterleaveFastPureGoSmall tests edge case with small input
func TestDeinterleaveFastPureGoSmall(t *testing.T) {
	// Empty input
	result := DeinterleaveFastPureGo(nil)
	if result != nil {
		t.Error("nil input should return nil")
	}

	result = DeinterleaveFastPureGo([]byte{})
	if result != nil {
		t.Error("empty input should return nil")
	}

	// Small input (< 32 bytes) should fall back to Deinterleave
	small := []byte{1, 2, 3, 4, 5, 6, 7, 8}
	expected := Deinterleave(small)
	got := DeinterleaveFastPureGo(small)
	if !bytes.Equal(expected, got) {
		t.Errorf("small input mismatch: expected %v, got %v", expected, got)
	}
}

// TestInterleaveFastPureGo tests the pure Go implementation directly
func TestInterleaveFastPureGo(t *testing.T) {
	sizes := []int{32, 33, 64, 100, 256}
	r := rand.New(rand.NewSource(42))

	for _, size := range sizes {
		t.Run("", func(t *testing.T) {
			original := make([]byte, size)
			r.Read(original)

			// Compare PureGo version with original Interleave
			expected := Interleave(original)
			got := InterleaveFastPureGo(original)

			if !bytes.Equal(expected, got) {
				t.Errorf("InterleaveFastPureGo mismatch for size %d", size)
			}
		})
	}
}

// TestInterleaveFastPureGoSmall tests edge case with small input
func TestInterleaveFastPureGoSmall(t *testing.T) {
	// Empty input
	result := InterleaveFastPureGo(nil)
	if result != nil {
		t.Error("nil input should return nil")
	}

	result = InterleaveFastPureGo([]byte{})
	if result != nil {
		t.Error("empty input should return nil")
	}

	// Small input (< 32 bytes) should fall back to Interleave
	small := []byte{1, 2, 3, 4, 5, 6, 7, 8}
	expected := Interleave(small)
	got := InterleaveFastPureGo(small)
	if !bytes.Equal(expected, got) {
		t.Errorf("small input mismatch: expected %v, got %v", expected, got)
	}
}

// TestInterleaveBytes4 tests the interleaveBytes4 helper function
func TestInterleaveBytes4(t *testing.T) {
	tests := []struct {
		a, b uint32
	}{
		{0x00000000, 0x00000000},
		{0xFFFFFFFF, 0x00000000},
		{0x00000000, 0xFFFFFFFF},
		{0xFFFFFFFF, 0xFFFFFFFF},
		{0x01020304, 0x05060708},
	}

	for i, tt := range tests {
		got := interleaveBytes4(tt.a, tt.b)

		// Verify the function works correctly by checking the byte pattern
		// Extract and verify each byte pair
		for j := 0; j < 4; j++ {
			aByte := byte(tt.a >> (j * 8))
			bByte := byte(tt.b >> (j * 8))
			gotA := byte(got >> (j * 16))
			gotB := byte(got >> (j*16 + 8))

			if gotA != aByte || gotB != bByte {
				t.Errorf("test %d, pair %d: interleaveBytes4(0x%08X, 0x%08X) has wrong bytes at position %d",
					i, j, tt.a, tt.b, j)
			}
		}
	}
}

// TestDeinterleaveBytes8 tests the deinterleaveBytes8 helper function
func TestDeinterleaveBytes8(t *testing.T) {
	tests := []struct {
		lo, hi uint64
	}{
		{0x0000000000000000, 0x0000000000000000},
		{0xFFFFFFFFFFFFFFFF, 0xFFFFFFFFFFFFFFFF},
		{0x0102030405060708, 0x090A0B0C0D0E0F10},
	}

	for i, tt := range tests {
		even, odd := deinterleaveBytes8(tt.lo, tt.hi)

		// Verify by checking that even bytes come from positions 0, 2, 4, 6
		// and odd bytes come from positions 1, 3, 5, 7
		for j := 0; j < 4; j++ {
			// From lo
			expectedEven := byte(tt.lo >> (j * 16))
			expectedOdd := byte(tt.lo >> (j*16 + 8))
			gotEven := byte(even >> (j * 8))
			gotOdd := byte(odd >> (j * 8))

			if gotEven != expectedEven {
				t.Errorf("test %d: even byte %d mismatch: got 0x%02X, want 0x%02X",
					i, j, gotEven, expectedEven)
			}
			if gotOdd != expectedOdd {
				t.Errorf("test %d: odd byte %d mismatch: got 0x%02X, want 0x%02X",
					i, j, gotOdd, expectedOdd)
			}
		}
		for j := 0; j < 4; j++ {
			// From hi
			expectedEven := byte(tt.hi >> (j * 16))
			expectedOdd := byte(tt.hi >> (j*16 + 8))
			gotEven := byte(even >> ((j + 4) * 8))
			gotOdd := byte(odd >> ((j + 4) * 8))

			if gotEven != expectedEven {
				t.Errorf("test %d: even byte %d (hi) mismatch: got 0x%02X, want 0x%02X",
					i, j+4, gotEven, expectedEven)
			}
			if gotOdd != expectedOdd {
				t.Errorf("test %d: odd byte %d (hi) mismatch: got 0x%02X, want 0x%02X",
					i, j+4, gotOdd, expectedOdd)
			}
		}
	}
}

// TestPureGoRoundTrip tests round-trip through pure Go implementations
func TestPureGoRoundTrip(t *testing.T) {
	sizes := []int{32, 48, 64, 100, 256, 1000}
	r := rand.New(rand.NewSource(42))

	for _, size := range sizes {
		t.Run("", func(t *testing.T) {
			original := make([]byte, size)
			r.Read(original)

			// InterleaveFastPureGo -> DeinterleaveFastPureGo
			interleaved := InterleaveFastPureGo(original)
			restored := DeinterleaveFastPureGo(interleaved)

			if !bytes.Equal(original, restored) {
				t.Errorf("PureGo round-trip failed for size %d", size)
			}
		})
	}
}

// TestFastOddSizes tests odd-sized inputs that don't align to 8-byte boundaries
func TestFastOddSizes(t *testing.T) {
	sizes := []int{33, 47, 63, 65, 127, 255}
	r := rand.New(rand.NewSource(42))

	for _, size := range sizes {
		t.Run("", func(t *testing.T) {
			original := make([]byte, size)
			r.Read(original)

			// Test ASM versions match original
			expectedInterleaved := Interleave(original)
			gotInterleaved := InterleaveFast(original)
			if !bytes.Equal(expectedInterleaved, gotInterleaved) {
				t.Errorf("InterleaveFast mismatch for odd size %d", size)
			}

			expectedDeinterleaved := Deinterleave(original)
			gotDeinterleaved := DeinterleaveFast(original)
			if !bytes.Equal(expectedDeinterleaved, gotDeinterleaved) {
				t.Errorf("DeinterleaveFast mismatch for odd size %d", size)
			}

			// Test PureGo versions match original
			gotInterleavedPure := InterleaveFastPureGo(original)
			if !bytes.Equal(expectedInterleaved, gotInterleavedPure) {
				t.Errorf("InterleaveFastPureGo mismatch for odd size %d", size)
			}

			gotDeinterleavedPure := DeinterleaveFastPureGo(original)
			if !bytes.Equal(expectedDeinterleaved, gotDeinterleavedPure) {
				t.Errorf("DeinterleaveFastPureGo mismatch for odd size %d", size)
			}
		})
	}
}
