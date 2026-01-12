package compression

import (
	"math"
	"testing"
)

func TestFloatToFloat24(t *testing.T) {
	tests := []struct {
		name    string
		input   float32
		wantBig bool // For NaN/Inf special cases
	}{
		{"zero", 0.0, false},
		{"one", 1.0, false},
		{"negative", -1.5, false},
		{"small", 0.00001, false},
		{"large", 1000000.0, false},
		{"infinity", float32(math.Inf(1)), true},
		{"neg_infinity", float32(math.Inf(-1)), true},
		{"nan", float32(math.NaN()), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f24 := floatToFloat24(tt.input)

			// The result should fit in 24 bits
			if f24 > 0xFFFFFF {
				t.Errorf("floatToFloat24(%v) = %x, exceeds 24 bits", tt.input, f24)
			}

			// Convert back
			f32 := float24ToFloat32(f24)

			if tt.wantBig {
				// Check special values preserved
				if math.IsNaN(float64(tt.input)) {
					if !math.IsNaN(float64(f32)) {
						t.Errorf("NaN not preserved: got %v", f32)
					}
				} else if math.IsInf(float64(tt.input), 1) {
					if !math.IsInf(float64(f32), 1) {
						t.Errorf("+Inf not preserved: got %v", f32)
					}
				} else if math.IsInf(float64(tt.input), -1) {
					if !math.IsInf(float64(f32), -1) {
						t.Errorf("-Inf not preserved: got %v", f32)
					}
				}
			} else {
				// For normal values, check approximate equality
				// PXR24 loses 8 bits of precision in the mantissa
				diff := math.Abs(float64(f32 - tt.input))
				tolerance := math.Abs(float64(tt.input)) * 0.001 // 0.1% tolerance
				if tolerance < 1e-10 {
					tolerance = 1e-10
				}
				if diff > tolerance {
					t.Errorf("floatToFloat24(%v) -> float24ToFloat32 = %v, diff = %v", tt.input, f32, diff)
				}
			}
		})
	}
}

func TestPXR24RoundtripHalf(t *testing.T) {
	// Test with Half (16-bit) data - should be lossless
	channels := []ChannelInfo{
		{Type: pxr24PixelTypeHalf, Width: 8, Height: 4},
	}

	width := 8
	height := 4

	// Create test data
	data := make([]byte, width*height*2)
	for i := 0; i < len(data)/2; i++ {
		// Some test half values
		val := uint16(i * 100)
		data[i*2] = byte(val)
		data[i*2+1] = byte(val >> 8)
	}

	compressed, err := PXR24Compress(data, channels, width, height)
	if err != nil {
		t.Fatalf("PXR24Compress failed: %v", err)
	}

	decompressed, err := PXR24Decompress(compressed, channels, width, height, len(data))
	if err != nil {
		t.Fatalf("PXR24Decompress failed: %v", err)
	}

	// Should be exactly equal for Half data
	for i := 0; i < len(data); i++ {
		if data[i] != decompressed[i] {
			t.Errorf("Mismatch at byte %d: want %d, got %d", i, data[i], decompressed[i])
		}
	}
}

func TestPXR24RoundtripUint(t *testing.T) {
	// Test with Uint (32-bit) data - should be lossless
	channels := []ChannelInfo{
		{Type: pxr24PixelTypeUint, Width: 8, Height: 4},
	}

	width := 8
	height := 4

	// Create test data
	data := make([]byte, width*height*4)
	for i := 0; i < len(data)/4; i++ {
		val := uint32(i * 12345)
		data[i*4] = byte(val)
		data[i*4+1] = byte(val >> 8)
		data[i*4+2] = byte(val >> 16)
		data[i*4+3] = byte(val >> 24)
	}

	compressed, err := PXR24Compress(data, channels, width, height)
	if err != nil {
		t.Fatalf("PXR24Compress failed: %v", err)
	}

	decompressed, err := PXR24Decompress(compressed, channels, width, height, len(data))
	if err != nil {
		t.Fatalf("PXR24Decompress failed: %v", err)
	}

	// Should be exactly equal for Uint data
	for i := 0; i < len(data); i++ {
		if data[i] != decompressed[i] {
			t.Errorf("Mismatch at byte %d: want %d, got %d", i, data[i], decompressed[i])
		}
	}
}

func TestPXR24RoundtripFloat(t *testing.T) {
	// Test with Float (32-bit) data - lossy
	channels := []ChannelInfo{
		{Type: pxr24PixelTypeFloat, Width: 8, Height: 4},
	}

	width := 8
	height := 4

	// Create test data
	data := make([]byte, width*height*4)
	for i := 0; i < len(data)/4; i++ {
		f := float32(i) * 0.1
		bits := math.Float32bits(f)
		data[i*4] = byte(bits)
		data[i*4+1] = byte(bits >> 8)
		data[i*4+2] = byte(bits >> 16)
		data[i*4+3] = byte(bits >> 24)
	}

	compressed, err := PXR24Compress(data, channels, width, height)
	if err != nil {
		t.Fatalf("PXR24Compress failed: %v", err)
	}

	t.Logf("Original size: %d, compressed size: %d", len(data), len(compressed))

	decompressed, err := PXR24Decompress(compressed, channels, width, height, len(data))
	if err != nil {
		t.Fatalf("PXR24Decompress failed: %v", err)
	}

	// Check approximate equality for Float data (lossy compression)
	for i := 0; i < len(data)/4; i++ {
		origBits := uint32(data[i*4]) | uint32(data[i*4+1])<<8 |
			uint32(data[i*4+2])<<16 | uint32(data[i*4+3])<<24
		origF := math.Float32frombits(origBits)

		decBits := uint32(decompressed[i*4]) | uint32(decompressed[i*4+1])<<8 |
			uint32(decompressed[i*4+2])<<16 | uint32(decompressed[i*4+3])<<24
		decF := math.Float32frombits(decBits)

		diff := math.Abs(float64(decF - origF))
		tolerance := math.Abs(float64(origF)) * 0.001
		if tolerance < 1e-10 {
			tolerance = 1e-10
		}

		if diff > tolerance {
			t.Errorf("Float mismatch at pixel %d: orig=%v, decompressed=%v, diff=%v",
				i, origF, decF, diff)
		}
	}
}

func TestPXR24RoundtripMixedChannels(t *testing.T) {
	// Test with multiple channels of different types
	channels := []ChannelInfo{
		{Type: pxr24PixelTypeFloat, Width: 4, Height: 2}, // A
		{Type: pxr24PixelTypeFloat, Width: 4, Height: 2}, // B
		{Type: pxr24PixelTypeFloat, Width: 4, Height: 2}, // G
		{Type: pxr24PixelTypeFloat, Width: 4, Height: 2}, // R
	}

	width := 4
	height := 2

	// Calculate data size
	dataSize := 0
	for _, ch := range channels {
		switch ch.Type {
		case pxr24PixelTypeUint:
			dataSize += ch.Width * ch.Height * 4
		case pxr24PixelTypeHalf:
			dataSize += ch.Width * ch.Height * 2
		case pxr24PixelTypeFloat:
			dataSize += ch.Width * ch.Height * 4
		}
	}

	// Create test data
	data := make([]byte, dataSize)
	offset := 0
	for y := 0; y < height; y++ {
		for _, ch := range channels {
			for x := 0; x < ch.Width; x++ {
				f := float32(x+y) * 0.25
				bits := math.Float32bits(f)
				data[offset] = byte(bits)
				data[offset+1] = byte(bits >> 8)
				data[offset+2] = byte(bits >> 16)
				data[offset+3] = byte(bits >> 24)
				offset += 4
			}
		}
	}

	compressed, err := PXR24Compress(data, channels, width, height)
	if err != nil {
		t.Fatalf("PXR24Compress failed: %v", err)
	}

	decompressed, err := PXR24Decompress(compressed, channels, width, height, len(data))
	if err != nil {
		t.Fatalf("PXR24Decompress failed: %v", err)
	}

	// Check approximate equality
	for i := 0; i < len(data)/4; i++ {
		origBits := uint32(data[i*4]) | uint32(data[i*4+1])<<8 |
			uint32(data[i*4+2])<<16 | uint32(data[i*4+3])<<24
		origF := math.Float32frombits(origBits)

		decBits := uint32(decompressed[i*4]) | uint32(decompressed[i*4+1])<<8 |
			uint32(decompressed[i*4+2])<<16 | uint32(decompressed[i*4+3])<<24
		decF := math.Float32frombits(decBits)

		diff := math.Abs(float64(decF - origF))
		tolerance := math.Abs(float64(origF)) * 0.001
		if tolerance < 1e-10 {
			tolerance = 1e-10
		}

		if diff > tolerance {
			t.Errorf("Mixed channel mismatch at pixel %d: orig=%v, decompressed=%v",
				i, origF, decF)
		}
	}
}

// TestPXR24DecompressUncompressed tests fallback when data is not compressed
func TestPXR24DecompressUncompressed(t *testing.T) {
	// When compression expands the data, PXR24Compress returns uncompressed data
	// Create highly random data that doesn't compress well
	channels := []ChannelInfo{
		{Type: pxr24PixelTypeUint, Width: 4, Height: 4},
	}

	width := 4
	height := 4

	// Create random data
	data := make([]byte, width*height*4)
	for i := range data {
		data[i] = byte((i*17 + 31) % 256)
	}

	compressed, err := PXR24Compress(data, channels, width, height)
	if err != nil {
		t.Fatalf("PXR24Compress failed: %v", err)
	}

	// Whether compressed or not, decompression should work
	decompressed, err := PXR24Decompress(compressed, channels, width, height, len(data))
	if err != nil {
		t.Fatalf("PXR24Decompress failed: %v", err)
	}

	// Verify data matches
	for i := 0; i < len(data); i++ {
		if data[i] != decompressed[i] {
			t.Errorf("Mismatch at byte %d: want %d, got %d", i, data[i], decompressed[i])
		}
	}
}

// TestPXR24RoundtripMixedTypes tests with different channel types in one image
func TestPXR24RoundtripMixedTypes(t *testing.T) {
	channels := []ChannelInfo{
		{Type: pxr24PixelTypeHalf, Width: 8, Height: 4},  // Half channel
		{Type: pxr24PixelTypeUint, Width: 8, Height: 4},  // Uint channel
		{Type: pxr24PixelTypeFloat, Width: 8, Height: 4}, // Float channel
	}

	width := 8
	height := 4

	// Calculate data size
	dataSize := 0
	for _, ch := range channels {
		switch ch.Type {
		case pxr24PixelTypeUint, pxr24PixelTypeFloat:
			dataSize += ch.Width * ch.Height * 4
		case pxr24PixelTypeHalf:
			dataSize += ch.Width * ch.Height * 2
		}
	}

	// Create test data
	data := make([]byte, dataSize)
	offset := 0
	for y := 0; y < height; y++ {
		for _, ch := range channels {
			for x := 0; x < ch.Width; x++ {
				switch ch.Type {
				case pxr24PixelTypeHalf:
					val := uint16((x + y) * 100)
					data[offset] = byte(val)
					data[offset+1] = byte(val >> 8)
					offset += 2
				case pxr24PixelTypeUint:
					val := uint32((x + y) * 1000)
					data[offset] = byte(val)
					data[offset+1] = byte(val >> 8)
					data[offset+2] = byte(val >> 16)
					data[offset+3] = byte(val >> 24)
					offset += 4
				case pxr24PixelTypeFloat:
					f := float32(x+y) * 0.5
					bits := math.Float32bits(f)
					data[offset] = byte(bits)
					data[offset+1] = byte(bits >> 8)
					data[offset+2] = byte(bits >> 16)
					data[offset+3] = byte(bits >> 24)
					offset += 4
				}
			}
		}
	}

	compressed, err := PXR24Compress(data, channels, width, height)
	if err != nil {
		t.Fatalf("PXR24Compress failed: %v", err)
	}

	decompressed, err := PXR24Decompress(compressed, channels, width, height, len(data))
	if err != nil {
		t.Fatalf("PXR24Decompress failed: %v", err)
	}

	// Verify Half and Uint channels are lossless
	offset = 0
	for y := 0; y < height; y++ {
		for _, ch := range channels {
			for x := 0; x < ch.Width; x++ {
				switch ch.Type {
				case pxr24PixelTypeHalf:
					if data[offset] != decompressed[offset] || data[offset+1] != decompressed[offset+1] {
						t.Errorf("Half channel mismatch at (%d,%d)", x, y)
					}
					offset += 2
				case pxr24PixelTypeUint:
					for i := 0; i < 4; i++ {
						if data[offset+i] != decompressed[offset+i] {
							t.Errorf("Uint channel mismatch at (%d,%d)", x, y)
							break
						}
					}
					offset += 4
				case pxr24PixelTypeFloat:
					// Float is lossy, just check approximate
					origBits := uint32(data[offset]) | uint32(data[offset+1])<<8 |
						uint32(data[offset+2])<<16 | uint32(data[offset+3])<<24
					origF := math.Float32frombits(origBits)

					decBits := uint32(decompressed[offset]) | uint32(decompressed[offset+1])<<8 |
						uint32(decompressed[offset+2])<<16 | uint32(decompressed[offset+3])<<24
					decF := math.Float32frombits(decBits)

					diff := math.Abs(float64(decF - origF))
					tolerance := math.Abs(float64(origF)) * 0.01
					if tolerance < 1e-10 {
						tolerance = 1e-10
					}
					if diff > tolerance {
						t.Errorf("Float channel mismatch at (%d,%d): %v vs %v", x, y, origF, decF)
					}
					offset += 4
				}
			}
		}
	}
}

// TestPXR24EmptyChannel tests channels with zero height
func TestPXR24EmptyChannel(t *testing.T) {
	channels := []ChannelInfo{
		{Type: pxr24PixelTypeHalf, Width: 8, Height: 0}, // Empty channel
		{Type: pxr24PixelTypeUint, Width: 8, Height: 4}, // Normal channel
	}

	width := 8
	height := 4

	// Only the Uint channel has data
	dataSize := width * height * 4
	data := make([]byte, dataSize)
	for i := range data {
		data[i] = byte(i % 256)
	}

	compressed, err := PXR24Compress(data, channels, width, height)
	if err != nil {
		t.Fatalf("PXR24Compress failed: %v", err)
	}

	decompressed, err := PXR24Decompress(compressed, channels, width, height, len(data))
	if err != nil {
		t.Fatalf("PXR24Decompress failed: %v", err)
	}

	for i := 0; i < len(data); i++ {
		if data[i] != decompressed[i] {
			t.Errorf("Mismatch at byte %d", i)
		}
	}
}

// TestPXR24LargeImage tests with larger image sizes
func TestPXR24LargeImage(t *testing.T) {
	channels := []ChannelInfo{
		{Type: pxr24PixelTypeFloat, Width: 64, Height: 64},
		{Type: pxr24PixelTypeFloat, Width: 64, Height: 64},
		{Type: pxr24PixelTypeFloat, Width: 64, Height: 64},
	}

	width := 64
	height := 64

	// Calculate data size
	dataSize := 0
	for _, ch := range channels {
		dataSize += ch.Width * ch.Height * 4
	}

	// Create gradient data
	data := make([]byte, dataSize)
	offset := 0
	for y := 0; y < height; y++ {
		for _, ch := range channels {
			for x := 0; x < ch.Width; x++ {
				f := float32(x+y) / float32(width+height)
				bits := math.Float32bits(f)
				data[offset] = byte(bits)
				data[offset+1] = byte(bits >> 8)
				data[offset+2] = byte(bits >> 16)
				data[offset+3] = byte(bits >> 24)
				offset += 4
			}
		}
	}

	compressed, err := PXR24Compress(data, channels, width, height)
	if err != nil {
		t.Fatalf("PXR24Compress failed: %v", err)
	}

	t.Logf("Large image: %d -> %d bytes (%.1f%%)", len(data), len(compressed),
		100.0*float64(len(compressed))/float64(len(data)))

	decompressed, err := PXR24Decompress(compressed, channels, width, height, len(data))
	if err != nil {
		t.Fatalf("PXR24Decompress failed: %v", err)
	}

	// Spot check some values
	for i := 0; i < len(data)/4; i += 100 {
		origBits := uint32(data[i*4]) | uint32(data[i*4+1])<<8 |
			uint32(data[i*4+2])<<16 | uint32(data[i*4+3])<<24
		origF := math.Float32frombits(origBits)

		decBits := uint32(decompressed[i*4]) | uint32(decompressed[i*4+1])<<8 |
			uint32(decompressed[i*4+2])<<16 | uint32(decompressed[i*4+3])<<24
		decF := math.Float32frombits(decBits)

		diff := math.Abs(float64(decF - origF))
		tolerance := math.Abs(float64(origF)) * 0.01
		if tolerance < 1e-10 {
			tolerance = 1e-10
		}
		if diff > tolerance {
			t.Errorf("Large image mismatch at pixel %d: %v vs %v", i, origF, decF)
		}
	}
}

// TestFloat24SpecialValues tests handling of special float values
func TestFloat24SpecialValues(t *testing.T) {
	tests := []struct {
		name      string
		val       float32
		checkZero bool // If true, check exact zero; otherwise check approximate equality
	}{
		{"zero", 0.0, true},
		{"neg_zero", float32(math.Copysign(0, -1)), true},
		{"one", 1.0, false},
		{"neg_one", -1.0, false},
		{"max_normal", math.MaxFloat32, false},
		{"small_normal", 1e-10, false}, // Normal small value (not subnormal)
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f24 := floatToFloat24(tt.val)

			// Should fit in 24 bits
			if f24 > 0xFFFFFF {
				t.Errorf("floatToFloat24(%v) = %x, exceeds 24 bits", tt.val, f24)
			}

			// Convert back
			f32 := float24ToFloat32(f24)

			// For finite values, check approximate equality
			if !math.IsNaN(float64(tt.val)) && !math.IsInf(float64(tt.val), 0) {
				if tt.checkZero {
					if f32 != 0 {
						t.Errorf("Zero not preserved: got %v", f32)
					}
				} else {
					relErr := math.Abs(float64(f32-tt.val)) / math.Abs(float64(tt.val))
					if relErr > 0.01 { // 1% tolerance
						t.Errorf("floatToFloat24(%v) round-trip: got %v, relErr=%v", tt.val, f32, relErr)
					}
				}
			}
		})
	}
}

// TestPXR24NonAlignedWidth tests with widths that don't align to 4-byte boundaries
func TestPXR24NonAlignedWidth(t *testing.T) {
	widths := []int{1, 3, 5, 7, 9, 15, 17}

	for _, width := range widths {
		t.Run("", func(t *testing.T) {
			height := 4
			channels := []ChannelInfo{
				{Type: pxr24PixelTypeUint, Width: width, Height: height},
			}

			dataSize := width * height * 4
			data := make([]byte, dataSize)
			for i := range data {
				data[i] = byte(i % 256)
			}

			compressed, err := PXR24Compress(data, channels, width, height)
			if err != nil {
				t.Fatalf("PXR24Compress failed for width %d: %v", width, err)
			}

			decompressed, err := PXR24Decompress(compressed, channels, width, height, len(data))
			if err != nil {
				t.Fatalf("PXR24Decompress failed for width %d: %v", width, err)
			}

			for i := 0; i < len(data); i++ {
				if data[i] != decompressed[i] {
					t.Errorf("Width %d: mismatch at byte %d", width, i)
					break
				}
			}
		})
	}
}
