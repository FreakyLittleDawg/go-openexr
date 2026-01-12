package compression

import (
	"math"
	"testing"
)

func TestBatchWaveletEncodeDecodeRow(t *testing.T) {
	// Test with various sizes
	sizes := []int{8, 16, 32, 64, 100, 128, 255, 256}

	for _, n := range sizes {
		data := make([]uint16, n)
		for i := range data {
			data[i] = uint16(i * 100)
		}
		original := make([]uint16, n)
		copy(original, data)

		temp := make([]uint16, n)

		// Encode
		WaveletEncodeRow(data, temp, n)

		// Decode
		WaveletDecodeRow(data, temp, n)

		// Verify round-trip
		for i := range data {
			if data[i] != original[i] {
				t.Errorf("n=%d: round-trip failed at index %d: got %d, want %d",
					n, i, data[i], original[i])
			}
		}
	}
}

func TestWaveletEncodeDecode2D(t *testing.T) {
	// Test 2D wavelet transform
	widths := []int{8, 16, 32, 64}
	heights := []int{8, 16, 32, 64}

	for _, width := range widths {
		for _, height := range heights {
			data := make([]uint16, width*height)
			for i := range data {
				data[i] = uint16(i % 65536)
			}
			original := make([]uint16, width*height)
			copy(original, data)

			// Encode
			WaveletEncode2D(data, width, height)

			// Decode
			WaveletDecode2D(data, width, height)

			// Verify round-trip
			for i := range data {
				if data[i] != original[i] {
					t.Errorf("%dx%d: round-trip failed at index %d: got %d, want %d",
						width, height, i, data[i], original[i])
				}
			}
		}
	}
}

func TestWavelet2DMatchesOriginal(t *testing.T) {
	// Compare optimized 2D wavelet with original implementation
	width, height := 64, 64
	data1 := make([]uint16, width*height)
	data2 := make([]uint16, width*height)

	for i := range data1 {
		data1[i] = uint16(i * 17 % 65536)
		data2[i] = data1[i]
	}

	// Use original implementation
	WaveletEncode(data1, width, height)

	// Use optimized implementation
	WaveletEncode2D(data2, width, height)

	// Both should produce the same result
	for i := range data1 {
		if data1[i] != data2[i] {
			t.Errorf("index %d: original=%d, optimized=%d", i, data1[i], data2[i])
		}
	}
}

func TestDCT8x8RoundTrip(t *testing.T) {
	var data [64]float32
	var original [64]float32

	// Initialize with test data
	for i := 0; i < 64; i++ {
		data[i] = float32(i) / 64.0
		original[i] = data[i]
	}

	// Forward DCT
	DCT8x8Forward(&data)

	// Inverse DCT
	DCT8x8Inverse(&data)

	// Verify round-trip (with some tolerance for floating point)
	const epsilon = 1e-5
	for i := 0; i < 64; i++ {
		if math.Abs(float64(data[i]-original[i])) > epsilon {
			t.Errorf("DCT round-trip failed at index %d: got %f, want %f",
				i, data[i], original[i])
		}
	}
}

func TestDCT8x8MatchesOriginal(t *testing.T) {
	var data1 [64]float32
	var data2 [64]float32

	// Initialize with test data
	for i := 0; i < 64; i++ {
		data1[i] = float32(i) / 64.0
		data2[i] = data1[i]
	}

	// Use original implementation
	dctForward8x8(&data1)

	// Use optimized implementation
	DCT8x8Forward(&data2)

	// Both should produce similar results (within floating point tolerance)
	const epsilon = 1e-5
	for i := 0; i < 64; i++ {
		if math.Abs(float64(data1[i]-data2[i])) > epsilon {
			t.Errorf("index %d: original=%f, optimized=%f", i, data1[i], data2[i])
		}
	}
}

func TestTranspose8x8(t *testing.T) {
	var src, dst [64]float32

	// Initialize with known values
	for i := 0; i < 64; i++ {
		src[i] = float32(i)
	}

	transpose8x8(&src, &dst)

	// Verify transpose
	for row := 0; row < 8; row++ {
		for col := 0; col < 8; col++ {
			srcIdx := row*8 + col
			dstIdx := col*8 + row
			if dst[dstIdx] != src[srcIdx] {
				t.Errorf("transpose failed at (%d,%d): got %f, want %f",
					row, col, dst[dstIdx], src[srcIdx])
			}
		}
	}
}

func TestCSC709RoundTrip(t *testing.T) {
	// Test RGB -> YCbCr -> RGB conversion
	// Note: Color space conversion has inherent floating point precision loss
	n := 64
	r := make([]float32, n)
	g := make([]float32, n)
	b := make([]float32, n)
	origR := make([]float32, n)
	origG := make([]float32, n)
	origB := make([]float32, n)

	// Initialize with test data
	for i := 0; i < n; i++ {
		r[i] = float32(i) / float32(n)
		g[i] = float32(n-i) / float32(n)
		b[i] = 0.5
		origR[i] = r[i]
		origG[i] = g[i]
		origB[i] = b[i]
	}

	// Forward (RGB -> YCbCr)
	CSC709ForwardBatch(r, g, b)

	// Inverse (YCbCr -> RGB)
	CSC709InverseBatch(r, g, b)

	// Verify round-trip with tolerance for floating point precision
	// Color space conversions have inherent precision loss in matrix operations
	const epsilon = 1e-3
	for i := 0; i < n; i++ {
		if math.Abs(float64(r[i]-origR[i])) > epsilon {
			t.Errorf("R round-trip failed at index %d: got %f, want %f", i, r[i], origR[i])
		}
		if math.Abs(float64(g[i]-origG[i])) > epsilon {
			t.Errorf("G round-trip failed at index %d: got %f, want %f", i, g[i], origG[i])
		}
		if math.Abs(float64(b[i]-origB[i])) > epsilon {
			t.Errorf("B round-trip failed at index %d: got %f, want %f", i, b[i], origB[i])
		}
	}
}

func TestZigzagRoundTrip(t *testing.T) {
	var src, zigzagged, result [64]float32

	// Initialize with test data
	for i := 0; i < 64; i++ {
		src[i] = float32(i)
	}

	// Forward zigzag
	ZigzagReorderBatch(&zigzagged, &src)

	// Inverse zigzag
	ZigzagUnreorderBatch(&result, &zigzagged)

	// Verify round-trip
	for i := 0; i < 64; i++ {
		if result[i] != src[i] {
			t.Errorf("Zigzag round-trip failed at index %d: got %f, want %f",
				i, result[i], src[i])
		}
	}
}

func TestCopyBatch16(t *testing.T) {
	sizes := []int{1, 7, 8, 15, 16, 100, 128}

	for _, n := range sizes {
		src := make([]uint16, n)
		dst := make([]uint16, n)

		for i := range src {
			src[i] = uint16(i)
		}

		copyBatch16(dst, src)

		for i := range src {
			if dst[i] != src[i] {
				t.Errorf("n=%d: copy failed at index %d: got %d, want %d",
					n, i, dst[i], src[i])
			}
		}
	}
}

func TestExtractStoreColumn(t *testing.T) {
	width, height := 16, 16
	data := make([]uint16, width*height)

	// Initialize with test data
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			data[y*width+x] = uint16(y*100 + x)
		}
	}

	col := make([]uint16, height)

	// Test each column
	for x := 0; x < width; x++ {
		extractColumn(data, col, x, width, height)

		// Verify extracted values
		for y := 0; y < height; y++ {
			expected := uint16(y*100 + x)
			if col[y] != expected {
				t.Errorf("column %d, row %d: got %d, want %d", x, y, col[y], expected)
			}
		}

		// Modify column
		for y := 0; y < height; y++ {
			col[y] = col[y] + 1000
		}

		// Store back
		storeColumn(data, col, x, width, height)

		// Verify stored values
		for y := 0; y < height; y++ {
			expected := uint16(y*100 + x + 1000)
			if data[y*width+x] != expected {
				t.Errorf("stored column %d, row %d: got %d, want %d",
					x, y, data[y*width+x], expected)
			}
		}
	}
}

func TestInterleaveDeinterleave(t *testing.T) {
	numChannels := 4
	pixelsPerChannel := 64
	bytesPerSample := 2

	// Create channel data
	channels := make([][]byte, numChannels)
	for ch := 0; ch < numChannels; ch++ {
		channels[ch] = make([]byte, pixelsPerChannel*bytesPerSample)
		for p := 0; p < pixelsPerChannel; p++ {
			channels[ch][p*2] = byte(ch*64 + p)
			channels[ch][p*2+1] = byte((ch*64 + p) >> 8)
		}
	}

	// Interleave
	interleaved := make([]byte, numChannels*pixelsPerChannel*bytesPerSample)
	InterleaveChannelsBatch(interleaved, channels, bytesPerSample)

	// Deinterleave
	resultChannels := make([][]byte, numChannels)
	for ch := 0; ch < numChannels; ch++ {
		resultChannels[ch] = make([]byte, pixelsPerChannel*bytesPerSample)
	}
	DeinterleaveChannelsBatch(interleaved, resultChannels, bytesPerSample)

	// Verify round-trip
	for ch := 0; ch < numChannels; ch++ {
		for i := 0; i < len(channels[ch]); i++ {
			if resultChannels[ch][i] != channels[ch][i] {
				t.Errorf("channel %d, byte %d: got %d, want %d",
					ch, i, resultChannels[ch][i], channels[ch][i])
			}
		}
	}
}

// Benchmarks

func BenchmarkWaveletEncodeRow(b *testing.B) {
	data := make([]uint16, 1024)
	temp := make([]uint16, 1024)
	for i := range data {
		data[i] = uint16(i)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		WaveletEncodeRow(data, temp, len(data))
	}
}

func BenchmarkWaveletEncode2D(b *testing.B) {
	data := make([]uint16, 64*64)
	for i := range data {
		data[i] = uint16(i)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		WaveletEncode2D(data, 64, 64)
	}
}

func BenchmarkWaveletEncodeOriginal(b *testing.B) {
	data := make([]uint16, 64*64)
	for i := range data {
		data[i] = uint16(i)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		WaveletEncode(data, 64, 64)
	}
}

func BenchmarkDCT8x8Forward(b *testing.B) {
	var data [64]float32
	for i := range data {
		data[i] = float32(i)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		DCT8x8Forward(&data)
	}
}

func BenchmarkDCT8x8ForwardOriginal(b *testing.B) {
	var data [64]float32
	for i := range data {
		data[i] = float32(i)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		dctForward8x8(&data)
	}
}

func BenchmarkCSC709Forward(b *testing.B) {
	n := 1024
	r := make([]float32, n)
	g := make([]float32, n)
	bb := make([]float32, n)
	for i := 0; i < n; i++ {
		r[i] = float32(i) / float32(n)
		g[i] = float32(n-i) / float32(n)
		bb[i] = 0.5
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		CSC709ForwardBatch(r, g, bb)
	}
}

func BenchmarkTranspose8x8(b *testing.B) {
	var src, dst [64]float32
	for i := range src {
		src[i] = float32(i)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		transpose8x8(&src, &dst)
	}
}

func BenchmarkZigzagReorder(b *testing.B) {
	var src, dst [64]float32
	for i := range src {
		src[i] = float32(i)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ZigzagReorderBatch(&dst, &src)
	}
}

func BenchmarkCopyBatch16(b *testing.B) {
	src := make([]uint16, 1024)
	dst := make([]uint16, 1024)
	for i := range src {
		src[i] = uint16(i)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		copyBatch16(dst, src)
	}
}
