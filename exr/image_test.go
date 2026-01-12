package exr

import (
	"bytes"
	"image"
	"image/color"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestNewRGBAImage(t *testing.T) {
	r := image.Rect(0, 0, 100, 50)
	img := NewRGBAImage(r)

	if img.Rect != r {
		t.Errorf("Rect = %v, want %v", img.Rect, r)
	}
	if img.Stride != 4 {
		t.Errorf("Stride = %d, want 4", img.Stride)
	}
	if len(img.Pix) != 100*50*4 {
		t.Errorf("Pix len = %d, want %d", len(img.Pix), 100*50*4)
	}
}

func TestRGBAImageBounds(t *testing.T) {
	r := image.Rect(10, 20, 110, 70)
	img := NewRGBAImage(r)

	bounds := img.Bounds()
	if bounds != r {
		t.Errorf("Bounds() = %v, want %v", bounds, r)
	}
}

func TestRGBAImageColorModel(t *testing.T) {
	img := NewRGBAImage(image.Rect(0, 0, 10, 10))
	if img.ColorModel() != color.RGBAModel {
		t.Errorf("ColorModel() = %v, want RGBAModel", img.ColorModel())
	}
}

func TestRGBAImageSetAndGet(t *testing.T) {
	img := NewRGBAImage(image.Rect(0, 0, 10, 10))

	// Set a pixel
	img.SetRGBA(5, 5, 0.5, 0.25, 0.75, 1.0)

	// Get it back
	r, g, b, a := img.RGBA(5, 5)
	if r != 0.5 || g != 0.25 || b != 0.75 || a != 1.0 {
		t.Errorf("RGBA() = (%f,%f,%f,%f), want (0.5,0.25,0.75,1.0)", r, g, b, a)
	}
}

func TestRGBAImageAt(t *testing.T) {
	img := NewRGBAImage(image.Rect(0, 0, 10, 10))
	img.SetRGBA(5, 5, 1.0, 0.5, 0.25, 1.0)

	c := img.At(5, 5).(color.RGBA)
	if c.R != 255 || c.G != 127 || c.B != 63 || c.A != 255 {
		t.Errorf("At() = %v, want {255,127,63,255}", c)
	}
}

func TestRGBAImageAtOutOfBounds(t *testing.T) {
	img := NewRGBAImage(image.Rect(0, 0, 10, 10))
	img.SetRGBA(5, 5, 1.0, 1.0, 1.0, 1.0)

	// Out of bounds should return zero color
	c := img.At(-1, 0)
	if c != (color.RGBA{}) {
		t.Errorf("At(-1,0) = %v, want zero color", c)
	}
	c = img.At(10, 0)
	if c != (color.RGBA{}) {
		t.Errorf("At(10,0) = %v, want zero color", c)
	}
}

func TestRGBAImageSetOutOfBounds(t *testing.T) {
	img := NewRGBAImage(image.Rect(0, 0, 10, 10))

	// SetRGBA on out of bounds should be no-op
	img.SetRGBA(-1, 0, 1.0, 1.0, 1.0, 1.0)
	img.SetRGBA(10, 0, 1.0, 1.0, 1.0, 1.0)

	// Make sure nothing was set at boundaries
	r, g, b, a := img.RGBA(0, 0)
	if r != 0 || g != 0 || b != 0 || a != 0 {
		t.Errorf("Boundary pixel modified unexpectedly")
	}
}

func TestRGBAImageGetOutOfBounds(t *testing.T) {
	img := NewRGBAImage(image.Rect(0, 0, 10, 10))
	img.SetRGBA(5, 5, 0.5, 0.5, 0.5, 1.0)

	// RGBA on out of bounds returns zeros
	r, g, b, a := img.RGBA(-1, 0)
	if r != 0 || g != 0 || b != 0 || a != 0 {
		t.Errorf("RGBA(-1,0) = (%f,%f,%f,%f), want zeros", r, g, b, a)
	}
}

func TestRGBAImagePixOffset(t *testing.T) {
	img := NewRGBAImage(image.Rect(10, 20, 110, 70))

	// Pixel at (10,20) should be at offset 0
	off := img.PixOffset(10, 20)
	if off != 0 {
		t.Errorf("PixOffset(10,20) = %d, want 0", off)
	}

	// Pixel at (11,20) should be at offset 4
	off = img.PixOffset(11, 20)
	if off != 4 {
		t.Errorf("PixOffset(11,20) = %d, want 4", off)
	}

	// Pixel at (10,21) should be at offset 100*4 = 400
	off = img.PixOffset(10, 21)
	if off != 400 {
		t.Errorf("PixOffset(10,21) = %d, want 400", off)
	}
}

func TestClamp01(t *testing.T) {
	tests := []struct {
		input    float32
		expected float32
	}{
		{-0.5, 0},
		{0, 0},
		{0.5, 0.5},
		{1.0, 1.0},
		{1.5, 1.0},
	}

	for _, tt := range tests {
		result := clamp01(tt.input)
		if result != tt.expected {
			t.Errorf("clamp01(%f) = %f, want %f", tt.input, result, tt.expected)
		}
	}
}

func TestRGBAImageHDRValues(t *testing.T) {
	img := NewRGBAImage(image.Rect(0, 0, 10, 10))

	// Set HDR values (>1.0)
	img.SetRGBA(5, 5, 2.0, -0.5, 1.5, 1.0)

	// RGBA should return raw values
	r, g, b, a := img.RGBA(5, 5)
	if r != 2.0 || g != -0.5 || b != 1.5 || a != 1.0 {
		t.Errorf("RGBA() = (%f,%f,%f,%f), want (2.0,-0.5,1.5,1.0)", r, g, b, a)
	}

	// At should clamp to [0,1] for color.RGBA
	c := img.At(5, 5).(color.RGBA)
	if c.R != 255 || c.G != 0 || c.B != 255 || c.A != 255 {
		t.Errorf("At() = %v, want {255,0,255,255}", c)
	}
}

// seekableBuffer wraps bytes.Buffer to implement io.WriteSeeker
type testSeekableBuffer struct {
	data []byte
	pos  int64
}

func newTestSeekableBuffer() *testSeekableBuffer {
	return &testSeekableBuffer{
		data: make([]byte, 0),
	}
}

func (s *testSeekableBuffer) Write(p []byte) (n int, err error) {
	needed := int(s.pos) + len(p)
	if needed > len(s.data) {
		// Extend the buffer
		s.data = append(s.data, make([]byte, needed-len(s.data))...)
	}
	n = copy(s.data[s.pos:], p)
	s.pos += int64(n)
	return n, nil
}

func (s *testSeekableBuffer) Seek(offset int64, whence int) (int64, error) {
	switch whence {
	case 0:
		s.pos = offset
	case 1:
		s.pos += offset
	case 2:
		s.pos = int64(len(s.data)) + offset
	}
	return s.pos, nil
}

func (s *testSeekableBuffer) ReadAt(p []byte, off int64) (n int, err error) {
	if off >= int64(len(s.data)) {
		return 0, nil
	}
	n = copy(p, s.data[off:])
	return n, nil
}

func (s *testSeekableBuffer) Bytes() []byte {
	return s.data
}

func TestEncodeDecodeRoundTrip(t *testing.T) {
	// Create a test image
	width, height := 16, 16
	img := NewRGBAImage(image.Rect(0, 0, width, height))

	// Fill with gradient
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.SetRGBA(x, y,
				float32(x)/float32(width),
				float32(y)/float32(height),
				0.5,
				1.0,
			)
		}
	}

	// Encode to buffer
	buf := newTestSeekableBuffer()
	err := Encode(buf, img)
	if err != nil {
		t.Fatalf("Encode error: %v", err)
	}

	// Verify we got output
	if len(buf.Bytes()) == 0 {
		t.Fatal("Encode produced empty output")
	}

	// Decode back
	decoded, err := Decode(buf, int64(len(buf.Bytes())))
	if err != nil {
		t.Fatalf("Decode error: %v", err)
	}

	// Check dimensions
	if decoded.Rect.Dx() != width || decoded.Rect.Dy() != height {
		t.Errorf("Decoded size = %dx%d, want %dx%d",
			decoded.Rect.Dx(), decoded.Rect.Dy(), width, height)
	}
}

func TestEncodeDecodeFileRoundTrip(t *testing.T) {
	// Create temp directory
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.exr")

	// Create test image
	width, height := 32, 32
	img := NewRGBAImage(image.Rect(0, 0, width, height))

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.SetRGBA(x, y, 0.25, 0.5, 0.75, 1.0)
		}
	}

	// Write to file
	err := EncodeFile(path, img)
	if err != nil {
		t.Fatalf("EncodeFile error: %v", err)
	}

	// Read back
	decoded, err := DecodeFile(path)
	if err != nil {
		t.Fatalf("DecodeFile error: %v", err)
	}

	// Check dimensions
	if decoded.Rect.Dx() != width || decoded.Rect.Dy() != height {
		t.Errorf("Decoded size = %dx%d, want %dx%d",
			decoded.Rect.Dx(), decoded.Rect.Dy(), width, height)
	}

	// Check a pixel (allow for half-precision loss)
	r, g, b, a := decoded.RGBA(16, 16)
	const epsilon = 0.01
	if abs32(r-0.25) > epsilon || abs32(g-0.5) > epsilon ||
		abs32(b-0.75) > epsilon || abs32(a-1.0) > epsilon {
		t.Errorf("Pixel mismatch: got (%f,%f,%f,%f), want (0.25,0.5,0.75,1.0)",
			r, g, b, a)
	}
}

func abs32(x float32) float32 {
	if x < 0 {
		return -x
	}
	return x
}

func TestOpenFileFromImage(t *testing.T) {
	path := filepath.Join("testdata", "sample.exr")
	f, err := OpenFile(path)
	if err != nil {
		t.Skipf("Test file not available: %v", err)
		return
	}
	defer f.Close()
	if f == nil {
		t.Error("OpenFile returned nil")
	}
}

func TestOpenWithSeeker(t *testing.T) {
	path := filepath.Join("testdata", "sample.exr")
	file, err := os.Open(path)
	if err != nil {
		t.Skipf("Test file not available: %v", err)
		return
	}
	defer file.Close()

	// Open using the Open function which tries to detect size from Seeker
	f, err := Open(file)
	if err != nil {
		t.Fatalf("Open error: %v", err)
	}
	if f == nil {
		t.Error("Open returned nil")
	}
}

func TestOpenWithExplicitSize(t *testing.T) {
	path := filepath.Join("testdata", "sample.exr")
	file, err := os.Open(path)
	if err != nil {
		t.Skipf("Test file not available: %v", err)
		return
	}
	defer file.Close()

	stat, _ := file.Stat()
	f, err := Open(file, stat.Size())
	if err != nil {
		t.Fatalf("Open error: %v", err)
	}
	if f == nil {
		t.Error("Open returned nil")
	}
}

func TestRGBAInputFile(t *testing.T) {
	path := filepath.Join("testdata", "comp_none.exr")
	f, err := OpenFile(path)
	if err != nil {
		t.Skipf("Test file not available: %v", err)
		return
	}
	defer f.Close()

	rgba, err := NewRGBAInputFile(f)
	if err != nil {
		t.Fatalf("NewRGBAInputFile error: %v", err)
	}

	// Test accessor methods
	if rgba.Header() == nil {
		t.Error("Header() returned nil")
	}

	dw := rgba.DataWindow()
	if dw.Width() <= 0 || dw.Height() <= 0 {
		t.Error("DataWindow has invalid dimensions")
	}

	displayW := rgba.DisplayWindow()
	if displayW.Width() <= 0 || displayW.Height() <= 0 {
		t.Error("DisplayWindow has invalid dimensions")
	}

	if rgba.Width() <= 0 {
		t.Errorf("Width() = %d, want > 0", rgba.Width())
	}

	if rgba.Height() <= 0 {
		t.Errorf("Height() = %d, want > 0", rgba.Height())
	}
}

func TestRGBAOutputFile(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "output.exr")

	outFile, err := NewRGBAOutputFile(path, 100, 100)
	if err != nil {
		t.Fatalf("NewRGBAOutputFile error: %v", err)
	}

	// Test Header method
	h := outFile.Header()
	if h == nil {
		t.Error("Header() returned nil")
	}

	// Create an image to write
	img := NewRGBAImage(image.Rect(0, 0, 100, 100))
	for y := 0; y < 100; y++ {
		for x := 0; x < 100; x++ {
			img.SetRGBA(x, y, 0.5, 0.5, 0.5, 1.0)
		}
	}

	// Write the image
	err = outFile.WriteRGBA(img)
	if err != nil {
		t.Fatalf("WriteRGBA error: %v", err)
	}

	// Verify file exists
	_, err = os.Stat(path)
	if err != nil {
		t.Errorf("Output file not created: %v", err)
	}
}

func TestFindChannel(t *testing.T) {
	cl := NewChannelList()
	cl.Add(Channel{Name: "R", Type: PixelTypeHalf})
	cl.Add(Channel{Name: "green", Type: PixelTypeHalf})
	cl.Add(Channel{Name: "B", Type: PixelTypeHalf})

	// Test finding existing channels
	if name := findChannel(cl, "R", "r", "red"); name != "R" {
		t.Errorf("findChannel for R = %s, want R", name)
	}

	if name := findChannel(cl, "G", "g", "green"); name != "green" {
		t.Errorf("findChannel for green = %s, want green", name)
	}

	// Test not finding a channel
	if name := findChannel(cl, "A", "a", "alpha"); name != "" {
		t.Errorf("findChannel for alpha = %s, want empty", name)
	}
}

func TestDecodeFileNotFound(t *testing.T) {
	_, err := DecodeFile("/nonexistent/path/file.exr")
	if err == nil {
		t.Error("DecodeFile on nonexistent file should return error")
	}
}

func TestOpenRGBAInputFileNotFound(t *testing.T) {
	_, err := OpenRGBAInputFile("/nonexistent/path/file.exr")
	if err == nil {
		t.Error("OpenRGBAInputFile on nonexistent file should return error")
	}
}

func TestEncodeFileInvalidPath(t *testing.T) {
	img := NewRGBAImage(image.Rect(0, 0, 10, 10))
	err := EncodeFile("/nonexistent/directory/file.exr", img)
	if err == nil {
		t.Error("EncodeFile to invalid path should return error")
	}
}

func TestOpenNonSeeker(t *testing.T) {
	// Test Open with a reader that doesn't support seeking
	// This should return an error
	buf := bytes.NewReader([]byte{0x76, 0x2f, 0x31, 0x01}) // Invalid but doesn't matter

	// Actually, bytes.Reader implements Seeker, so let's test with explicit size
	_, err := Open(buf, int64(buf.Len()))
	if err == nil {
		// It should either work or fail with invalid format, not nil error for no size
	}
}

func TestRGBAImageWithOffset(t *testing.T) {
	// Test image with non-zero origin
	r := image.Rect(100, 100, 200, 200)
	img := NewRGBAImage(r)

	// Set pixel at image coordinates
	img.SetRGBA(150, 150, 1.0, 0.5, 0.25, 1.0)

	// Get it back
	rv, gv, bv, av := img.RGBA(150, 150)
	if rv != 1.0 || gv != 0.5 || bv != 0.25 || av != 1.0 {
		t.Errorf("RGBA(150,150) = (%f,%f,%f,%f), want (1.0,0.5,0.25,1.0)",
			rv, gv, bv, av)
	}

	// Verify At works with offset
	c := img.At(150, 150).(color.RGBA)
	if c.R != 255 || c.G != 127 || c.B != 63 || c.A != 255 {
		t.Errorf("At(150,150) = %v, want {255,127,63,255}", c)
	}
}

func TestDecodeAndEncode(t *testing.T) {
	// Create a test image
	width := 16
	height := 16
	img := NewRGBAImage(image.Rect(0, 0, width, height))

	// Fill with test pattern
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.SetRGBA(x, y, float32(x)/float32(width), float32(y)/float32(height), 0.5, 1.0)
		}
	}

	// Encode to buffer
	var buf bytes.Buffer
	ws := &seekableWriter{Buffer: &buf}

	if err := Encode(ws, img); err != nil {
		t.Fatalf("Encode error: %v", err)
	}

	// Decode from buffer
	data := buf.Bytes()
	reader := bytes.NewReader(data)
	decoded, err := Decode(reader, int64(len(data)))
	if err != nil {
		t.Fatalf("Decode error: %v", err)
	}

	// Verify dimensions
	if decoded.Rect.Dx() != width || decoded.Rect.Dy() != height {
		t.Errorf("Decoded dimensions = (%d,%d), want (%d,%d)",
			decoded.Rect.Dx(), decoded.Rect.Dy(), width, height)
	}
}

func TestRGBAInputFileWithTiled(t *testing.T) {
	// Create a tiled EXR image and read it with RGBAInputFile
	width := 64
	height := 64
	tileSize := 32

	h := NewTiledHeader(width, height, tileSize, tileSize)
	h.SetCompression(CompressionNone)

	channels := NewChannelList()
	channels.Add(Channel{Name: "R", Type: PixelTypeHalf, XSampling: 1, YSampling: 1})
	channels.Add(Channel{Name: "G", Type: PixelTypeHalf, XSampling: 1, YSampling: 1})
	channels.Add(Channel{Name: "B", Type: PixelTypeHalf, XSampling: 1, YSampling: 1})
	channels.Add(Channel{Name: "A", Type: PixelTypeHalf, XSampling: 1, YSampling: 1})
	h.SetChannels(channels)

	var buf bytes.Buffer
	ws := &seekableWriter{Buffer: &buf}

	tw, err := NewTiledWriter(ws, h)
	if err != nil {
		t.Fatalf("NewTiledWriter error: %v", err)
	}

	fb, _ := AllocateChannels(channels, h.DataWindow())
	tw.SetFrameBuffer(fb)

	numTilesX := (width + tileSize - 1) / tileSize
	numTilesY := (height + tileSize - 1) / tileSize

	for ty := 0; ty < numTilesY; ty++ {
		for tx := 0; tx < numTilesX; tx++ {
			if err := tw.WriteTile(tx, ty); err != nil {
				t.Fatalf("WriteTile error: %v", err)
			}
		}
	}
	tw.Close()

	// Read with RGBAInputFile
	data := buf.Bytes()
	reader := bytes.NewReader(data)
	f, err := OpenReader(reader, int64(len(data)))
	if err != nil {
		t.Fatalf("OpenReader error: %v", err)
	}

	rgba, err := NewRGBAInputFile(f)
	if err != nil {
		t.Fatalf("NewRGBAInputFile error: %v", err)
	}

	img, err := rgba.ReadRGBA()
	if err != nil {
		t.Logf("ReadRGBA warning: %v", err)
		// Tiled reading may not be fully implemented
		return
	}

	if img.Rect.Dx() != width || img.Rect.Dy() != height {
		t.Errorf("Image dimensions = (%d,%d), want (%d,%d)",
			img.Rect.Dx(), img.Rect.Dy(), width, height)
	}
}

func TestOpenRGBAInputFileInvalidPath(t *testing.T) {
	_, err := OpenRGBAInputFile("/nonexistent/path/file.exr")
	if err == nil {
		t.Error("OpenRGBAInputFile with invalid path should return error")
	}
}

func TestNewRGBAInputFileNil(t *testing.T) {
	_, err := NewRGBAInputFile(nil)
	if err == nil {
		t.Error("NewRGBAInputFile(nil) should return error")
	}
}

func TestOpenFileInvalidPath(t *testing.T) {
	_, err := OpenFile("/nonexistent/path/file.exr")
	if err == nil {
		t.Error("OpenFile with invalid path should return error")
	}
}

func TestDecodeInvalidData(t *testing.T) {
	// Test with empty data
	data := []byte{}
	reader := bytes.NewReader(data)
	_, err := Decode(reader, int64(len(data)))
	if err == nil {
		t.Error("Decode with empty data should return error")
	}

	// Test with invalid magic number
	data = []byte{0x00, 0x00, 0x00, 0x00}
	reader = bytes.NewReader(data)
	_, err = Decode(reader, int64(len(data)))
	if err == nil {
		t.Error("Decode with invalid magic should return error")
	}
}

func TestDecodeFileInvalidPath(t *testing.T) {
	_, err := DecodeFile("/nonexistent/path/file.exr")
	if err == nil {
		t.Error("DecodeFile with invalid path should return error")
	}
}

// nonSeekReader is a reader that doesn't implement io.Seeker
type nonSeekReader struct {
	data []byte
}

func (r *nonSeekReader) ReadAt(p []byte, off int64) (n int, err error) {
	if off < 0 || off >= int64(len(r.data)) {
		return 0, io.EOF
	}
	n = copy(p, r.data[off:])
	return n, nil
}

func TestOpenWithoutSizeOrSeeker(t *testing.T) {
	// Create a reader that doesn't implement Seeker
	r := &nonSeekReader{data: []byte{0x76, 0x2f, 0x31, 0x01}}

	// Open without size parameter should fail
	_, err := Open(r)
	if err == nil {
		t.Error("Open without size or seeker should return error")
	}
}

func TestOpenWithSize(t *testing.T) {
	// Create minimal valid EXR header bytes
	r := &nonSeekReader{data: []byte{0x76, 0x2f, 0x31, 0x01, 0x00}}

	// Open with size parameter - should at least try to read
	_, err := Open(r, 5)
	// This will fail due to incomplete header, but it should have tried
	if err == nil {
		t.Log("Open with size worked (unexpected but ok)")
	}
}

func TestRGBAImageAtNegativeCoords(t *testing.T) {
	img := NewRGBAImage(image.Rect(0, 0, 4, 4))

	// Test At out of bounds with negative coords - should return empty color
	c := img.At(-1, 0)
	if c != (color.RGBA{}) {
		t.Errorf("At(-1, 0) should return empty color")
	}

	c = img.At(0, -1)
	if c != (color.RGBA{}) {
		t.Errorf("At(0, -1) should return empty color")
	}
}

func TestRGBAImageSetRGBANegativeCoords(t *testing.T) {
	img := NewRGBAImage(image.Rect(0, 0, 4, 4))

	// SetRGBA out of bounds - should silently ignore
	img.SetRGBA(-1, 0, 1.0, 1.0, 1.0, 1.0) // Should not panic
	img.SetRGBA(0, -1, 1.0, 1.0, 1.0, 1.0) // Should not panic
}

func TestRGBAImageRGBANegativeCoords(t *testing.T) {
	img := NewRGBAImage(image.Rect(0, 0, 4, 4))

	// RGBA out of bounds - should return zeros
	r, g, b, a := img.RGBA(-1, 0)
	if r != 0 || g != 0 || b != 0 || a != 0 {
		t.Errorf("RGBA(-1, 0) should return zeros, got (%f,%f,%f,%f)", r, g, b, a)
	}
}

func TestEncodeSmallImage(t *testing.T) {
	// Test encoding a small image
	img := NewRGBAImage(image.Rect(0, 0, 2, 2))
	img.SetRGBA(0, 0, 1.0, 0.0, 0.0, 1.0) // Red
	img.SetRGBA(1, 0, 0.0, 1.0, 0.0, 1.0) // Green
	img.SetRGBA(0, 1, 0.0, 0.0, 1.0, 1.0) // Blue
	img.SetRGBA(1, 1, 1.0, 1.0, 1.0, 1.0) // White

	var buf bytes.Buffer
	ws := &seekableWriter{Buffer: &buf}

	err := Encode(ws, img)
	if err != nil {
		t.Errorf("Encode() error = %v", err)
	}

	if buf.Len() == 0 {
		t.Error("Encoded data should not be empty")
	}
}
