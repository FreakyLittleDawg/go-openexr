package compression

import (
	"testing"
)

func TestHTJ2KChunkHeader(t *testing.T) {
	// Test channel map with identity mapping
	channelMap := []uint16{0, 1, 2}

	// Test header read/write roundtrip
	var buf []byte
	buf = make([]byte, 0, 100)

	// Write header
	header := make([]byte, htj2kHeaderSize+2+len(channelMap)*2)
	header[0] = 'H'
	header[1] = 'T'
	header[2] = 0
	header[3] = 0
	header[4] = 0
	header[5] = byte(2 + len(channelMap)*2) // payload length
	header[6] = 0
	header[7] = byte(len(channelMap)) // channel count
	for i, ch := range channelMap {
		header[8+i*2] = 0
		header[8+i*2+1] = byte(ch)
	}
	buf = append(buf, header...)

	// Read header back
	headerSize, readMap, err := readHTJ2KHeader(buf)
	if err != nil {
		t.Fatalf("readHTJ2KHeader failed: %v", err)
	}

	expectedHeaderSize := htj2kHeaderSize + 2 + len(channelMap)*2
	if headerSize != expectedHeaderSize {
		t.Errorf("header size: got %d, want %d", headerSize, expectedHeaderSize)
	}

	if len(readMap) != len(channelMap) {
		t.Fatalf("channel map length: got %d, want %d", len(readMap), len(channelMap))
	}

	for i, ch := range channelMap {
		if readMap[i] != ch {
			t.Errorf("channel map[%d]: got %d, want %d", i, readMap[i], ch)
		}
	}
}

func TestHTJ2KMakeChannelMapRGB(t *testing.T) {
	channels := []HTJ2KChannelInfo{
		{Type: HTJ2KPixelTypeHalf, Width: 100, Height: 100, XSampling: 1, YSampling: 1, Name: "B"},
		{Type: HTJ2KPixelTypeHalf, Width: 100, Height: 100, XSampling: 1, YSampling: 1, Name: "G"},
		{Type: HTJ2KPixelTypeHalf, Width: 100, Height: 100, XSampling: 1, YSampling: 1, Name: "R"},
	}

	channelMap, isRGB := makeChannelMap(channels)

	if !isRGB {
		t.Error("expected isRGB to be true")
	}

	// For RGB, the map should reorder to R=0, G=1, B=2
	// So map[0] should point to the R channel (index 2 in original)
	// map[1] should point to G (index 1)
	// map[2] should point to B (index 0)
	if channelMap[0] != 2 {
		t.Errorf("channel map[0] (R): got %d, want 2", channelMap[0])
	}
	if channelMap[1] != 1 {
		t.Errorf("channel map[1] (G): got %d, want 1", channelMap[1])
	}
	if channelMap[2] != 0 {
		t.Errorf("channel map[2] (B): got %d, want 0", channelMap[2])
	}
}

func TestHTJ2KMakeChannelMapPrefixedRGB(t *testing.T) {
	channels := []HTJ2KChannelInfo{
		{Type: HTJ2KPixelTypeHalf, Width: 100, Height: 100, XSampling: 1, YSampling: 1, Name: "main.B"},
		{Type: HTJ2KPixelTypeHalf, Width: 100, Height: 100, XSampling: 1, YSampling: 1, Name: "main.G"},
		{Type: HTJ2KPixelTypeHalf, Width: 100, Height: 100, XSampling: 1, YSampling: 1, Name: "main.R"},
	}

	_, isRGB := makeChannelMap(channels)

	if !isRGB {
		t.Error("expected isRGB to be true for prefixed channels")
	}
}

func TestHTJ2KMakeChannelMapNonRGB(t *testing.T) {
	channels := []HTJ2KChannelInfo{
		{Type: HTJ2KPixelTypeHalf, Width: 100, Height: 100, XSampling: 1, YSampling: 1, Name: "A"},
		{Type: HTJ2KPixelTypeHalf, Width: 100, Height: 100, XSampling: 1, YSampling: 1, Name: "Y"},
		{Type: HTJ2KPixelTypeHalf, Width: 100, Height: 100, XSampling: 1, YSampling: 1, Name: "Z"},
	}

	channelMap, isRGB := makeChannelMap(channels)

	if isRGB {
		t.Error("expected isRGB to be false")
	}

	// Non-RGB: identity mapping
	for i := range channelMap {
		if channelMap[i] != uint16(i) {
			t.Errorf("channel map[%d]: got %d, want %d", i, channelMap[i], i)
		}
	}
}

func TestHTJ2KMakeChannelMapMixedTypes(t *testing.T) {
	// RGB with mismatched types should NOT be detected as RGB
	channels := []HTJ2KChannelInfo{
		{Type: HTJ2KPixelTypeHalf, Width: 100, Height: 100, XSampling: 1, YSampling: 1, Name: "B"},
		{Type: HTJ2KPixelTypeUint, Width: 100, Height: 100, XSampling: 1, YSampling: 1, Name: "G"}, // Different type
		{Type: HTJ2KPixelTypeHalf, Width: 100, Height: 100, XSampling: 1, YSampling: 1, Name: "R"},
	}

	_, isRGB := makeChannelMap(channels)

	if isRGB {
		t.Error("expected isRGB to be false for mixed types")
	}
}

func TestHTJ2KInvalidMagic(t *testing.T) {
	data := []byte("XX\x00\x00\x00\x02\x00\x01\x00\x00")

	_, _, err := readHTJ2KHeader(data)
	if err != ErrHTJ2KInvalidMagic {
		t.Errorf("expected ErrHTJ2KInvalidMagic, got %v", err)
	}
}

func TestHTJ2KTruncatedHeader(t *testing.T) {
	data := []byte("HT\x00")

	_, _, err := readHTJ2KHeader(data)
	if err != ErrHTJ2KCorrupted {
		t.Errorf("expected ErrHTJ2KCorrupted, got %v", err)
	}
}
