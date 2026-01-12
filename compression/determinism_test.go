package compression

import (
	"crypto/sha256"
	"encoding/binary"
	"testing"
)

// TestZIPCompressionDeterminism verifies that compressing the same data
// always produces identical output.
func TestZIPCompressionDeterminism(t *testing.T) {
	// Create test data with some repetition for meaningful compression
	data := make([]byte, 4096)
	for i := range data {
		data[i] = byte(i % 64) // Creates patterns
	}

	var hashes [][32]byte
	for i := 0; i < 10; i++ {
		compressed, err := ZIPCompress(data)
		if err != nil {
			t.Fatalf("ZIPCompress error: %v", err)
		}
		hashes = append(hashes, sha256.Sum256(compressed))
	}

	// All hashes must be identical
	for i := 1; i < len(hashes); i++ {
		if hashes[i] != hashes[0] {
			t.Errorf("Non-deterministic ZIP compression: hash[0] != hash[%d]", i)
		}
	}
	t.Logf("ZIP compression is deterministic (10 runs, hash=%x)", hashes[0][:8])
}

// TestRLECompressionDeterminism verifies RLE determinism.
func TestRLECompressionDeterminism(t *testing.T) {
	// Data with runs for RLE
	data := make([]byte, 1000)
	for i := range data {
		data[i] = byte(i / 50) // Creates runs of 50 identical bytes
	}

	var hashes [][32]byte
	for i := 0; i < 10; i++ {
		compressed := RLECompress(data)
		hashes = append(hashes, sha256.Sum256(compressed))
	}

	for i := 1; i < len(hashes); i++ {
		if hashes[i] != hashes[0] {
			t.Errorf("Non-deterministic RLE: hash[0] != hash[%d]", i)
		}
	}
	t.Logf("RLE compression is deterministic (10 runs, hash=%x)", hashes[0][:8])
}

// TestPIZCompressionDeterminism verifies PIZ determinism.
func TestPIZCompressionDeterminism(t *testing.T) {
	// Create test image data - PIZCompress takes []uint16
	width, height, numChannels := 64, 32, 3
	pixelCount := width * height * numChannels
	data := make([]uint16, pixelCount)
	for i := range data {
		data[i] = uint16(i % 65536)
	}

	var hashes [][32]byte
	for i := 0; i < 5; i++ {
		compressed, err := PIZCompress(data, width, height, numChannels)
		if err != nil {
			t.Fatalf("PIZCompress error: %v", err)
		}
		hashes = append(hashes, sha256.Sum256(compressed))
	}

	for i := 1; i < len(hashes); i++ {
		if hashes[i] != hashes[0] {
			t.Errorf("Non-deterministic PIZ: hash[0] != hash[%d]", i)
		}
	}
	t.Logf("PIZ compression is deterministic (5 runs, hash=%x)", hashes[0][:8])
}

// TestB44CompressionDeterminism verifies B44 determinism.
func TestB44CompressionDeterminism(t *testing.T) {
	// B44 requires 4x4 aligned dimensions
	width, height := 64, 64
	channels := []B44ChannelInfo{
		{Type: 1, Width: width, Height: height, XSampling: 1, YSampling: 1}, // HALF
	}

	// Create pixel data as bytes (half-float = 2 bytes per pixel)
	dataSize := 2 * width * height
	data := make([]byte, dataSize)
	for i := 0; i < width*height; i++ {
		// Store uint16 values as little-endian bytes
		binary.LittleEndian.PutUint16(data[i*2:], uint16(i%65536))
	}

	var hashes [][32]byte
	for i := 0; i < 5; i++ {
		compressed, err := B44Compress(data, channels, width, height, false)
		if err != nil {
			t.Fatalf("B44Compress error: %v", err)
		}
		hashes = append(hashes, sha256.Sum256(compressed))
	}

	for i := 1; i < len(hashes); i++ {
		if hashes[i] != hashes[0] {
			t.Errorf("Non-deterministic B44: hash[0] != hash[%d]", i)
		}
	}
	t.Logf("B44 compression is deterministic (5 runs, hash=%x)", hashes[0][:8])
}

// TestDetectZlibFLevel verifies FLEVEL detection from compressed data.
func TestDetectZlibFLevel(t *testing.T) {
	data := make([]byte, 4096)
	for i := range data {
		data[i] = byte(i % 64)
	}

	tests := []struct {
		level          CompressionLevel
		expectedFLevel FLevel
	}{
		{1, FLevelFastest},  // Level 1 -> FLEVEL 0
		{4, FLevelFast},     // Level 4 -> FLEVEL 1
		{-1, FLevelDefault}, // Default -> FLEVEL 2
		{6, FLevelDefault},  // Level 6 -> FLEVEL 2
		{9, FLevelBest},     // Level 9 -> FLEVEL 3
	}

	for _, tc := range tests {
		compressed, err := ZIPCompressLevel(data, tc.level)
		if err != nil {
			t.Fatalf("ZIPCompressLevel(%d) error: %v", tc.level, err)
		}

		flevel, ok := DetectZlibFLevel(compressed)
		if !ok {
			t.Errorf("DetectZlibFLevel failed for level %d", tc.level)
			continue
		}

		if flevel != tc.expectedFLevel {
			t.Errorf("Level %d: expected FLEVEL %d, got %d", tc.level, tc.expectedFLevel, flevel)
		}
	}
}

// TestZIPCompressLevelRoundTrip verifies compression level round-trip.
func TestZIPCompressLevelRoundTrip(t *testing.T) {
	data := make([]byte, 4096)
	for i := range data {
		data[i] = byte(i % 64)
	}

	// Test each compression level
	for level := CompressionLevel(1); level <= 9; level++ {
		compressed, err := ZIPCompressLevel(data, level)
		if err != nil {
			t.Fatalf("ZIPCompressLevel(%d) error: %v", level, err)
		}

		// Decompress with level detection
		decompressed, flevel, err := ZIPDecompressWithLevel(compressed, len(data))
		if err != nil {
			t.Fatalf("ZIPDecompressWithLevel error: %v", err)
		}

		// Verify data integrity
		for i, b := range decompressed {
			if b != data[i] {
				t.Fatalf("Data mismatch at level %d, byte %d: %d != %d", level, i, b, data[i])
			}
		}

		// Re-compress with detected level
		recommendedLevel := FLevelToLevel(flevel)
		recompressed, err := ZIPCompressLevel(data, recommendedLevel)
		if err != nil {
			t.Fatalf("Re-compress error: %v", err)
		}

		// Verify recompressed data decompresses correctly
		redecompressed, _, err := ZIPDecompressWithLevel(recompressed, len(data))
		if err != nil {
			t.Fatalf("Re-decompress error: %v", err)
		}

		for i, b := range redecompressed {
			if b != data[i] {
				t.Fatalf("Re-decompressed data mismatch at byte %d", i)
			}
		}

		t.Logf("Level %d -> FLEVEL %d -> Recommended level %d: OK", level, flevel, recommendedLevel)
	}
}

// TestZIPCompressLevelDeterminism verifies each compression level is deterministic.
func TestZIPCompressLevelDeterminism(t *testing.T) {
	data := make([]byte, 4096)
	for i := range data {
		data[i] = byte(i % 64)
	}

	for level := CompressionLevel(1); level <= 9; level++ {
		var hashes [][32]byte
		for i := 0; i < 5; i++ {
			compressed, err := ZIPCompressLevel(data, level)
			if err != nil {
				t.Fatalf("ZIPCompressLevel(%d) error: %v", level, err)
			}
			hashes = append(hashes, sha256.Sum256(compressed))
		}

		for i := 1; i < len(hashes); i++ {
			if hashes[i] != hashes[0] {
				t.Errorf("Non-deterministic ZIP level %d: hash[0] != hash[%d]", level, i)
			}
		}
	}
	t.Log("All ZIP compression levels are deterministic")
}

// TestFLevelToLevel verifies FLevel to level mapping.
func TestFLevelToLevel(t *testing.T) {
	tests := []struct {
		flevel   FLevel
		expected CompressionLevel
	}{
		{FLevelFastest, 1},
		{FLevelFast, 4},
		{FLevelDefault, CompressionLevelDefault},
		{FLevelBest, 9},
	}

	for _, tc := range tests {
		got := FLevelToLevel(tc.flevel)
		if got != tc.expected {
			t.Errorf("FLevelToLevel(%d) = %d, want %d", tc.flevel, got, tc.expected)
		}
	}
}
