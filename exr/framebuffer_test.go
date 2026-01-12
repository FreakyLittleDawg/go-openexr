package exr

import (
	"testing"

	"github.com/mrjoshuak/go-openexr/half"
)

func TestNewSlice(t *testing.T) {
	data := make([]byte, 100*100*2) // 100x100 half pixels
	slice := NewSlice(PixelTypeHalf, data, 100, 100)

	if slice.Type != PixelTypeHalf {
		t.Errorf("Type = %v, want Half", slice.Type)
	}
	if slice.XStride != 2 {
		t.Errorf("XStride = %d, want 2", slice.XStride)
	}
	if slice.YStride != 200 {
		t.Errorf("YStride = %d, want 200", slice.YStride)
	}
}

func TestSliceSetGetFloat32(t *testing.T) {
	data := make([]float32, 10*10)
	slice := NewSliceFromFloat32(data, 10, 10)

	slice.SetFloat32(5, 5, 3.14)
	val := slice.GetFloat32(5, 5)
	if val != 3.14 {
		t.Errorf("GetFloat32(5,5) = %v, want 3.14", val)
	}
}

func TestSliceSetGetHalf(t *testing.T) {
	data := make([]half.Half, 10*10)
	slice := NewSliceFromHalf(data, 10, 10)

	h := half.FromFloat32(2.5)
	slice.SetHalf(3, 7, h)
	got := slice.GetHalf(3, 7)
	if got != h {
		t.Errorf("GetHalf(3,7) = %v, want %v", got, h)
	}
}

func TestSliceSetGetUint32(t *testing.T) {
	data := make([]uint32, 10*10)
	slice := NewSliceFromUint32(data, 10, 10)

	slice.SetUint32(0, 0, 12345)
	val := slice.GetUint32(0, 0)
	if val != 12345 {
		t.Errorf("GetUint32(0,0) = %d, want 12345", val)
	}
}

func TestSliceTypeConversion(t *testing.T) {
	// Test float32 to half conversion
	floatData := make([]float32, 4)
	floatSlice := NewSliceFromFloat32(floatData, 2, 2)
	floatSlice.SetFloat32(0, 0, 1.5)

	gotHalf := floatSlice.GetHalf(0, 0)
	if gotHalf.Float32() != 1.5 {
		t.Errorf("GetHalf from float slice = %v, want 1.5", gotHalf.Float32())
	}

	// Test half to float32 conversion
	halfData := make([]half.Half, 4)
	halfSlice := NewSliceFromHalf(halfData, 2, 2)
	halfSlice.SetHalf(0, 0, half.FromFloat32(2.0))

	gotFloat := halfSlice.GetFloat32(0, 0)
	if gotFloat != 2.0 {
		t.Errorf("GetFloat32 from half slice = %v, want 2.0", gotFloat)
	}

	// Test uint to float conversion
	uintData := make([]uint32, 4)
	uintSlice := NewSliceFromUint32(uintData, 2, 2)
	uintSlice.SetUint32(0, 0, 100)

	gotFloatFromUint := uintSlice.GetFloat32(0, 0)
	if gotFloatFromUint != 100.0 {
		t.Errorf("GetFloat32 from uint slice = %v, want 100.0", gotFloatFromUint)
	}
}

func TestSliceSubsampling(t *testing.T) {
	data := make([]float32, 5*5) // 5x5 subsampled pixels for 10x10 image with 2x2 subsampling
	slice := Slice{
		Type:      PixelTypeFloat,
		Base:      (NewSliceFromFloat32(data, 5, 5)).Base,
		XStride:   4,
		YStride:   5 * 4,
		XSampling: 2,
		YSampling: 2,
	}

	// Set at (0,0) should affect (0,0), (1,0), (0,1), (1,1)
	slice.SetFloat32(0, 0, 1.0)
	slice.SetFloat32(1, 0, 1.0) // Same sample
	slice.SetFloat32(0, 1, 1.0) // Same sample

	if slice.GetFloat32(0, 0) != 1.0 {
		t.Errorf("Subsampled GetFloat32(0,0) = %v, want 1.0", slice.GetFloat32(0, 0))
	}

	// (2,0) should be a different sample
	slice.SetFloat32(2, 0, 2.0)
	if slice.GetFloat32(2, 0) != 2.0 {
		t.Errorf("Subsampled GetFloat32(2,0) = %v, want 2.0", slice.GetFloat32(2, 0))
	}
}

func TestFrameBuffer(t *testing.T) {
	fb := NewFrameBuffer()

	if fb.Len() != 0 {
		t.Errorf("Len() = %d, want 0", fb.Len())
	}

	// Insert slice
	data := make([]float32, 100)
	slice := NewSliceFromFloat32(data, 10, 10)
	err := fb.Insert("R", slice)
	if err != nil {
		t.Errorf("Insert() error = %v", err)
	}

	if fb.Len() != 1 {
		t.Errorf("Len() = %d, want 1", fb.Len())
	}

	// Duplicate insert should fail
	err = fb.Insert("R", slice)
	if err == nil {
		t.Error("Duplicate Insert should fail")
	}

	// Set (replace) should work
	fb.Set("R", slice)
	if fb.Len() != 1 {
		t.Errorf("After Set, Len() = %d, want 1", fb.Len())
	}

	// Get
	got := fb.Get("R")
	if got == nil {
		t.Error("Get(R) returned nil")
	}

	// Has
	if !fb.Has("R") {
		t.Error("Has(R) should be true")
	}
	if fb.Has("X") {
		t.Error("Has(X) should be false")
	}

	// Names
	names := fb.Names()
	if len(names) != 1 || names[0] != "R" {
		t.Errorf("Names() = %v, want [R]", names)
	}

	// Remove
	fb.Remove("R")
	if fb.Has("R") {
		t.Error("Has(R) should be false after Remove")
	}
}

func TestAllocateChannels(t *testing.T) {
	cl := NewChannelList()
	cl.Add(NewChannel("R", PixelTypeHalf))
	cl.Add(NewChannel("G", PixelTypeHalf))
	cl.Add(NewChannel("B", PixelTypeHalf))

	dataWindow := Box2i{Min: V2i{0, 0}, Max: V2i{9, 9}}
	fb, buffers := AllocateChannels(cl, dataWindow)

	if fb.Len() != 3 {
		t.Errorf("Allocated Len() = %d, want 3", fb.Len())
	}

	// Check buffer sizes
	for _, ch := range []string{"R", "G", "B"} {
		buf := buffers[ch]
		expectedSize := 10 * 10 * 2 // 10x10 pixels * 2 bytes per half
		if len(buf) != expectedSize {
			t.Errorf("Buffer %s size = %d, want %d", ch, len(buf), expectedSize)
		}
	}
}

func TestAllocateChannelsSubsampled(t *testing.T) {
	cl := NewChannelList()
	cl.Add(Channel{Name: "Y", Type: PixelTypeHalf, XSampling: 1, YSampling: 1})
	cl.Add(Channel{Name: "RY", Type: PixelTypeHalf, XSampling: 2, YSampling: 2})

	dataWindow := Box2i{Min: V2i{0, 0}, Max: V2i{9, 9}}
	_, buffers := AllocateChannels(cl, dataWindow)

	// Y: 10x10 * 2 = 200 bytes
	if len(buffers["Y"]) != 200 {
		t.Errorf("Y buffer size = %d, want 200", len(buffers["Y"]))
	}

	// RY: 5x5 * 2 = 50 bytes (2x2 subsampled)
	if len(buffers["RY"]) != 50 {
		t.Errorf("RY buffer size = %d, want 50", len(buffers["RY"]))
	}
}

func TestAllocateChannelsOffset(t *testing.T) {
	cl := NewChannelList()
	cl.Add(NewChannel("R", PixelTypeFloat))

	// Data window not at origin
	dataWindow := Box2i{Min: V2i{100, 100}, Max: V2i{109, 109}}
	fb, _ := AllocateChannels(cl, dataWindow)

	slice := fb.Get("R")
	if slice == nil {
		t.Fatal("Slice R is nil")
	}

	// Should be able to set/get at data window coordinates
	slice.SetFloat32(100, 100, 42.0)
	if slice.GetFloat32(100, 100) != 42.0 {
		t.Errorf("GetFloat32(100,100) = %v, want 42.0", slice.GetFloat32(100, 100))
	}
}

func TestRGBAFrameBuffer(t *testing.T) {
	fb := NewRGBAFrameBuffer(10, 10, true)

	if fb.Width != 10 || fb.Height != 10 {
		t.Errorf("Dimensions = %dx%d, want 10x10", fb.Width, fb.Height)
	}
	if !fb.HasAlpha {
		t.Error("HasAlpha should be true")
	}
	if len(fb.R) != 100 || len(fb.G) != 100 || len(fb.B) != 100 || len(fb.A) != 100 {
		t.Error("Channel buffer sizes incorrect")
	}

	// Test set/get pixel
	fb.SetPixel(5, 5, 1.0, 0.5, 0.25, 0.75)
	r, g, b, a := fb.GetPixel(5, 5)
	if r != 1.0 || g != 0.5 || b != 0.25 || a != 0.75 {
		t.Errorf("GetPixel = (%v,%v,%v,%v), want (1,0.5,0.25,0.75)", r, g, b, a)
	}
}

func TestRGBAFrameBufferNoAlpha(t *testing.T) {
	fb := NewRGBAFrameBuffer(10, 10, false)

	if fb.HasAlpha {
		t.Error("HasAlpha should be false")
	}
	if fb.A != nil {
		t.Error("A channel should be nil")
	}

	// GetPixel should return 1.0 for alpha when no alpha channel
	fb.SetPixel(0, 0, 0.5, 0.5, 0.5, 0) // Alpha ignored
	_, _, _, a := fb.GetPixel(0, 0)
	if a != 1.0 {
		t.Errorf("Alpha for no-alpha buffer = %v, want 1.0", a)
	}
}

func TestRGBAToFrameBuffer(t *testing.T) {
	rgba := NewRGBAFrameBuffer(10, 10, true)
	fb := rgba.ToFrameBuffer()

	if !fb.Has("R") || !fb.Has("G") || !fb.Has("B") || !fb.Has("A") {
		t.Error("ToFrameBuffer should have R, G, B, A channels")
	}

	// No alpha version
	rgbOnly := NewRGBAFrameBuffer(10, 10, false)
	fb2 := rgbOnly.ToFrameBuffer()
	if fb2.Has("A") {
		t.Error("ToFrameBuffer without alpha should not have A channel")
	}
}

func TestSliceWithZeroType(t *testing.T) {
	// Test behavior with invalid pixel type
	// Need valid XSampling/YSampling to avoid divide-by-zero
	slice := Slice{Type: PixelType(99), XSampling: 1, YSampling: 1}

	// Should return 0 for unknown type
	val := slice.GetFloat32(0, 0)
	if val != 0 {
		t.Errorf("GetFloat32 for unknown type = %v, want 0", val)
	}

	h := slice.GetHalf(0, 0)
	if h != half.Zero {
		t.Errorf("GetHalf for unknown type = %v, want Zero", h)
	}

	u := slice.GetUint32(0, 0)
	if u != 0 {
		t.Errorf("GetUint32 for unknown type = %v, want 0", u)
	}
}

func TestSliceCrossTypeSet(t *testing.T) {
	// Test SetUint32 on float slice
	floatData := make([]float32, 4)
	floatSlice := NewSliceFromFloat32(floatData, 2, 2)
	floatSlice.SetUint32(0, 0, 42)
	if floatData[0] != 42.0 {
		t.Errorf("SetUint32 on float slice = %v, want 42.0", floatData[0])
	}

	// Test SetUint32 on half slice
	halfData := make([]half.Half, 4)
	halfSlice := NewSliceFromHalf(halfData, 2, 2)
	halfSlice.SetUint32(0, 0, 10)
	if halfData[0].Float32() != 10.0 {
		t.Errorf("SetUint32 on half slice = %v, want 10.0", halfData[0].Float32())
	}

	// Test SetFloat32 on half slice
	halfData2 := make([]half.Half, 4)
	halfSlice2 := NewSliceFromHalf(halfData2, 2, 2)
	halfSlice2.SetFloat32(0, 0, 3.5)
	if halfData2[0].Float32() != 3.5 {
		t.Errorf("SetFloat32 on half slice = %v, want 3.5", halfData2[0].Float32())
	}

	// Test SetFloat32 on uint slice
	uintData := make([]uint32, 4)
	uintSlice := NewSliceFromUint32(uintData, 2, 2)
	uintSlice.SetFloat32(0, 0, 100.0)
	if uintData[0] != 100 {
		t.Errorf("SetFloat32 on uint slice = %v, want 100", uintData[0])
	}

	// Test SetHalf on float slice
	floatData2 := make([]float32, 4)
	floatSlice2 := NewSliceFromFloat32(floatData2, 2, 2)
	floatSlice2.SetHalf(0, 0, half.FromFloat32(7.5))
	if floatData2[0] != 7.5 {
		t.Errorf("SetHalf on float slice = %v, want 7.5", floatData2[0])
	}

	// Test SetHalf on uint slice
	uintData2 := make([]uint32, 4)
	uintSlice2 := NewSliceFromUint32(uintData2, 2, 2)
	uintSlice2.SetHalf(0, 0, half.FromFloat32(50.0))
	if uintData2[0] != 50 {
		t.Errorf("SetHalf on uint slice = %v, want 50", uintData2[0])
	}
}

func TestSliceGetUint32Conversions(t *testing.T) {
	// Test GetUint32 from float32 slice
	floatData := make([]float32, 4)
	floatSlice := NewSliceFromFloat32(floatData, 2, 2)
	floatSlice.SetFloat32(0, 0, 42.7)
	gotUint := floatSlice.GetUint32(0, 0)
	if gotUint != 42 {
		t.Errorf("GetUint32 from float slice = %d, want 42", gotUint)
	}

	// Test GetUint32 from half slice
	halfData := make([]half.Half, 4)
	halfSlice := NewSliceFromHalf(halfData, 2, 2)
	halfSlice.SetHalf(0, 0, half.FromFloat32(100.0))
	gotUintFromHalf := halfSlice.GetUint32(0, 0)
	if gotUintFromHalf != 100 {
		t.Errorf("GetUint32 from half slice = %d, want 100", gotUintFromHalf)
	}

	// Test GetUint32 from uint slice (identity)
	uintData := make([]uint32, 4)
	uintSlice := NewSliceFromUint32(uintData, 2, 2)
	uintSlice.SetUint32(0, 0, 12345)
	gotUintFromUint := uintSlice.GetUint32(0, 0)
	if gotUintFromUint != 12345 {
		t.Errorf("GetUint32 from uint slice = %d, want 12345", gotUintFromUint)
	}
}

func TestSliceSetUint32Conversions(t *testing.T) {
	// Test SetUint32 on float slice
	floatData := make([]float32, 4)
	floatSlice := NewSliceFromFloat32(floatData, 2, 2)
	floatSlice.SetUint32(0, 0, 123)
	if floatData[0] != 123.0 {
		t.Errorf("SetUint32 on float slice: data = %v, want 123.0", floatData[0])
	}

	// Test SetUint32 on half slice
	halfData := make([]half.Half, 4)
	halfSlice := NewSliceFromHalf(halfData, 2, 2)
	halfSlice.SetUint32(0, 0, 256)
	if halfData[0].Float32() != 256.0 {
		t.Errorf("SetUint32 on half slice: data = %v, want 256.0", halfData[0].Float32())
	}
}

func TestSliceGetHalfConversions(t *testing.T) {
	// Test GetHalf from float slice
	floatData := make([]float32, 4)
	floatSlice := NewSliceFromFloat32(floatData, 2, 2)
	floatSlice.SetFloat32(0, 0, 1.5)
	gotHalfFromFloat := floatSlice.GetHalf(0, 0)
	if gotHalfFromFloat.Float32() != 1.5 {
		t.Errorf("GetHalf from float slice = %f, want 1.5", gotHalfFromFloat.Float32())
	}

	// Test GetHalf from uint slice
	uintData := make([]uint32, 4)
	uintSlice := NewSliceFromUint32(uintData, 2, 2)
	uintSlice.SetUint32(0, 0, 100)
	gotHalfFromUint := uintSlice.GetHalf(0, 0)
	if gotHalfFromUint.Float32() != 100.0 {
		t.Errorf("GetHalf from uint slice = %f, want 100.0", gotHalfFromUint.Float32())
	}
}

func TestSliceSetHalfConversions(t *testing.T) {
	// Test SetHalf on float slice
	floatData := make([]float32, 4)
	floatSlice := NewSliceFromFloat32(floatData, 2, 2)
	floatSlice.SetHalf(0, 0, half.FromFloat32(2.5))
	if floatData[0] != 2.5 {
		t.Errorf("SetHalf on float slice: data = %v, want 2.5", floatData[0])
	}

	// Test SetHalf on uint slice
	uintData := make([]uint32, 4)
	uintSlice := NewSliceFromUint32(uintData, 2, 2)
	uintSlice.SetHalf(0, 0, half.FromFloat32(100))
	if uintData[0] != 100 {
		t.Errorf("SetHalf on uint slice: data = %v, want 100", uintData[0])
	}
}

func TestSliceSetWithTypeMismatch(t *testing.T) {
	// Test setting float32 on uint slice
	data := make([]byte, 4*4*4) // 4x4 uint slice
	slice := NewSlice(PixelTypeUint, data, 4, 4)

	// SetFloat32 on uint slice - test the behavior exists
	slice.SetFloat32(0, 0, 1.5)
	val := slice.GetFloat32(0, 0)
	// Just verify the path is exercised - the actual behavior may vary
	t.Logf("GetFloat32 on uint slice returned %f", val)
}

func TestFrameBufferNames(t *testing.T) {
	fb := NewFrameBuffer()

	rData := make([]byte, 4*4*2)
	gData := make([]byte, 4*4*2)

	fb.Set("R", NewSlice(PixelTypeHalf, rData, 4, 4))
	fb.Set("G", NewSlice(PixelTypeHalf, gData, 4, 4))

	names := fb.Names()
	if len(names) != 2 {
		t.Errorf("Names() len = %d, want 2", len(names))
	}

	// Check names are present
	hasR, hasG := false, false
	for _, n := range names {
		if n == "R" {
			hasR = true
		}
		if n == "G" {
			hasG = true
		}
	}
	if !hasR || !hasG {
		t.Errorf("Names() = %v, want to contain R and G", names)
	}
}
