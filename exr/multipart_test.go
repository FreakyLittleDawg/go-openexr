package exr

import (
	"bytes"
	"testing"
)

func TestMultiPartInputFile(t *testing.T) {
	// Create a test multi-part file in memory
	width := 32
	height := 16

	// Create two headers
	h1 := NewScanlineHeader(width, height)
	h1.Set(&Attribute{Name: AttrNameName, Type: AttrTypeString, Value: "part1"})
	h1.Set(&Attribute{Name: AttrNameType, Type: AttrTypeString, Value: PartTypeScanline})
	h1.SetCompression(CompressionNone) // 1 scanline per chunk

	h2 := NewScanlineHeader(width, height)
	h2.Set(&Attribute{Name: AttrNameName, Type: AttrTypeString, Value: "part2"})
	h2.Set(&Attribute{Name: AttrNameType, Type: AttrTypeString, Value: PartTypeScanline})
	h2.SetCompression(CompressionNone) // 1 scanline per chunk

	// Write multi-part file
	var buf bytes.Buffer
	ws := &seekableWriter{Buffer: &buf}

	w, err := NewMultiPartWriter(ws, []*Header{h1, h2})
	if err != nil {
		t.Fatalf("NewMultiPartWriter() error = %v", err)
	}

	// Write dummy data for each part
	for part := 0; part < 2; part++ {
		for y := 0; y < height; y++ {
			data := make([]byte, width*8) // 2 bytes per channel * 4 channels = 8 bytes per pixel
			if err := w.WriteChunkPart(part, int32(y), data); err != nil {
				t.Fatalf("WriteChunkPart(%d, %d) error = %v", part, y, err)
			}
		}
	}

	if err := w.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	// Read the file back
	data := buf.Bytes()
	reader := bytes.NewReader(data)
	f, err := OpenReader(reader, int64(len(data)))
	if err != nil {
		t.Fatalf("OpenReader() error = %v", err)
	}

	// Create MultiPartInputFile
	mpi := NewMultiPartInputFile(f)

	// Test NumParts
	if mpi.NumParts() != 2 {
		t.Errorf("NumParts() = %d, want 2", mpi.NumParts())
	}

	// Test IsMultiPart
	if !mpi.IsMultiPart() {
		t.Error("IsMultiPart() should be true")
	}

	// Test PartInfo
	info1, err := mpi.PartInfo(0)
	if err != nil {
		t.Fatalf("PartInfo(0) error = %v", err)
	}
	if info1.Name != "part1" {
		t.Errorf("PartInfo(0).Name = %q, want %q", info1.Name, "part1")
	}
	if info1.Type != PartTypeScanline {
		t.Errorf("PartInfo(0).Type = %q, want %q", info1.Type, PartTypeScanline)
	}

	info2, err := mpi.PartInfo(1)
	if err != nil {
		t.Fatalf("PartInfo(1) error = %v", err)
	}
	if info2.Name != "part2" {
		t.Errorf("PartInfo(1).Name = %q, want %q", info2.Name, "part2")
	}

	// Test FindPartByName
	if idx := mpi.FindPartByName("part1"); idx != 0 {
		t.Errorf("FindPartByName(part1) = %d, want 0", idx)
	}
	if idx := mpi.FindPartByName("part2"); idx != 1 {
		t.Errorf("FindPartByName(part2) = %d, want 1", idx)
	}
	if idx := mpi.FindPartByName("nonexistent"); idx != -1 {
		t.Errorf("FindPartByName(nonexistent) = %d, want -1", idx)
	}

	// Test ListParts
	parts := mpi.ListParts()
	if len(parts) != 2 {
		t.Errorf("ListParts() returned %d parts, want 2", len(parts))
	}
}

func TestMultiPartOutputFile(t *testing.T) {
	width := 16
	height := 8

	// Create headers
	h1 := NewScanlineHeader(width, height)
	h1.Set(&Attribute{Name: AttrNameName, Type: AttrTypeString, Value: "rgba"})
	h1.Set(&Attribute{Name: AttrNameType, Type: AttrTypeString, Value: PartTypeScanline})
	h1.SetCompression(CompressionNone)

	h2 := NewScanlineHeader(width, height)
	h2.Set(&Attribute{Name: AttrNameName, Type: AttrTypeString, Value: "depth"})
	h2.Set(&Attribute{Name: AttrNameType, Type: AttrTypeString, Value: PartTypeScanline})
	h2.SetCompression(CompressionNone)

	// Create frame buffers
	fb1 := NewFrameBuffer()
	fb1.Set("R", NewSlice(PixelTypeHalf, make([]byte, width*height*2), width, height))
	fb1.Set("G", NewSlice(PixelTypeHalf, make([]byte, width*height*2), width, height))
	fb1.Set("B", NewSlice(PixelTypeHalf, make([]byte, width*height*2), width, height))
	fb1.Set("A", NewSlice(PixelTypeHalf, make([]byte, width*height*2), width, height))

	fb2 := NewFrameBuffer()
	fb2.Set("Z", NewSlice(PixelTypeFloat, make([]byte, width*height*4), width, height))

	// Write multi-part file
	var buf bytes.Buffer
	ws := &seekableWriter{Buffer: &buf}

	mpo, err := NewMultiPartOutputFile(ws, []*Header{h1, h2})
	if err != nil {
		t.Fatalf("NewMultiPartOutputFile() error = %v", err)
	}

	if mpo.NumParts() != 2 {
		t.Errorf("NumParts() = %d, want 2", mpo.NumParts())
	}

	// Set frame buffers
	if err := mpo.SetFrameBuffer(0, fb1); err != nil {
		t.Fatalf("SetFrameBuffer(0) error = %v", err)
	}
	if err := mpo.SetFrameBuffer(1, fb2); err != nil {
		t.Fatalf("SetFrameBuffer(1) error = %v", err)
	}

	// Write pixels
	if err := mpo.WritePixels(0, height); err != nil {
		t.Fatalf("WritePixels(0) error = %v", err)
	}
	if err := mpo.WritePixels(1, height); err != nil {
		t.Fatalf("WritePixels(1) error = %v", err)
	}

	if err := mpo.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	// Verify file was written
	if buf.Len() < 100 {
		t.Errorf("File too small: %d bytes", buf.Len())
	}

	// Verify magic number
	data := buf.Bytes()
	if data[0] != 0x76 || data[1] != 0x2f || data[2] != 0x31 || data[3] != 0x01 {
		t.Errorf("Invalid magic number: %x", data[0:4])
	}
}

func TestPartInfoInvalidPart(t *testing.T) {
	// Create a single-part file
	width := 16
	height := 8

	h := NewScanlineHeader(width, height)
	h.SetCompression(CompressionNone) // 1 scanline per chunk

	var buf bytes.Buffer
	ws := &seekableWriter{Buffer: &buf}

	w, err := NewWriter(ws, h)
	if err != nil {
		t.Fatalf("NewWriter() error = %v", err)
	}

	for y := 0; y < height; y++ {
		if err := w.WriteChunk(int32(y), make([]byte, width*8)); err != nil {
			t.Fatalf("WriteChunk() error = %v", err)
		}
	}
	w.Close()

	// Read back
	data := buf.Bytes()
	reader := bytes.NewReader(data)
	f, err := OpenReader(reader, int64(len(data)))
	if err != nil {
		t.Fatalf("OpenReader() error = %v", err)
	}

	mpi := NewMultiPartInputFile(f)

	// Try to get invalid part
	_, err = mpi.PartInfo(5)
	if err != ErrPartNotFound {
		t.Errorf("PartInfo(5) error = %v, want ErrPartNotFound", err)
	}
}

// seekableWriter implements io.WriteSeeker for testing
type seekableWriter struct {
	Buffer *bytes.Buffer
	pos    int64
}

func (w *seekableWriter) Write(p []byte) (n int, err error) {
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

func (w *seekableWriter) Seek(offset int64, whence int) (int64, error) {
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

func TestMultiPartInputFileFile(t *testing.T) {
	// Create a test multi-part file in memory
	width := 16
	height := 8

	h1 := NewScanlineHeader(width, height)
	h1.Set(&Attribute{Name: AttrNameName, Type: AttrTypeString, Value: "part1"})
	h1.Set(&Attribute{Name: AttrNameType, Type: AttrTypeString, Value: PartTypeScanline})
	h1.SetCompression(CompressionNone)

	var buf bytes.Buffer
	ws := &seekableWriter{Buffer: &buf}

	w, err := NewMultiPartWriter(ws, []*Header{h1})
	if err != nil {
		t.Fatalf("NewMultiPartWriter() error = %v", err)
	}

	for y := 0; y < height; y++ {
		data := make([]byte, width*8)
		if err := w.WriteChunkPart(0, int32(y), data); err != nil {
			t.Fatalf("WriteChunkPart error = %v", err)
		}
	}
	w.Close()

	// Read back
	data := buf.Bytes()
	reader := bytes.NewReader(data)
	f, err := OpenReader(reader, int64(len(data)))
	if err != nil {
		t.Fatalf("OpenReader() error = %v", err)
	}

	mpi := NewMultiPartInputFile(f)

	// Test File() method
	file := mpi.File()
	if file == nil {
		t.Error("File() should not return nil")
	}
	if file != f {
		t.Error("File() should return the same file")
	}
}

func TestMultiPartInputFileHeader(t *testing.T) {
	// Create a test file
	width := 16
	height := 8

	h1 := NewScanlineHeader(width, height)
	h1.Set(&Attribute{Name: AttrNameName, Type: AttrTypeString, Value: "part1"})
	h1.Set(&Attribute{Name: AttrNameType, Type: AttrTypeString, Value: PartTypeScanline})
	h1.SetCompression(CompressionNone)

	var buf bytes.Buffer
	ws := &seekableWriter{Buffer: &buf}

	w, err := NewMultiPartWriter(ws, []*Header{h1})
	if err != nil {
		t.Fatalf("NewMultiPartWriter() error = %v", err)
	}

	for y := 0; y < height; y++ {
		if err := w.WriteChunkPart(0, int32(y), make([]byte, width*8)); err != nil {
			t.Fatalf("WriteChunkPart error = %v", err)
		}
	}
	w.Close()

	data := buf.Bytes()
	reader := bytes.NewReader(data)
	f, _ := OpenReader(reader, int64(len(data)))

	mpi := NewMultiPartInputFile(f)

	// Test Header() method
	header := mpi.Header(0)
	if header == nil {
		t.Error("Header() should not return nil")
	}

	// Invalid part
	header = mpi.Header(10)
	if header != nil {
		t.Error("Header(10) should return nil")
	}
}

func TestMultiPartInputFileScanlineReader(t *testing.T) {
	// Create a test file
	width := 16
	height := 8

	h1 := NewScanlineHeader(width, height)
	h1.Set(&Attribute{Name: AttrNameName, Type: AttrTypeString, Value: "part1"})
	h1.Set(&Attribute{Name: AttrNameType, Type: AttrTypeString, Value: PartTypeScanline})
	h1.SetCompression(CompressionNone)

	var buf bytes.Buffer
	ws := &seekableWriter{Buffer: &buf}

	w, err := NewMultiPartWriter(ws, []*Header{h1})
	if err != nil {
		t.Fatalf("NewMultiPartWriter() error = %v", err)
	}

	for y := 0; y < height; y++ {
		if err := w.WriteChunkPart(0, int32(y), make([]byte, width*8)); err != nil {
			t.Fatalf("WriteChunkPart error = %v", err)
		}
	}
	w.Close()

	data := buf.Bytes()
	reader := bytes.NewReader(data)
	f, _ := OpenReader(reader, int64(len(data)))

	mpi := NewMultiPartInputFile(f)

	// Test ScanlineReader() method
	sr, err := mpi.ScanlineReader(0)
	if err != nil {
		t.Fatalf("ScanlineReader(0) error = %v", err)
	}
	if sr == nil {
		t.Error("ScanlineReader() should not return nil")
	}

	// Invalid part
	_, err = mpi.ScanlineReader(10)
	if err == nil {
		t.Error("ScanlineReader(10) should return error")
	}
}

func TestMultiPartOutputFileHeader(t *testing.T) {
	width := 16
	height := 8

	h1 := NewScanlineHeader(width, height)
	h1.Set(&Attribute{Name: AttrNameName, Type: AttrTypeString, Value: "part1"})
	h1.Set(&Attribute{Name: AttrNameType, Type: AttrTypeString, Value: PartTypeScanline})
	h1.SetCompression(CompressionNone)

	var buf bytes.Buffer
	ws := &seekableWriter{Buffer: &buf}

	mpo, err := NewMultiPartOutputFile(ws, []*Header{h1})
	if err != nil {
		t.Fatalf("NewMultiPartOutputFile() error = %v", err)
	}

	// Test Header() method
	header := mpo.Header(0)
	if header == nil {
		t.Error("Header(0) should not return nil")
	}

	// Invalid part
	header = mpo.Header(10)
	if header != nil {
		t.Error("Header(10) should return nil")
	}

	mpo.Close()
}

func TestMultiPartInputFileTiledReader(t *testing.T) {
	// Create a tiled multi-part file
	width := 32
	height := 32

	h1 := NewTiledHeader(width, height, 16, 16)
	h1.Set(&Attribute{Name: AttrNameName, Type: AttrTypeString, Value: "tiled_part"})
	h1.Set(&Attribute{Name: AttrNameType, Type: AttrTypeString, Value: PartTypeTiled})
	h1.SetCompression(CompressionNone)

	var buf bytes.Buffer
	ws := &seekableWriter{Buffer: &buf}

	w, err := NewMultiPartWriter(ws, []*Header{h1})
	if err != nil {
		t.Fatalf("NewMultiPartWriter() error = %v", err)
	}

	// Write all tiles
	tileData := make([]byte, 16*16*8) // 16x16 tile, 8 bytes per pixel (4 channels x 2 bytes)
	if err := w.WriteTileChunkPart(0, 0, 0, 0, 0, tileData); err != nil {
		t.Fatalf("WriteTileChunkPart error = %v", err)
	}
	if err := w.WriteTileChunkPart(0, 1, 0, 0, 0, tileData); err != nil {
		t.Fatalf("WriteTileChunkPart error = %v", err)
	}
	if err := w.WriteTileChunkPart(0, 0, 1, 0, 0, tileData); err != nil {
		t.Fatalf("WriteTileChunkPart error = %v", err)
	}
	if err := w.WriteTileChunkPart(0, 1, 1, 0, 0, tileData); err != nil {
		t.Fatalf("WriteTileChunkPart error = %v", err)
	}
	w.Close()

	// Read back
	data := buf.Bytes()
	reader := bytes.NewReader(data)
	f, err := OpenReader(reader, int64(len(data)))
	if err != nil {
		t.Fatalf("OpenReader() error = %v", err)
	}

	mpi := NewMultiPartInputFile(f)

	// Test TiledReader() method
	tr, err := mpi.TiledReader(0)
	if err != nil {
		t.Fatalf("TiledReader(0) error = %v", err)
	}
	if tr == nil {
		t.Error("TiledReader() should not return nil")
	}

	// Invalid part
	_, err = mpi.TiledReader(10)
	if err != ErrPartNotFound {
		t.Errorf("TiledReader(10) error = %v, want ErrPartNotFound", err)
	}
}

func TestMultiPartInputFileTiledReaderOnScanline(t *testing.T) {
	// Create a scanline multi-part file
	width := 16
	height := 8

	h1 := NewScanlineHeader(width, height)
	h1.Set(&Attribute{Name: AttrNameName, Type: AttrTypeString, Value: "part1"})
	h1.Set(&Attribute{Name: AttrNameType, Type: AttrTypeString, Value: PartTypeScanline})
	h1.SetCompression(CompressionNone)

	var buf bytes.Buffer
	ws := &seekableWriter{Buffer: &buf}

	w, err := NewMultiPartWriter(ws, []*Header{h1})
	if err != nil {
		t.Fatalf("NewMultiPartWriter() error = %v", err)
	}

	for y := 0; y < height; y++ {
		if err := w.WriteChunkPart(0, int32(y), make([]byte, width*8)); err != nil {
			t.Fatalf("WriteChunkPart error = %v", err)
		}
	}
	w.Close()

	// Read back
	data := buf.Bytes()
	reader := bytes.NewReader(data)
	f, _ := OpenReader(reader, int64(len(data)))

	mpi := NewMultiPartInputFile(f)

	// Try to get TiledReader on scanline part - should fail
	_, err = mpi.TiledReader(0)
	if err != ErrInvalidPartType {
		t.Errorf("TiledReader on scanline part should return ErrInvalidPartType, got %v", err)
	}
}

func TestMultiPartInputFileDeepScanlineReader(t *testing.T) {
	// Create a simple scanline file and test that DeepScanlineReader fails correctly
	width := 16
	height := 8

	h1 := NewScanlineHeader(width, height)
	h1.Set(&Attribute{Name: AttrNameName, Type: AttrTypeString, Value: "part1"})
	h1.Set(&Attribute{Name: AttrNameType, Type: AttrTypeString, Value: PartTypeScanline})
	h1.SetCompression(CompressionNone)

	var buf bytes.Buffer
	ws := &seekableWriter{Buffer: &buf}

	w, err := NewMultiPartWriter(ws, []*Header{h1})
	if err != nil {
		t.Fatalf("NewMultiPartWriter() error = %v", err)
	}

	for y := 0; y < height; y++ {
		if err := w.WriteChunkPart(0, int32(y), make([]byte, width*8)); err != nil {
			t.Fatalf("WriteChunkPart error = %v", err)
		}
	}
	w.Close()

	// Read back
	data := buf.Bytes()
	reader := bytes.NewReader(data)
	f, _ := OpenReader(reader, int64(len(data)))

	mpi := NewMultiPartInputFile(f)

	// Try to get DeepScanlineReader on regular scanline part - should fail
	_, err = mpi.DeepScanlineReader(0)
	if err != ErrInvalidPartType {
		t.Errorf("DeepScanlineReader on regular scanline should return ErrInvalidPartType, got %v", err)
	}

	// Invalid part
	_, err = mpi.DeepScanlineReader(10)
	if err != ErrPartNotFound {
		t.Errorf("DeepScanlineReader(10) error = %v, want ErrPartNotFound", err)
	}
}

func TestMultiPartInputFileDeepTiledReader(t *testing.T) {
	// Create a regular tiled file and test that DeepTiledReader fails correctly
	width := 32
	height := 32

	h1 := NewTiledHeader(width, height, 16, 16)
	h1.Set(&Attribute{Name: AttrNameName, Type: AttrTypeString, Value: "tiled_part"})
	h1.Set(&Attribute{Name: AttrNameType, Type: AttrTypeString, Value: PartTypeTiled})
	h1.SetCompression(CompressionNone)

	var buf bytes.Buffer
	ws := &seekableWriter{Buffer: &buf}

	w, err := NewMultiPartWriter(ws, []*Header{h1})
	if err != nil {
		t.Fatalf("NewMultiPartWriter() error = %v", err)
	}

	tileData := make([]byte, 16*16*8)
	w.WriteTileChunkPart(0, 0, 0, 0, 0, tileData)
	w.WriteTileChunkPart(0, 1, 0, 0, 0, tileData)
	w.WriteTileChunkPart(0, 0, 1, 0, 0, tileData)
	w.WriteTileChunkPart(0, 1, 1, 0, 0, tileData)
	w.Close()

	// Read back
	data := buf.Bytes()
	reader := bytes.NewReader(data)
	f, _ := OpenReader(reader, int64(len(data)))

	mpi := NewMultiPartInputFile(f)

	// Try to get DeepTiledReader on regular tiled part - should fail
	_, err = mpi.DeepTiledReader(0)
	if err != ErrInvalidPartType {
		t.Errorf("DeepTiledReader on regular tiled should return ErrInvalidPartType, got %v", err)
	}

	// Invalid part
	_, err = mpi.DeepTiledReader(10)
	if err != ErrPartNotFound {
		t.Errorf("DeepTiledReader(10) error = %v, want ErrPartNotFound", err)
	}
}

func TestMultiPartOutputFileWriteTile(t *testing.T) {
	// Create a tiled multi-part file
	width := 32
	height := 32

	h1 := NewTiledHeader(width, height, 16, 16)
	h1.Set(&Attribute{Name: AttrNameName, Type: AttrTypeString, Value: "tiled_part"})
	h1.Set(&Attribute{Name: AttrNameType, Type: AttrTypeString, Value: PartTypeTiled})
	h1.SetCompression(CompressionNone)

	var buf bytes.Buffer
	ws := &seekableWriter{Buffer: &buf}

	mpo, err := NewMultiPartOutputFile(ws, []*Header{h1})
	if err != nil {
		t.Fatalf("NewMultiPartOutputFile() error = %v", err)
	}

	// Create frame buffer for the part
	fb := NewFrameBuffer()
	fb.Set("R", NewSlice(PixelTypeHalf, make([]byte, width*height*2), width, height))
	fb.Set("G", NewSlice(PixelTypeHalf, make([]byte, width*height*2), width, height))
	fb.Set("B", NewSlice(PixelTypeHalf, make([]byte, width*height*2), width, height))
	fb.Set("A", NewSlice(PixelTypeHalf, make([]byte, width*height*2), width, height))

	if err := mpo.SetFrameBuffer(0, fb); err != nil {
		t.Fatalf("SetFrameBuffer error = %v", err)
	}

	// Write tiles using WriteTile
	if err := mpo.WriteTile(0, 0, 0); err != nil {
		t.Logf("WriteTile(0, 0, 0) warning: %v", err)
	}
	if err := mpo.WriteTile(0, 1, 0); err != nil {
		t.Logf("WriteTile(0, 1, 0) warning: %v", err)
	}
	if err := mpo.WriteTile(0, 0, 1); err != nil {
		t.Logf("WriteTile(0, 0, 1) warning: %v", err)
	}
	if err := mpo.WriteTile(0, 1, 1); err != nil {
		t.Logf("WriteTile(0, 1, 1) warning: %v", err)
	}

	mpo.Close()

	// Verify file was written
	if buf.Len() < 100 {
		t.Errorf("File too small: %d bytes", buf.Len())
	}
}

func TestMultiPartOutputFileWriteTileLevel(t *testing.T) {
	// Create a tiled multi-part file
	width := 32
	height := 32

	h1 := NewTiledHeader(width, height, 16, 16)
	h1.Set(&Attribute{Name: AttrNameName, Type: AttrTypeString, Value: "tiled_part"})
	h1.Set(&Attribute{Name: AttrNameType, Type: AttrTypeString, Value: PartTypeTiled})
	h1.SetCompression(CompressionNone)

	var buf bytes.Buffer
	ws := &seekableWriter{Buffer: &buf}

	mpo, err := NewMultiPartOutputFile(ws, []*Header{h1})
	if err != nil {
		t.Fatalf("NewMultiPartOutputFile() error = %v", err)
	}

	// Create frame buffer for the part
	fb := NewFrameBuffer()
	fb.Set("R", NewSlice(PixelTypeHalf, make([]byte, width*height*2), width, height))
	fb.Set("G", NewSlice(PixelTypeHalf, make([]byte, width*height*2), width, height))
	fb.Set("B", NewSlice(PixelTypeHalf, make([]byte, width*height*2), width, height))
	fb.Set("A", NewSlice(PixelTypeHalf, make([]byte, width*height*2), width, height))

	if err := mpo.SetFrameBuffer(0, fb); err != nil {
		t.Fatalf("SetFrameBuffer error = %v", err)
	}

	// Write tiles using WriteTileLevel
	if err := mpo.WriteTileLevel(0, 0, 0, 0, 0); err != nil {
		t.Logf("WriteTileLevel(0, 0, 0, 0, 0) warning: %v", err)
	}
	if err := mpo.WriteTileLevel(0, 1, 0, 0, 0); err != nil {
		t.Logf("WriteTileLevel(0, 1, 0, 0, 0) warning: %v", err)
	}

	// Test error cases
	err = mpo.WriteTile(-1, 0, 0)
	if err != ErrPartNotFound {
		t.Errorf("WriteTile on invalid part should return ErrPartNotFound, got %v", err)
	}

	err = mpo.WriteTileLevel(10, 0, 0, 0, 0)
	if err != ErrPartNotFound {
		t.Errorf("WriteTileLevel on invalid part should return ErrPartNotFound, got %v", err)
	}

	mpo.Close()
}

func TestMultiPartOutputFileWriteTileOnScanline(t *testing.T) {
	// Create a scanline file and try to write tiles (should fail)
	width := 16
	height := 8

	h1 := NewScanlineHeader(width, height)
	h1.Set(&Attribute{Name: AttrNameName, Type: AttrTypeString, Value: "scanline_part"})
	h1.Set(&Attribute{Name: AttrNameType, Type: AttrTypeString, Value: PartTypeScanline})
	h1.SetCompression(CompressionNone)

	var buf bytes.Buffer
	ws := &seekableWriter{Buffer: &buf}

	mpo, err := NewMultiPartOutputFile(ws, []*Header{h1})
	if err != nil {
		t.Fatalf("NewMultiPartOutputFile() error = %v", err)
	}

	// Create frame buffer for the part
	fb := NewFrameBuffer()
	fb.Set("R", NewSlice(PixelTypeHalf, make([]byte, width*height*2), width, height))
	fb.Set("G", NewSlice(PixelTypeHalf, make([]byte, width*height*2), width, height))
	fb.Set("B", NewSlice(PixelTypeHalf, make([]byte, width*height*2), width, height))
	fb.Set("A", NewSlice(PixelTypeHalf, make([]byte, width*height*2), width, height))

	if err := mpo.SetFrameBuffer(0, fb); err != nil {
		t.Fatalf("SetFrameBuffer error = %v", err)
	}

	// Try to write tile on scanline part - should fail
	err = mpo.WriteTile(0, 0, 0)
	if err != ErrInvalidPartType {
		t.Errorf("WriteTile on scanline part should return ErrInvalidPartType, got %v", err)
	}

	mpo.Close()
}

func TestMultiPartOutputFileWithCompression(t *testing.T) {
	compressionTypes := []struct {
		name string
		comp Compression
	}{
		{"RLE", CompressionRLE},
		{"ZIPS", CompressionZIPS},
		{"ZIP", CompressionZIP},
		{"PIZ", CompressionPIZ},
		{"PXR24", CompressionPXR24},
		{"B44", CompressionB44},
		{"B44A", CompressionB44A},
		{"DWAA", CompressionDWAA},
		{"DWAB", CompressionDWAB},
	}

	for _, ct := range compressionTypes {
		t.Run(ct.name, func(t *testing.T) {
			width := 32
			height := 32

			h1 := NewScanlineHeader(width, height)
			h1.Set(&Attribute{Name: AttrNameName, Type: AttrTypeString, Value: "part1"})
			h1.Set(&Attribute{Name: AttrNameType, Type: AttrTypeString, Value: PartTypeScanline})
			h1.SetCompression(ct.comp)

			var buf bytes.Buffer
			ws := &seekableWriter{Buffer: &buf}

			mpo, err := NewMultiPartOutputFile(ws, []*Header{h1})
			if err != nil {
				t.Fatalf("NewMultiPartOutputFile() error = %v", err)
			}

			// Create frame buffer for the part with test data
			fb := NewFrameBuffer()
			rData := make([]byte, width*height*2)
			gData := make([]byte, width*height*2)
			bData := make([]byte, width*height*2)
			aData := make([]byte, width*height*2)

			// Fill with gradient data
			for y := 0; y < height; y++ {
				for x := 0; x < width; x++ {
					idx := (y*width + x) * 2
					// Use half float format
					rData[idx] = byte(x)
					gData[idx] = byte(y)
					bData[idx] = byte((x + y) % 256)
					aData[idx] = 0x3c // half(1.0) = 0x3c00
					aData[idx+1] = 0x00
				}
			}

			fb.Set("R", NewSlice(PixelTypeHalf, rData, width, height))
			fb.Set("G", NewSlice(PixelTypeHalf, gData, width, height))
			fb.Set("B", NewSlice(PixelTypeHalf, bData, width, height))
			fb.Set("A", NewSlice(PixelTypeHalf, aData, width, height))

			if err := mpo.SetFrameBuffer(0, fb); err != nil {
				t.Fatalf("SetFrameBuffer error = %v", err)
			}

			// Write all scanlines
			if err := mpo.WritePixels(0, height); err != nil {
				t.Fatalf("WritePixels error = %v", err)
			}

			mpo.Close()

			// Verify we wrote something
			if buf.Len() < 100 {
				t.Errorf("Output too small: %d bytes", buf.Len())
			}

			// Try to read the file back
			data := buf.Bytes()
			reader := bytes.NewReader(data)
			f, err := OpenReader(reader, int64(len(data)))
			if err != nil {
				t.Fatalf("OpenReader() error = %v", err)
			}

			if f.Header(0).Compression() != ct.comp {
				t.Errorf("Compression mismatch: got %v, want %v", f.Header(0).Compression(), ct.comp)
			}
		})
	}
}

func TestMultiPartTiledWithCompression(t *testing.T) {
	compressionTypes := []struct {
		name string
		comp Compression
	}{
		{"RLE", CompressionRLE},
		{"ZIP", CompressionZIP},
		{"PIZ", CompressionPIZ},
	}

	for _, ct := range compressionTypes {
		t.Run(ct.name, func(t *testing.T) {
			width := 64
			height := 64
			tileSize := 32

			h1 := NewTiledHeader(width, height, tileSize, tileSize)
			h1.Set(&Attribute{Name: AttrNameName, Type: AttrTypeString, Value: "part1"})
			h1.Set(&Attribute{Name: AttrNameType, Type: AttrTypeString, Value: PartTypeTiled})
			h1.SetCompression(ct.comp)

			var buf bytes.Buffer
			ws := &seekableWriter{Buffer: &buf}

			mpo, err := NewMultiPartOutputFile(ws, []*Header{h1})
			if err != nil {
				t.Fatalf("NewMultiPartOutputFile() error = %v", err)
			}

			// Create frame buffer with test data
			fb := NewFrameBuffer()
			rData := make([]byte, width*height*2)
			gData := make([]byte, width*height*2)
			bData := make([]byte, width*height*2)
			aData := make([]byte, width*height*2)

			for i := range rData {
				rData[i] = byte(i % 256)
				gData[i] = byte((i + 64) % 256)
				bData[i] = byte((i + 128) % 256)
				aData[i] = byte((i + 192) % 256)
			}

			fb.Set("R", NewSlice(PixelTypeHalf, rData, width, height))
			fb.Set("G", NewSlice(PixelTypeHalf, gData, width, height))
			fb.Set("B", NewSlice(PixelTypeHalf, bData, width, height))
			fb.Set("A", NewSlice(PixelTypeHalf, aData, width, height))

			if err := mpo.SetFrameBuffer(0, fb); err != nil {
				t.Fatalf("SetFrameBuffer error = %v", err)
			}

			// Write all tiles
			numTilesX := (width + tileSize - 1) / tileSize
			numTilesY := (height + tileSize - 1) / tileSize
			for ty := 0; ty < numTilesY; ty++ {
				for tx := 0; tx < numTilesX; tx++ {
					if err := mpo.WriteTile(0, tx, ty); err != nil {
						t.Fatalf("WriteTile(%d, %d) error = %v", tx, ty, err)
					}
				}
			}

			mpo.Close()

			// Verify output
			if buf.Len() < 100 {
				t.Errorf("Output too small: %d bytes", buf.Len())
			}
		})
	}
}

// TestBuildScanlineDataWithNilSlice tests buildScanlineData when framebuffer has nil slice for a channel.
// This exercises the nil slice branch in buildScanlineData.
func TestBuildScanlineDataWithNilSlice(t *testing.T) {
	width := 16
	height := 8

	// Create header with RGB channels
	h := NewScanlineHeader(width, height)
	h.SetCompression(CompressionNone)
	h.Set(&Attribute{Name: AttrNameName, Type: AttrTypeString, Value: "nil_slice_part"})
	h.Set(&Attribute{Name: AttrNameType, Type: AttrTypeString, Value: PartTypeScanline})

	// Create frame buffer with only R channel, missing G and B
	fb := NewFrameBuffer()
	rData := make([]byte, width*height*2)
	fb.Set("R", NewSlice(PixelTypeHalf, rData, width, height))
	// G and B are not set - they will be nil

	var buf bytes.Buffer
	ws := &seekableWriter{Buffer: &buf}

	mpo, err := NewMultiPartOutputFile(ws, []*Header{h})
	if err != nil {
		t.Fatalf("NewMultiPartOutputFile() error = %v", err)
	}

	// Set the incomplete frame buffer
	if err := mpo.SetFrameBuffer(0, fb); err != nil {
		t.Fatalf("SetFrameBuffer error = %v", err)
	}

	// Write pixels - this should exercise the nil slice handling in buildScanlineData
	if err := mpo.WritePixels(0, height); err != nil {
		t.Fatalf("WritePixels error = %v", err)
	}

	mpo.Close()

	// Verify file was written
	if buf.Len() < 100 {
		t.Errorf("File too small: %d bytes", buf.Len())
	}
}

// TestBuildTileDataWithNilSlice tests buildTileData when framebuffer has nil slice for a channel.
// This exercises the nil slice branch in buildTileData.
func TestBuildTileDataWithNilSlice(t *testing.T) {
	width := 32
	height := 32
	tileSize := 16

	// Create header with RGB channels
	h := NewTiledHeader(width, height, tileSize, tileSize)
	h.Set(&Attribute{Name: AttrNameName, Type: AttrTypeString, Value: "tiled"})
	h.Set(&Attribute{Name: AttrNameType, Type: AttrTypeString, Value: PartTypeTiled})
	h.SetCompression(CompressionNone)

	// Create frame buffer with only R channel, missing G and B
	fb := NewFrameBuffer()
	rData := make([]byte, width*height*2)
	fb.Set("R", NewSlice(PixelTypeHalf, rData, width, height))
	// G and B are not set - they will be nil

	var buf bytes.Buffer
	ws := &seekableWriter{Buffer: &buf}

	mpo, err := NewMultiPartOutputFile(ws, []*Header{h})
	if err != nil {
		t.Fatalf("NewMultiPartOutputFile() error = %v", err)
	}

	// Set the incomplete frame buffer
	if err := mpo.SetFrameBuffer(0, fb); err != nil {
		t.Fatalf("SetFrameBuffer error = %v", err)
	}

	// Write tiles - this should exercise the nil slice handling in buildTileData
	numTilesX := (width + tileSize - 1) / tileSize
	numTilesY := (height + tileSize - 1) / tileSize
	for ty := 0; ty < numTilesY; ty++ {
		for tx := 0; tx < numTilesX; tx++ {
			if err := mpo.WriteTile(0, tx, ty); err != nil {
				t.Fatalf("WriteTile(%d, %d) error = %v", tx, ty, err)
			}
		}
	}

	mpo.Close()

	// Verify file was written
	if buf.Len() < 100 {
		t.Errorf("File too small: %d bytes", buf.Len())
	}
}

// TestBuildScanlineDataWithUintChannel tests buildScanlineData with PixelTypeUint channel.
// This exercises the uint switch case in buildScanlineData.
func TestBuildScanlineDataWithUintChannel(t *testing.T) {
	width := 16
	height := 8

	// Create header with a single Uint channel (like object ID)
	h := NewScanlineHeader(width, height)
	h.SetCompression(CompressionNone)
	h.Set(&Attribute{Name: AttrNameName, Type: AttrTypeString, Value: "uint_part"})
	h.Set(&Attribute{Name: AttrNameType, Type: AttrTypeString, Value: PartTypeScanline})

	// Replace channels with a single Uint channel
	cl := NewChannelList()
	cl.Add(Channel{Name: "id", Type: PixelTypeUint, XSampling: 1, YSampling: 1})
	h.SetChannels(cl)

	// Create frame buffer with Uint data
	fb := NewFrameBuffer()
	idData := make([]byte, width*height*4) // 4 bytes per uint
	fb.Set("id", NewSlice(PixelTypeUint, idData, width, height))

	// Set some test values
	idSlice := fb.Get("id")
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			idSlice.SetUint32(x, y, uint32(y*width+x))
		}
	}

	var buf bytes.Buffer
	ws := &seekableWriter{Buffer: &buf}

	mpo, err := NewMultiPartOutputFile(ws, []*Header{h})
	if err != nil {
		t.Fatalf("NewMultiPartOutputFile() error = %v", err)
	}

	if err := mpo.SetFrameBuffer(0, fb); err != nil {
		t.Fatalf("SetFrameBuffer error = %v", err)
	}

	if err := mpo.WritePixels(0, height); err != nil {
		t.Fatalf("WritePixels error = %v", err)
	}

	mpo.Close()

	// Verify file was written
	if buf.Len() < 100 {
		t.Errorf("File too small: %d bytes", buf.Len())
	}

	// Read back and verify
	data := buf.Bytes()
	reader := bytes.NewReader(data)
	f, err := OpenReader(reader, int64(len(data)))
	if err != nil {
		t.Fatalf("OpenReader() error = %v", err)
	}

	// Verify channel type is Uint
	ch := f.Header(0).Channels().Get("id")
	if ch == nil {
		t.Fatal("id channel not found")
	}
	if ch.Type != PixelTypeUint {
		t.Errorf("Channel type = %v, want %v", ch.Type, PixelTypeUint)
	}
}

// TestBuildTileDataWithUintChannel tests buildTileData with PixelTypeUint channel.
// This exercises the uint switch case in buildTileData.
func TestBuildTileDataWithUintChannel(t *testing.T) {
	width := 32
	height := 32
	tileSize := 16

	// Create header with a single Uint channel
	h := NewTiledHeader(width, height, tileSize, tileSize)
	h.Set(&Attribute{Name: AttrNameName, Type: AttrTypeString, Value: "tiled"})
	h.Set(&Attribute{Name: AttrNameType, Type: AttrTypeString, Value: PartTypeTiled})
	h.SetCompression(CompressionNone)

	// Replace channels with a single Uint channel
	cl := NewChannelList()
	cl.Add(Channel{Name: "objectId", Type: PixelTypeUint, XSampling: 1, YSampling: 1})
	h.SetChannels(cl)

	// Create frame buffer with Uint data
	fb := NewFrameBuffer()
	idData := make([]byte, width*height*4)
	fb.Set("objectId", NewSlice(PixelTypeUint, idData, width, height))

	// Set some test values
	idSlice := fb.Get("objectId")
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			idSlice.SetUint32(x, y, uint32(y*width+x+1000))
		}
	}

	var buf bytes.Buffer
	ws := &seekableWriter{Buffer: &buf}

	mpo, err := NewMultiPartOutputFile(ws, []*Header{h})
	if err != nil {
		t.Fatalf("NewMultiPartOutputFile() error = %v", err)
	}

	if err := mpo.SetFrameBuffer(0, fb); err != nil {
		t.Fatalf("SetFrameBuffer error = %v", err)
	}

	// Write all tiles
	numTilesX := (width + tileSize - 1) / tileSize
	numTilesY := (height + tileSize - 1) / tileSize
	for ty := 0; ty < numTilesY; ty++ {
		for tx := 0; tx < numTilesX; tx++ {
			if err := mpo.WriteTile(0, tx, ty); err != nil {
				t.Fatalf("WriteTile(%d, %d) error = %v", tx, ty, err)
			}
		}
	}

	mpo.Close()

	// Verify file was written
	if buf.Len() < 100 {
		t.Errorf("File too small: %d bytes", buf.Len())
	}
}

// TestBuildScanlineDataWithFloatChannel tests buildScanlineData with PixelTypeFloat channel.
// This exercises the float switch case in buildScanlineData.
func TestBuildScanlineDataWithFloatChannel(t *testing.T) {
	width := 16
	height := 8

	// Create header with Float channels
	h := NewScanlineHeader(width, height)
	h.SetCompression(CompressionNone)
	h.Set(&Attribute{Name: AttrNameName, Type: AttrTypeString, Value: "float_part"})
	h.Set(&Attribute{Name: AttrNameType, Type: AttrTypeString, Value: PartTypeScanline})

	// Replace channels with Float channels
	cl := NewChannelList()
	cl.Add(Channel{Name: "R", Type: PixelTypeFloat, XSampling: 1, YSampling: 1})
	cl.Add(Channel{Name: "Z", Type: PixelTypeFloat, XSampling: 1, YSampling: 1})
	h.SetChannels(cl)

	// Create frame buffer with Float data
	fb := NewFrameBuffer()
	rData := make([]byte, width*height*4)
	zData := make([]byte, width*height*4)
	fb.Set("R", NewSlice(PixelTypeFloat, rData, width, height))
	fb.Set("Z", NewSlice(PixelTypeFloat, zData, width, height))

	// Set some test values
	rSlice := fb.Get("R")
	zSlice := fb.Get("Z")
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			rSlice.SetFloat32(x, y, float32(x)/float32(width))
			zSlice.SetFloat32(x, y, float32(y)/float32(height))
		}
	}

	var buf bytes.Buffer
	ws := &seekableWriter{Buffer: &buf}

	mpo, err := NewMultiPartOutputFile(ws, []*Header{h})
	if err != nil {
		t.Fatalf("NewMultiPartOutputFile() error = %v", err)
	}

	if err := mpo.SetFrameBuffer(0, fb); err != nil {
		t.Fatalf("SetFrameBuffer error = %v", err)
	}

	if err := mpo.WritePixels(0, height); err != nil {
		t.Fatalf("WritePixels error = %v", err)
	}

	mpo.Close()

	// Verify file was written
	if buf.Len() < 100 {
		t.Errorf("File too small: %d bytes", buf.Len())
	}
}

// TestBuildTileDataWithFloatChannel tests buildTileData with PixelTypeFloat channel.
// This exercises the float switch case in buildTileData.
func TestBuildTileDataWithFloatChannel(t *testing.T) {
	width := 32
	height := 32
	tileSize := 16

	// Create header with Float channels
	h := NewTiledHeader(width, height, tileSize, tileSize)
	h.Set(&Attribute{Name: AttrNameName, Type: AttrTypeString, Value: "tiled"})
	h.Set(&Attribute{Name: AttrNameType, Type: AttrTypeString, Value: PartTypeTiled})
	h.SetCompression(CompressionNone)

	// Replace channels with Float channels
	cl := NewChannelList()
	cl.Add(Channel{Name: "R", Type: PixelTypeFloat, XSampling: 1, YSampling: 1})
	cl.Add(Channel{Name: "depth", Type: PixelTypeFloat, XSampling: 1, YSampling: 1})
	h.SetChannels(cl)

	// Create frame buffer with Float data
	fb := NewFrameBuffer()
	rData := make([]byte, width*height*4)
	depthData := make([]byte, width*height*4)
	fb.Set("R", NewSlice(PixelTypeFloat, rData, width, height))
	fb.Set("depth", NewSlice(PixelTypeFloat, depthData, width, height))

	// Set some test values
	rSlice := fb.Get("R")
	depthSlice := fb.Get("depth")
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			rSlice.SetFloat32(x, y, float32(x+y)/64.0)
			depthSlice.SetFloat32(x, y, 1.0+float32(x+y)/100.0)
		}
	}

	var buf bytes.Buffer
	ws := &seekableWriter{Buffer: &buf}

	mpo, err := NewMultiPartOutputFile(ws, []*Header{h})
	if err != nil {
		t.Fatalf("NewMultiPartOutputFile() error = %v", err)
	}

	if err := mpo.SetFrameBuffer(0, fb); err != nil {
		t.Fatalf("SetFrameBuffer error = %v", err)
	}

	// Write all tiles
	numTilesX := (width + tileSize - 1) / tileSize
	numTilesY := (height + tileSize - 1) / tileSize
	for ty := 0; ty < numTilesY; ty++ {
		for tx := 0; tx < numTilesX; tx++ {
			if err := mpo.WriteTile(0, tx, ty); err != nil {
				t.Fatalf("WriteTile(%d, %d) error = %v", tx, ty, err)
			}
		}
	}

	mpo.Close()

	// Verify file was written
	if buf.Len() < 100 {
		t.Errorf("File too small: %d bytes", buf.Len())
	}
}

// TestBuildScanlineDataWithMixedNilSlices tests buildScanlineData with mixed nil slices
// across different pixel types.
func TestBuildScanlineDataWithMixedNilSlices(t *testing.T) {
	width := 16
	height := 8

	// Create header with mixed channel types
	h := NewScanlineHeader(width, height)
	h.SetCompression(CompressionNone)
	h.Set(&Attribute{Name: AttrNameName, Type: AttrTypeString, Value: "mixed_part"})
	h.Set(&Attribute{Name: AttrNameType, Type: AttrTypeString, Value: PartTypeScanline})

	cl := NewChannelList()
	cl.Add(Channel{Name: "R", Type: PixelTypeHalf, XSampling: 1, YSampling: 1})
	cl.Add(Channel{Name: "depth", Type: PixelTypeFloat, XSampling: 1, YSampling: 1})
	cl.Add(Channel{Name: "objectId", Type: PixelTypeUint, XSampling: 1, YSampling: 1})
	h.SetChannels(cl)

	// Create frame buffer with only R - depth and objectId will be nil
	fb := NewFrameBuffer()
	rData := make([]byte, width*height*2)
	fb.Set("R", NewSlice(PixelTypeHalf, rData, width, height))
	// depth and objectId are nil - this tests all three nil slice branches

	var buf bytes.Buffer
	ws := &seekableWriter{Buffer: &buf}

	mpo, err := NewMultiPartOutputFile(ws, []*Header{h})
	if err != nil {
		t.Fatalf("NewMultiPartOutputFile() error = %v", err)
	}

	if err := mpo.SetFrameBuffer(0, fb); err != nil {
		t.Fatalf("SetFrameBuffer error = %v", err)
	}

	if err := mpo.WritePixels(0, height); err != nil {
		t.Fatalf("WritePixels error = %v", err)
	}

	mpo.Close()

	// Verify file was written
	if buf.Len() < 100 {
		t.Errorf("File too small: %d bytes", buf.Len())
	}
}

// TestBuildTileDataWithMixedNilSlices tests buildTileData with mixed nil slices
// across different pixel types.
func TestBuildTileDataWithMixedNilSlices(t *testing.T) {
	width := 32
	height := 32
	tileSize := 16

	// Create header with mixed channel types
	h := NewTiledHeader(width, height, tileSize, tileSize)
	h.Set(&Attribute{Name: AttrNameName, Type: AttrTypeString, Value: "tiled"})
	h.Set(&Attribute{Name: AttrNameType, Type: AttrTypeString, Value: PartTypeTiled})
	h.SetCompression(CompressionNone)

	cl := NewChannelList()
	cl.Add(Channel{Name: "R", Type: PixelTypeHalf, XSampling: 1, YSampling: 1})
	cl.Add(Channel{Name: "depth", Type: PixelTypeFloat, XSampling: 1, YSampling: 1})
	cl.Add(Channel{Name: "objectId", Type: PixelTypeUint, XSampling: 1, YSampling: 1})
	h.SetChannels(cl)

	// Create frame buffer with only objectId - R and depth will be nil
	fb := NewFrameBuffer()
	idData := make([]byte, width*height*4)
	fb.Set("objectId", NewSlice(PixelTypeUint, idData, width, height))
	// R and depth are nil - this tests nil branches for half and float

	var buf bytes.Buffer
	ws := &seekableWriter{Buffer: &buf}

	mpo, err := NewMultiPartOutputFile(ws, []*Header{h})
	if err != nil {
		t.Fatalf("NewMultiPartOutputFile() error = %v", err)
	}

	if err := mpo.SetFrameBuffer(0, fb); err != nil {
		t.Fatalf("SetFrameBuffer error = %v", err)
	}

	// Write all tiles
	numTilesX := (width + tileSize - 1) / tileSize
	numTilesY := (height + tileSize - 1) / tileSize
	for ty := 0; ty < numTilesY; ty++ {
		for tx := 0; tx < numTilesX; tx++ {
			if err := mpo.WriteTile(0, tx, ty); err != nil {
				t.Fatalf("WriteTile(%d, %d) error = %v", tx, ty, err)
			}
		}
	}

	mpo.Close()

	// Verify file was written
	if buf.Len() < 100 {
		t.Errorf("File too small: %d bytes", buf.Len())
	}
}

// createSimpleMultiPartFile creates a minimal multi-part file for testing.
func createSimpleMultiPartFile(t *testing.T) []byte {
	t.Helper()
	width := 8
	height := 8

	h1 := NewScanlineHeader(width, height)
	h1.Set(&Attribute{Name: AttrNameName, Type: AttrTypeString, Value: "part1"})
	h1.Set(&Attribute{Name: AttrNameType, Type: AttrTypeString, Value: PartTypeScanline})
	h1.SetCompression(CompressionNone)

	h2 := NewScanlineHeader(width, height)
	h2.Set(&Attribute{Name: AttrNameName, Type: AttrTypeString, Value: "part2"})
	h2.Set(&Attribute{Name: AttrNameType, Type: AttrTypeString, Value: PartTypeScanline})
	h2.SetCompression(CompressionNone)

	var buf bytes.Buffer
	ws := &seekableWriter{Buffer: &buf}

	w, err := NewMultiPartWriter(ws, []*Header{h1, h2})
	if err != nil {
		t.Fatalf("NewMultiPartWriter() error = %v", err)
	}

	for part := 0; part < 2; part++ {
		for y := 0; y < height; y++ {
			data := make([]byte, width*8)
			if err := w.WriteChunkPart(part, int32(y), data); err != nil {
				t.Fatalf("WriteChunkPart error = %v", err)
			}
		}
	}

	if err := w.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	return buf.Bytes()
}

// TestMultiPartPartInfoError tests PartInfo error handling.
func TestMultiPartPartInfoError(t *testing.T) {
	fileData := createSimpleMultiPartFile(t)
	reader := bytes.NewReader(fileData)

	f, err := OpenReader(reader, int64(len(fileData)))
	if err != nil {
		t.Fatalf("OpenReader() error = %v", err)
	}
	mpf := NewMultiPartInputFile(f)

	// Test invalid part index
	_, err = mpf.PartInfo(-1)
	if err != ErrPartNotFound {
		t.Errorf("PartInfo(-1) error = %v, want ErrPartNotFound", err)
	}

	_, err = mpf.PartInfo(100)
	if err != ErrPartNotFound {
		t.Errorf("PartInfo(100) error = %v, want ErrPartNotFound", err)
	}
}

// TestMultiPartScanlineReaderError tests ScanlineReader error handling.
func TestMultiPartScanlineReaderError(t *testing.T) {
	fileData := createSimpleMultiPartFile(t)
	reader := bytes.NewReader(fileData)

	f, err := OpenReader(reader, int64(len(fileData)))
	if err != nil {
		t.Fatalf("OpenReader() error = %v", err)
	}
	mpf := NewMultiPartInputFile(f)

	// Test invalid part index
	_, err = mpf.ScanlineReader(-1)
	if err == nil {
		t.Error("ScanlineReader(-1) should fail")
	}

	_, err = mpf.ScanlineReader(100)
	if err == nil {
		t.Error("ScanlineReader(100) should fail")
	}
}

// TestMultiPartTiledReaderError tests TiledReader error handling.
func TestMultiPartTiledReaderError(t *testing.T) {
	fileData := createSimpleMultiPartFile(t)
	reader := bytes.NewReader(fileData)

	f, err := OpenReader(reader, int64(len(fileData)))
	if err != nil {
		t.Fatalf("OpenReader() error = %v", err)
	}
	mpf := NewMultiPartInputFile(f)

	// Test invalid part index
	_, err = mpf.TiledReader(-1)
	if err == nil {
		t.Error("TiledReader(-1) should fail")
	}

	_, err = mpf.TiledReader(100)
	if err == nil {
		t.Error("TiledReader(100) should fail")
	}
}

func TestMultiPartOutputWithPXR24(t *testing.T) {
	width := 16
	height := 8

	h := NewScanlineHeader(width, height)
	h.Set(&Attribute{Name: AttrNameName, Type: AttrTypeString, Value: "part1"})
	h.Set(&Attribute{Name: AttrNameType, Type: AttrTypeString, Value: PartTypeScanline})
	h.SetCompression(CompressionPXR24)

	var buf bytes.Buffer
	ws := &seekableWriter{Buffer: &buf}

	mpo, err := NewMultiPartOutputFile(ws, []*Header{h})
	if err != nil {
		t.Fatalf("NewMultiPartOutputFile() error = %v", err)
	}

	fb := NewFrameBuffer()
	rData := make([]byte, width*height*2)
	fb.Set("R", NewSlice(PixelTypeHalf, rData, width, height))
	mpo.SetFrameBuffer(0, fb)

	// Write all scanlines at once
	err = mpo.WritePixels(0, height)
	if err != nil {
		t.Fatalf("WritePixels error = %v", err)
	}

	mpo.Close()
	t.Logf("PXR24 multipart file size: %d bytes", buf.Len())
}

func TestMultiPartOutputWithB44(t *testing.T) {
	width := 16
	height := 8

	h := NewScanlineHeader(width, height)
	h.Set(&Attribute{Name: AttrNameName, Type: AttrTypeString, Value: "part1"})
	h.Set(&Attribute{Name: AttrNameType, Type: AttrTypeString, Value: PartTypeScanline})
	h.SetCompression(CompressionB44)

	var buf bytes.Buffer
	ws := &seekableWriter{Buffer: &buf}

	mpo, err := NewMultiPartOutputFile(ws, []*Header{h})
	if err != nil {
		t.Fatalf("NewMultiPartOutputFile() error = %v", err)
	}

	fb := NewFrameBuffer()
	rData := make([]byte, width*height*2)
	fb.Set("R", NewSlice(PixelTypeHalf, rData, width, height))
	mpo.SetFrameBuffer(0, fb)

	err = mpo.WritePixels(0, height)
	if err != nil {
		t.Fatalf("WritePixels error = %v", err)
	}

	mpo.Close()
	t.Logf("B44 multipart file size: %d bytes", buf.Len())
}

func TestMultiPartSetFrameBufferErrors(t *testing.T) {
	width, height := 8, 8

	h := NewScanlineHeader(width, height)
	h.Set(&Attribute{Name: AttrNameName, Type: AttrTypeString, Value: "part1"})
	h.Set(&Attribute{Name: AttrNameType, Type: AttrTypeString, Value: PartTypeScanline})
	h.SetCompression(CompressionNone)

	var buf bytes.Buffer
	ws := &seekableWriter{Buffer: &buf}

	mpo, err := NewMultiPartOutputFile(ws, []*Header{h})
	if err != nil {
		t.Fatalf("NewMultiPartOutputFile() error = %v", err)
	}
	defer mpo.Close()

	// Test invalid part index (negative)
	err = mpo.SetFrameBuffer(-1, NewFrameBuffer())
	if err != ErrPartNotFound {
		t.Errorf("SetFrameBuffer(-1) error = %v, want ErrPartNotFound", err)
	}

	// Test invalid part index (too large)
	err = mpo.SetFrameBuffer(100, NewFrameBuffer())
	if err != ErrPartNotFound {
		t.Errorf("SetFrameBuffer(100) error = %v, want ErrPartNotFound", err)
	}
}

func TestMultiPartWritePixelsErrors(t *testing.T) {
	width, height := 8, 8

	h := NewScanlineHeader(width, height)
	h.Set(&Attribute{Name: AttrNameName, Type: AttrTypeString, Value: "part1"})
	h.Set(&Attribute{Name: AttrNameType, Type: AttrTypeString, Value: PartTypeScanline})
	h.SetCompression(CompressionNone)

	var buf bytes.Buffer
	ws := &seekableWriter{Buffer: &buf}

	mpo, err := NewMultiPartOutputFile(ws, []*Header{h})
	if err != nil {
		t.Fatalf("NewMultiPartOutputFile() error = %v", err)
	}
	defer mpo.Close()

	// Test invalid part index (negative)
	err = mpo.WritePixels(-1, height)
	if err != ErrPartNotFound {
		t.Errorf("WritePixels(-1) error = %v, want ErrPartNotFound", err)
	}

	// Test invalid part index (too large)
	err = mpo.WritePixels(100, height)
	if err != ErrPartNotFound {
		t.Errorf("WritePixels(100) error = %v, want ErrPartNotFound", err)
	}

	// Test WritePixels without setting frame buffer first
	err = mpo.WritePixels(0, height)
	if err != ErrInvalidSlice {
		t.Errorf("WritePixels without frame buffer error = %v, want ErrInvalidSlice", err)
	}
}

func TestMultiPartDeepScanlineReaderErrors(t *testing.T) {
	// Create a regular (non-deep) scanline file
	width, height := 8, 8
	h := NewScanlineHeader(width, height)
	h.Set(&Attribute{Name: AttrNameName, Type: AttrTypeString, Value: "part1"})
	h.Set(&Attribute{Name: AttrNameType, Type: AttrTypeString, Value: PartTypeScanline})

	var buf bytes.Buffer
	ws := &seekableWriter{Buffer: &buf}

	mpo, err := NewMultiPartOutputFile(ws, []*Header{h})
	if err != nil {
		t.Fatalf("NewMultiPartOutputFile() error = %v", err)
	}

	fb := NewFrameBuffer()
	rData := make([]byte, width*height*2)
	fb.Set("R", NewSlice(PixelTypeHalf, rData, width, height))
	mpo.SetFrameBuffer(0, fb)
	mpo.WritePixels(0, height)
	mpo.Close()

	// Open as multi-part
	r := bytes.NewReader(buf.Bytes())
	f, err := OpenReader(r, int64(buf.Len()))
	if err != nil {
		t.Fatalf("OpenReader() error = %v", err)
	}

	mpi := NewMultiPartInputFile(f)

	// Try to get deep scanline reader from non-deep part
	_, err = mpi.DeepScanlineReader(0)
	if err != ErrInvalidPartType {
		t.Errorf("DeepScanlineReader on non-deep part error = %v, want ErrInvalidPartType", err)
	}

	// Try to get deep scanline reader for invalid part
	_, err = mpi.DeepScanlineReader(100)
	if err != ErrPartNotFound {
		t.Errorf("DeepScanlineReader(100) error = %v, want ErrPartNotFound", err)
	}
}

func TestMultiPartDeepTiledReaderErrors(t *testing.T) {
	// Create a regular (non-deep) tiled file
	width, height := 8, 8
	tileW, tileH := 4, 4
	h := NewTiledHeader(width, height, tileW, tileH)
	h.Set(&Attribute{Name: AttrNameName, Type: AttrTypeString, Value: "part1"})
	h.Set(&Attribute{Name: AttrNameType, Type: AttrTypeString, Value: PartTypeTiled})

	var buf bytes.Buffer
	ws := &seekableWriter{Buffer: &buf}

	mpo, err := NewMultiPartOutputFile(ws, []*Header{h})
	if err != nil {
		t.Fatalf("NewMultiPartOutputFile() error = %v", err)
	}

	fb := NewFrameBuffer()
	rData := make([]byte, width*height*2)
	fb.Set("R", NewSlice(PixelTypeHalf, rData, width, height))
	mpo.SetFrameBuffer(0, fb)
	mpo.WriteTile(0, 0, 0)
	mpo.WriteTile(0, 1, 0)
	mpo.WriteTile(0, 0, 1)
	mpo.WriteTile(0, 1, 1)
	mpo.Close()

	// Open as multi-part
	r := bytes.NewReader(buf.Bytes())
	f, err := OpenReader(r, int64(buf.Len()))
	if err != nil {
		t.Fatalf("OpenReader() error = %v", err)
	}

	mpi := NewMultiPartInputFile(f)

	// Try to get deep tiled reader from non-deep part
	_, err = mpi.DeepTiledReader(0)
	if err != ErrInvalidPartType {
		t.Errorf("DeepTiledReader on non-deep tiled part error = %v, want ErrInvalidPartType", err)
	}

	// Try to get deep tiled reader for invalid part
	_, err = mpi.DeepTiledReader(100)
	if err != ErrPartNotFound {
		t.Errorf("DeepTiledReader(100) error = %v, want ErrPartNotFound", err)
	}
}

func TestMultiPartOutputHeaderErrors(t *testing.T) {
	width, height := 8, 8

	h := NewScanlineHeader(width, height)
	h.Set(&Attribute{Name: AttrNameName, Type: AttrTypeString, Value: "part1"})
	h.Set(&Attribute{Name: AttrNameType, Type: AttrTypeString, Value: PartTypeScanline})

	var buf bytes.Buffer
	ws := &seekableWriter{Buffer: &buf}

	mpo, err := NewMultiPartOutputFile(ws, []*Header{h})
	if err != nil {
		t.Fatalf("NewMultiPartOutputFile() error = %v", err)
	}
	defer mpo.Close()

	// Test invalid part indices for Header
	if mpo.Header(-1) != nil {
		t.Error("Header(-1) should return nil")
	}
	if mpo.Header(100) != nil {
		t.Error("Header(100) should return nil")
	}
}

func TestMultiPartWriteTileLevelErrors(t *testing.T) {
	// Create a scanline file (not tiled)
	width, height := 8, 8
	h := NewScanlineHeader(width, height)
	h.Set(&Attribute{Name: AttrNameName, Type: AttrTypeString, Value: "part1"})
	h.Set(&Attribute{Name: AttrNameType, Type: AttrTypeString, Value: PartTypeScanline})

	var buf bytes.Buffer
	ws := &seekableWriter{Buffer: &buf}

	mpo, err := NewMultiPartOutputFile(ws, []*Header{h})
	if err != nil {
		t.Fatalf("NewMultiPartOutputFile() error = %v", err)
	}
	defer mpo.Close()

	// Test invalid part index
	err = mpo.WriteTileLevel(-1, 0, 0, 0, 0)
	if err != ErrPartNotFound {
		t.Errorf("WriteTileLevel(-1) error = %v, want ErrPartNotFound", err)
	}

	err = mpo.WriteTileLevel(100, 0, 0, 0, 0)
	if err != ErrPartNotFound {
		t.Errorf("WriteTileLevel(100) error = %v, want ErrPartNotFound", err)
	}

	// Set a framebuffer without tile description
	fb := NewFrameBuffer()
	rData := make([]byte, width*height*2)
	fb.Set("R", NewSlice(PixelTypeHalf, rData, width, height))
	mpo.SetFrameBuffer(0, fb)

	// Try to write tile without tile description - scanline files don't have tiles
	err = mpo.WriteTileLevel(0, 0, 0, 0, 0)
	if err == nil {
		t.Error("WriteTileLevel on scanline part should fail")
	}
}
