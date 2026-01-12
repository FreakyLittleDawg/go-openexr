package exr

import (
	"bytes"
	"image"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestGeneratePreview(t *testing.T) {
	// Create a test image
	img := NewRGBAImage(image.Rect(0, 0, 100, 100))
	for y := 0; y < 100; y++ {
		for x := 0; x < 100; x++ {
			img.SetRGBA(x, y, float32(x)/100.0, float32(y)/100.0, 0.5, 1.0)
		}
	}

	// Generate preview
	preview := GeneratePreview(img, 32, 32)
	if preview == nil {
		t.Fatal("GeneratePreview returned nil")
	}

	// Check dimensions
	if preview.Width != 32 || preview.Height != 32 {
		t.Errorf("Preview size = %dx%d, want 32x32", preview.Width, preview.Height)
	}

	// Check pixel data length
	expectedLen := 32 * 32 * 4
	if len(preview.Pixels) != expectedLen {
		t.Errorf("Preview pixels len = %d, want %d", len(preview.Pixels), expectedLen)
	}

	// Check that pixels are non-zero (not all black)
	hasNonZero := false
	for _, p := range preview.Pixels {
		if p != 0 {
			hasNonZero = true
			break
		}
	}
	if !hasNonZero {
		t.Error("Preview pixels are all zero")
	}
}

func TestGeneratePreviewAspectRatio(t *testing.T) {
	// Create a wide image
	img := NewRGBAImage(image.Rect(0, 0, 200, 100))
	for y := 0; y < 100; y++ {
		for x := 0; x < 200; x++ {
			img.SetRGBA(x, y, 0.5, 0.5, 0.5, 1.0)
		}
	}

	// Generate preview with max 64x64
	preview := GeneratePreview(img, 64, 64)
	if preview == nil {
		t.Fatal("GeneratePreview returned nil")
	}

	// Should be 64x32 to preserve 2:1 aspect ratio
	if preview.Width != 64 || preview.Height != 32 {
		t.Errorf("Preview size = %dx%d, want 64x32", preview.Width, preview.Height)
	}
}

func TestGeneratePreviewSmallSource(t *testing.T) {
	// Create a small image that doesn't need scaling
	img := NewRGBAImage(image.Rect(0, 0, 16, 16))
	for y := 0; y < 16; y++ {
		for x := 0; x < 16; x++ {
			img.SetRGBA(x, y, 1.0, 1.0, 1.0, 1.0)
		}
	}

	// Generate preview with max 64x64
	preview := GeneratePreview(img, 64, 64)
	if preview == nil {
		t.Fatal("GeneratePreview returned nil")
	}

	// Should keep original size
	if preview.Width != 16 || preview.Height != 16 {
		t.Errorf("Preview size = %dx%d, want 16x16", preview.Width, preview.Height)
	}
}

func TestGeneratePreviewHDR(t *testing.T) {
	// Create an HDR image with values > 1
	img := NewRGBAImage(image.Rect(0, 0, 10, 10))
	for y := 0; y < 10; y++ {
		for x := 0; x < 10; x++ {
			// HDR values
			img.SetRGBA(x, y, 5.0, 10.0, 0.0, 1.0)
		}
	}

	// Generate preview
	preview := GeneratePreview(img, 10, 10)
	if preview == nil {
		t.Fatal("GeneratePreview returned nil")
	}

	// Check that tone mapping worked (values should be < 255)
	for i := 0; i < len(preview.Pixels); i += 4 {
		r := preview.Pixels[i]
		g := preview.Pixels[i+1]
		b := preview.Pixels[i+2]
		a := preview.Pixels[i+3]

		// R and G should be bright but not saturated at 255 due to tone mapping
		// (Reinhard maps 5.0 -> 5/6 ≈ 0.83, which with gamma is ~0.92)
		if r == 255 {
			t.Errorf("R channel saturated, tone mapping may not be working")
		}
		if g == 255 {
			t.Errorf("G channel saturated, tone mapping may not be working")
		}
		if b != 0 {
			t.Errorf("B channel = %d, want 0", b)
		}
		if a != 255 {
			t.Errorf("A channel = %d, want 255", a)
		}
	}
}

func TestGeneratePreviewNil(t *testing.T) {
	// Empty image
	img := NewRGBAImage(image.Rect(0, 0, 0, 0))
	preview := GeneratePreview(img, 32, 32)
	if preview != nil {
		t.Error("GeneratePreview should return nil for empty image")
	}
}

func TestCalculatePreviewSize(t *testing.T) {
	tests := []struct {
		srcW, srcH   int
		maxW, maxH   int
		wantW, wantH int
	}{
		{100, 100, 50, 50, 50, 50},     // Square, scale down
		{200, 100, 50, 50, 50, 25},     // Wide, scale down
		{100, 200, 50, 50, 25, 50},     // Tall, scale down
		{10, 10, 50, 50, 10, 10},       // Small, no scale
		{100, 100, 100, 100, 100, 100}, // Exact fit
		{100, 50, 50, 50, 50, 25},      // 2:1 ratio
	}

	for _, tt := range tests {
		w, h := calculatePreviewSize(tt.srcW, tt.srcH, tt.maxW, tt.maxH)
		if w != tt.wantW || h != tt.wantH {
			t.Errorf("calculatePreviewSize(%d, %d, %d, %d) = %d, %d; want %d, %d",
				tt.srcW, tt.srcH, tt.maxW, tt.maxH, w, h, tt.wantW, tt.wantH)
		}
	}
}

func TestToneMap(t *testing.T) {
	tests := []struct {
		input float32
		want  float32
	}{
		{0.0, 0.0},
		{1.0, 0.5},           // 1/(1+1) = 0.5
		{-1.0, 0.0},          // Negative clamps to 0
		{100.0, 100.0 / 101}, // Large value compresses
	}

	for _, tt := range tests {
		got := toneMap(tt.input)
		if abs32(got-tt.want) > 0.001 {
			t.Errorf("toneMap(%f) = %f, want %f", tt.input, got, tt.want)
		}
	}
}

// abs32 is declared in image_test.go

func TestLinearToSRGB(t *testing.T) {
	tests := []struct {
		input float32
		want  float32
	}{
		{0.0, 0.0},
		{1.0, 1.0},
		{0.5, 0.735}, // Approximately
	}

	for _, tt := range tests {
		got := linearToSRGB(tt.input)
		if abs32(got-tt.want) > 0.01 {
			t.Errorf("linearToSRGB(%f) = %f, want ~%f", tt.input, got, tt.want)
		}
	}
}

func TestSRGBToLinear(t *testing.T) {
	// Test round-trip
	values := []float32{0.0, 0.1, 0.5, 0.9, 1.0}
	for _, v := range values {
		srgb := linearToSRGB(v)
		back := sRGBToLinear(srgb)
		if abs32(back-v) > 0.001 {
			t.Errorf("Round-trip failed: %f -> %f -> %f", v, srgb, back)
		}
	}
}

func TestPreviewToRGBA(t *testing.T) {
	// Create a preview
	preview := &Preview{
		Width:  2,
		Height: 2,
		Pixels: []byte{
			255, 0, 0, 255, // Red
			0, 255, 0, 255, // Green
			0, 0, 255, 255, // Blue
			128, 128, 128, 255, // Gray
		},
	}

	img := PreviewToRGBA(preview)
	if img == nil {
		t.Fatal("PreviewToRGBA returned nil")
	}

	if img.Rect.Dx() != 2 || img.Rect.Dy() != 2 {
		t.Errorf("Image size = %dx%d, want 2x2", img.Rect.Dx(), img.Rect.Dy())
	}

	// Check red pixel (after sRGB to linear conversion)
	r, _, _, _ := img.RGBA(0, 0)
	if r <= 0.5 {
		t.Errorf("Red pixel R = %f, expected > 0.5", r)
	}
}

func TestPreviewToRGBANil(t *testing.T) {
	img := PreviewToRGBA(nil)
	if img != nil {
		t.Error("PreviewToRGBA(nil) should return nil")
	}

	img = PreviewToRGBA(&Preview{Width: 0, Height: 0})
	if img != nil {
		t.Error("PreviewToRGBA(empty) should return nil")
	}
}

func TestHeaderPreviewMethods(t *testing.T) {
	h := NewHeader()

	// Initially no preview
	if h.HasPreview() {
		t.Error("New header should not have preview")
	}
	if h.Preview() != nil {
		t.Error("Preview() should return nil for new header")
	}

	// Set preview
	preview := Preview{
		Width:  4,
		Height: 4,
		Pixels: make([]byte, 4*4*4),
	}
	h.SetPreview(preview)

	// Check preview exists
	if !h.HasPreview() {
		t.Error("HasPreview() should return true after SetPreview")
	}
	if h.Preview() == nil {
		t.Error("Preview() should not return nil after SetPreview")
	}
	if h.Preview().Width != 4 || h.Preview().Height != 4 {
		t.Errorf("Preview size = %dx%d, want 4x4", h.Preview().Width, h.Preview().Height)
	}
}

func TestPreviewRoundTrip(t *testing.T) {
	// Create a test image
	img := NewRGBAImage(image.Rect(0, 0, 64, 64))
	for y := 0; y < 64; y++ {
		for x := 0; x < 64; x++ {
			img.SetRGBA(x, y, float32(x)/64.0, float32(y)/64.0, 0.3, 1.0)
		}
	}

	// Generate preview
	preview := GeneratePreview(img, 16, 16)
	if preview == nil {
		t.Fatal("GeneratePreview returned nil")
	}

	// Write EXR with preview
	dir := t.TempDir()
	path := filepath.Join(dir, "with_preview.exr")

	h := NewScanlineHeader(64, 64)
	h.SetPreview(*preview)

	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("Failed to create file: %v", err)
	}

	// Create frame buffer
	fb := NewFrameBuffer()
	rData := make([]byte, 64*64*2)
	gData := make([]byte, 64*64*2)
	bData := make([]byte, 64*64*2)
	fb.Set("R", NewSlice(PixelTypeHalf, rData, 64, 64))
	fb.Set("G", NewSlice(PixelTypeHalf, gData, 64, 64))
	fb.Set("B", NewSlice(PixelTypeHalf, bData, 64, 64))

	sw, err := NewScanlineWriter(f, h)
	if err != nil {
		f.Close()
		t.Fatalf("Failed to create writer: %v", err)
	}
	sw.SetFrameBuffer(fb)
	if err := sw.WritePixels(0, 63); err != nil {
		t.Fatalf("Failed to write pixels: %v", err)
	}
	if err := sw.Close(); err != nil {
		t.Fatalf("Failed to close writer: %v", err)
	}
	f.Close()

	// Read back and check preview
	file, err := OpenFile(path)
	if err != nil {
		t.Fatalf("Failed to open file: %v", err)
	}
	defer file.Close()

	readHeader := file.Header(0)
	if readHeader == nil {
		t.Fatal("Failed to get header")
	}

	if !readHeader.HasPreview() {
		t.Error("File should have preview")
	}

	readPreview := readHeader.Preview()
	if readPreview == nil {
		t.Fatal("Preview should not be nil")
	}

	if readPreview.Width != preview.Width || readPreview.Height != preview.Height {
		t.Errorf("Preview size = %dx%d, want %dx%d",
			readPreview.Width, readPreview.Height, preview.Width, preview.Height)
	}

	if !bytes.Equal(readPreview.Pixels, preview.Pixels) {
		t.Error("Preview pixels don't match")
	}
}

func TestExtractPreview(t *testing.T) {
	// Use a manual temp directory to avoid TempDir cleanup race on Windows
	dir, err := os.MkdirTemp("", "exr_test_preview_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	// Clean up with retries for Windows
	t.Cleanup(func() {
		for i := 0; i < 10; i++ {
			runtime.GC()
			if err := os.RemoveAll(dir); err == nil {
				return
			}
			time.Sleep(50 * time.Millisecond)
		}
		if err := os.RemoveAll(dir); err != nil {
			t.Logf("Warning: failed to clean up temp dir %s: %v", dir, err)
		}
	})

	path := filepath.Join(dir, "extract_preview.exr")

	preview := &Preview{
		Width:  8,
		Height: 8,
		Pixels: make([]byte, 8*8*4),
	}
	// Fill with recognizable pattern
	for i := 0; i < len(preview.Pixels); i += 4 {
		preview.Pixels[i] = 255   // R
		preview.Pixels[i+1] = 0   // G
		preview.Pixels[i+2] = 0   // B
		preview.Pixels[i+3] = 255 // A
	}

	h := NewScanlineHeader(32, 32)
	h.SetPreview(*preview)

	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("Failed to create file: %v", err)
	}

	fb := NewFrameBuffer()
	rData := make([]byte, 32*32*2)
	gData := make([]byte, 32*32*2)
	bData := make([]byte, 32*32*2)
	fb.Set("R", NewSlice(PixelTypeHalf, rData, 32, 32))
	fb.Set("G", NewSlice(PixelTypeHalf, gData, 32, 32))
	fb.Set("B", NewSlice(PixelTypeHalf, bData, 32, 32))

	sw, _ := NewScanlineWriter(f, h)
	sw.SetFrameBuffer(fb)
	sw.WritePixels(0, 31)
	sw.Close()
	f.Close()

	// Extract preview without reading full image
	extracted, err := ExtractPreview(path)
	if err != nil {
		t.Fatalf("ExtractPreview failed: %v", err)
	}
	if extracted == nil {
		t.Fatal("ExtractPreview returned nil")
	}

	if extracted.Width != 8 || extracted.Height != 8 {
		t.Errorf("Extracted preview size = %dx%d, want 8x8", extracted.Width, extracted.Height)
	}

	// Check first pixel is red
	if extracted.Pixels[0] != 255 || extracted.Pixels[1] != 0 || extracted.Pixels[2] != 0 {
		t.Errorf("First pixel = RGB(%d,%d,%d), want (255,0,0)",
			extracted.Pixels[0], extracted.Pixels[1], extracted.Pixels[2])
	}

	// Force cleanup before test ends
	runtime.GC()
}

func TestExtractPreviewNoPreview(t *testing.T) {
	// Use manual temp dir with retry cleanup for Windows compatibility
	dir, err := os.MkdirTemp("", "exr_test_preview_no_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	t.Cleanup(func() {
		for i := 0; i < 10; i++ {
			runtime.GC()
			if err := os.RemoveAll(dir); err == nil {
				return
			}
			time.Sleep(50 * time.Millisecond)
		}
		if err := os.RemoveAll(dir); err != nil {
			t.Logf("Warning: failed to clean up temp dir %s: %v", dir, err)
		}
	})

	path := filepath.Join(dir, "no_preview.exr")

	h := NewScanlineHeader(16, 16)

	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	fb := NewFrameBuffer()
	rData := make([]byte, 16*16*2)
	gData := make([]byte, 16*16*2)
	bData := make([]byte, 16*16*2)
	fb.Set("R", NewSlice(PixelTypeHalf, rData, 16, 16))
	fb.Set("G", NewSlice(PixelTypeHalf, gData, 16, 16))
	fb.Set("B", NewSlice(PixelTypeHalf, bData, 16, 16))

	sw, err := NewScanlineWriter(f, h)
	if err != nil {
		f.Close()
		t.Fatalf("Failed to create scanline writer: %v", err)
	}
	sw.SetFrameBuffer(fb)
	sw.WritePixels(0, 15)
	sw.Close()
	f.Close()

	// Force garbage collection to release file handles
	runtime.GC()

	// Extract preview
	preview, err := ExtractPreview(path)
	if err != nil {
		t.Fatalf("ExtractPreview failed: %v", err)
	}
	if preview != nil {
		t.Error("ExtractPreview should return nil for file without preview")
	}
}

func TestRectFromSize(t *testing.T) {
	r := RectFromSize(100, 50)
	if r.Dx() != 100 || r.Dy() != 50 {
		t.Errorf("RectFromSize(100, 50) = %dx%d, want 100x50", r.Dx(), r.Dy())
	}
	if r.Min.X != 0 || r.Min.Y != 0 {
		t.Errorf("RectFromSize min = (%d,%d), want (0,0)", r.Min.X, r.Min.Y)
	}
}

func TestExtractPreviewInvalidPath(t *testing.T) {
	_, err := ExtractPreview("/nonexistent/path/file.exr")
	if err == nil {
		t.Error("ExtractPreview with invalid path should return error")
	}
}

func TestCalculatePreviewSizeEdgeCases(t *testing.T) {
	// Test with zero maxWidth
	w, h := calculatePreviewSize(100, 100, 0, 100)
	if w != 0 || h != 0 {
		t.Errorf("calculatePreviewSize with maxWidth=0: got (%d,%d), want (0,0)", w, h)
	}

	// Test with zero maxHeight
	w, h = calculatePreviewSize(100, 100, 100, 0)
	if w != 0 || h != 0 {
		t.Errorf("calculatePreviewSize with maxHeight=0: got (%d,%d), want (0,0)", w, h)
	}

	// Test when source is smaller than max
	w, h = calculatePreviewSize(50, 50, 100, 100)
	if w != 50 || h != 50 {
		t.Errorf("calculatePreviewSize when source smaller: got (%d,%d), want (50,50)", w, h)
	}

	// Test when source is larger - scale down
	w, h = calculatePreviewSize(200, 100, 100, 100)
	if w > 100 || h > 100 {
		t.Errorf("calculatePreviewSize should scale down: got (%d,%d)", w, h)
	}

	// Test ensuring minimum 1 pixel (very large source, very small max)
	w, h = calculatePreviewSize(10000, 10000, 1, 1)
	if w < 1 || h < 1 {
		t.Errorf("calculatePreviewSize minimum 1 pixel: got (%d,%d)", w, h)
	}
}

func TestLinearToSRGBEdgeCases(t *testing.T) {
	// Test with negative value
	result := linearToSRGB(-1.0)
	if result != 0 {
		t.Errorf("linearToSRGB(-1.0) = %f, want 0", result)
	}

	// Test with value > 1
	result = linearToSRGB(2.0)
	if result != 1.0 {
		t.Errorf("linearToSRGB(2.0) = %f, want 1.0", result)
	}
}

func TestSRGBToLinearEdgeCases(t *testing.T) {
	// Test with negative value
	result := sRGBToLinear(-1.0)
	if result != 0 {
		t.Errorf("sRGBToLinear(-1.0) = %f, want 0", result)
	}

	// Test with value > 1
	result = sRGBToLinear(2.0)
	if result != 1.0 {
		t.Errorf("sRGBToLinear(2.0) = %f, want 1.0", result)
	}
}

func TestGeneratePreviewInvalidInput(t *testing.T) {
	// Test with zero-sized image
	img := &RGBAImage{
		Pix:    make([]float32, 0),
		Stride: 4,
		Rect:   image.Rect(0, 0, 0, 0),
	}
	preview := GeneratePreview(img, 100, 100)
	if preview != nil {
		t.Error("GeneratePreview with zero-sized image should return nil")
	}

	// Test with negative maxWidth/maxHeight
	img = NewRGBAImage(image.Rect(0, 0, 10, 10))
	preview = GeneratePreview(img, 0, 0)
	if preview != nil {
		t.Error("GeneratePreview with zero max dimensions should return nil")
	}
}
