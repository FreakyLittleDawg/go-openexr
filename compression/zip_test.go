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
