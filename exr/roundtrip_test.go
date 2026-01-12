package exr

import (
	"crypto/sha512"
	"encoding/hex"
	"testing"
)

// TestAttributeOrderDeterminism verifies that header attributes are written
// in a deterministic order for byte-exact round-trip.
func TestAttributeOrderDeterminism(t *testing.T) {
	// Create a header with multiple attributes
	h := NewHeader()
	h.SetDataWindow(Box2i{Min: V2i{0, 0}, Max: V2i{99, 99}})
	h.SetDisplayWindow(Box2i{Min: V2i{0, 0}, Max: V2i{99, 99}})
	h.SetCompression(CompressionZIP)
	h.SetLineOrder(LineOrderIncreasing)
	h.SetPixelAspectRatio(1.0)
	h.SetScreenWindowCenter(V2f{0, 0})
	h.SetScreenWindowWidth(1.0)

	cl := NewChannelList()
	cl.Add(NewChannel("R", PixelTypeHalf))
	cl.Add(NewChannel("G", PixelTypeHalf))
	cl.Add(NewChannel("B", PixelTypeHalf))
	h.SetChannels(cl)

	// Add some custom attributes in non-alphabetical order
	h.Set(&Attribute{Name: "zOwner", Type: AttrTypeString, Value: "Test"})
	h.Set(&Attribute{Name: "customFloat", Type: AttrTypeFloat, Value: float32(1.5)})
	h.Set(&Attribute{Name: "anotherAttr", Type: AttrTypeInt, Value: int32(42)})

	// Serialize multiple times and check for identical output
	var hashes []string
	for i := 0; i < 10; i++ {
		data := h.SerializeForTest()
		hash := sha512.Sum512(data)
		hashes = append(hashes, hex.EncodeToString(hash[:]))
	}

	// All hashes should be identical
	for i := 1; i < len(hashes); i++ {
		if hashes[i] != hashes[0] {
			t.Errorf("Non-deterministic serialization detected:\n  hash[0]=%s\n  hash[%d]=%s",
				hashes[0][:32], i, hashes[i][:32])
		}
	}
}

// TestAttributeOrderIsAlphabetical verifies attributes are written in alphabetical order.
func TestAttributeOrderIsAlphabetical(t *testing.T) {
	h := NewHeader()
	// Add attributes in reverse alphabetical order
	h.Set(&Attribute{Name: "zAttr", Type: AttrTypeInt, Value: int32(3)})
	h.Set(&Attribute{Name: "mAttr", Type: AttrTypeInt, Value: int32(2)})
	h.Set(&Attribute{Name: "aAttr", Type: AttrTypeInt, Value: int32(1)})

	names := h.sortedAttributeNames()

	if len(names) != 3 {
		t.Fatalf("Expected 3 names, got %d", len(names))
	}
	if names[0] != "aAttr" || names[1] != "mAttr" || names[2] != "zAttr" {
		t.Errorf("Names not in alphabetical order: %v", names)
	}
}

// TestHeaderRoundTrip tests that a header can be serialized and deserialized
// with identical content.
func TestHeaderRoundTrip(t *testing.T) {
	original := NewHeader()
	original.SetDataWindow(Box2i{Min: V2i{0, 0}, Max: V2i{1919, 1079}})
	original.SetDisplayWindow(Box2i{Min: V2i{0, 0}, Max: V2i{1919, 1079}})
	original.SetCompression(CompressionPIZ)
	original.SetLineOrder(LineOrderIncreasing)
	original.SetPixelAspectRatio(1.0)
	original.SetScreenWindowCenter(V2f{0.5, 0.5})
	original.SetScreenWindowWidth(1.5)

	cl := NewChannelList()
	cl.Add(NewChannel("R", PixelTypeHalf))
	cl.Add(NewChannel("G", PixelTypeHalf))
	cl.Add(NewChannel("B", PixelTypeHalf))
	cl.Add(NewChannel("A", PixelTypeHalf))
	original.SetChannels(cl)

	// Add various attribute types
	original.Set(&Attribute{Name: "owner", Type: AttrTypeString, Value: "Test Owner"})
	original.Set(&Attribute{Name: "customV2d", Type: AttrTypeV2d, Value: V2d{1.234567890123, 9.876543210987}})
	original.Set(&Attribute{Name: "customV3d", Type: AttrTypeV3d, Value: V3d{1.1, 2.2, 3.3}})
	original.Set(&Attribute{Name: "customM33d", Type: AttrTypeM33d, Value: M33d{1, 0, 0, 0, 1, 0, 0, 0, 1}})
	original.Set(&Attribute{Name: "customM44d", Type: AttrTypeM44d, Value: M44d{1, 0, 0, 0, 0, 1, 0, 0, 0, 0, 1, 0, 0, 0, 0, 1}})
	original.Set(&Attribute{Name: "customFV", Type: AttrTypeFloatVector, Value: FloatVector{0.1, 0.2, 0.3}})
	original.Set(&Attribute{Name: "timecode", Type: AttrTypeTimecode, Value: MustNewTimeCode(1, 30, 45, 12, true)})

	// Serialize
	data := original.SerializeForTest()

	// Deserialize
	restored, err := ReadHeaderFromBytes(data)
	if err != nil {
		t.Fatalf("ReadHeaderFromBytes error: %v", err)
	}

	// Verify key attributes
	if restored.DataWindow() != original.DataWindow() {
		t.Errorf("DataWindow mismatch: %v != %v", restored.DataWindow(), original.DataWindow())
	}
	if restored.Compression() != original.Compression() {
		t.Errorf("Compression mismatch: %v != %v", restored.Compression(), original.Compression())
	}
	if restored.Channels().Len() != original.Channels().Len() {
		t.Errorf("Channels count mismatch: %d != %d", restored.Channels().Len(), original.Channels().Len())
	}

	// Verify custom attributes
	v2dAttr := restored.Get("customV2d")
	if v2dAttr == nil {
		t.Error("customV2d attribute not found")
	} else {
		expected := V2d{1.234567890123, 9.876543210987}
		if v2dAttr.Value.(V2d) != expected {
			t.Errorf("customV2d value mismatch: %v", v2dAttr.Value)
		}
	}

	fvAttr := restored.Get("customFV")
	if fvAttr == nil {
		t.Error("customFV attribute not found")
	} else {
		fv := fvAttr.Value.(FloatVector)
		if len(fv) != 3 || fv[0] != 0.1 || fv[1] != 0.2 || fv[2] != 0.3 {
			t.Errorf("customFV value mismatch: %v", fv)
		}
	}
}

// TestRoundTripCauses documents the specific causes of non-identical round-trips.
func TestRoundTripCauses(t *testing.T) {
	t.Log(`
Round-trip passthrough may not produce identical SHA-512 file hashes due to:

1. ATTRIBUTE ORDER - FIXED:
   - Header attributes are now written in sorted order
   - This ensures deterministic header serialization

2. LOSSY COMPRESSION - EXPECTED DIFFERENCE:
   - PXR24: Converts 32-bit floats to 24-bit (lossy by design)
   - B44/B44A: 4x4 block compression with quantization
   - DWAA/DWAB: DCT-based lossy compression
   - These CANNOT produce identical output by design

3. COMPRESSION ALGORITHM OUTPUT - POSSIBLE DIFFERENCE:
   - Different zlib implementations may produce different compressed bytes
   - Same decompressed content, different compressed representation
   - This is acceptable - pixel data is preserved

4. OFFSET TABLE RECALCULATION - EXPECTED:
   - Chunk offsets depend on exact compressed byte positions
   - Different compression = different offsets

5. CHANNEL ORDER - PRESERVED:
   - ChannelList uses []Channel slice
   - Order is preserved during read/write

What IS guaranteed for lossless compression:
- Pixel data values are exactly preserved
- All attributes are preserved with exact values
- Channel definitions are preserved
`)
}
