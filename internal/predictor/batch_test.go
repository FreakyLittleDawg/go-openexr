package predictor

import (
	"bytes"
	"testing"
)

func TestEncodeBatch(t *testing.T) {
	// Test that EncodeBatch produces same results as Encode
	original := []byte{10, 15, 20, 25, 30, 35, 40, 45, 50, 55, 60, 65}

	data1 := make([]byte, len(original))
	copy(data1, original)
	Encode(data1)

	data2 := make([]byte, len(original))
	copy(data2, original)
	EncodeBatch(data2)

	if !bytes.Equal(data1, data2) {
		t.Errorf("EncodeBatch result differs from Encode:\ngot:  %v\nwant: %v", data2, data1)
	}
}

func TestDecodeBatch(t *testing.T) {
	// Test that DecodeBatch produces same results as Decode
	original := []byte{10, 15, 20, 25, 30, 35, 40, 45, 50, 55, 60, 65}

	// Encode first
	encoded := make([]byte, len(original))
	copy(encoded, original)
	Encode(encoded)

	// Test regular decode
	data1 := make([]byte, len(encoded))
	copy(data1, encoded)
	Decode(data1)

	// Test batch decode
	data2 := make([]byte, len(encoded))
	copy(data2, encoded)
	DecodeBatch(data2)

	if !bytes.Equal(data1, data2) {
		t.Errorf("DecodeBatch result differs from Decode:\ngot:  %v\nwant: %v", data2, data1)
	}

	// Verify it matches original
	if !bytes.Equal(data1, original) {
		t.Errorf("Decode did not restore original:\ngot:  %v\nwant: %v", data1, original)
	}
}

func TestEncodeBatchRoundTrip(t *testing.T) {
	// Test larger data with round-trip
	original := make([]byte, 1000)
	for i := range original {
		original[i] = byte(i % 256)
	}

	data := make([]byte, len(original))
	copy(data, original)

	EncodeBatch(data)
	DecodeBatch(data)

	if !bytes.Equal(data, original) {
		t.Errorf("Round-trip failed: data differs from original")
	}
}

func TestEncodeMultiRow(t *testing.T) {
	// Test encoding multiple independent rows
	rowLen := 10
	numRows := 3
	data := make([]byte, rowLen*numRows)
	for i := range data {
		data[i] = byte(i)
	}

	expected := make([]byte, len(data))
	copy(expected, data)

	// Encode each row independently
	for row := 0; row < numRows; row++ {
		start := row * rowLen
		Encode(expected[start : start+rowLen])
	}

	// Use EncodeMultiRow
	EncodeMultiRow(data, rowLen, numRows)

	if !bytes.Equal(data, expected) {
		t.Errorf("EncodeMultiRow result differs:\ngot:  %v\nwant: %v", data, expected)
	}
}

func TestDecodeMultiRow(t *testing.T) {
	// Test decoding multiple independent rows
	rowLen := 10
	numRows := 3
	original := make([]byte, rowLen*numRows)
	for i := range original {
		original[i] = byte(i)
	}

	data := make([]byte, len(original))
	copy(data, original)

	// Encode then decode
	EncodeMultiRow(data, rowLen, numRows)
	DecodeMultiRow(data, rowLen, numRows)

	if !bytes.Equal(data, original) {
		t.Errorf("Round-trip failed:\ngot:  %v\nwant: %v", data, original)
	}
}

func TestEncodeParallel(t *testing.T) {
	data := make([]byte, 100)
	for i := range data {
		data[i] = byte(i)
	}

	blockCount := 0
	EncodeParallel(data, 20, func(block []byte) {
		blockCount++
		Encode(block)
	})

	if blockCount != 5 {
		t.Errorf("EncodeParallel called process %d times, want 5", blockCount)
	}
}

func BenchmarkEncodeBaseline(b *testing.B) {
	data := make([]byte, 64*1024) // 64KB chunk
	for i := range data {
		data[i] = byte(i)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		original := make([]byte, len(data))
		copy(original, data)
		Encode(original)
	}
}

func BenchmarkEncodeBatchOptimized(b *testing.B) {
	data := make([]byte, 64*1024) // 64KB chunk
	for i := range data {
		data[i] = byte(i)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		original := make([]byte, len(data))
		copy(original, data)
		EncodeBatch(original)
	}
}

func BenchmarkDecodeBaseline(b *testing.B) {
	data := make([]byte, 64*1024) // 64KB chunk
	for i := range data {
		data[i] = byte(i)
	}
	Encode(data)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		work := make([]byte, len(data))
		copy(work, data)
		Decode(work)
	}
}

func BenchmarkDecodeBatchOptimized(b *testing.B) {
	data := make([]byte, 64*1024) // 64KB chunk
	for i := range data {
		data[i] = byte(i)
	}
	Encode(data)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		work := make([]byte, len(data))
		copy(work, data)
		DecodeBatch(work)
	}
}

func BenchmarkEncodeMultiRow(b *testing.B) {
	rowLen := 1920 * 4 // Full HD row, 4 channels
	numRows := 16      // 16 scanlines per chunk
	data := make([]byte, rowLen*numRows)
	for i := range data {
		data[i] = byte(i)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		work := make([]byte, len(data))
		copy(work, data)
		EncodeMultiRow(work, rowLen, numRows)
	}
}
