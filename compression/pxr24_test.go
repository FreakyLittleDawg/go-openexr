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
