package compression

import (
	"bytes"
	"testing"
)

func TestZIPCompressEmpty(t *testing.T) {
	result, err := ZIPCompress(nil)
	if err != nil || result != nil {
		t.Error("Compressing nil should return nil, nil")
	}

	result, err = ZIPCompress([]byte{})
	if err != nil || result != nil {
		t.Error("Compressing empty should return nil, nil")
	}
}

func TestZIPDecompressEmpty(t *testing.T) {
	result, err := ZIPDecompress(nil, 0)
	if err != nil || result != nil {
		t.Error("Decompressing nil should return nil, nil")
	}

	result, err = ZIPDecompress([]byte{}, 0)
	if err != nil || result != nil {
		t.Error("Decompressing empty should return nil, nil")
	}
}

func TestZIPRoundTrip(t *testing.T) {
	tests := [][]byte{
		{1},
		{1, 2},
		{1, 2, 3, 4, 5},
		{100, 100, 100, 100, 100, 100, 100, 100},
		{1, 2, 3, 3, 3, 3, 4, 5, 6},
		{1, 1, 1, 1, 2, 2, 2, 2, 3, 3, 3, 3},
	}

	for i, original := range tests {
		compressed, err := ZIPCompress(original)
		if err != nil {
			t.Errorf("test %d: compress error: %v", i, err)
			continue
		}

		decompressed, err := ZIPDecompress(compressed, len(original))
		if err != nil {
			t.Errorf("test %d: decompress error: %v", i, err)
			continue
		}
		if !bytes.Equal(decompressed, original) {
			t.Errorf("test %d: round-trip failed:\ngot  %v\nwant %v", i, decompressed, original)
		}
	}
}

func TestZIPRoundTripLarge(t *testing.T) {
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

	compressed, err := ZIPCompress(data)
	if err != nil {
		t.Fatalf("Compress error: %v", err)
	}

	decompressed, err := ZIPDecompress(compressed, len(data))
	if err != nil {
		t.Fatalf("Decompress error: %v", err)
	}
	if !bytes.Equal(decompressed, data) {
		t.Error("Large round-trip failed")
	}

	t.Logf("ZIP compression ratio: %d -> %d (%.1f%%)", len(data), len(compressed), 100.0*float64(len(compressed))/float64(len(data)))
}

func TestZIPDecompressErrors(t *testing.T) {
	// Wrong expected size
	data := []byte{1, 2, 3, 4, 5}
	compressed, _ := ZIPCompress(data)
	_, err := ZIPDecompress(compressed, 10) // Wrong size
	if err == nil {
		t.Error("Should error on wrong expected size")
	}

	// Corrupted data
	_, err = ZIPDecompress([]byte{0x78, 0x9c, 0xff, 0xff}, 5)
	if err == nil {
		t.Error("Should error on corrupted data")
	}

	// Empty compressed data expecting non-empty result
	_, err = ZIPDecompress(nil, 10)
	if err == nil {
		t.Error("Should error when expecting data from nil")
	}
}

func TestInterleaveEmpty(t *testing.T) {
	result := Interleave(nil)
	if result != nil {
		t.Error("Interleaving nil should return nil")
	}

	result = Interleave([]byte{})
	if result != nil {
		t.Error("Interleaving empty should return nil")
	}
}

func TestDeinterleaveEmpty(t *testing.T) {
	result := Deinterleave(nil)
	if result != nil {
		t.Error("Deinterleaving nil should return nil")
	}

	result = Deinterleave([]byte{})
	if result != nil {
		t.Error("Deinterleaving empty should return nil")
	}
}

func TestInterleaveDeinterleaveRoundTrip(t *testing.T) {
	tests := [][]byte{
		{1},
		{1, 2},
		{1, 2, 3},
		{1, 2, 3, 4},
		{1, 2, 3, 4, 5},
		{1, 2, 3, 4, 5, 6, 7, 8},
		{0, 1, 0, 2, 0, 3, 0, 4}, // Simulated half-precision values
	}

	for i, original := range tests {
		interleaved := Interleave(original)
		restored := Deinterleave(interleaved)
		if !bytes.Equal(restored, original) {
			t.Errorf("test %d: round-trip failed:\noriginal:    %v\ninterleaved: %v\nrestored:    %v",
				i, original, interleaved, restored)
		}
	}
}

func TestInterleavePattern(t *testing.T) {
	// Test specific interleave pattern
	// Input: [A0, A1, B0, B1, C0, C1] where subscript indicates byte position
	// Output: [A0, B0, C0, A1, B1, C1]
	input := []byte{0x10, 0x11, 0x20, 0x21, 0x30, 0x31}
	expected := []byte{0x10, 0x20, 0x30, 0x11, 0x21, 0x31}

	result := Interleave(input)
	if !bytes.Equal(result, expected) {
		t.Errorf("Interleave pattern:\ngot  %v\nwant %v", result, expected)
	}

	// Test deinterleave reverses it
	restored := Deinterleave(result)
	if !bytes.Equal(restored, input) {
		t.Errorf("Deinterleave pattern:\ngot  %v\nwant %v", restored, input)
	}
}

func TestInterleaveOddLength(t *testing.T) {
	// Odd length: extra byte goes to first half
	input := []byte{1, 2, 3, 4, 5}
	// Evens: 1, 3, 5 (indices 0, 2, 4)
	// Odds: 2, 4 (indices 1, 3)
	expected := []byte{1, 3, 5, 2, 4}

	result := Interleave(input)
	if !bytes.Equal(result, expected) {
		t.Errorf("Interleave odd length:\ngot  %v\nwant %v", result, expected)
	}

	restored := Deinterleave(result)
	if !bytes.Equal(restored, input) {
		t.Errorf("Deinterleave odd length:\ngot  %v\nwant %v", restored, input)
	}
}

func BenchmarkZIPCompress(b *testing.B) {
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
		ZIPCompress(data)
	}
}

func BenchmarkZIPDecompress(b *testing.B) {
	data := make([]byte, 4096)
	for i := range data {
		if i%10 < 5 {
			data[i] = 0
		} else {
			data[i] = byte(i)
		}
	}
	compressed, _ := ZIPCompress(data)

	b.ResetTimer()
	b.SetBytes(int64(len(data)))

	for i := 0; i < b.N; i++ {
		ZIPDecompress(compressed, len(data))
	}
}

func BenchmarkInterleave(b *testing.B) {
	data := make([]byte, 4096)
	for i := range data {
		data[i] = byte(i)
	}

	b.ResetTimer()
	b.SetBytes(int64(len(data)))

	for i := 0; i < b.N; i++ {
		Interleave(data)
	}
}

func BenchmarkDeinterleave(b *testing.B) {
	data := make([]byte, 4096)
	for i := range data {
		data[i] = byte(i)
	}

	b.ResetTimer()
	b.SetBytes(int64(len(data)))

	for i := 0; i < b.N; i++ {
		Deinterleave(data)
	}
}

// TestZIPCompressLevel tests compression at different levels
func TestZIPCompressLevel(t *testing.T) {
	data := make([]byte, 1024)
	for i := range data {
		data[i] = byte(i % 256)
	}

	levels := []CompressionLevel{
		CompressionLevelHuffmanOnly,
		CompressionLevelDefault,
		CompressionLevelNone,
		CompressionLevelBestSpeed,
		CompressionLevelBestSize,
	}

	for _, level := range levels {
		t.Run("", func(t *testing.T) {
			compressed, err := ZIPCompressLevel(data, level)
			if err != nil {
				t.Fatalf("ZIPCompressLevel(%d) error: %v", level, err)
			}

			// Decompress and verify
			decompressed, err := ZIPDecompress(compressed, len(data))
			if err != nil {
				t.Fatalf("ZIPDecompress error: %v", err)
			}

			if !bytes.Equal(decompressed, data) {
				t.Errorf("Round-trip failed for level %d", level)
			}
		})
	}
}

// TestZIPCompressLevelEmpty tests empty input
func TestZIPCompressLevelEmpty(t *testing.T) {
	result, err := ZIPCompressLevel(nil, CompressionLevelDefault)
	if err != nil || result != nil {
		t.Error("Empty compress should return nil, nil")
	}

	result, err = ZIPCompressLevel([]byte{}, CompressionLevelBestSpeed)
	if err != nil || result != nil {
		t.Error("Empty compress should return nil, nil")
	}
}

// TestZIPDecompressTo tests decompression into pre-allocated buffer
func TestZIPDecompressToBasic(t *testing.T) {
	tests := [][]byte{
		{1},
		{1, 2, 3, 4, 5},
		{100, 100, 100, 100, 100, 100, 100, 100},
	}

	for i, original := range tests {
		compressed, err := ZIPCompress(original)
		if err != nil {
			t.Errorf("test %d: compress error: %v", i, err)
			continue
		}

		dst := make([]byte, len(original))
		err = ZIPDecompressTo(dst, compressed)
		if err != nil {
			t.Errorf("test %d: ZIPDecompressTo error: %v", i, err)
			continue
		}

		if !bytes.Equal(dst, original) {
			t.Errorf("test %d: ZIPDecompressTo mismatch", i)
		}
	}
}

// TestZIPDecompressToEmpty tests empty input
func TestZIPDecompressToEmpty(t *testing.T) {
	// Empty src and dst should succeed
	err := ZIPDecompressTo(nil, nil)
	if err != nil {
		t.Errorf("ZIPDecompressTo(nil, nil) error: %v", err)
	}

	err = ZIPDecompressTo([]byte{}, []byte{})
	if err != nil {
		t.Errorf("ZIPDecompressTo(empty, empty) error: %v", err)
	}

	// Empty src with non-empty dst should error
	dst := make([]byte, 10)
	err = ZIPDecompressTo(dst, nil)
	if err != ErrZIPCorrupted {
		t.Errorf("Expected ErrZIPCorrupted, got %v", err)
	}

	err = ZIPDecompressTo(dst, []byte{})
	if err != ErrZIPCorrupted {
		t.Errorf("Expected ErrZIPCorrupted for empty src, got %v", err)
	}
}

// TestZIPDecompressToErrors tests error handling
func TestZIPDecompressToErrors(t *testing.T) {
	// Corrupted zlib data
	dst := make([]byte, 10)
	err := ZIPDecompressTo(dst, []byte{0x78, 0x9c, 0xFF, 0xFF})
	if err == nil {
		t.Error("Expected error for corrupted data")
	}

	// Invalid zlib header
	err = ZIPDecompressTo(dst, []byte{0x00, 0x00, 0x00, 0x00})
	if err != ErrZIPCorrupted {
		t.Errorf("Expected ErrZIPCorrupted for invalid header, got %v", err)
	}
}

// TestDetectZlibFLevelErrors tests error cases
func TestDetectZlibFLevelErrors(t *testing.T) {
	// Too short
	_, ok := DetectZlibFLevel(nil)
	if ok {
		t.Error("Expected false for nil input")
	}

	_, ok = DetectZlibFLevel([]byte{0x78})
	if ok {
		t.Error("Expected false for 1-byte input")
	}

	// Invalid compression method
	_, ok = DetectZlibFLevel([]byte{0x00, 0x00})
	if ok {
		t.Error("Expected false for invalid compression method")
	}

	// Invalid header checksum
	_, ok = DetectZlibFLevel([]byte{0x78, 0x00})
	if ok {
		t.Error("Expected false for invalid checksum")
	}
}

// TestFLevelToLevelUnknown tests FLevel to CompressionLevel conversion for unknown values
func TestFLevelToLevelUnknown(t *testing.T) {
	// Unknown level should default
	got := FLevelToLevel(FLevel(100))
	if got != CompressionLevelDefault {
		t.Errorf("FLevelToLevel(100) = %d, want %d", got, CompressionLevelDefault)
	}
}

// TestZIPDecompressWithLevel tests decompression with level detection
func TestZIPDecompressWithLevel(t *testing.T) {
	data := make([]byte, 1024)
	for i := range data {
		data[i] = byte(i % 256)
	}

	// Compress at different levels and verify detection
	levels := []CompressionLevel{
		CompressionLevelBestSpeed,
		CompressionLevelDefault,
		CompressionLevelBestSize,
	}

	for _, level := range levels {
		t.Run("", func(t *testing.T) {
			compressed, err := ZIPCompressLevel(data, level)
			if err != nil {
				t.Fatalf("Compress error: %v", err)
			}

			decompressed, flevel, err := ZIPDecompressWithLevel(compressed, len(data))
			if err != nil {
				t.Fatalf("DecompressWithLevel error: %v", err)
			}

			if !bytes.Equal(decompressed, data) {
				t.Error("Decompressed data mismatch")
			}

			// Verify flevel is reasonable
			if flevel < FLevelFastest || flevel > FLevelBest {
				t.Errorf("Invalid FLevel: %d", flevel)
			}
		})
	}
}

// TestZIPDecompressWithLevelErrors tests error handling
func TestZIPDecompressWithLevelErrors(t *testing.T) {
	// Empty with non-zero expected size
	_, _, err := ZIPDecompressWithLevel(nil, 10)
	if err != ErrZIPCorrupted {
		t.Errorf("Expected ErrZIPCorrupted, got %v", err)
	}

	// Invalid header
	_, _, err = ZIPDecompressWithLevel([]byte{0x00, 0x00}, 10)
	if err != ErrZIPCorrupted {
		t.Errorf("Expected ErrZIPCorrupted for invalid header, got %v", err)
	}
}

// TestZIPPoolReuse tests that pooled writers are properly reused
func TestZIPPoolReuse(t *testing.T) {
	data := make([]byte, 100)
	for i := range data {
		data[i] = byte(i)
	}

	// Compress multiple times to exercise pool
	for i := 0; i < 10; i++ {
		compressed, err := ZIPCompress(data)
		if err != nil {
			t.Fatalf("Compress %d error: %v", i, err)
		}

		decompressed, err := ZIPDecompress(compressed, len(data))
		if err != nil {
			t.Fatalf("Decompress %d error: %v", i, err)
		}

		if !bytes.Equal(decompressed, data) {
			t.Errorf("Round-trip %d failed", i)
		}
	}
}

// TestZIPCompressLevelNonDefault tests non-default compression levels
func TestZIPCompressLevelNonDefault(t *testing.T) {
	data := make([]byte, 2048)
	for i := range data {
		data[i] = byte(i % 256)
	}

	// Test a range of non-default levels
	nonDefaultLevels := []CompressionLevel{
		CompressionLevelHuffmanOnly, // -2
		CompressionLevelNone,        // 0
		CompressionLevelBestSpeed,   // 1
		2, 3, 4, 5,                  // Fast range
		7, 8, // Better range
		CompressionLevelBestSize, // 9
	}

	for _, level := range nonDefaultLevels {
		t.Run("", func(t *testing.T) {
			compressed, err := ZIPCompressLevel(data, level)
			if err != nil {
				t.Fatalf("ZIPCompressLevel(%d) error: %v", level, err)
			}

			// Verify decompression
			decompressed, err := ZIPDecompress(compressed, len(data))
			if err != nil {
				t.Fatalf("ZIPDecompress error for level %d: %v", level, err)
			}

			if !bytes.Equal(decompressed, data) {
				t.Errorf("Round-trip failed for level %d", level)
			}

			t.Logf("Level %d: %d -> %d bytes", level, len(data), len(compressed))
		})
	}
}

// TestZIPDecompressToPoolReuse tests pooled reader reuse
func TestZIPDecompressToPoolReuse(t *testing.T) {
	data := make([]byte, 500)
	for i := range data {
		data[i] = byte(i % 256)
	}

	compressed, err := ZIPCompress(data)
	if err != nil {
		t.Fatalf("Compress error: %v", err)
	}

	// Decompress multiple times to exercise reader pool reuse
	for i := 0; i < 20; i++ {
		dst := make([]byte, len(data))
		err := ZIPDecompressTo(dst, compressed)
		if err != nil {
			t.Fatalf("Decompress %d error: %v", i, err)
		}

		if !bytes.Equal(dst, data) {
			t.Errorf("Round-trip %d failed", i)
		}
	}
}

// TestZIPDecompressToLargeData tests with larger data
func TestZIPDecompressToLargeData(t *testing.T) {
	data := make([]byte, 65536)
	for i := range data {
		data[i] = byte(i % 256)
	}

	compressed, err := ZIPCompress(data)
	if err != nil {
		t.Fatalf("Compress error: %v", err)
	}

	dst := make([]byte, len(data))
	err = ZIPDecompressTo(dst, compressed)
	if err != nil {
		t.Fatalf("DecompressTo error: %v", err)
	}

	if !bytes.Equal(dst, data) {
		t.Error("Large data round-trip failed")
	}
}

// TestZIPDecompressToSizeMismatch tests when decompressed size doesn't match expected
func TestZIPDecompressToSizeMismatch(t *testing.T) {
	data := []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	compressed, err := ZIPCompress(data)
	if err != nil {
		t.Fatalf("Compress error: %v", err)
	}

	// Try with dst larger than actual decompressed size
	dst := make([]byte, len(data)+10)
	err = ZIPDecompressTo(dst, compressed)
	if err == nil {
		t.Error("Expected error when dst is larger than decompressed data")
	}
}

// TestZIPCompressLevelWriteError tests error path during write
func TestZIPCompressLevelLargeData(t *testing.T) {
	// Test with large data to exercise the full compression path
	data := make([]byte, 1024*1024) // 1MB
	for i := range data {
		data[i] = byte(i % 256)
	}

	compressed, err := ZIPCompressLevel(data, CompressionLevelDefault)
	if err != nil {
		t.Fatalf("ZIPCompressLevel error: %v", err)
	}

	// Verify round-trip
	decompressed, err := ZIPDecompress(compressed, len(data))
	if err != nil {
		t.Fatalf("Decompress error: %v", err)
	}

	if !bytes.Equal(decompressed, data) {
		t.Error("Large data round-trip failed")
	}

	t.Logf("Large data compression: %d -> %d bytes (%.1f%%)",
		len(data), len(compressed), 100.0*float64(len(compressed))/float64(len(data)))
}

// TestZIPDecompressToSequential exercises the reader pool reset path
func TestZIPDecompressToSequential(t *testing.T) {
	// Run many decompressions sequentially to exercise pool reuse and reset paths
	dataSizes := []int{100, 200, 500, 1000, 2000, 5000}

	for _, size := range dataSizes {
		data := make([]byte, size)
		for i := range data {
			data[i] = byte(i % 256)
		}

		compressed, err := ZIPCompress(data)
		if err != nil {
			t.Fatalf("Compress error for size %d: %v", size, err)
		}

		// Decompress multiple times
		for i := 0; i < 5; i++ {
			dst := make([]byte, size)
			err = ZIPDecompressTo(dst, compressed)
			if err != nil {
				t.Fatalf("DecompressTo error for size %d, iteration %d: %v", size, i, err)
			}

			if !bytes.Equal(dst, data) {
				t.Errorf("Mismatch for size %d, iteration %d", size, i)
			}
		}
	}
}
