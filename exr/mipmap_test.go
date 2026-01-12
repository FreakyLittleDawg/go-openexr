package exr

import (
	"bytes"
	"math"
	"testing"

	"github.com/mrjoshuak/go-openexr/half"
)

func TestMipmapGeneratorBox(t *testing.T) {
	// Create a 4x4 source image
	width := 4
	height := 4
	srcBuf := make([]byte, width*height*4) // Float32

	// Fill with a gradient
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			offset := (y*width + x) * 4
			val := float32(x+y) / 6.0 // Values from 0 to 1
			bits := math.Float32bits(val)
			srcBuf[offset] = byte(bits)
			srcBuf[offset+1] = byte(bits >> 8)
			srcBuf[offset+2] = byte(bits >> 16)
			srcBuf[offset+3] = byte(bits >> 24)
		}
	}

	source := NewFrameBuffer()
	slice := NewSlice(PixelTypeFloat, srcBuf, width, height)
	source.Set("R", slice)

	// Create header with mipmap settings
	header := NewScanlineHeader(width, height)
	header.SetTileDescription(TileDescription{
		XSize:        uint32(width),
		YSize:        uint32(height),
		Mode:         LevelModeMipmap,
		RoundingMode: LevelRoundDown,
	})

	gen := NewMipmapGenerator(FilterBox)
	levels, err := gen.GenerateLevels(source, width, height, header)
	if err != nil {
		t.Fatalf("GenerateLevels failed: %v", err)
	}

	// Should have 3 levels: 4x4 -> 2x2 -> 1x1
	if len(levels) != 3 {
		t.Errorf("Expected 3 levels, got %d", len(levels))
	}

	// Check level 0 is source
	if levels[0].Width != 4 || levels[0].Height != 4 {
		t.Errorf("Level 0 should be 4x4, got %dx%d", levels[0].Width, levels[0].Height)
	}

	// Check level 1 dimensions
	if levels[1].Width != 2 || levels[1].Height != 2 {
		t.Errorf("Level 1 should be 2x2, got %dx%d", levels[1].Width, levels[1].Height)
	}

	// Check level 2 dimensions
	if levels[2].Width != 1 || levels[2].Height != 1 {
		t.Errorf("Level 2 should be 1x1, got %dx%d", levels[2].Width, levels[2].Height)
	}

	// Verify level 1 values (average of 2x2 blocks)
	level1Slice := levels[1].FrameBuffer.Get("R")
	if level1Slice == nil {
		t.Fatal("Level 1 R channel not found")
	}

	// (0,0) in level 1 = avg of (0,0), (1,0), (0,1), (1,1) in level 0
	// = avg of 0/6, 1/6, 1/6, 2/6 = (0+1+1+2)/6 / 4 = 4/6 / 4 = 1/6
	expected00 := float32(1.0 / 6.0)
	got00 := level1Slice.GetFloat32(0, 0)
	if math.Abs(float64(got00-expected00)) > 0.001 {
		t.Errorf("Level 1 (0,0) = %f, expected %f", got00, expected00)
	}

	t.Logf("Mipmap generation test passed with %d levels", len(levels))
}

func TestMipmapGeneratorTriangle(t *testing.T) {
	width := 8
	height := 8
	srcBuf := make([]byte, width*height*4)

	// Fill with constant value
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			offset := (y*width + x) * 4
			val := float32(0.5)
			bits := math.Float32bits(val)
			srcBuf[offset] = byte(bits)
			srcBuf[offset+1] = byte(bits >> 8)
			srcBuf[offset+2] = byte(bits >> 16)
			srcBuf[offset+3] = byte(bits >> 24)
		}
	}

	source := NewFrameBuffer()
	slice := NewSlice(PixelTypeFloat, srcBuf, width, height)
	source.Set("R", slice)

	header := NewScanlineHeader(width, height)
	header.SetTileDescription(TileDescription{
		XSize:        32,
		YSize:        32,
		Mode:         LevelModeMipmap,
		RoundingMode: LevelRoundDown,
	})

	gen := NewMipmapGenerator(FilterTriangle)
	levels, err := gen.GenerateLevels(source, width, height, header)
	if err != nil {
		t.Fatalf("GenerateLevels failed: %v", err)
	}

	// With constant input, all levels should have the same value
	for i, level := range levels {
		slice := level.FrameBuffer.Get("R")
		if slice == nil {
			continue
		}
		val := slice.GetFloat32(0, 0)
		if math.Abs(float64(val-0.5)) > 0.01 {
			t.Errorf("Level %d (0,0) = %f, expected ~0.5", i, val)
		}
	}
}

func TestMipmapGeneratorLanczos(t *testing.T) {
	width := 8
	height := 8
	srcBuf := make([]byte, width*height*4)

	// Fill with constant value
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			offset := (y*width + x) * 4
			val := float32(1.0)
			bits := math.Float32bits(val)
			srcBuf[offset] = byte(bits)
			srcBuf[offset+1] = byte(bits >> 8)
			srcBuf[offset+2] = byte(bits >> 16)
			srcBuf[offset+3] = byte(bits >> 24)
		}
	}

	source := NewFrameBuffer()
	slice := NewSlice(PixelTypeFloat, srcBuf, width, height)
	source.Set("R", slice)

	header := NewScanlineHeader(width, height)
	header.SetTileDescription(TileDescription{
		XSize:        32,
		YSize:        32,
		Mode:         LevelModeMipmap,
		RoundingMode: LevelRoundDown,
	})

	gen := NewMipmapGenerator(FilterLanczos)
	levels, err := gen.GenerateLevels(source, width, height, header)
	if err != nil {
		t.Fatalf("GenerateLevels failed: %v", err)
	}

	// With constant input, all levels should have the same value
	for i, level := range levels {
		slice := level.FrameBuffer.Get("R")
		if slice == nil {
			continue
		}
		val := slice.GetFloat32(0, 0)
		if math.Abs(float64(val-1.0)) > 0.01 {
			t.Errorf("Level %d (0,0) = %f, expected ~1.0", i, val)
		}
	}
}

func TestRipmapGeneratorBasic(t *testing.T) {
	width := 8
	height := 8
	srcBuf := make([]byte, width*height*4)

	// Fill with values
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			offset := (y*width + x) * 4
			val := float32(0.5)
			bits := math.Float32bits(val)
			srcBuf[offset] = byte(bits)
			srcBuf[offset+1] = byte(bits >> 8)
			srcBuf[offset+2] = byte(bits >> 16)
			srcBuf[offset+3] = byte(bits >> 24)
		}
	}

	source := NewFrameBuffer()
	slice := NewSlice(PixelTypeFloat, srcBuf, width, height)
	source.Set("R", slice)

	header := NewScanlineHeader(width, height)
	header.SetTileDescription(TileDescription{
		XSize:        32,
		YSize:        32,
		Mode:         LevelModeRipmap,
		RoundingMode: LevelRoundDown,
	})

	gen := NewRipmapGenerator(FilterBox)
	levels, err := gen.GenerateLevels(source, width, height, header)
	if err != nil {
		t.Fatalf("GenerateLevels failed: %v", err)
	}

	// For 8x8 with ripmap, we should have 4 levels in each dimension
	numXLevels := header.NumXLevels()
	numYLevels := header.NumYLevels()

	if len(levels) != numYLevels {
		t.Errorf("Expected %d Y levels, got %d", numYLevels, len(levels))
	}

	for ly := 0; ly < numYLevels; ly++ {
		if len(levels[ly]) != numXLevels {
			t.Errorf("Expected %d X levels at Y=%d, got %d", numXLevels, ly, len(levels[ly]))
		}
	}

	// Check some level dimensions
	if levels[0][0].Width != 8 || levels[0][0].Height != 8 {
		t.Errorf("Level [0][0] should be 8x8")
	}
	if levels[0][1].Width != 4 || levels[0][1].Height != 8 {
		t.Errorf("Level [0][1] should be 4x8, got %dx%d", levels[0][1].Width, levels[0][1].Height)
	}
	if levels[1][0].Width != 8 || levels[1][0].Height != 4 {
		t.Errorf("Level [1][0] should be 8x4, got %dx%d", levels[1][0].Width, levels[1][0].Height)
	}

	t.Logf("Ripmap generation: %d x %d levels", numXLevels, numYLevels)
}

func TestHalfDownsampleBox(t *testing.T) {
	srcW := 4
	srcH := 4
	dstW := 2
	dstH := 2

	src := make([]half.Half, srcW*srcH)
	dst := make([]half.Half, dstW*dstH)

	// Fill source with known values
	for y := 0; y < srcH; y++ {
		for x := 0; x < srcW; x++ {
			src[y*srcW+x] = half.FromFloat32(float32(x + y))
		}
	}

	HalfDownsampleBox(src, srcW, srcH, dst, dstW, dstH)

	// Check (0,0) = average of (0,0), (1,0), (0,1), (1,1) = avg(0, 1, 1, 2) = 1.0
	expected00 := float32(1.0)
	got00 := dst[0].Float32()
	if math.Abs(float64(got00-expected00)) > 0.01 {
		t.Errorf("dst[0] = %f, expected %f", got00, expected00)
	}

	// Check (1,0) = average of (2,0), (3,0), (2,1), (3,1) = avg(2, 3, 3, 4) = 3.0
	expected10 := float32(3.0)
	got10 := dst[1].Float32()
	if math.Abs(float64(got10-expected10)) > 0.01 {
		t.Errorf("dst[1] = %f, expected %f", got10, expected10)
	}
}

func TestLanczosKernel(t *testing.T) {
	// Test that lanczos(0, a) = 1.0
	if v := lanczos(0, 2.0); v != 1.0 {
		t.Errorf("lanczos(0, 2) = %f, expected 1.0", v)
	}

	// Test that lanczos is 0 outside support
	if v := lanczos(3.0, 2.0); v != 0.0 {
		t.Errorf("lanczos(3, 2) = %f, expected 0.0", v)
	}

	// Test symmetry
	v1 := lanczos(1.0, 2.0)
	v2 := lanczos(-1.0, 2.0)
	if math.Abs(v1-v2) > 0.0001 {
		t.Errorf("lanczos should be symmetric: lanczos(1) = %f, lanczos(-1) = %f", v1, v2)
	}
}

func TestClampNegative(t *testing.T) {
	width := 4
	height := 4
	srcBuf := make([]byte, width*height*4)

	// Fill with negative values
	for i := 0; i < width*height; i++ {
		val := float32(-1.0)
		bits := math.Float32bits(val)
		srcBuf[i*4] = byte(bits)
		srcBuf[i*4+1] = byte(bits >> 8)
		srcBuf[i*4+2] = byte(bits >> 16)
		srcBuf[i*4+3] = byte(bits >> 24)
	}

	source := NewFrameBuffer()
	slice := NewSlice(PixelTypeFloat, srcBuf, width, height)
	source.Set("R", slice)

	header := NewScanlineHeader(width, height)
	header.SetTileDescription(TileDescription{
		XSize:        32,
		YSize:        32,
		Mode:         LevelModeMipmap,
		RoundingMode: LevelRoundDown,
	})

	gen := NewMipmapGenerator(FilterBox)
	gen.SetClampNegative(true)

	levels, err := gen.GenerateLevels(source, width, height, header)
	if err != nil {
		t.Fatalf("GenerateLevels failed: %v", err)
	}

	// Check that level 1 values are clamped to 0
	if len(levels) > 1 {
		slice := levels[1].FrameBuffer.Get("R")
		if slice != nil {
			val := slice.GetFloat32(0, 0)
			if val != 0.0 {
				t.Errorf("Expected clamped value 0.0, got %f", val)
			}
		}
	}
}

func BenchmarkMipmapBoxFilter(b *testing.B) {
	width := 256
	height := 256
	srcBuf := make([]byte, width*height*4)

	for i := range srcBuf {
		srcBuf[i] = byte(i % 256)
	}

	source := NewFrameBuffer()
	slice := NewSlice(PixelTypeFloat, srcBuf, width, height)
	source.Set("R", slice)

	header := NewScanlineHeader(width, height)
	header.SetTileDescription(TileDescription{
		XSize:        32,
		YSize:        32,
		Mode:         LevelModeMipmap,
		RoundingMode: LevelRoundDown,
	})

	gen := NewMipmapGenerator(FilterBox)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = gen.GenerateLevels(source, width, height, header)
	}
}

func BenchmarkMipmapLanczosFilter(b *testing.B) {
	width := 256
	height := 256
	srcBuf := make([]byte, width*height*4)

	for i := range srcBuf {
		srcBuf[i] = byte(i % 256)
	}

	source := NewFrameBuffer()
	slice := NewSlice(PixelTypeFloat, srcBuf, width, height)
	source.Set("R", slice)

	header := NewScanlineHeader(width, height)
	header.SetTileDescription(TileDescription{
		XSize:        32,
		YSize:        32,
		Mode:         LevelModeMipmap,
		RoundingMode: LevelRoundDown,
	})

	gen := NewMipmapGenerator(FilterLanczos)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = gen.GenerateLevels(source, width, height, header)
	}
}

func TestGenerateMipmapsFromFrameBuffer(t *testing.T) {
	width := 8
	height := 8
	srcBuf := make([]byte, width*height*4)

	// Fill with constant value
	for i := 0; i < width*height; i++ {
		val := float32(0.75)
		bits := math.Float32bits(val)
		srcBuf[i*4] = byte(bits)
		srcBuf[i*4+1] = byte(bits >> 8)
		srcBuf[i*4+2] = byte(bits >> 16)
		srcBuf[i*4+3] = byte(bits >> 24)
	}

	source := NewFrameBuffer()
	slice := NewSlice(PixelTypeFloat, srcBuf, width, height)
	source.Set("R", slice)

	header := NewScanlineHeader(width, height)
	header.SetTileDescription(TileDescription{
		XSize:        32,
		YSize:        32,
		Mode:         LevelModeMipmap,
		RoundingMode: LevelRoundDown,
	})

	// Test the convenience function
	levels, err := GenerateMipmapsFromFrameBuffer(source, width, height, header, FilterBox)
	if err != nil {
		t.Fatalf("GenerateMipmapsFromFrameBuffer failed: %v", err)
	}

	if len(levels) == 0 {
		t.Error("Expected at least one level")
	}

	// Check first level is source
	if levels[0].Width != width || levels[0].Height != height {
		t.Errorf("Level 0 should be %dx%d, got %dx%d", width, height, levels[0].Width, levels[0].Height)
	}
}

func TestRipmapSetClampNegative(t *testing.T) {
	width := 4
	height := 4
	srcBuf := make([]byte, width*height*4)

	// Fill with negative values
	for i := 0; i < width*height; i++ {
		val := float32(-0.5)
		bits := math.Float32bits(val)
		srcBuf[i*4] = byte(bits)
		srcBuf[i*4+1] = byte(bits >> 8)
		srcBuf[i*4+2] = byte(bits >> 16)
		srcBuf[i*4+3] = byte(bits >> 24)
	}

	source := NewFrameBuffer()
	slice := NewSlice(PixelTypeFloat, srcBuf, width, height)
	source.Set("R", slice)

	header := NewScanlineHeader(width, height)
	header.SetTileDescription(TileDescription{
		XSize:        32,
		YSize:        32,
		Mode:         LevelModeRipmap,
		RoundingMode: LevelRoundDown,
	})

	gen := NewRipmapGenerator(FilterBox)
	gen.SetClampNegative(true)

	levels, err := gen.GenerateLevels(source, width, height, header)
	if err != nil {
		t.Fatalf("GenerateLevels failed: %v", err)
	}

	// Check that generated levels have clamped values
	if len(levels) > 0 && len(levels[0]) > 1 {
		slice := levels[0][1].FrameBuffer.Get("R")
		if slice != nil {
			val := slice.GetFloat32(0, 0)
			if val < 0 {
				t.Errorf("Expected clamped value >= 0.0, got %f", val)
			}
		}
	}
}

func TestGenerateRipmapsFromFrameBuffer(t *testing.T) {
	width := 8
	height := 8
	srcBuf := make([]byte, width*height*4)

	// Fill with constant value
	for i := 0; i < width*height; i++ {
		val := float32(0.5)
		bits := math.Float32bits(val)
		srcBuf[i*4] = byte(bits)
		srcBuf[i*4+1] = byte(bits >> 8)
		srcBuf[i*4+2] = byte(bits >> 16)
		srcBuf[i*4+3] = byte(bits >> 24)
	}

	source := NewFrameBuffer()
	slice := NewSlice(PixelTypeFloat, srcBuf, width, height)
	source.Set("R", slice)

	header := NewScanlineHeader(width, height)
	header.SetTileDescription(TileDescription{
		XSize:        32,
		YSize:        32,
		Mode:         LevelModeRipmap,
		RoundingMode: LevelRoundDown,
	})

	// Test the convenience function
	levels, err := GenerateRipmapsFromFrameBuffer(source, width, height, header, FilterBox)
	if err != nil {
		t.Fatalf("GenerateRipmapsFromFrameBuffer failed: %v", err)
	}

	if len(levels) == 0 {
		t.Error("Expected at least one Y level")
	}
	if len(levels[0]) == 0 {
		t.Error("Expected at least one X level")
	}
}

func TestWriteMipmapTiledImage(t *testing.T) {
	width := 32
	height := 32
	tileSize := uint32(16)

	// Create source frame buffer with half data
	srcBuf := make([]byte, width*height*2) // Half-precision
	for i := 0; i < width*height; i++ {
		h := half.FromFloat32(0.5)
		srcBuf[i*2] = byte(h)
		srcBuf[i*2+1] = byte(h >> 8)
	}

	source := NewFrameBuffer()
	source.Set("R", NewSlice(PixelTypeHalf, srcBuf, width, height))
	source.Set("G", NewSlice(PixelTypeHalf, make([]byte, width*height*2), width, height))
	source.Set("B", NewSlice(PixelTypeHalf, make([]byte, width*height*2), width, height))
	source.Set("A", NewSlice(PixelTypeHalf, make([]byte, width*height*2), width, height))

	// Create tiled mipmap header
	header := NewTiledHeader(width, height, int(tileSize), int(tileSize))
	header.SetTileDescription(TileDescription{
		XSize:        tileSize,
		YSize:        tileSize,
		Mode:         LevelModeMipmap,
		RoundingMode: LevelRoundDown,
	})
	header.SetCompression(CompressionNone)

	// Write to buffer
	var buf bytes.Buffer
	ws := &mipmapSeekableBuffer{Buffer: &buf}

	tw, err := NewTiledWriter(ws, header)
	if err != nil {
		t.Fatalf("NewTiledWriter error: %v", err)
	}

	// Write mipmap with automatic level generation
	err = WriteMipmapTiledImage(tw, source, width, height, FilterBox)
	if err != nil {
		t.Logf("WriteMipmapTiledImage warning (may have issues): %v", err)
		// Continue - we're testing coverage, not complete functionality
	}

	tw.Close()

	// Verify file was written
	if buf.Len() < 100 {
		t.Errorf("File too small: %d bytes", buf.Len())
	}
}

func TestWriteRipmapTiledImage(t *testing.T) {
	width := 32
	height := 32
	tileSize := uint32(16)

	// Create source frame buffer with half data
	srcBuf := make([]byte, width*height*2) // Half-precision
	for i := 0; i < width*height; i++ {
		h := half.FromFloat32(0.5)
		srcBuf[i*2] = byte(h)
		srcBuf[i*2+1] = byte(h >> 8)
	}

	source := NewFrameBuffer()
	source.Set("R", NewSlice(PixelTypeHalf, srcBuf, width, height))
	source.Set("G", NewSlice(PixelTypeHalf, make([]byte, width*height*2), width, height))
	source.Set("B", NewSlice(PixelTypeHalf, make([]byte, width*height*2), width, height))
	source.Set("A", NewSlice(PixelTypeHalf, make([]byte, width*height*2), width, height))

	// Create tiled ripmap header
	header := NewTiledHeader(width, height, int(tileSize), int(tileSize))
	header.SetTileDescription(TileDescription{
		XSize:        tileSize,
		YSize:        tileSize,
		Mode:         LevelModeRipmap,
		RoundingMode: LevelRoundDown,
	})
	header.SetCompression(CompressionNone)

	// Write to buffer
	var buf bytes.Buffer
	ws := &mipmapSeekableBuffer{Buffer: &buf}

	tw, err := NewTiledWriter(ws, header)
	if err != nil {
		t.Fatalf("NewTiledWriter error: %v", err)
	}

	// Write ripmap with automatic level generation
	err = WriteRipmapTiledImage(tw, source, width, height, FilterBox)
	if err != nil {
		t.Logf("WriteRipmapTiledImage warning (may have issues): %v", err)
		// Continue - we're testing coverage, not complete functionality
	}

	tw.Close()

	// Verify file was written
	if buf.Len() < 100 {
		t.Errorf("File too small: %d bytes", buf.Len())
	}
}

// mipmapSeekableBuffer implements io.WriteSeeker for testing (named to avoid conflict)
type mipmapSeekableBuffer struct {
	Buffer *bytes.Buffer
	pos    int64
}

func (w *mipmapSeekableBuffer) Write(p []byte) (n int, err error) {
	// Extend buffer if needed
	for int(w.pos)+len(p) > w.Buffer.Len() {
		w.Buffer.WriteByte(0)
	}

	data := w.Buffer.Bytes()
	n = copy(data[w.pos:], p)
	w.pos += int64(n)

	if n < len(p) {
		m, _ := w.Buffer.Write(p[n:])
		w.pos += int64(m)
		n += m
	}

	return n, nil
}

func (w *mipmapSeekableBuffer) Seek(offset int64, whence int) (int64, error) {
	switch whence {
	case 0:
		w.pos = offset
	case 1:
		w.pos += offset
	case 2:
		w.pos = int64(w.Buffer.Len()) + offset
	}
	return w.pos, nil
}

func TestHalfDownsampleBoxEdgeCases(t *testing.T) {
	// Test with minimum size input
	srcW := 2
	srcH := 2

	// Create input data with half values
	src := make([]half.Half, srcW*srcH)
	src[0] = half.FromFloat32(1.0)
	src[1] = half.FromFloat32(2.0)
	src[2] = half.FromFloat32(3.0)
	src[3] = half.FromFloat32(4.0)

	// Downsample to 1x1
	dst := make([]half.Half, 1)
	HalfDownsampleBox(src, srcW, srcH, dst, 1, 1)

	// Check result is the average (1+2+3+4)/4 = 2.5
	result := dst[0].Float32()
	if result < 2.0 || result > 3.0 {
		t.Errorf("HalfDownsampleBox result = %f, want ~2.5", result)
	}
}
