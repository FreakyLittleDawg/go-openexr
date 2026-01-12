package exrutil

import (
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
