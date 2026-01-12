package exrutil

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mrjoshuak/go-openexr/exr"
	"github.com/mrjoshuak/go-openexr/half"
)

// createTiledTestFile creates a tiled EXR file for testing.
func createTiledTestFile(t *testing.T, dir string, name string, width, height, tileSize int) string {
	t.Helper()

	path := filepath.Join(dir, name)

	// Create header for tiled image
	header := exr.NewTiledHeader(width, height, tileSize, tileSize)
	header.SetCompression(exr.CompressionZIP)

	// Add RGBA channels
	cl := exr.NewChannelList()
	cl.Add(exr.Channel{Name: "R", Type: exr.PixelTypeHalf, XSampling: 1, YSampling: 1})
	cl.Add(exr.Channel{Name: "G", Type: exr.PixelTypeHalf, XSampling: 1, YSampling: 1})
	cl.Add(exr.Channel{Name: "B", Type: exr.PixelTypeHalf, XSampling: 1, YSampling: 1})
	cl.Add(exr.Channel{Name: "A", Type: exr.PixelTypeHalf, XSampling: 1, YSampling: 1})
	header.SetChannels(cl)

	// Create pixel data
	pixels := width * height
	rData := make([]half.Half, pixels)
	gData := make([]half.Half, pixels)
	bData := make([]half.Half, pixels)
	aData := make([]half.Half, pixels)

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			idx := y*width + x
			rData[idx] = half.FromFloat32(float32(x) / float32(width))
			gData[idx] = half.FromFloat32(float32(y) / float32(height))
			bData[idx] = half.FromFloat32(0.5)
			aData[idx] = half.FromFloat32(1.0)
		}
	}

	// Create frame buffer
	fb := exr.NewFrameBuffer()
	fb.Insert("R", exr.NewSliceFromHalf(rData, width, height))
	fb.Insert("G", exr.NewSliceFromHalf(gData, width, height))
	fb.Insert("B", exr.NewSliceFromHalf(bData, width, height))
	fb.Insert("A", exr.NewSliceFromHalf(aData, width, height))

	// Write tiled file
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("Failed to create file: %v", err)
	}
	defer f.Close()

	writer, err := exr.NewTiledWriter(f, header)
	if err != nil {
		t.Fatalf("Failed to create tiled writer: %v", err)
	}

	writer.SetFrameBuffer(fb)

	// Write all tiles
	numXTiles := header.NumXTiles(0)
	numYTiles := header.NumYTiles(0)
	// WriteTiles takes (tileX1, tileY1, tileX2, tileY2)
	if err := writer.WriteTiles(0, 0, numXTiles-1, numYTiles-1); err != nil {
		t.Fatalf("Failed to write tiles: %v", err)
	}

	if err := writer.Close(); err != nil {
		t.Fatalf("Failed to close writer: %v", err)
	}

	return path
}

func TestExtractChannelFromTiledImage(t *testing.T) {
	dir := t.TempDir()
	path := createTiledTestFile(t, dir, "tiled.exr", 64, 64, 32)

	f, err := exr.OpenFile(path)
	if err != nil {
		t.Fatalf("Failed to open tiled file: %v", err)
	}
	defer f.Close()

	// Verify it's actually tiled
	if !f.Header(0).IsTiled() {
		t.Fatal("Test file is not tiled")
	}

	// Extract R channel
	data, err := ExtractChannel(f, "R")
	if err != nil {
		t.Fatalf("ExtractChannel(R) error: %v", err)
	}

	expectedLen := 64 * 64
	if len(data) != expectedLen {
		t.Errorf("len(data) = %d, want %d", len(data), expectedLen)
	}

	// Verify first row values (R should increase left to right)
	if data[0] != 0 {
		t.Errorf("data[0] = %f, want 0", data[0])
	}

	// Check a value in the middle
	midX := 32
	midY := 32
	midIdx := midY*64 + midX
	expectedR := float32(midX) / 64.0
	tolerance := float32(0.02) // Allow some tolerance for half precision
	if diff := data[midIdx] - expectedR; diff < -tolerance || diff > tolerance {
		t.Errorf("data[%d] = %f, want ~%f", midIdx, data[midIdx], expectedR)
	}
}

func TestExtractChannelsFromTiledImage(t *testing.T) {
	dir := t.TempDir()
	path := createTiledTestFile(t, dir, "tiled.exr", 32, 32, 16)

	f, err := exr.OpenFile(path)
	if err != nil {
		t.Fatalf("Failed to open tiled file: %v", err)
	}
	defer f.Close()

	// Extract multiple channels
	channels, err := ExtractChannels(f, "R", "G", "B")
	if err != nil {
		t.Fatalf("ExtractChannels error: %v", err)
	}

	if len(channels) != 3 {
		t.Errorf("len(channels) = %d, want 3", len(channels))
	}

	for _, name := range []string{"R", "G", "B"} {
		if _, ok := channels[name]; !ok {
			t.Errorf("Channel %q not found", name)
		}
	}

	// Verify each channel has correct length
	expectedLen := 32 * 32
	for name, data := range channels {
		if len(data) != expectedLen {
			t.Errorf("len(channels[%q]) = %d, want %d", name, len(data), expectedLen)
		}
	}
}

func TestGetFileInfoTiled(t *testing.T) {
	dir := t.TempDir()
	tileSize := 64
	path := createTiledTestFile(t, dir, "tiled.exr", 128, 128, tileSize)

	info, err := GetFileInfo(path)
	if err != nil {
		t.Fatalf("GetFileInfo error: %v", err)
	}

	if info.Width != 128 {
		t.Errorf("Width = %d, want 128", info.Width)
	}
	if info.Height != 128 {
		t.Errorf("Height = %d, want 128", info.Height)
	}
	if !info.IsTiled {
		t.Error("IsTiled = false, want true")
	}
	if info.TileWidth != tileSize {
		t.Errorf("TileWidth = %d, want %d", info.TileWidth, tileSize)
	}
	if info.TileHeight != tileSize {
		t.Errorf("TileHeight = %d, want %d", info.TileHeight, tileSize)
	}
	if len(info.Channels) != 4 {
		t.Errorf("len(Channels) = %d, want 4", len(info.Channels))
	}
}

func TestCompareTiledFiles(t *testing.T) {
	dir := t.TempDir()

	// Create two identical tiled files
	path1 := createTiledTestFile(t, dir, "tiled1.exr", 32, 32, 16)
	path2 := createTiledTestFile(t, dir, "tiled2.exr", 32, 32, 16)

	// Compare them
	match, diffs, err := CompareFiles(path1, path2, CompareOptions{Tolerance: 0.001})
	if err != nil {
		t.Fatalf("CompareFiles error: %v", err)
	}

	if !match {
		t.Errorf("Identical tiled files should match. Diffs: %v", diffs)
	}
}

func TestCompareTiledVsScanline(t *testing.T) {
	dir := t.TempDir()

	// Create a tiled file
	tiledPath := createTiledTestFile(t, dir, "tiled.exr", 32, 32, 16)

	// Create a scanline file with same content
	scanlinePath := createTestFile(t, dir, "scanline.exr", 32, 32, exr.CompressionNone)

	// Compare them - should have different compression but pixel data might differ slightly
	match, diffs, err := CompareFiles(tiledPath, scanlinePath, CompareOptions{
		Tolerance:      0.02, // Allow more tolerance due to different pixel fill
		IgnoreMetadata: true,
	})
	if err != nil {
		t.Fatalf("CompareFiles error: %v", err)
	}

	// Log results (we don't fail here because the test files have different pixel data)
	t.Logf("Tiled vs Scanline comparison: match=%v, diffs=%v", match, diffs)
}

func TestExtractChannelFromTiledNonMultipleOfTileSize(t *testing.T) {
	dir := t.TempDir()
	// Create tiled image where dimensions are not multiples of tile size
	// 100x100 with 32x32 tiles means partial tiles at edges
	path := createTiledTestFile(t, dir, "tiled_partial.exr", 100, 100, 32)

	f, err := exr.OpenFile(path)
	if err != nil {
		t.Fatalf("Failed to open tiled file: %v", err)
	}
	defer f.Close()

	// Extract channel - this tests handling of partial tiles
	data, err := ExtractChannel(f, "R")
	if err != nil {
		t.Fatalf("ExtractChannel error: %v", err)
	}

	expectedLen := 100 * 100
	if len(data) != expectedLen {
		t.Errorf("len(data) = %d, want %d", len(data), expectedLen)
	}

	// Verify corner values
	// Top-left
	if data[0] != 0 {
		t.Errorf("data[0] (top-left) = %f, want 0", data[0])
	}

	// Bottom-right should be close to 0.99
	lastIdx := 99*100 + 99
	expectedLast := float32(99) / 100.0
	tolerance := float32(0.02)
	if diff := data[lastIdx] - expectedLast; diff < -tolerance || diff > tolerance {
		t.Errorf("data[%d] (bottom-right) = %f, want ~%f", lastIdx, data[lastIdx], expectedLast)
	}
}
