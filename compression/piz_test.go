package compression

import (
	"testing"
)

func TestWaveletEncodeDecodeEmpty(t *testing.T) {
	var data []uint16
	WaveletEncode(data, 0, 0)
	WaveletDecode(data, 0, 0)
	// Should not crash
}

func TestWaveletEncodeDecodeSingle(t *testing.T) {
	data := []uint16{42}
	original := make([]uint16, len(data))
	copy(original, data)

	WaveletEncode(data, 1, 1)
	WaveletDecode(data, 1, 1)

	if data[0] != original[0] {
		t.Errorf("Single value: got %d, want %d", data[0], original[0])
	}
}

func TestWaveletEncodeDecodeRow(t *testing.T) {
	data := []uint16{1, 2, 3, 4, 5, 6, 7, 8}
	original := make([]uint16, len(data))
	copy(original, data)

	WaveletEncode(data, 8, 1)
	WaveletDecode(data, 8, 1)

	for i := range original {
		if data[i] != original[i] {
			t.Errorf("Index %d: got %d, want %d", i, data[i], original[i])
		}
	}
}

func TestWaveletEncodeDecodeSquare(t *testing.T) {
	// 4x4 test
	data := []uint16{
		1, 2, 3, 4,
		5, 6, 7, 8,
		9, 10, 11, 12,
		13, 14, 15, 16,
	}
	original := make([]uint16, len(data))
	copy(original, data)

	WaveletEncode(data, 4, 4)
	WaveletDecode(data, 4, 4)

	for i := range original {
		if data[i] != original[i] {
			t.Errorf("Index %d: got %d, want %d", i, data[i], original[i])
		}
	}
}

func TestWaveletEncodeDecodeRectangle(t *testing.T) {
	// 8x4 test
	data := make([]uint16, 32)
	for i := range data {
		data[i] = uint16(i * 100)
	}
	original := make([]uint16, len(data))
	copy(original, data)

	WaveletEncode(data, 8, 4)
	WaveletDecode(data, 8, 4)

	for i := range original {
		if data[i] != original[i] {
			t.Errorf("Index %d: got %d, want %d", i, data[i], original[i])
		}
	}
}

func TestWaveletEncodeDecodeOddSize(t *testing.T) {
	// 5x3 test (odd dimensions)
	data := make([]uint16, 15)
	for i := range data {
		data[i] = uint16(i * 50)
	}
	original := make([]uint16, len(data))
	copy(original, data)

	WaveletEncode(data, 5, 3)
	WaveletDecode(data, 5, 3)

	for i := range original {
		if data[i] != original[i] {
			t.Errorf("Index %d: got %d, want %d", i, data[i], original[i])
		}
	}
}

func TestHuffmanEncodeDecodeEmpty(t *testing.T) {
	encoder := NewHuffmanEncoder(nil)
	result := encoder.Encode(nil)
	if result != nil {
		t.Error("Empty encode should return nil")
	}
}

func TestHuffmanEncodeDecodeSingleSymbol(t *testing.T) {
	freqs := make([]uint64, 256)
	freqs[42] = 100

	encoder := NewHuffmanEncoder(freqs)
	values := []uint16{42, 42, 42, 42, 42}
	encoded := encoder.Encode(values)

	// Create decoder from code lengths
	codes := encoder.GetCodes()
	codeLengths := make([]int, len(codes))
	for i, c := range codes {
		codeLengths[i] = c.length
	}

	decoder := NewHuffmanDecoder(codeLengths)
	decoded, err := decoder.Decode(encoded, len(values))
	if err != nil {
		t.Fatalf("Decode error: %v", err)
	}

	for i, v := range decoded {
		if v != values[i] {
			t.Errorf("Index %d: got %d, want %d", i, v, values[i])
		}
	}
}

func TestHuffmanEncodeDecodeMultipleSymbols(t *testing.T) {
	freqs := make([]uint64, 256)
	freqs[0] = 50
	freqs[1] = 30
	freqs[2] = 15
	freqs[3] = 5

	encoder := NewHuffmanEncoder(freqs)
	values := []uint16{0, 0, 1, 0, 2, 1, 0, 3, 0, 0}
	encoded := encoder.Encode(values)

	codes := encoder.GetCodes()
	codeLengths := make([]int, len(codes))
	for i, c := range codes {
		codeLengths[i] = c.length
	}

	decoder := NewHuffmanDecoder(codeLengths)
	decoded, err := decoder.Decode(encoded, len(values))
	if err != nil {
		t.Fatalf("Decode error: %v", err)
	}

	for i, v := range decoded {
		if v != values[i] {
			t.Errorf("Index %d: got %d, want %d", i, v, values[i])
		}
	}
}

func TestPIZCompressDecompressEmpty(t *testing.T) {
	compressed, err := PIZCompress(nil, 0, 0, 0)
	if err != nil || compressed != nil {
		t.Error("Empty compress should return nil, nil")
	}

	decompressed, err := PIZDecompress(nil, 0, 0, 0)
	if err != nil || decompressed != nil {
		t.Error("Empty decompress should return nil, nil")
	}
}

func TestPIZCompressDecompressSimple(t *testing.T) {
	// Simple 4x4 single channel image
	data := make([]uint16, 16)
	for i := range data {
		data[i] = uint16(i * 100)
	}

	// Test wavelet alone first
	waveletData := make([]uint16, len(data))
	copy(waveletData, data)
	WaveletEncode(waveletData, 4, 4)
	WaveletDecode(waveletData, 4, 4)
	for i := range data {
		if waveletData[i] != data[i] {
			t.Errorf("Wavelet round-trip failed at %d: got %d, want %d", i, waveletData[i], data[i])
		}
	}

	compressed, err := PIZCompress(data, 4, 4, 1)
	if err != nil {
		t.Fatalf("Compress error: %v", err)
	}

	t.Logf("PIZ compression: %d -> %d bytes (%.1f%%)", len(data)*2, len(compressed), 100.0*float64(len(compressed))/float64(len(data)*2))

	decompressed, err := PIZDecompress(compressed, 4, 4, 1)
	if err != nil {
		t.Fatalf("Decompress error: %v", err)
	}

	if len(decompressed) != len(data) {
		t.Fatalf("Length mismatch: got %d, want %d", len(decompressed), len(data))
	}

	for i := range data {
		if decompressed[i] != data[i] {
			t.Errorf("Index %d: got %d, want %d", i, decompressed[i], data[i])
		}
	}
}

func TestPIZCompressDecompressMultiChannel(t *testing.T) {

	// 8x8 image with 3 channels
	width, height, channels := 8, 8, 3
	data := make([]uint16, width*height*channels)
	for ch := 0; ch < channels; ch++ {
		for i := 0; i < width*height; i++ {
			data[ch*width*height+i] = uint16(ch*1000 + i*10)
		}
	}

	compressed, err := PIZCompress(data, width, height, channels)
	if err != nil {
		t.Fatalf("Compress error: %v", err)
	}

	t.Logf("PIZ multi-channel: %d -> %d bytes (%.1f%%)", len(data)*2, len(compressed), 100.0*float64(len(compressed))/float64(len(data)*2))

	decompressed, err := PIZDecompress(compressed, width, height, channels)
	if err != nil {
		t.Fatalf("Decompress error: %v", err)
	}

	for i := range data {
		if decompressed[i] != data[i] {
			t.Errorf("Index %d: got %d, want %d", i, decompressed[i], data[i])
		}
	}
}

func TestPIZCompressDecompressUniform(t *testing.T) {
	// Uniform data (should compress very well)
	data := make([]uint16, 256)
	for i := range data {
		data[i] = 12345
	}

	compressed, err := PIZCompress(data, 16, 16, 1)
	if err != nil {
		t.Fatalf("Compress error: %v", err)
	}

	t.Logf("PIZ uniform: %d -> %d bytes (%.1f%%)", len(data)*2, len(compressed), 100.0*float64(len(compressed))/float64(len(data)*2))

	decompressed, err := PIZDecompress(compressed, 16, 16, 1)
	if err != nil {
		t.Fatalf("Decompress error: %v", err)
	}

	for i := range data {
		if decompressed[i] != data[i] {
			t.Errorf("Index %d: got %d, want %d", i, decompressed[i], data[i])
		}
	}
}

func BenchmarkWaveletEncode(b *testing.B) {
	data := make([]uint16, 256*256)
	for i := range data {
		data[i] = uint16(i)
	}

	b.ResetTimer()
	b.SetBytes(int64(len(data) * 2))

	for i := 0; i < b.N; i++ {
		WaveletEncode(data, 256, 256)
	}
}

func BenchmarkWaveletDecode(b *testing.B) {
	data := make([]uint16, 256*256)
	for i := range data {
		data[i] = uint16(i)
	}
	WaveletEncode(data, 256, 256)

	b.ResetTimer()
	b.SetBytes(int64(len(data) * 2))

	for i := 0; i < b.N; i++ {
		WaveletDecode(data, 256, 256)
	}
}

func BenchmarkPIZCompress(b *testing.B) {
	data := make([]uint16, 256*256)
	for i := range data {
		data[i] = uint16(i % 1000)
	}

	b.ResetTimer()
	b.SetBytes(int64(len(data) * 2))

	for i := 0; i < b.N; i++ {
		PIZCompress(data, 256, 256, 1)
	}
}

func BenchmarkPIZDecompress(b *testing.B) {
	data := make([]uint16, 256*256)
	for i := range data {
		data[i] = uint16(i % 1000)
	}
	compressed, _ := PIZCompress(data, 256, 256, 1)

	b.ResetTimer()
	b.SetBytes(int64(len(data) * 2))

	for i := 0; i < b.N; i++ {
		PIZDecompress(compressed, 256, 256, 1)
	}
}

// BenchmarkHuffmanDecoders compares old HuffmanDecoder vs new FastHufDecoder
func BenchmarkHuffmanDecoders(b *testing.B) {
	// Create test data with realistic distribution
	numValues := 64 * 1024
	data := make([]uint16, numValues)
	for i := range data {
		// Mix of values to create a realistic Huffman tree
		data[i] = uint16(i % 500)
	}

	// Build frequency table
	freqs := make([]uint64, 500)
	for _, v := range data {
		freqs[v]++
	}

	// Create encoder and encode data
	encoder := NewHuffmanEncoder(freqs)
	encoded := encoder.Encode(data)
	codeLengths := encoder.GetLengths()

	b.Run("HuffmanDecoder", func(b *testing.B) {
		decoder := NewHuffmanDecoder(codeLengths)
		b.ResetTimer()
		b.SetBytes(int64(len(encoded)))

		for i := 0; i < b.N; i++ {
			decoder.Decode(encoded, numValues)
		}
	})

	b.Run("FastHufDecoder", func(b *testing.B) {
		decoder := NewFastHufDecoder(codeLengths)
		b.ResetTimer()
		b.SetBytes(int64(len(encoded)))

		for i := 0; i < b.N; i++ {
			decoder.Decode(encoded, numValues)
		}
	})
}
