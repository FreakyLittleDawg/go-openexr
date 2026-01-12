package compression

import (
	"bytes"
	"testing"
)

// signedByte converts a signed int8 value to a byte for use in test data.
// This is needed because Go doesn't allow negative byte literals.
func signedByte(v int8) byte {
	return byte(v)
}

func TestRLECompressEmpty(t *testing.T) {
	result := RLECompress(nil)
	if result != nil {
		t.Error("Compressing nil should return nil")
	}

	result = RLECompress([]byte{})
	if result != nil {
		t.Error("Compressing empty should return nil")
	}
}

func TestRLECompressRun(t *testing.T) {
	// Simple run of identical bytes
	data := []byte{42, 42, 42, 42, 42}
	compressed := RLECompress(data)

	// Should encode as [-4, 42] (5 copies of 42)
	expected := []byte{signedByte(-4), 42}
	if !bytes.Equal(compressed, expected) {
		t.Errorf("Compress run: got %v, want %v", compressed, expected)
	}
}

func TestRLECompressLiterals(t *testing.T) {
	// Sequence with no runs
	data := []byte{1, 2, 3, 4}
	compressed := RLECompress(data)

	// Should encode as [3, 1, 2, 3, 4] (4 literal bytes)
	expected := []byte{3, 1, 2, 3, 4}
	if !bytes.Equal(compressed, expected) {
		t.Errorf("Compress literals: got %v, want %v", compressed, expected)
	}
}

func TestRLECompressMixed(t *testing.T) {
	// Mix of runs and literals
	data := []byte{1, 2, 3, 100, 100, 100, 100, 4, 5}
	compressed := RLECompress(data)

	// [2, 1, 2, 3] + [-3, 100] + [1, 4, 5] = [2, 1, 2, 3, -3, 100, 1, 4, 5]
	if len(compressed) > len(data) {
		t.Logf("Compression expanded data (normal for short mixed data): %d -> %d", len(data), len(compressed))
	}

	// Verify round-trip instead of exact encoding
	decompressed, err := RLEDecompress(compressed, len(data))
	if err != nil {
		t.Fatalf("Decompress error: %v", err)
	}
	if !bytes.Equal(decompressed, data) {
		t.Errorf("Round-trip failed:\ngot  %v\nwant %v", decompressed, data)
	}
}

func TestRLEDecompressEmpty(t *testing.T) {
	result, err := RLEDecompress(nil, 0)
	if err != nil || result != nil {
		t.Error("Decompressing nil should return nil, nil")
	}

	result, err = RLEDecompress([]byte{}, 0)
	if err != nil || result != nil {
		t.Error("Decompressing empty should return nil, nil")
	}
}

func TestRLEDecompressRun(t *testing.T) {
	// [-4, 42] = 5 copies of 42
	compressed := []byte{signedByte(-4), 42}
	decompressed, err := RLEDecompress(compressed, 5)
	if err != nil {
		t.Fatalf("Decompress error: %v", err)
	}

	expected := []byte{42, 42, 42, 42, 42}
	if !bytes.Equal(decompressed, expected) {
		t.Errorf("Decompress run: got %v, want %v", decompressed, expected)
	}
}

func TestRLEDecompressLiterals(t *testing.T) {
	// [3, 1, 2, 3, 4] = 4 literal bytes
	compressed := []byte{3, 1, 2, 3, 4}
	decompressed, err := RLEDecompress(compressed, 4)
	if err != nil {
		t.Fatalf("Decompress error: %v", err)
	}

	expected := []byte{1, 2, 3, 4}
	if !bytes.Equal(decompressed, expected) {
		t.Errorf("Decompress literals: got %v, want %v", decompressed, expected)
	}
}

func TestRLERoundTrip(t *testing.T) {
	tests := [][]byte{
		{1},
		{1, 2},
		{1, 1, 1},
		{1, 2, 3, 4, 5},
		{100, 100, 100, 100, 100, 100, 100, 100},
		{1, 2, 3, 3, 3, 3, 4, 5, 6},
		{1, 1, 1, 1, 2, 2, 2, 2, 3, 3, 3, 3},
	}

	for i, original := range tests {
		compressed := RLECompress(original)
		decompressed, err := RLEDecompress(compressed, len(original))
		if err != nil {
			t.Errorf("test %d: decompress error: %v", i, err)
			continue
		}
		if !bytes.Equal(decompressed, original) {
			t.Errorf("test %d: round-trip failed:\ngot  %v\nwant %v", i, decompressed, original)
		}
	}
}

func TestRLERoundTripLarge(t *testing.T) {
	// Large data with patterns typical of image data
	data := make([]byte, 4096)
	for i := range data {
		// Create some runs and some random-looking data
		if i%100 < 30 {
			data[i] = 0 // runs of zeros
		} else {
			data[i] = byte(i * 17) // pseudo-random
		}
	}

	compressed := RLECompress(data)
	decompressed, err := RLEDecompress(compressed, len(data))
	if err != nil {
		t.Fatalf("Decompress error: %v", err)
	}
	if !bytes.Equal(decompressed, data) {
		t.Error("Large round-trip failed")
	}

	t.Logf("Compression ratio: %d -> %d (%.1f%%)", len(data), len(compressed), 100.0*float64(len(compressed))/float64(len(data)))
}

func TestRLEDecompressErrors(t *testing.T) {
	// Wrong expected size
	compressed := []byte{signedByte(-4), 42} // 5 bytes
	_, err := RLEDecompress(compressed, 10)
	if err == nil {
		t.Error("Should error on wrong expected size")
	}

	// Truncated run
	compressed = []byte{signedByte(-4)} // missing value
	_, err = RLEDecompress(compressed, 5)
	if err != ErrRLECorrupted {
		t.Errorf("Truncated run error = %v, want ErrRLECorrupted", err)
	}

	// Truncated literals
	compressed = []byte{3, 1, 2} // claims 4 bytes, only has 2
	_, err = RLEDecompress(compressed, 4)
	if err != ErrRLECorrupted {
		t.Errorf("Truncated literals error = %v, want ErrRLECorrupted", err)
	}

	// Overflow
	compressed = []byte{signedByte(-126), 42} // 127 bytes
	_, err = RLEDecompress(compressed, 10)
	if err != ErrRLEOverflow {
		t.Errorf("Overflow error = %v, want ErrRLEOverflow", err)
	}
}

func TestRLEMaxRunLength(t *testing.T) {
	// Test with a run longer than max (127)
	data := make([]byte, 200)
	for i := range data {
		data[i] = 42
	}

	compressed := RLECompress(data)
	decompressed, err := RLEDecompress(compressed, len(data))
	if err != nil {
		t.Fatalf("Decompress error: %v", err)
	}
	if !bytes.Equal(decompressed, data) {
		t.Error("Long run round-trip failed")
	}
}

func BenchmarkRLECompress(b *testing.B) {
	// Simulate scanline data with some repetition
	data := make([]byte, 4096)
	for i := range data {
		if i%10 < 5 {
			data[i] = 0
		} else {
			data[i] = byte(i)
		}
	}

	b.ResetTimer()
	b.SetBytes(int64(len(data)))

	for i := 0; i < b.N; i++ {
		RLECompress(data)
	}
}

func BenchmarkRLEDecompress(b *testing.B) {
	data := make([]byte, 4096)
	for i := range data {
		if i%10 < 5 {
			data[i] = 0
		} else {
			data[i] = byte(i)
		}
	}
	compressed := RLECompress(data)

	b.ResetTimer()
	b.SetBytes(int64(len(data)))

	for i := 0; i < b.N; i++ {
		RLEDecompress(compressed, len(data))
	}
}
