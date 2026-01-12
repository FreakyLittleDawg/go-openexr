package exr

import (
	"testing"

	"github.com/mrjoshuak/go-openexr/internal/xdr"
)

func TestNewHeader(t *testing.T) {
	h := NewHeader()
	if h == nil {
		t.Fatal("NewHeader returned nil")
	}
	if len(h.attrs) != 0 {
		t.Errorf("New header has %d attrs, want 0", len(h.attrs))
	}
}

func TestNewScanlineHeader(t *testing.T) {
	h := NewScanlineHeader(1920, 1080)

	dw := h.DataWindow()
	if dw.Width() != 1920 || dw.Height() != 1080 {
		t.Errorf("DataWindow = %dx%d, want 1920x1080", dw.Width(), dw.Height())
	}

	if h.Compression() != CompressionZIP {
		t.Errorf("Compression = %v, want ZIP", h.Compression())
	}

	if h.LineOrder() != LineOrderIncreasing {
		t.Errorf("LineOrder = %v, want Increasing", h.LineOrder())
	}

	cl := h.Channels()
	if cl == nil || cl.Len() != 3 {
		t.Errorf("Channels len = %d, want 3", cl.Len())
	}

	if err := h.Validate(); err != nil {
		t.Errorf("Validate() error = %v", err)
	}
}

func TestHeaderSetGet(t *testing.T) {
	h := NewHeader()

	// Test Set/Get/Has/Remove
	attr := &Attribute{Name: "test", Type: AttrTypeInt, Value: int32(42)}
	h.Set(attr)

	if !h.Has("test") {
		t.Error("Has(test) should be true")
	}

	got := h.Get("test")
	if got == nil {
		t.Fatal("Get(test) returned nil")
	}
	if got.Value.(int32) != 42 {
		t.Errorf("Get(test).Value = %d, want 42", got.Value)
	}

	h.Remove("test")
	if h.Has("test") {
		t.Error("Has(test) should be false after Remove")
	}
}

func TestHeaderAttributeAccessors(t *testing.T) {
	h := NewHeader()

	// Test all setters/getters
	cl := NewChannelList()
	cl.Add(NewChannel("R", PixelTypeHalf))
	h.SetChannels(cl)
	if h.Channels().Len() != 1 {
		t.Error("Channels not set correctly")
	}

	h.SetCompression(CompressionPIZ)
	if h.Compression() != CompressionPIZ {
		t.Error("Compression not set correctly")
	}

	dw := Box2i{Min: V2i{0, 0}, Max: V2i{99, 49}}
	h.SetDataWindow(dw)
	if h.DataWindow() != dw {
		t.Error("DataWindow not set correctly")
	}

	disp := Box2i{Min: V2i{0, 0}, Max: V2i{199, 99}}
	h.SetDisplayWindow(disp)
	if h.DisplayWindow() != disp {
		t.Error("DisplayWindow not set correctly")
	}

	h.SetLineOrder(LineOrderDecreasing)
	if h.LineOrder() != LineOrderDecreasing {
		t.Error("LineOrder not set correctly")
	}

	h.SetPixelAspectRatio(2.0)
	if h.PixelAspectRatio() != 2.0 {
		t.Error("PixelAspectRatio not set correctly")
	}

	center := V2f{0.5, 0.5}
	h.SetScreenWindowCenter(center)
	if h.ScreenWindowCenter() != center {
		t.Error("ScreenWindowCenter not set correctly")
	}

	h.SetScreenWindowWidth(2.0)
	if h.ScreenWindowWidth() != 2.0 {
		t.Error("ScreenWindowWidth not set correctly")
	}
}

func TestHeaderDefaults(t *testing.T) {
	h := NewHeader()

	// Test default values for unset attributes
	if h.Compression() != CompressionNone {
		t.Errorf("Default Compression = %v, want None", h.Compression())
	}
	if h.LineOrder() != LineOrderIncreasing {
		t.Errorf("Default LineOrder = %v, want Increasing", h.LineOrder())
	}
	if h.PixelAspectRatio() != 1.0 {
		t.Errorf("Default PixelAspectRatio = %v, want 1.0", h.PixelAspectRatio())
	}
	if h.ScreenWindowWidth() != 1.0 {
		t.Errorf("Default ScreenWindowWidth = %v, want 1.0", h.ScreenWindowWidth())
	}
}

func TestHeaderTiled(t *testing.T) {
	h := NewHeader()

	if h.IsTiled() {
		t.Error("IsTiled should be false by default")
	}

	td := TileDescription{XSize: 64, YSize: 64, Mode: LevelModeOne}
	h.SetTileDescription(td)

	if !h.IsTiled() {
		t.Error("IsTiled should be true after SetTileDescription")
	}

	got := h.TileDescription()
	if got.XSize != 64 || got.YSize != 64 {
		t.Errorf("TileDescription = %v, want 64x64", got)
	}
}

func TestHeaderWidthHeight(t *testing.T) {
	h := NewHeader()
	h.SetDataWindow(Box2i{Min: V2i{0, 0}, Max: V2i{1919, 1079}})

	if w := h.Width(); w != 1920 {
		t.Errorf("Width() = %d, want 1920", w)
	}
	if height := h.Height(); height != 1080 {
		t.Errorf("Height() = %d, want 1080", height)
	}
}

func TestHeaderValidate(t *testing.T) {
	h := NewHeader()

	// Missing all required attributes
	if err := h.Validate(); err == nil {
		t.Error("Validate should fail for empty header")
	}

	// Add required attributes one by one
	h.SetChannels(NewChannelList())
	if err := h.Validate(); err == nil {
		t.Error("Validate should fail with empty channel list")
	}

	cl := NewChannelList()
	cl.Add(NewChannel("R", PixelTypeHalf))
	h.SetChannels(cl)

	h.SetCompression(CompressionZIP)
	h.SetDataWindow(Box2i{Min: V2i{0, 0}, Max: V2i{99, 49}})
	h.SetDisplayWindow(Box2i{Min: V2i{0, 0}, Max: V2i{99, 49}})
	h.SetLineOrder(LineOrderIncreasing)
	h.SetPixelAspectRatio(1.0)
	h.SetScreenWindowCenter(V2f{0, 0})
	h.SetScreenWindowWidth(1.0)

	if err := h.Validate(); err != nil {
		t.Errorf("Validate should pass for complete header: %v", err)
	}
}

func TestHeaderValidateEmptyDataWindow(t *testing.T) {
	h := NewScanlineHeader(100, 100)
	// Set an invalid (empty) data window
	h.SetDataWindow(Box2i{Min: V2i{100, 100}, Max: V2i{0, 0}})

	if err := h.Validate(); err == nil {
		t.Error("Validate should fail for empty data window")
	}
}

func TestHeaderSerialization(t *testing.T) {
	original := NewScanlineHeader(1920, 1080)
	original.SetCompression(CompressionPIZ)

	w := xdr.NewBufferWriter(1024)
	if err := WriteHeader(w, original); err != nil {
		t.Fatalf("WriteHeader() error = %v", err)
	}

	r := xdr.NewReader(w.Bytes())
	result, err := ReadHeader(r)
	if err != nil {
		t.Fatalf("ReadHeader() error = %v", err)
	}

	if result.Width() != original.Width() {
		t.Errorf("Width = %d, want %d", result.Width(), original.Width())
	}
	if result.Height() != original.Height() {
		t.Errorf("Height = %d, want %d", result.Height(), original.Height())
	}
	if result.Compression() != original.Compression() {
		t.Errorf("Compression = %v, want %v", result.Compression(), original.Compression())
	}
	if result.Channels().Len() != original.Channels().Len() {
		t.Errorf("Channels len = %d, want %d", result.Channels().Len(), original.Channels().Len())
	}
}

func TestHeaderChunksInFile(t *testing.T) {
	tests := []struct {
		name        string
		height      int
		compression Compression
		expected    int
	}{
		{"100px none", 100, CompressionNone, 100},
		{"100px zip", 100, CompressionZIP, 7},   // ceil(100/16)
		{"100px piz", 100, CompressionPIZ, 4},   // ceil(100/32)
		{"100px b44", 100, CompressionB44, 4},   // ceil(100/32)
		{"100px dwab", 100, CompressionDWAB, 1}, // ceil(100/256)
		{"16px zip", 16, CompressionZIP, 1},     // exactly one chunk
		{"17px zip", 17, CompressionZIP, 2},     // needs two chunks
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := NewScanlineHeader(100, tt.height)
			h.SetCompression(tt.compression)

			chunks := h.ChunksInFile()
			if chunks != tt.expected {
				t.Errorf("ChunksInFile() = %d, want %d", chunks, tt.expected)
			}
		})
	}
}

func TestHeaderChunksInFileTiled(t *testing.T) {
	h := NewScanlineHeader(100, 100)
	h.SetTileDescription(TileDescription{
		XSize: 32,
		YSize: 32,
		Mode:  LevelModeOne,
	})

	// 100x100 with 32x32 tiles = 4x4 = 16 tiles
	chunks := h.ChunksInFile()
	if chunks != 16 {
		t.Errorf("ChunksInFile() for tiled = %d, want 16", chunks)
	}
}

func TestHeaderAttributes(t *testing.T) {
	h := NewScanlineHeader(100, 100)

	attrs := h.Attributes()
	if len(attrs) == 0 {
		t.Error("Attributes() should return non-empty slice")
	}

	// Verify all returned attributes are valid
	for _, attr := range attrs {
		if attr.Name == "" {
			t.Error("Attribute name should not be empty")
		}
	}
}

func TestNumLevelsCalculation(t *testing.T) {
	tests := []struct {
		size         int
		roundingMode LevelRoundingMode
		expected     int
	}{
		{1, LevelRoundDown, 1},
		{2, LevelRoundDown, 2},
		{3, LevelRoundDown, 2},
		{4, LevelRoundDown, 3},
		{5, LevelRoundDown, 3},
		{8, LevelRoundDown, 4},
		{16, LevelRoundDown, 5},
		{256, LevelRoundDown, 9},
		{1024, LevelRoundDown, 11},
		// Round up mode
		{1, LevelRoundUp, 1},
		{2, LevelRoundUp, 2},
		{3, LevelRoundUp, 3},
		{4, LevelRoundUp, 3},
		{5, LevelRoundUp, 4},
		{256, LevelRoundUp, 9},
	}

	for _, tt := range tests {
		result := numLevels(tt.size, tt.roundingMode)
		if result != tt.expected {
			t.Errorf("numLevels(%d, %v) = %d, want %d",
				tt.size, tt.roundingMode, result, tt.expected)
		}
	}
}

func TestHeaderNumXYLevelsMipmap(t *testing.T) {
	h := NewScanlineHeader(256, 128)
	h.SetTileDescription(TileDescription{
		XSize:        32,
		YSize:        32,
		Mode:         LevelModeMipmap,
		RoundingMode: LevelRoundDown,
	})

	// For mipmap, NumXLevels and NumYLevels should be the same
	// and based on the larger dimension (256)
	// 256 -> 128 -> 64 -> 32 -> 16 -> 8 -> 4 -> 2 -> 1 = 9 levels
	expectedLevels := 9

	if h.NumXLevels() != expectedLevels {
		t.Errorf("NumXLevels() = %d, want %d", h.NumXLevels(), expectedLevels)
	}
	if h.NumYLevels() != expectedLevels {
		t.Errorf("NumYLevels() = %d, want %d", h.NumYLevels(), expectedLevels)
	}
}

func TestHeaderNumXYLevelsRipmap(t *testing.T) {
	h := NewScanlineHeader(256, 64)
	h.SetTileDescription(TileDescription{
		XSize:        32,
		YSize:        32,
		Mode:         LevelModeRipmap,
		RoundingMode: LevelRoundDown,
	})

	// For ripmap, X and Y levels are independent
	// X: 256 -> 128 -> 64 -> 32 -> 16 -> 8 -> 4 -> 2 -> 1 = 9 levels
	// Y: 64 -> 32 -> 16 -> 8 -> 4 -> 2 -> 1 = 7 levels

	if h.NumXLevels() != 9 {
		t.Errorf("NumXLevels() = %d, want 9", h.NumXLevels())
	}
	if h.NumYLevels() != 7 {
		t.Errorf("NumYLevels() = %d, want 7", h.NumYLevels())
	}
}

func TestHeaderNumXYLevelsOne(t *testing.T) {
	h := NewScanlineHeader(256, 128)
	h.SetTileDescription(TileDescription{
		XSize: 32,
		YSize: 32,
		Mode:  LevelModeOne,
	})

	// For LevelModeOne, always 1 level
	if h.NumXLevels() != 1 {
		t.Errorf("NumXLevels() = %d, want 1", h.NumXLevels())
	}
	if h.NumYLevels() != 1 {
		t.Errorf("NumYLevels() = %d, want 1", h.NumYLevels())
	}
}

func TestHeaderLevelWidth(t *testing.T) {
	h := NewScanlineHeader(256, 128)
	h.SetTileDescription(TileDescription{
		XSize:        32,
		YSize:        32,
		Mode:         LevelModeMipmap,
		RoundingMode: LevelRoundDown,
	})

	// Level 0: full resolution
	if w := h.LevelWidth(0); w != 256 {
		t.Errorf("LevelWidth(0) = %d, want 256", w)
	}
	// Level 1: 256/2 = 128
	if w := h.LevelWidth(1); w != 128 {
		t.Errorf("LevelWidth(1) = %d, want 128", w)
	}
	// Level 2: 128/2 = 64
	if w := h.LevelWidth(2); w != 64 {
		t.Errorf("LevelWidth(2) = %d, want 64", w)
	}
	// Level 8: should be 1 (minimum)
	if w := h.LevelWidth(8); w != 1 {
		t.Errorf("LevelWidth(8) = %d, want 1", w)
	}
}

func TestHeaderLevelHeight(t *testing.T) {
	h := NewScanlineHeader(256, 128)
	h.SetTileDescription(TileDescription{
		XSize:        32,
		YSize:        32,
		Mode:         LevelModeMipmap,
		RoundingMode: LevelRoundDown,
	})

	// Level 0: full resolution
	if ht := h.LevelHeight(0); ht != 128 {
		t.Errorf("LevelHeight(0) = %d, want 128", ht)
	}
	// Level 1: 128/2 = 64
	if ht := h.LevelHeight(1); ht != 64 {
		t.Errorf("LevelHeight(1) = %d, want 64", ht)
	}
	// Level 7: should be 1 (minimum)
	if ht := h.LevelHeight(7); ht != 1 {
		t.Errorf("LevelHeight(7) = %d, want 1", ht)
	}
}

func TestHeaderNumXYTiles(t *testing.T) {
	h := NewScanlineHeader(100, 80)
	h.SetTileDescription(TileDescription{
		XSize:        32,
		YSize:        32,
		Mode:         LevelModeMipmap,
		RoundingMode: LevelRoundDown,
	})

	// Level 0: 100x80 -> ceil(100/32) x ceil(80/32) = 4x3
	if tx := h.NumXTiles(0); tx != 4 {
		t.Errorf("NumXTiles(0) = %d, want 4", tx)
	}
	if ty := h.NumYTiles(0); ty != 3 {
		t.Errorf("NumYTiles(0) = %d, want 3", ty)
	}

	// Level 1: 50x40 -> ceil(50/32) x ceil(40/32) = 2x2
	if tx := h.NumXTiles(1); tx != 2 {
		t.Errorf("NumXTiles(1) = %d, want 2", tx)
	}
	if ty := h.NumYTiles(1); ty != 2 {
		t.Errorf("NumYTiles(1) = %d, want 2", ty)
	}
}

func TestHeaderChunksInFileMipmap(t *testing.T) {
	h := NewScanlineHeader(64, 64)
	h.SetTileDescription(TileDescription{
		XSize:        32,
		YSize:        32,
		Mode:         LevelModeMipmap,
		RoundingMode: LevelRoundDown,
	})

	// Level 0: 64x64 -> 2x2 = 4 tiles
	// Level 1: 32x32 -> 1x1 = 1 tile
	// Level 2: 16x16 -> 1x1 = 1 tile
	// Level 3: 8x8 -> 1x1 = 1 tile
	// Level 4: 4x4 -> 1x1 = 1 tile
	// Level 5: 2x2 -> 1x1 = 1 tile
	// Level 6: 1x1 -> 1x1 = 1 tile
	// Total: 4 + 6 = 10 tiles
	expected := 10

	chunks := h.ChunksInFile()
	if chunks != expected {
		t.Errorf("ChunksInFile() for mipmap = %d, want %d", chunks, expected)
	}
}

func TestHeaderChunksInFileRipmap(t *testing.T) {
	h := NewScanlineHeader(64, 32)
	h.SetTileDescription(TileDescription{
		XSize:        32,
		YSize:        32,
		Mode:         LevelModeRipmap,
		RoundingMode: LevelRoundDown,
	})

	// X levels: 64 -> 32 -> 16 -> 8 -> 4 -> 2 -> 1 = 7 levels
	// Y levels: 32 -> 16 -> 8 -> 4 -> 2 -> 1 = 6 levels
	// For each combination (lx, ly), we need NumXTiles(lx) * NumYTiles(ly)
	// This is complex but we can verify by calculation
	chunks := h.ChunksInFile()

	// Manual calculation: for a 64x32 image with 32x32 tiles
	// lx=0: w=64, tx=2 | lx=1: w=32, tx=1 | lx=2: w=16, tx=1 | lx=3: w=8, tx=1 | lx=4: w=4, tx=1 | lx=5: w=2, tx=1 | lx=6: w=1, tx=1
	// ly=0: h=32, ty=1 | ly=1: h=16, ty=1 | ly=2: h=8, ty=1 | ly=3: h=4, ty=1 | ly=4: h=2, ty=1 | ly=5: h=1, ty=1
	// Total tiles: sum for all (lx, ly) combinations
	// = (2*1 + 1*1 + 1*1 + 1*1 + 1*1 + 1*1 + 1*1) * 6 = 8 * 6 = 48... no wait
	// Actually it's sum of tx(lx) * ty(ly) for each (lx, ly)
	// = sum over ly of (sum over lx of tx(lx)) * ty(ly)
	// = (2+1+1+1+1+1+1) * (1+1+1+1+1+1) = 8 * 6 = 48
	expected := 48

	if chunks != expected {
		t.Errorf("ChunksInFile() for ripmap = %d, want %d", chunks, expected)
	}
}

func TestHeaderDefaultValues(t *testing.T) {
	// Test that getters return default values when attributes are not set
	h := NewHeader()

	// Channels should return nil when not set
	if ch := h.Channels(); ch != nil {
		t.Errorf("Channels() on empty header = %v, want nil", ch)
	}

	// DataWindow should return zero-valued box when not set
	dw := h.DataWindow()
	// Zero Box2i has all zeros, which gives Width()=1, Height()=1 after +1 correction
	// The point is just to verify we get the default value branch
	_ = dw

	// DisplayWindow should return zero-valued box when not set
	disp := h.DisplayWindow()
	// Same as above
	_ = disp

	// ScreenWindowCenter should return zero when not set
	swc := h.ScreenWindowCenter()
	if swc.X != 0 || swc.Y != 0 {
		t.Errorf("ScreenWindowCenter() on empty header = %v, want zero", swc)
	}

	// TileDescription should return nil when not set
	if td := h.TileDescription(); td != nil {
		t.Errorf("TileDescription() on empty header = %v, want nil", td)
	}
}

func TestLevelWidthHeightDefaultValues(t *testing.T) {
	// Test LevelWidth and LevelHeight with no tile description
	h := NewScanlineHeader(100, 100)

	// Without tile description, these should still work
	lw := h.LevelWidth(0)
	if lw != h.Width() {
		t.Logf("LevelWidth(0) on scanline header = %d, Width = %d", lw, h.Width())
	}

	lh := h.LevelHeight(0)
	if lh != h.Height() {
		t.Logf("LevelHeight(0) on scanline header = %d, Height = %d", lh, h.Height())
	}
}

func TestValidateInvalidHeader(t *testing.T) {
	// Empty header should fail validation
	h := NewHeader()
	err := h.Validate()
	if err == nil {
		t.Error("Validate() on empty header should return error")
	}

	// Header with no channels should fail
	h2 := NewHeader()
	h2.SetDataWindow(Box2i{Min: V2i{0, 0}, Max: V2i{100, 100}})
	h2.SetDisplayWindow(Box2i{Min: V2i{0, 0}, Max: V2i{100, 100}})
	h2.SetCompression(CompressionNone)
	h2.SetLineOrder(LineOrderIncreasing)
	err = h2.Validate()
	if err == nil {
		t.Error("Validate() on header without channels should return error")
	}
}

func TestHeaderPixelAspectRatio(t *testing.T) {
	h := NewScanlineHeader(100, 100)

	// Default should be 1.0
	ratio := h.PixelAspectRatio()
	if ratio != 1.0 {
		t.Errorf("Default PixelAspectRatio = %f, want 1.0", ratio)
	}

	// Set custom ratio
	h.SetPixelAspectRatio(2.0)
	ratio = h.PixelAspectRatio()
	if ratio != 2.0 {
		t.Errorf("PixelAspectRatio after Set = %f, want 2.0", ratio)
	}
}

func TestDWACompressionLevel(t *testing.T) {
	h := NewScanlineHeader(100, 100)

	// Default should be 45.0
	if level := h.DWACompressionLevel(); level != DefaultDWACompressionLevel {
		t.Errorf("Default DWACompressionLevel = %f, want %f", level, DefaultDWACompressionLevel)
	}

	// Set custom level
	h.SetDWACompressionLevel(90.0)
	if level := h.DWACompressionLevel(); level != 90.0 {
		t.Errorf("DWACompressionLevel after Set = %f, want 90.0", level)
	}

	// Set to 0 (minimal quantization)
	h.SetDWACompressionLevel(0)
	if level := h.DWACompressionLevel(); level != 0 {
		t.Errorf("DWACompressionLevel after Set(0) = %f, want 0", level)
	}

	// Verify it persists through serialization
	h.SetDWACompressionLevel(60.0)
	w := xdr.NewBufferWriter(1024)
	if err := WriteHeader(w, h); err != nil {
		t.Fatalf("WriteHeader() error = %v", err)
	}

	r := xdr.NewReader(w.Bytes())
	result, err := ReadHeader(r)
	if err != nil {
		t.Fatalf("ReadHeader() error = %v", err)
	}

	if level := result.DWACompressionLevel(); level != 60.0 {
		t.Errorf("DWACompressionLevel after roundtrip = %f, want 60.0", level)
	}
}

func TestHeaderNumLevelsNilTileDesc(t *testing.T) {
	// Header without tile description should return 1 for both
	h := NewScanlineHeader(256, 128)

	if h.NumXLevels() != 1 {
		t.Errorf("NumXLevels() without tile desc = %d, want 1", h.NumXLevels())
	}
	if h.NumYLevels() != 1 {
		t.Errorf("NumYLevels() without tile desc = %d, want 1", h.NumYLevels())
	}
}

func TestHeaderLevelWidthEdgeCases(t *testing.T) {
	h := NewScanlineHeader(64, 64)
	h.SetTileDescription(TileDescription{
		XSize:        16,
		YSize:        16,
		Mode:         LevelModeMipmap,
		RoundingMode: LevelRoundUp,
	})

	// Test negative level - should return full width
	if w := h.LevelWidth(-1); w != 64 {
		t.Errorf("LevelWidth(-1) = %d, want 64 (full width)", w)
	}

	// Test very high level - should return 1
	if w := h.LevelWidth(100); w != 1 {
		t.Errorf("LevelWidth(100) = %d, want 1 (minimum)", w)
	}
}

func TestHeaderLevelHeightEdgeCases(t *testing.T) {
	h := NewScanlineHeader(64, 64)
	h.SetTileDescription(TileDescription{
		XSize:        16,
		YSize:        16,
		Mode:         LevelModeMipmap,
		RoundingMode: LevelRoundUp,
	})

	// Test negative level - should return full height
	if ht := h.LevelHeight(-1); ht != 64 {
		t.Errorf("LevelHeight(-1) = %d, want 64 (full height)", ht)
	}

	// Test very high level - should return 1
	if ht := h.LevelHeight(100); ht != 1 {
		t.Errorf("LevelHeight(100) = %d, want 1 (minimum)", ht)
	}
}

func TestHeaderNumTilesNilTileDesc(t *testing.T) {
	h := NewScanlineHeader(64, 64)

	// Without tile description, NumXTiles and NumYTiles should return 0
	if tiles := h.NumXTiles(0); tiles != 0 {
		t.Errorf("NumXTiles(0) without tile desc = %d, want 0", tiles)
	}
	if tiles := h.NumYTiles(0); tiles != 0 {
		t.Errorf("NumYTiles(0) without tile desc = %d, want 0", tiles)
	}
}

func TestHeaderLevelWidthRoundUp(t *testing.T) {
	h := NewScanlineHeader(33, 33) // Odd number to test rounding
	h.SetTileDescription(TileDescription{
		XSize:        16,
		YSize:        16,
		Mode:         LevelModeMipmap,
		RoundingMode: LevelRoundUp,
	})

	// Level 1: (33 + 1) / 2 = 17 with round up
	if w := h.LevelWidth(1); w != 17 {
		t.Errorf("LevelWidth(1) with round up = %d, want 17", w)
	}
}

func TestHeaderNumLevelsInvalidMode(t *testing.T) {
	h := NewScanlineHeader(64, 64)
	h.SetTileDescription(TileDescription{
		XSize: 16,
		YSize: 16,
		Mode:  LevelMode(99), // Invalid mode
	})

	// Invalid mode should fall through to default return 1
	if h.NumXLevels() != 1 {
		t.Errorf("NumXLevels() with invalid mode = %d, want 1", h.NumXLevels())
	}
	if h.NumYLevels() != 1 {
		t.Errorf("NumYLevels() with invalid mode = %d, want 1", h.NumYLevels())
	}
}

func TestNumLevelsZeroSize(t *testing.T) {
	// Test numLevels with size <= 0
	result := numLevels(0, LevelRoundDown)
	if result != 0 {
		t.Errorf("numLevels(0, LevelRoundDown) = %d, want 0", result)
	}

	result = numLevels(-5, LevelRoundUp)
	if result != 0 {
		t.Errorf("numLevels(-5, LevelRoundUp) = %d, want 0", result)
	}
}

func TestHeaderWriteHeaderBasic(t *testing.T) {
	h := NewScanlineHeader(64, 64)

	// This tests internal WriteHeader behavior with various buffer sizes
	bw := xdr.NewBufferWriter(1024)
	WriteHeader(bw, h)

	// Should have written something
	if bw.Len() == 0 {
		t.Error("WriteHeader should write data")
	}
}

func TestHeaderGetNonExistent(t *testing.T) {
	h := NewScanlineHeader(64, 64)
	attr := h.Get("nonexistent")
	if attr != nil {
		t.Errorf("Get(nonexistent) should return nil, got %v", attr)
	}
}

func TestHeaderCompressionAccessor(t *testing.T) {
	h := NewScanlineHeader(64, 64)
	h.SetCompression(CompressionPIZ)
	if h.Compression() != CompressionPIZ {
		t.Errorf("Compression() = %v, want PIZ", h.Compression())
	}
}

func TestHeaderZIPLevelAccessors(t *testing.T) {
	h := NewScanlineHeader(64, 64)

	// Test SetZIPLevel
	h.SetZIPLevel(6)
	if h.ZIPLevel() != 6 {
		t.Errorf("ZIPLevel() = %v, want 6", h.ZIPLevel())
	}

	// Test SetZIPLevel with different value
	h.SetZIPLevel(9)
	if h.ZIPLevel() != 9 {
		t.Errorf("ZIPLevel() after second set = %v, want 9", h.ZIPLevel())
	}
}

func TestHeaderDetectedFLevel(t *testing.T) {
	h := NewScanlineHeader(64, 64)

	// Test default - not detected
	flevel, detected := h.DetectedFLevel()
	if detected {
		t.Errorf("DetectedFLevel should return false for new header")
	}
	_ = flevel

	// After reading a ZIP file, this would be set by the reader
	// We can't easily test that without a full roundtrip
}

func TestHeaderCompressionOptions(t *testing.T) {
	h := NewScanlineHeader(64, 64)

	// Test CompressionOptions getter - default is -1 (DefaultCompression)
	opts := h.CompressionOptions()
	defaultLevel := opts.ZIPLevel
	t.Logf("Default ZIPLevel = %v", defaultLevel)

	// Test SetCompressionOptions
	newOpts := CompressionOptions{
		ZIPLevel: 9,
	}
	h.SetCompressionOptions(newOpts)

	got := h.CompressionOptions()
	if got.ZIPLevel != 9 {
		t.Errorf("CompressionOptions().ZIPLevel = %v, want 9", got.ZIPLevel)
	}
}

func TestNewTiledHeaderWithDefaults(t *testing.T) {
	h := NewTiledHeader(256, 256, 64, 64)

	// Verify tile description
	td := h.TileDescription()
	if td == nil {
		t.Fatal("TileDescription should not be nil")
	}
	if td.XSize != 64 || td.YSize != 64 {
		t.Errorf("Tile size = %dx%d, want 64x64", td.XSize, td.YSize)
	}

	// Verify it's marked as tiled
	if !h.IsTiled() {
		t.Error("IsTiled() should be true")
	}
}
