package compression

import (
	"testing"
)

func TestPackUnpack14(t *testing.T) {
	// Test pack/unpack round-trip for normal blocks
	original := [16]uint16{
		0x3c00, 0x3c00, 0x3c00, 0x3c00, // 1.0
		0x3c00, 0x4000, 0x4000, 0x3c00, // 1.0, 2.0, 2.0, 1.0
		0x3c00, 0x4000, 0x4000, 0x3c00,
		0x3c00, 0x3c00, 0x3c00, 0x3c00,
	}

	var packed [14]byte
	n := packB44(original, packed[:], false, true)

	if n != 14 {
		t.Errorf("packB44 returned %d bytes, want 14", n)
	}

	var unpacked [16]uint16
	unpack14(packed[:], &unpacked)

	// B44 is lossy, so we check within tolerance
	for i := 0; i < 16; i++ {
		origF := halfToFloat32(original[i])
		unpackF := halfToFloat32(unpacked[i])
		diff := origF - unpackF
		if diff < 0 {
			diff = -diff
		}
		// Allow ~1% tolerance
		tolerance := origF * 0.05
		if tolerance < 0.01 {
			tolerance = 0.01
		}
		if diff > tolerance {
			t.Errorf("Pixel %d: original %v, unpacked %v, diff %v", i, origF, unpackF, diff)
		}
	}
}

func TestPackUnpack3FlatField(t *testing.T) {
	// Test flat field (all same value) round-trip
	original := [16]uint16{
		0x3c00, 0x3c00, 0x3c00, 0x3c00, // All 1.0
		0x3c00, 0x3c00, 0x3c00, 0x3c00,
		0x3c00, 0x3c00, 0x3c00, 0x3c00,
		0x3c00, 0x3c00, 0x3c00, 0x3c00,
	}

	var packed [14]byte
	n := packB44(original, packed[:], true, true) // flatfields = true

	if n != 3 {
		t.Errorf("packB44 for flat field returned %d bytes, want 3", n)
	}

	var unpacked [16]uint16
	unpack3(packed[:], &unpacked)

	// Flat field should be exact
	for i := 0; i < 16; i++ {
		if unpacked[i] != original[i] {
			t.Errorf("Pixel %d: original %x, unpacked %x", i, original[i], unpacked[i])
		}
	}
}

func TestB44RoundtripSimple(t *testing.T) {
	// Simple test with uniform half data
	channels := []B44ChannelInfo{
		{Type: b44PixelTypeHalf, Width: 8, Height: 8},
	}

	// Create test data - all 1.0 (0x3c00)
	data := make([]byte, 8*8*2)
	for i := 0; i < len(data)/2; i++ {
		data[i*2] = 0x00
		data[i*2+1] = 0x3c
	}

	compressed, err := B44Compress(data, channels, 8, 8, false)
	if err != nil {
		t.Fatalf("B44Compress failed: %v", err)
	}

	t.Logf("Original size: %d, compressed size: %d", len(data), len(compressed))

	decompressed, err := B44Decompress(compressed, channels, 8, 8, len(data))
	if err != nil {
		t.Fatalf("B44Decompress failed: %v", err)
	}

	// B44 is lossy, check approximate equality
	for i := 0; i < len(data)/2; i++ {
		origV := uint16(data[i*2]) | uint16(data[i*2+1])<<8
		decV := uint16(decompressed[i*2]) | uint16(decompressed[i*2+1])<<8

		origF := halfToFloat32(origV)
		decF := halfToFloat32(decV)

		diff := origF - decF
		if diff < 0 {
			diff = -diff
		}
		if diff > 0.1 {
			t.Errorf("Pixel %d: original %v, decompressed %v", i, origF, decF)
		}
	}
}

func TestB44RoundtripVaried(t *testing.T) {
	// Test with varied data (gradient)
	channels := []B44ChannelInfo{
		{Type: b44PixelTypeHalf, Width: 8, Height: 8},
	}

	// Create gradient data
	data := make([]byte, 8*8*2)
	for i := 0; i < 64; i++ {
		// Values from 0.0 to 1.0
		f := float32(i) / 63.0
		h := float32ToHalf(f)
		data[i*2] = byte(h)
		data[i*2+1] = byte(h >> 8)
	}

	compressed, err := B44Compress(data, channels, 8, 8, false)
	if err != nil {
		t.Fatalf("B44Compress failed: %v", err)
	}

	t.Logf("Gradient: original size: %d, compressed size: %d", len(data), len(compressed))

	decompressed, err := B44Decompress(compressed, channels, 8, 8, len(data))
	if err != nil {
		t.Fatalf("B44Decompress failed: %v", err)
	}

	// B44 is lossy, check approximate equality with higher tolerance
	maxDiff := float32(0)
	for i := 0; i < len(data)/2; i++ {
		origV := uint16(data[i*2]) | uint16(data[i*2+1])<<8
		decV := uint16(decompressed[i*2]) | uint16(decompressed[i*2+1])<<8

		origF := halfToFloat32(origV)
		decF := halfToFloat32(decV)

		diff := origF - decF
		if diff < 0 {
			diff = -diff
		}
		if diff > maxDiff {
			maxDiff = diff
		}
	}

	t.Logf("Max difference: %v", maxDiff)

	// B44 can have up to ~5% error for lossy compression
	if maxDiff > 0.1 {
		t.Errorf("Max difference %v exceeds tolerance", maxDiff)
	}
}

func TestB44AFlatFieldCompression(t *testing.T) {
	// Test B44A (with flat field detection)
	channels := []B44ChannelInfo{
		{Type: b44PixelTypeHalf, Width: 8, Height: 8},
	}

	// Create uniform data
	data := make([]byte, 8*8*2)
	for i := 0; i < len(data)/2; i++ {
		data[i*2] = 0x00
		data[i*2+1] = 0x3c // 1.0
	}

	// With flatfields=true (B44A mode), should get better compression
	compressedA, err := B44Compress(data, channels, 8, 8, true)
	if err != nil {
		t.Fatalf("B44A Compress failed: %v", err)
	}

	// With flatfields=false (B44 mode)
	compressed, err := B44Compress(data, channels, 8, 8, false)
	if err != nil {
		t.Fatalf("B44 Compress failed: %v", err)
	}

	t.Logf("B44 size: %d, B44A size: %d", len(compressed), len(compressedA))

	// B44A should be smaller or equal for uniform data
	if len(compressedA) > len(compressed) {
		t.Errorf("B44A should be at least as good as B44 for uniform data")
	}
}

func TestHalfConversions(t *testing.T) {
	// Test half-to-float32 conversions
	tests := []struct {
		half uint16
		want float32
	}{
		{0x0000, 0.0},        // +0
		{0x8000, 0.0},        // -0 (treated as 0)
		{0x3c00, 1.0},        // 1.0
		{0x4000, 2.0},        // 2.0
		{0xbc00, -1.0},       // -1.0
		{0x3800, 0.5},        // 0.5
		{0x7c00, float32(1)}, // +Inf (will be special)
	}

	for _, tt := range tests {
		got := halfToFloat32(tt.half)
		// Handle special cases
		if tt.half == 0x7c00 {
			// Check for infinity
			if got < 1e30 {
				t.Errorf("halfToFloat32(0x%04x) = %v, want +Inf", tt.half, got)
			}
			continue
		}
		if got != tt.want {
			// Allow small tolerance for float comparison
			diff := got - tt.want
			if diff < 0 {
				diff = -diff
			}
			if diff > 0.001 {
				t.Errorf("halfToFloat32(0x%04x) = %v, want %v", tt.half, got, tt.want)
			}
		}
	}
}

// Benchmarks

func BenchmarkPackB44(b *testing.B) {
	// Create test data with some variation
	var s [16]uint16
	for i := 0; i < 16; i++ {
		s[i] = float32ToHalf(float32(i) / 15.0)
	}
	var packed [14]byte

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		packB44(s, packed[:], false, true)
	}
}

func BenchmarkUnpack14(b *testing.B) {
	// Create packed data
	var s [16]uint16
	for i := 0; i < 16; i++ {
		s[i] = float32ToHalf(float32(i) / 15.0)
	}
	var packed [14]byte
	packB44(s, packed[:], false, true)

	var unpacked [16]uint16

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		unpack14(packed[:], &unpacked)
	}
}

func BenchmarkB44Compress(b *testing.B) {
	// Create test image 64x64 with 3 half channels
	width, height := 64, 64
	channels := []B44ChannelInfo{
		{Type: b44PixelTypeHalf, Width: width, Height: height},
		{Type: b44PixelTypeHalf, Width: width, Height: height},
		{Type: b44PixelTypeHalf, Width: width, Height: height},
	}

	// Create gradient data
	data := make([]byte, width*height*3*2)
	for c := 0; c < 3; c++ {
		for i := 0; i < width*height; i++ {
			f := float32(i) / float32(width*height-1)
			h := float32ToHalf(f)
			offset := (c*width*height + i) * 2
			data[offset] = byte(h)
			data[offset+1] = byte(h >> 8)
		}
	}

	b.SetBytes(int64(len(data)))
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, _ = B44Compress(data, channels, width, height, false)
	}
}

func BenchmarkB44Decompress(b *testing.B) {
	// Create test image and compress it first
	width, height := 64, 64
	channels := []B44ChannelInfo{
		{Type: b44PixelTypeHalf, Width: width, Height: height},
		{Type: b44PixelTypeHalf, Width: width, Height: height},
		{Type: b44PixelTypeHalf, Width: width, Height: height},
	}

	// Create gradient data
	data := make([]byte, width*height*3*2)
	for c := 0; c < 3; c++ {
		for i := 0; i < width*height; i++ {
			f := float32(i) / float32(width*height-1)
			h := float32ToHalf(f)
			offset := (c*width*height + i) * 2
			data[offset] = byte(h)
			data[offset+1] = byte(h >> 8)
		}
	}

	compressed, _ := B44Compress(data, channels, width, height, false)
	expectedSize := len(data)

	b.SetBytes(int64(expectedSize))
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, _ = B44Decompress(compressed, channels, width, height, expectedSize)
	}
}
