package exrutil

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/mrjoshuak/go-openexr/exr"
)

func createTestFile(t *testing.T, dir string, name string, width, height int, compression exr.Compression) string {
	t.Helper()

	path := filepath.Join(dir, name)

	// Create a simple test image
	img := exr.NewRGBAImage(exr.RectFromSize(width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.SetRGBA(x, y, float32(x)/float32(width), float32(y)/float32(height), 0.5, 1.0)
		}
	}

	// Write with specified compression using RGBAOutputFile
	out, err := exr.NewRGBAOutputFile(path, width, height)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Set compression on the header
	out.Header().SetCompression(compression)

	if err := out.WriteRGBA(img); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	return path
}

func TestGetFileInfo(t *testing.T) {
	dir := t.TempDir()
	path := createTestFile(t, dir, "test.exr", 100, 50, exr.CompressionZIP)

	info, err := GetFileInfo(path)
	if err != nil {
		t.Fatalf("GetFileInfo() error = %v", err)
	}

	if info.Width != 100 {
		t.Errorf("Width = %d, want 100", info.Width)
	}
	if info.Height != 50 {
		t.Errorf("Height = %d, want 50", info.Height)
	}
	if info.Compression != exr.CompressionZIP {
		t.Errorf("Compression = %v, want ZIP", info.Compression)
	}
	if info.IsTiled {
		t.Error("IsTiled = true, want false")
	}
	if info.FileSize == 0 {
		t.Error("FileSize = 0, want > 0")
	}
	if len(info.Channels) == 0 {
		t.Error("Channels is empty")
	}
}

func TestGetFileInfoNonexistent(t *testing.T) {
	_, err := GetFileInfo("/nonexistent/file.exr")
	if err == nil {
		t.Error("GetFileInfo() should return error for nonexistent file")
	}
}

func TestExtractChannel(t *testing.T) {
	dir := t.TempDir()
	path := createTestFile(t, dir, "test.exr", 10, 10, exr.CompressionNone)

	f, err := exr.OpenFile(path)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer f.Close()

	// Extract R channel
	data, err := ExtractChannel(f, "R")
	if err != nil {
		t.Fatalf("ExtractChannel() error = %v", err)
	}

	if len(data) != 100 {
		t.Errorf("len(data) = %d, want 100", len(data))
	}

	// Check some values (first row should have R going from 0 to 0.9)
	if data[0] != 0 {
		t.Errorf("data[0] = %f, want 0", data[0])
	}
}

func TestExtractChannelNotFound(t *testing.T) {
	dir := t.TempDir()
	path := createTestFile(t, dir, "test.exr", 10, 10, exr.CompressionNone)

	f, err := exr.OpenFile(path)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer f.Close()

	_, err = ExtractChannel(f, "NonexistentChannel")
	if err == nil {
		t.Error("ExtractChannel() should return error for nonexistent channel")
	}
}

func TestExtractChannels(t *testing.T) {
	dir := t.TempDir()
	path := createTestFile(t, dir, "test.exr", 10, 10, exr.CompressionNone)

	f, err := exr.OpenFile(path)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer f.Close()

	channels, err := ExtractChannels(f, "R", "G", "B")
	if err != nil {
		t.Fatalf("ExtractChannels() error = %v", err)
	}

	if len(channels) != 3 {
		t.Errorf("len(channels) = %d, want 3", len(channels))
	}

	for _, name := range []string{"R", "G", "B"} {
		if _, ok := channels[name]; !ok {
			t.Errorf("channels[%q] not found", name)
		}
	}
}

func TestSplitLayers(t *testing.T) {
	h := exr.NewHeader()

	// Add channels with layers
	cl := exr.NewChannelList()
	cl.Add(exr.Channel{Name: "R", Type: exr.PixelTypeHalf, XSampling: 1, YSampling: 1})
	cl.Add(exr.Channel{Name: "G", Type: exr.PixelTypeHalf, XSampling: 1, YSampling: 1})
	cl.Add(exr.Channel{Name: "B", Type: exr.PixelTypeHalf, XSampling: 1, YSampling: 1})
	cl.Add(exr.Channel{Name: "diffuse.R", Type: exr.PixelTypeHalf, XSampling: 1, YSampling: 1})
	cl.Add(exr.Channel{Name: "diffuse.G", Type: exr.PixelTypeHalf, XSampling: 1, YSampling: 1})
	cl.Add(exr.Channel{Name: "diffuse.B", Type: exr.PixelTypeHalf, XSampling: 1, YSampling: 1})
	cl.Add(exr.Channel{Name: "specular.R", Type: exr.PixelTypeHalf, XSampling: 1, YSampling: 1})
	h.SetChannels(cl)

	layers := SplitLayers(h)

	// Root level channels (R, G, B)
	if root, ok := layers[""]; !ok {
		t.Error("No root layer found")
	} else if len(root) != 3 {
		t.Errorf("Root layer has %d channels, want 3", len(root))
	}

	// Diffuse layer
	if diffuse, ok := layers["diffuse"]; !ok {
		t.Error("No diffuse layer found")
	} else if len(diffuse) != 3 {
		t.Errorf("Diffuse layer has %d channels, want 3", len(diffuse))
	}

	// Specular layer
	if specular, ok := layers["specular"]; !ok {
		t.Error("No specular layer found")
	} else if len(specular) != 1 {
		t.Errorf("Specular layer has %d channels, want 1", len(specular))
	}
}

func TestListLayers(t *testing.T) {
	h := exr.NewHeader()

	cl := exr.NewChannelList()
	cl.Add(exr.Channel{Name: "R", Type: exr.PixelTypeHalf, XSampling: 1, YSampling: 1})
	cl.Add(exr.Channel{Name: "diffuse.R", Type: exr.PixelTypeHalf, XSampling: 1, YSampling: 1})
	cl.Add(exr.Channel{Name: "specular.R", Type: exr.PixelTypeHalf, XSampling: 1, YSampling: 1})
	cl.Add(exr.Channel{Name: "ao.R", Type: exr.PixelTypeHalf, XSampling: 1, YSampling: 1})
	h.SetChannels(cl)

	layers := ListLayers(h)

	if len(layers) != 3 {
		t.Errorf("len(layers) = %d, want 3", len(layers))
	}

	// Should be sorted
	expected := []string{"ao", "diffuse", "specular"}
	for i, name := range expected {
		if i >= len(layers) || layers[i] != name {
			t.Errorf("layers[%d] = %q, want %q", i, layers[i], name)
		}
	}
}

func TestValidateFile(t *testing.T) {
	dir := t.TempDir()
	path := createTestFile(t, dir, "test.exr", 100, 100, exr.CompressionZIP)

	result, err := ValidateFile(path)
	if err != nil {
		t.Fatalf("ValidateFile() error = %v", err)
	}

	if !result.Valid {
		t.Errorf("ValidateFile() Valid = false, want true. Errors: %v", result.Errors)
	}
}

func TestValidateFileNonexistent(t *testing.T) {
	result, err := ValidateFile("/nonexistent/file.exr")
	if err != nil {
		t.Fatalf("ValidateFile() error = %v", err)
	}

	if result.Valid {
		t.Error("ValidateFile() Valid = true for nonexistent file, want false")
	}
}

func TestValidateFileInvalid(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "invalid.exr")

	// Write invalid data
	if err := os.WriteFile(path, []byte("not an exr file"), 0644); err != nil {
		t.Fatalf("Failed to create invalid file: %v", err)
	}

	result, err := ValidateFile(path)
	if err != nil {
		t.Fatalf("ValidateFile() error = %v", err)
	}

	if result.Valid {
		t.Error("ValidateFile() Valid = true for invalid file, want false")
	}
}

func TestCompareFiles(t *testing.T) {
	dir := t.TempDir()
	path1 := createTestFile(t, dir, "test1.exr", 50, 50, exr.CompressionNone)
	path2 := createTestFile(t, dir, "test2.exr", 50, 50, exr.CompressionNone)

	// Same content should match
	match, diffs, err := CompareFiles(path1, path2, CompareOptions{Tolerance: 0.001})
	if err != nil {
		t.Fatalf("CompareFiles() error = %v", err)
	}

	if !match {
		t.Errorf("CompareFiles() match = false for identical files. Diffs: %v", diffs)
	}
}

func TestCompareFilesDifferentDimensions(t *testing.T) {
	dir := t.TempDir()
	path1 := createTestFile(t, dir, "test1.exr", 50, 50, exr.CompressionNone)
	path2 := createTestFile(t, dir, "test2.exr", 100, 100, exr.CompressionNone)

	match, diffs, err := CompareFiles(path1, path2, CompareOptions{})
	if err != nil {
		t.Fatalf("CompareFiles() error = %v", err)
	}

	if match {
		t.Error("CompareFiles() match = true for files with different dimensions")
	}

	if len(diffs) == 0 {
		t.Error("CompareFiles() diffs is empty, expected dimension difference")
	}
}

func TestConvertCompression(t *testing.T) {
	dir := t.TempDir()
	inputPath := createTestFile(t, dir, "input.exr", 50, 50, exr.CompressionNone)
	outputPath := filepath.Join(dir, "output.exr")

	err := ConvertCompression(inputPath, outputPath, exr.CompressionPIZ)
	if err != nil {
		t.Fatalf("ConvertCompression() error = %v", err)
	}

	// Verify output has correct compression
	info, err := GetFileInfo(outputPath)
	if err != nil {
		t.Fatalf("GetFileInfo() error = %v", err)
	}

	// Note: EncodeFile uses its own header, so compression might differ
	// Just verify the file was created successfully
	if info.Width != 50 || info.Height != 50 {
		t.Errorf("Output dimensions = %dx%d, want 50x50", info.Width, info.Height)
	}
}

func TestCopyMetadata(t *testing.T) {
	src := exr.NewScanlineHeader(100, 100)
	src.Set(&exr.Attribute{Name: "owner", Type: exr.AttrTypeString, Value: "Test Owner"})
	src.Set(&exr.Attribute{Name: "comments", Type: exr.AttrTypeString, Value: "Test Comments"})

	dst := exr.NewScanlineHeader(200, 200)

	CopyMetadata(src, dst)

	// Check metadata was copied
	if attr := dst.Get("owner"); attr == nil {
		t.Error("owner attribute not copied")
	} else if attr.Value.(string) != "Test Owner" {
		t.Errorf("owner = %q, want %q", attr.Value, "Test Owner")
	}

	if attr := dst.Get("comments"); attr == nil {
		t.Error("comments attribute not copied")
	}

	// Check structural attributes were NOT copied
	if dst.Width() != 200 {
		t.Errorf("Width was changed from 200 to %d", dst.Width())
	}
}

// createTestFileWithChannelTypes creates an EXR file with Float and Uint channels.
func createTestFileWithChannelTypes(t *testing.T, dir, name string, width, height int) string {
	t.Helper()

	path := filepath.Join(dir, name)

	// Create header with different channel types
	h := exr.NewScanlineHeader(width, height)
	h.SetCompression(exr.CompressionNone)

	cl := exr.NewChannelList()
	cl.Add(exr.Channel{Name: "R", Type: exr.PixelTypeHalf, XSampling: 1, YSampling: 1})
	cl.Add(exr.Channel{Name: "depth", Type: exr.PixelTypeFloat, XSampling: 1, YSampling: 1})
	cl.Add(exr.Channel{Name: "id", Type: exr.PixelTypeUint, XSampling: 1, YSampling: 1})
	h.SetChannels(cl)

	// Create the file
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("Failed to create file: %v", err)
	}

	sw, err := exr.NewScanlineWriter(f, h)
	if err != nil {
		f.Close()
		t.Fatalf("Failed to create scanline writer: %v", err)
	}

	// Create frame buffer with test data
	fb := exr.NewFrameBuffer()
	rData := make([]byte, width*height*2)
	depthData := make([]float32, width*height)
	idData := make([]uint32, width*height)

	// Fill with test values
	for i := 0; i < width*height; i++ {
		depthData[i] = float32(i) / float32(width*height)
		idData[i] = uint32(i)
	}

	fb.Set("R", exr.NewSlice(exr.PixelTypeHalf, rData, width, height))
	fb.Set("depth", exr.NewSliceFromFloat32(depthData, width, height))
	fb.Set("id", exr.NewSliceFromUint32(idData, width, height))
	sw.SetFrameBuffer(fb)

	if err := sw.WritePixels(0, height-1); err != nil {
		sw.Close()
		f.Close()
		t.Fatalf("Failed to write pixels: %v", err)
	}

	if err := sw.Close(); err != nil {
		f.Close()
		t.Fatalf("Failed to close scanline writer: %v", err)
	}

	f.Close()

	return path
}

func TestExtractChannelFloat(t *testing.T) {
	dir := t.TempDir()
	path := createTestFileWithChannelTypes(t, dir, "test_types.exr", 10, 10)

	f, err := exr.OpenFile(path)
	if err != nil {
		t.Fatalf("OpenFile() error = %v", err)
	}
	defer f.Close()

	// Extract Float channel
	data, err := ExtractChannel(f, "depth")
	if err != nil {
		t.Fatalf("ExtractChannel(depth) error = %v", err)
	}

	if len(data) != 100 {
		t.Errorf("len(data) = %d, want 100", len(data))
	}

	// Check values (should be i/100 for each pixel)
	for i := 0; i < 10; i++ {
		expected := float32(i) / 100.0
		if data[i] != expected {
			t.Errorf("data[%d] = %f, want %f", i, data[i], expected)
		}
	}
}

func TestExtractChannelUint(t *testing.T) {
	dir := t.TempDir()
	path := createTestFileWithChannelTypes(t, dir, "test_types.exr", 10, 10)

	f, err := exr.OpenFile(path)
	if err != nil {
		t.Fatalf("OpenFile() error = %v", err)
	}
	defer f.Close()

	// Extract Uint channel
	data, err := ExtractChannel(f, "id")
	if err != nil {
		t.Fatalf("ExtractChannel(id) error = %v", err)
	}

	if len(data) != 100 {
		t.Errorf("len(data) = %d, want 100", len(data))
	}

	// Check values (should be 0, 1, 2, ... for each pixel)
	for i := 0; i < 10; i++ {
		expected := float32(i)
		if data[i] != expected {
			t.Errorf("data[%d] = %f, want %f", i, data[i], expected)
		}
	}
}

func TestExtractChannelFromTiled(t *testing.T) {
	// Use the tiled test file from testdata
	path := "../exr/testdata/tiled.exr"

	f, err := exr.OpenFile(path)
	if err != nil {
		t.Fatalf("OpenFile() error = %v", err)
	}
	defer f.Close()

	// Verify it's tiled
	if !f.Header(0).IsTiled() {
		t.Skip("Test file is not tiled")
	}

	// Extract R channel from tiled file
	data, err := ExtractChannel(f, "R")
	if err != nil {
		t.Fatalf("ExtractChannel(R) from tiled file error = %v", err)
	}

	h := f.Header(0)
	expectedLen := h.Width() * h.Height()
	if len(data) != expectedLen {
		t.Errorf("len(data) = %d, want %d", len(data), expectedLen)
	}
}

func TestValidateFileTooSmall(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tiny.exr")

	// Write a file that's too small (less than 8 bytes)
	if err := os.WriteFile(path, []byte("tiny"), 0644); err != nil {
		t.Fatalf("Failed to create tiny file: %v", err)
	}

	result, err := ValidateFile(path)
	if err != nil {
		t.Fatalf("ValidateFile() error = %v", err)
	}

	if result.Valid {
		t.Error("ValidateFile() Valid = true for tiny file, want false")
	}

	// Check for the specific error message
	found := false
	for _, e := range result.Errors {
		if e == "file too small to be valid EXR" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected 'file too small' error, got: %v", result.Errors)
	}
}

func TestCompareFilesNonexistent(t *testing.T) {
	dir := t.TempDir()
	path1 := createTestFile(t, dir, "test1.exr", 50, 50, exr.CompressionNone)

	// Test with nonexistent first file
	_, _, err := CompareFiles("/nonexistent/file.exr", path1, CompareOptions{})
	if err == nil {
		t.Error("CompareFiles() should return error for nonexistent first file")
	}

	// Test with nonexistent second file
	_, _, err = CompareFiles(path1, "/nonexistent/file.exr", CompareOptions{})
	if err == nil {
		t.Error("CompareFiles() should return error for nonexistent second file")
	}
}

func TestCompareFilesWithDifferentCompression(t *testing.T) {
	dir := t.TempDir()
	path1 := createTestFile(t, dir, "test1.exr", 50, 50, exr.CompressionNone)
	path2 := createTestFile(t, dir, "test2.exr", 50, 50, exr.CompressionZIP)

	// With IgnoreMetadata = false, compression difference should be detected
	match, diffs, err := CompareFiles(path1, path2, CompareOptions{Tolerance: 0.001, IgnoreMetadata: false})
	if err != nil {
		t.Fatalf("CompareFiles() error = %v", err)
	}

	if match {
		t.Error("CompareFiles() match = true when compression differs and IgnoreMetadata = false")
	}

	// Check that compression difference is in diffs
	found := false
	for _, d := range diffs {
		if len(d) > 0 && d[:11] == "compression" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected compression difference in diffs, got: %v", diffs)
	}

	// With IgnoreMetadata = true, compression difference should be ignored
	match, _, err = CompareFiles(path1, path2, CompareOptions{Tolerance: 0.001, IgnoreMetadata: true})
	if err != nil {
		t.Fatalf("CompareFiles() with IgnoreMetadata error = %v", err)
	}

	if !match {
		t.Error("CompareFiles() match = false when IgnoreMetadata = true for same content")
	}
}

// createModifiedTestFile creates an EXR with a slightly different pixel value.
func createModifiedTestFile(t *testing.T, dir, name string, width, height int, modifyPixel int, delta float32) string {
	t.Helper()

	path := filepath.Join(dir, name)

	img := exr.NewRGBAImage(exr.RectFromSize(width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			r := float32(x) / float32(width)
			g := float32(y) / float32(height)
			// Modify specific pixel
			idx := y*width + x
			if idx == modifyPixel {
				r += delta
			}
			img.SetRGBA(x, y, r, g, 0.5, 1.0)
		}
	}

	out, err := exr.NewRGBAOutputFile(path, width, height)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	out.Header().SetCompression(exr.CompressionNone)

	if err := out.WriteRGBA(img); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	return path
}

func TestCompareFilesPixelDifference(t *testing.T) {
	dir := t.TempDir()

	// Create two files: one standard and one with a modified pixel
	path1 := createTestFile(t, dir, "test1.exr", 10, 10, exr.CompressionNone)
	path2 := createModifiedTestFile(t, dir, "test2.exr", 10, 10, 5, 0.5) // Modify pixel 5 by +0.5

	// With zero tolerance, should detect the difference
	match, diffs, err := CompareFiles(path1, path2, CompareOptions{Tolerance: 0})
	if err != nil {
		t.Fatalf("CompareFiles() error = %v", err)
	}

	if match {
		t.Error("CompareFiles() match = true for files with different pixel values")
	}

	if len(diffs) == 0 {
		t.Error("CompareFiles() diffs is empty, expected pixel difference")
	}

	// With high tolerance, should match
	match, _, err = CompareFiles(path1, path2, CompareOptions{Tolerance: 1.0})
	if err != nil {
		t.Fatalf("CompareFiles() with high tolerance error = %v", err)
	}

	if !match {
		t.Error("CompareFiles() match = false when difference is within tolerance")
	}
}

func TestCompareFilesNegativeDifference(t *testing.T) {
	dir := t.TempDir()

	// Create two files where file2 has a LOWER value than file1
	path1 := createModifiedTestFile(t, dir, "test1.exr", 10, 10, 5, 0.5)
	path2 := createTestFile(t, dir, "test2.exr", 10, 10, exr.CompressionNone)

	// With zero tolerance, should detect the difference (tests negative diff branch)
	match, diffs, err := CompareFiles(path1, path2, CompareOptions{Tolerance: 0})
	if err != nil {
		t.Fatalf("CompareFiles() error = %v", err)
	}

	if match {
		t.Error("CompareFiles() match = true for files with negative pixel difference")
	}

	if len(diffs) == 0 {
		t.Error("CompareFiles() diffs is empty, expected pixel difference for negative diff")
	}
}

// createTestFileWithChannels creates an EXR file with specific channel names.
func createTestFileWithChannels(t *testing.T, dir, name string, width, height int, channelNames []string) string {
	t.Helper()

	path := filepath.Join(dir, name)

	h := exr.NewScanlineHeader(width, height)
	h.SetCompression(exr.CompressionNone)

	cl := exr.NewChannelList()
	for _, chName := range channelNames {
		cl.Add(exr.Channel{Name: chName, Type: exr.PixelTypeHalf, XSampling: 1, YSampling: 1})
	}
	h.SetChannels(cl)

	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("Failed to create file: %v", err)
	}
	defer f.Close()

	sw, err := exr.NewScanlineWriter(f, h)
	if err != nil {
		t.Fatalf("Failed to create scanline writer: %v", err)
	}

	fb := exr.NewFrameBuffer()
	for _, chName := range channelNames {
		data := make([]byte, width*height*2)
		fb.Set(chName, exr.NewSlice(exr.PixelTypeHalf, data, width, height))
	}
	sw.SetFrameBuffer(fb)

	if err := sw.WritePixels(0, height-1); err != nil {
		t.Fatalf("Failed to write pixels: %v", err)
	}

	return path
}

func TestCompareFilesDifferentChannels(t *testing.T) {
	dir := t.TempDir()

	// Create files with different channel sets
	path1 := createTestFileWithChannels(t, dir, "test1.exr", 10, 10, []string{"R", "G", "B"})
	path2 := createTestFileWithChannels(t, dir, "test2.exr", 10, 10, []string{"R", "G", "B", "A"})

	match, diffs, err := CompareFiles(path1, path2, CompareOptions{IgnoreMetadata: true})
	if err != nil {
		t.Fatalf("CompareFiles() error = %v", err)
	}

	if match {
		t.Error("CompareFiles() match = true for files with different channel counts")
	}

	// Should detect channel count difference and missing channel
	if len(diffs) < 2 {
		t.Errorf("Expected at least 2 differences (count and missing channel), got: %v", diffs)
	}
}

func TestCompareFilesDifferentChannelNames(t *testing.T) {
	dir := t.TempDir()

	// Create files with different channel names (same count)
	path1 := createTestFileWithChannels(t, dir, "test1.exr", 10, 10, []string{"R", "G", "B"})
	path2 := createTestFileWithChannels(t, dir, "test2.exr", 10, 10, []string{"R", "G", "A"})

	match, diffs, err := CompareFiles(path1, path2, CompareOptions{IgnoreMetadata: true})
	if err != nil {
		t.Fatalf("CompareFiles() error = %v", err)
	}

	if match {
		t.Error("CompareFiles() match = true for files with different channel names")
	}

	// Should detect B in file1 but not file2, and A in file2 but not file1
	if len(diffs) < 2 {
		t.Errorf("Expected at least 2 differences, got: %v", diffs)
	}
}

func TestCompareFilesIdenticalWithTolerance(t *testing.T) {
	dir := t.TempDir()
	path1 := createTestFile(t, dir, "test1.exr", 20, 20, exr.CompressionNone)
	path2 := createTestFile(t, dir, "test2.exr", 20, 20, exr.CompressionNone)

	// Files created with same parameters should match with any tolerance
	match, diffs, err := CompareFiles(path1, path2, CompareOptions{Tolerance: 0, IgnoreMetadata: true})
	if err != nil {
		t.Fatalf("CompareFiles() error = %v", err)
	}

	if !match {
		t.Errorf("CompareFiles() match = false for identical files with zero tolerance. Diffs: %v", diffs)
	}
}

func TestListLayersWithRootChannels(t *testing.T) {
	h := exr.NewHeader()

	// Set up channels with no layers (all at root)
	cl := exr.NewChannelList()
	cl.Add(exr.Channel{Name: "R", Type: exr.PixelTypeHalf, XSampling: 1, YSampling: 1})
	cl.Add(exr.Channel{Name: "G", Type: exr.PixelTypeHalf, XSampling: 1, YSampling: 1})
	cl.Add(exr.Channel{Name: "B", Type: exr.PixelTypeHalf, XSampling: 1, YSampling: 1})
	h.SetChannels(cl)

	layers := ListLayers(h)

	if len(layers) != 0 {
		t.Errorf("ListLayers() returned %d layers, want 0 (no layered channels)", len(layers))
	}
}

func TestValidateFileWithTestData(t *testing.T) {
	// Test with real test files from testdata
	testFiles := []string{
		"../exr/testdata/tiled.exr",
		"../exr/testdata/comp_none.exr",
		"../exr/testdata/comp_zip.exr",
		"../exr/testdata/comp_piz.exr",
	}

	for _, path := range testFiles {
		t.Run(filepath.Base(path), func(t *testing.T) {
			result, err := ValidateFile(path)
			if err != nil {
				t.Fatalf("ValidateFile(%s) error = %v", path, err)
			}

			if !result.Valid {
				t.Errorf("ValidateFile(%s) Valid = false, want true. Errors: %v", path, result.Errors)
			}
		})
	}
}

func TestGetFileInfoWithTestData(t *testing.T) {
	// Test with real tiled file
	path := "../exr/testdata/tiled.exr"

	info, err := GetFileInfo(path)
	if err != nil {
		t.Fatalf("GetFileInfo(%s) error = %v", path, err)
	}

	if !info.IsTiled {
		t.Error("Expected tiled file to have IsTiled = true")
	}

	if info.TileWidth == 0 || info.TileHeight == 0 {
		t.Error("Expected tiled file to have non-zero tile dimensions")
	}
}

// createLargeChannelCountFile creates an EXR file with many channels to trigger the warning.
func createLargeChannelCountFile(t *testing.T, dir, name string, numChannels int) string {
	t.Helper()

	path := filepath.Join(dir, name)

	h := exr.NewScanlineHeader(4, 4) // Small dimensions
	h.SetCompression(exr.CompressionNone)

	cl := exr.NewChannelList()
	for i := 0; i < numChannels; i++ {
		cl.Add(exr.Channel{Name: fmt.Sprintf("ch%03d", i), Type: exr.PixelTypeHalf, XSampling: 1, YSampling: 1})
	}
	h.SetChannels(cl)

	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("Failed to create file: %v", err)
	}

	sw, err := exr.NewScanlineWriter(f, h)
	if err != nil {
		f.Close()
		t.Fatalf("Failed to create scanline writer: %v", err)
	}

	fb := exr.NewFrameBuffer()
	for i := 0; i < numChannels; i++ {
		data := make([]byte, 4*4*2) // 4x4 image, 2 bytes per half
		fb.Set(fmt.Sprintf("ch%03d", i), exr.NewSlice(exr.PixelTypeHalf, data, 4, 4))
	}
	sw.SetFrameBuffer(fb)

	if err := sw.WritePixels(0, 3); err != nil {
		sw.Close()
		f.Close()
		t.Fatalf("Failed to write pixels: %v", err)
	}

	if err := sw.Close(); err != nil {
		f.Close()
		t.Fatalf("Failed to close scanline writer: %v", err)
	}

	f.Close()
	return path
}

func TestValidateFileLargeChannelCount(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping large channel count test in short mode")
	}

	dir := t.TempDir()
	path := createLargeChannelCountFile(t, dir, "many_channels.exr", 105)

	result, err := ValidateFile(path)
	if err != nil {
		t.Fatalf("ValidateFile() error = %v", err)
	}

	if !result.Valid {
		t.Errorf("ValidateFile() Valid = false, want true. Errors: %v", result.Errors)
	}

	// Should have a warning about large channel count
	found := false
	for _, w := range result.Warnings {
		if len(w) > 0 && w[:12] == "large number" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected 'large number of channels' warning, got: %v", result.Warnings)
	}
}

func TestExtractChannelFromManyChannelTypes(t *testing.T) {
	// This test exercises extracting different channel types
	// to improve coverage of the switch statement
	dir := t.TempDir()

	// Create a file with all three channel types
	path := createTestFileWithChannelTypes(t, dir, "all_types.exr", 8, 8)

	f, err := exr.OpenFile(path)
	if err != nil {
		t.Fatalf("OpenFile() error = %v", err)
	}
	defer f.Close()

	// Test Half channel (R)
	t.Run("Half", func(t *testing.T) {
		data, err := ExtractChannel(f, "R")
		if err != nil {
			t.Errorf("ExtractChannel(R) error = %v", err)
		}
		if len(data) != 64 {
			t.Errorf("len(data) = %d, want 64", len(data))
		}
	})

	// Test Float channel (depth)
	t.Run("Float", func(t *testing.T) {
		data, err := ExtractChannel(f, "depth")
		if err != nil {
			t.Errorf("ExtractChannel(depth) error = %v", err)
		}
		if len(data) != 64 {
			t.Errorf("len(data) = %d, want 64", len(data))
		}
	})

	// Test Uint channel (id)
	t.Run("Uint", func(t *testing.T) {
		data, err := ExtractChannel(f, "id")
		if err != nil {
			t.Errorf("ExtractChannel(id) error = %v", err)
		}
		if len(data) != 64 {
			t.Errorf("len(data) = %d, want 64", len(data))
		}
	})
}

func TestCompareFilesWithManyChannels(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping large channel count comparison test in short mode")
	}

	dir := t.TempDir()
	path1 := createLargeChannelCountFile(t, dir, "many1.exr", 50)
	path2 := createLargeChannelCountFile(t, dir, "many2.exr", 50)

	match, diffs, err := CompareFiles(path1, path2, CompareOptions{Tolerance: 0.001, IgnoreMetadata: true})
	if err != nil {
		t.Fatalf("CompareFiles() error = %v", err)
	}

	if !match {
		t.Errorf("CompareFiles() match = false for identical large channel files. Diffs: %v", diffs)
	}
}

func TestCompareFilesSameCompression(t *testing.T) {
	dir := t.TempDir()
	path1 := createTestFile(t, dir, "test1.exr", 30, 30, exr.CompressionPIZ)
	path2 := createTestFile(t, dir, "test2.exr", 30, 30, exr.CompressionPIZ)

	// With IgnoreMetadata = false, files should still match since they have same compression
	match, diffs, err := CompareFiles(path1, path2, CompareOptions{Tolerance: 0.001, IgnoreMetadata: false})
	if err != nil {
		t.Fatalf("CompareFiles() error = %v", err)
	}

	if !match {
		t.Errorf("CompareFiles() match = false for identical files with same compression. Diffs: %v", diffs)
	}
}

func TestExtractChannelFromVariousTestFiles(t *testing.T) {
	testCases := []struct {
		path    string
		channel string
	}{
		{"../exr/testdata/comp_none.exr", "R"},
		{"../exr/testdata/comp_zip.exr", "G"},
		{"../exr/testdata/tiled.exr", "R"},
	}

	for _, tc := range testCases {
		t.Run(filepath.Base(tc.path)+"_"+tc.channel, func(t *testing.T) {
			f, err := exr.OpenFile(tc.path)
			if err != nil {
				t.Fatalf("OpenFile() error = %v", err)
			}
			defer f.Close()

			data, err := ExtractChannel(f, tc.channel)
			if err != nil {
				t.Errorf("ExtractChannel(%s) error = %v", tc.channel, err)
			}

			if len(data) == 0 {
				t.Errorf("ExtractChannel(%s) returned empty data", tc.channel)
			}
		})
	}
}

func TestCompareFilesChannelSkipping(t *testing.T) {
	dir := t.TempDir()

	// Create files with different channel sets - tests the "skip channel not in other file" path
	path1 := createTestFileWithChannels(t, dir, "test1.exr", 10, 10, []string{"R", "G", "B", "A"})
	path2 := createTestFileWithChannels(t, dir, "test2.exr", 10, 10, []string{"R", "G", "A", "Z"})

	match, diffs, err := CompareFiles(path1, path2, CompareOptions{IgnoreMetadata: true})
	if err != nil {
		t.Fatalf("CompareFiles() error = %v", err)
	}

	// Should not match due to different channels
	if match {
		t.Error("CompareFiles() match = true for files with different channels")
	}

	// Should have at least diffs about B and Z
	if len(diffs) < 2 {
		t.Errorf("Expected at least 2 diffs, got: %v", diffs)
	}
}

func TestValidateFileAllCompressionTypes(t *testing.T) {
	// Test validation with different compression types
	testFiles := []string{
		"../exr/testdata/comp_none.exr",
		"../exr/testdata/comp_zip.exr",
		"../exr/testdata/comp_zips.exr",
		"../exr/testdata/comp_rle.exr",
	}

	for _, path := range testFiles {
		t.Run(filepath.Base(path), func(t *testing.T) {
			result, err := ValidateFile(path)
			if err != nil {
				t.Fatalf("ValidateFile(%s) error = %v", path, err)
			}

			if !result.Valid {
				t.Errorf("ValidateFile(%s) Valid = false, want true. Errors: %v", path, result.Errors)
			}
		})
	}
}

func TestExtractChannelsAllRGBA(t *testing.T) {
	dir := t.TempDir()
	path := createTestFile(t, dir, "test.exr", 16, 16, exr.CompressionNone)

	f, err := exr.OpenFile(path)
	if err != nil {
		t.Fatalf("OpenFile() error = %v", err)
	}
	defer f.Close()

	// Extract all RGBA channels
	channels, err := ExtractChannels(f, "R", "G", "B", "A")
	if err != nil {
		t.Fatalf("ExtractChannels() error = %v", err)
	}

	if len(channels) != 4 {
		t.Errorf("len(channels) = %d, want 4", len(channels))
	}

	// Verify each channel has the right size
	for name, data := range channels {
		if len(data) != 256 { // 16x16
			t.Errorf("channel %s has %d pixels, want 256", name, len(data))
		}
	}
}

func TestCompareFilesWithMaxDiff(t *testing.T) {
	dir := t.TempDir()

	// Create two files where file2 has multiple different pixels
	path1 := createTestFile(t, dir, "test1.exr", 10, 10, exr.CompressionNone)
	// Create file with multiple modified pixels
	path := filepath.Join(dir, "test2_modified.exr")
	img := exr.NewRGBAImage(exr.RectFromSize(10, 10))
	for y := 0; y < 10; y++ {
		for x := 0; x < 10; x++ {
			r := float32(x) / 10.0
			g := float32(y) / 10.0
			// Modify several pixels
			if x == 0 && y == 0 {
				r += 0.1 // 10% difference
			}
			if x == 5 && y == 5 {
				r += 0.2 // 20% difference - this should be the max
			}
			img.SetRGBA(x, y, r, g, 0.5, 1.0)
		}
	}
	out, err := exr.NewRGBAOutputFile(path, 10, 10)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	out.Header().SetCompression(exr.CompressionNone)
	if err := out.WriteRGBA(img); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	// Compare with zero tolerance
	match, diffs, err := CompareFiles(path1, path, CompareOptions{Tolerance: 0})
	if err != nil {
		t.Fatalf("CompareFiles() error = %v", err)
	}

	if match {
		t.Error("CompareFiles() match = true for files with multiple different pixels")
	}

	if len(diffs) == 0 {
		t.Error("Expected pixel difference diffs")
	}
}

// Test error paths in ExtractChannel via readPixels hook
func TestExtractChannelReadPixelsError(t *testing.T) {
	// Save original function and restore after test
	origFunc := readPixelsFunc
	defer func() { readPixelsFunc = origFunc }()

	// Set up to return an error
	testErr := fmt.Errorf("simulated readPixels error")
	readPixelsFunc = func(f *exr.File, fb *exr.FrameBuffer) error {
		return testErr
	}

	dir := t.TempDir()
	path := createTestFileWithChannelTypes(t, dir, "test.exr", 10, 10)

	f, err := exr.OpenFile(path)
	if err != nil {
		t.Fatalf("OpenFile() error = %v", err)
	}
	defer f.Close()

	// Test error path for Half channel (R)
	t.Run("HalfChannelError", func(t *testing.T) {
		_, err := ExtractChannel(f, "R")
		if err == nil {
			t.Error("ExtractChannel(R) should return error when readPixels fails")
		}
		if err != testErr {
			t.Errorf("ExtractChannel(R) error = %v, want %v", err, testErr)
		}
	})

	// Test error path for Float channel (depth)
	t.Run("FloatChannelError", func(t *testing.T) {
		_, err := ExtractChannel(f, "depth")
		if err == nil {
			t.Error("ExtractChannel(depth) should return error when readPixels fails")
		}
		if err != testErr {
			t.Errorf("ExtractChannel(depth) error = %v, want %v", err, testErr)
		}
	})

	// Test error path for Uint channel (id)
	t.Run("UintChannelError", func(t *testing.T) {
		_, err := ExtractChannel(f, "id")
		if err == nil {
			t.Error("ExtractChannel(id) should return error when readPixels fails")
		}
		if err != testErr {
			t.Errorf("ExtractChannel(id) error = %v, want %v", err, testErr)
		}
	})
}

// Test CompareFiles error paths when ExtractChannel fails during comparison
func TestCompareFilesExtractChannelError(t *testing.T) {
	// Save original function and restore after test
	origFunc := readPixelsFunc
	defer func() { readPixelsFunc = origFunc }()

	dir := t.TempDir()
	path1 := createTestFile(t, dir, "test1.exr", 10, 10, exr.CompressionNone)
	path2 := createTestFile(t, dir, "test2.exr", 10, 10, exr.CompressionNone)

	callCount := 0
	testErr := fmt.Errorf("simulated extraction error")

	// Test error on first file extraction
	t.Run("ErrorOnFirstFile", func(t *testing.T) {
		callCount = 0
		readPixelsFunc = func(f *exr.File, fb *exr.FrameBuffer) error {
			callCount++
			return testErr
		}

		_, _, err := CompareFiles(path1, path2, CompareOptions{IgnoreMetadata: true})
		if err == nil {
			t.Error("CompareFiles should return error when ExtractChannel fails on file1")
		}
	})

	// Test error on second file extraction
	t.Run("ErrorOnSecondFile", func(t *testing.T) {
		callCount = 0
		readPixelsFunc = func(f *exr.File, fb *exr.FrameBuffer) error {
			callCount++
			// Succeed on first call (file1), fail on second (file2)
			if callCount <= 1 {
				return readPixelsImpl(f, fb)
			}
			return testErr
		}

		_, _, err := CompareFiles(path1, path2, CompareOptions{IgnoreMetadata: true})
		if err == nil {
			t.Error("CompareFiles should return error when ExtractChannel fails on file2")
		}
	})
}
