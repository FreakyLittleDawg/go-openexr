package exr

import (
	"testing"

	"github.com/mrjoshuak/go-openexr/internal/xdr"
)

func TestCompression(t *testing.T) {
	tests := []struct {
		c     Compression
		str   string
		lines int
		lossy bool
	}{
		{CompressionNone, "none", 1, false},
		{CompressionRLE, "rle", 1, false},
		{CompressionZIPS, "zips", 1, false},
		{CompressionZIP, "zip", 16, false},
		{CompressionPIZ, "piz", 32, false},
		{CompressionPXR24, "pxr24", 16, true},
		{CompressionB44, "b44", 32, true},
		{CompressionB44A, "b44a", 32, true},
		{CompressionDWAA, "dwaa", 32, true},
		{CompressionDWAB, "dwab", 256, true},
		{Compression(99), "unknown", 1, false},
	}

	for _, tt := range tests {
		if s := tt.c.String(); s != tt.str {
			t.Errorf("%d.String() = %q, want %q", tt.c, s, tt.str)
		}
		if lines := tt.c.ScanlinesPerChunk(); lines != tt.lines {
			t.Errorf("%d.ScanlinesPerChunk() = %d, want %d", tt.c, lines, tt.lines)
		}
		if lossy := tt.c.IsLossy(); lossy != tt.lossy {
			t.Errorf("%d.IsLossy() = %v, want %v", tt.c, lossy, tt.lossy)
		}
	}
}

func TestLineOrder(t *testing.T) {
	tests := []struct {
		lo  LineOrder
		str string
	}{
		{LineOrderIncreasing, "increasing_y"},
		{LineOrderDecreasing, "decreasing_y"},
		{LineOrderRandom, "random_y"},
		{LineOrder(99), "unknown"},
	}

	for _, tt := range tests {
		if s := tt.lo.String(); s != tt.str {
			t.Errorf("%d.String() = %q, want %q", tt.lo, s, tt.str)
		}
	}
}

func TestAttributeReadWrite(t *testing.T) {
	tests := []struct {
		name string
		attr *Attribute
	}{
		{
			name: "box2i",
			attr: &Attribute{
				Name:  "dataWindow",
				Type:  AttrTypeBox2i,
				Value: Box2i{Min: V2i{0, 0}, Max: V2i{1919, 1079}},
			},
		},
		{
			name: "box2f",
			attr: &Attribute{
				Name:  "displayWindow",
				Type:  AttrTypeBox2f,
				Value: Box2f{Min: V2f{0, 0}, Max: V2f{1, 1}},
			},
		},
		{
			name: "compression",
			attr: &Attribute{
				Name:  "compression",
				Type:  AttrTypeCompression,
				Value: CompressionZIP,
			},
		},
		{
			name: "lineOrder",
			attr: &Attribute{
				Name:  "lineOrder",
				Type:  AttrTypeLineOrder,
				Value: LineOrderIncreasing,
			},
		},
		{
			name: "float",
			attr: &Attribute{
				Name:  "pixelAspectRatio",
				Type:  AttrTypeFloat,
				Value: float32(1.0),
			},
		},
		{
			name: "double",
			attr: &Attribute{
				Name:  "expTime",
				Type:  AttrTypeDouble,
				Value: float64(0.041666),
			},
		},
		{
			name: "int",
			attr: &Attribute{
				Name:  "xDensity",
				Type:  AttrTypeInt,
				Value: int32(72),
			},
		},
		{
			name: "string",
			attr: &Attribute{
				Name:  "owner",
				Type:  AttrTypeString,
				Value: "Test Owner",
			},
		},
		{
			name: "v2i",
			attr: &Attribute{
				Name:  "screenWindowCenter",
				Type:  AttrTypeV2i,
				Value: V2i{0, 0},
			},
		},
		{
			name: "v2f",
			attr: &Attribute{
				Name:  "screenWindowCenterF",
				Type:  AttrTypeV2f,
				Value: V2f{0.5, 0.5},
			},
		},
		{
			name: "v3i",
			attr: &Attribute{
				Name:  "vec3i",
				Type:  AttrTypeV3i,
				Value: V3i{1, 2, 3},
			},
		},
		{
			name: "v3f",
			attr: &Attribute{
				Name:  "vec3f",
				Type:  AttrTypeV3f,
				Value: V3f{1.0, 2.0, 3.0},
			},
		},
		{
			name: "m33f",
			attr: &Attribute{
				Name:  "matrix33",
				Type:  AttrTypeM33f,
				Value: Identity33(),
			},
		},
		{
			name: "m44f",
			attr: &Attribute{
				Name:  "matrix44",
				Type:  AttrTypeM44f,
				Value: Identity44(),
			},
		},
		{
			name: "rational",
			attr: &Attribute{
				Name:  "frameRate",
				Type:  AttrTypeRational,
				Value: Rational{Num: 24000, Denom: 1001},
			},
		},
		{
			name: "chromaticities",
			attr: &Attribute{
				Name:  "chromaticities",
				Type:  AttrTypeChromaticities,
				Value: DefaultChromaticities(),
			},
		},
		{
			name: "timecode",
			attr: &Attribute{
				Name:  "timeCode",
				Type:  AttrTypeTimecode,
				Value: MustNewTimeCode(1, 30, 45, 12, true),
			},
		},
		{
			name: "keycode",
			attr: &Attribute{
				Name:  "keyCode",
				Type:  AttrTypeKeycode,
				Value: KeyCode{FilmMfcCode: 1, FilmType: 2, Prefix: 3, Count: 4, PerfOffset: 5, PerfsPerFrame: 4, PerfsPerCount: 64},
			},
		},
		{
			name: "envmap",
			attr: &Attribute{
				Name:  "envmap",
				Type:  AttrTypeEnvmap,
				Value: EnvMapLatLong,
			},
		},
		{
			name: "tiledesc",
			attr: &Attribute{
				Name:  "tiles",
				Type:  AttrTypeTileDesc,
				Value: TileDescription{XSize: 64, YSize: 64, Mode: LevelModeMipmap, RoundingMode: LevelRoundDown},
			},
		},
		// Double-precision types for round-trip passthrough
		{
			name: "v2d",
			attr: &Attribute{
				Name:  "highPrecCoord",
				Type:  AttrTypeV2d,
				Value: V2d{1.23456789012345, 9.87654321098765},
			},
		},
		{
			name: "v3d",
			attr: &Attribute{
				Name:  "highPrecVec",
				Type:  AttrTypeV3d,
				Value: V3d{1.111111111111111, 2.222222222222222, 3.333333333333333},
			},
		},
		{
			name: "m33d",
			attr: &Attribute{
				Name:  "colorMatrix",
				Type:  AttrTypeM33d,
				Value: M33d{1, 0, 0, 0, 1, 0, 0, 0, 1},
			},
		},
		{
			name: "m44d",
			attr: &Attribute{
				Name:  "worldMatrix",
				Type:  AttrTypeM44d,
				Value: M44d{1, 0, 0, 0, 0, 1, 0, 0, 0, 0, 1, 0, 0, 0, 0, 1},
			},
		},
		{
			name: "floatvector",
			attr: &Attribute{
				Name:  "weights",
				Type:  AttrTypeFloatVector,
				Value: FloatVector{0.1, 0.2, 0.3, 0.4, 0.5},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := xdr.NewBufferWriter(512)
			if err := WriteAttribute(w, tt.attr); err != nil {
				t.Fatalf("WriteAttribute() error = %v", err)
			}
			// Add header terminator (empty name)
			w.WriteByte(0)

			r := xdr.NewReader(w.Bytes())
			result, err := ReadAttribute(r)
			if err != nil {
				t.Fatalf("ReadAttribute() error = %v", err)
			}
			if result == nil {
				t.Fatal("ReadAttribute() returned nil")
			}

			if result.Name != tt.attr.Name {
				t.Errorf("Name = %q, want %q", result.Name, tt.attr.Name)
			}
			if result.Type != tt.attr.Type {
				t.Errorf("Type = %q, want %q", result.Type, tt.attr.Type)
			}

			// Type-specific value comparisons
			switch result.Type {
			case AttrTypeCompression:
				if result.Value.(Compression) != tt.attr.Value.(Compression) {
					t.Errorf("Value = %v, want %v", result.Value, tt.attr.Value)
				}
			case AttrTypeLineOrder:
				if result.Value.(LineOrder) != tt.attr.Value.(LineOrder) {
					t.Errorf("Value = %v, want %v", result.Value, tt.attr.Value)
				}
			case AttrTypeFloat:
				if result.Value.(float32) != tt.attr.Value.(float32) {
					t.Errorf("Value = %v, want %v", result.Value, tt.attr.Value)
				}
			case AttrTypeDouble:
				if result.Value.(float64) != tt.attr.Value.(float64) {
					t.Errorf("Value = %v, want %v", result.Value, tt.attr.Value)
				}
			case AttrTypeInt:
				if result.Value.(int32) != tt.attr.Value.(int32) {
					t.Errorf("Value = %v, want %v", result.Value, tt.attr.Value)
				}
			case AttrTypeString:
				if result.Value.(string) != tt.attr.Value.(string) {
					t.Errorf("Value = %q, want %q", result.Value, tt.attr.Value)
				}
			case AttrTypeEnvmap:
				if result.Value.(EnvMap) != tt.attr.Value.(EnvMap) {
					t.Errorf("Value = %v, want %v", result.Value, tt.attr.Value)
				}
			case AttrTypeV2d:
				if result.Value.(V2d) != tt.attr.Value.(V2d) {
					t.Errorf("Value = %v, want %v", result.Value, tt.attr.Value)
				}
			case AttrTypeV3d:
				if result.Value.(V3d) != tt.attr.Value.(V3d) {
					t.Errorf("Value = %v, want %v", result.Value, tt.attr.Value)
				}
			case AttrTypeM33d:
				if result.Value.(M33d) != tt.attr.Value.(M33d) {
					t.Errorf("Value = %v, want %v", result.Value, tt.attr.Value)
				}
			case AttrTypeM44d:
				if result.Value.(M44d) != tt.attr.Value.(M44d) {
					t.Errorf("Value = %v, want %v", result.Value, tt.attr.Value)
				}
			case AttrTypeFloatVector:
				got := result.Value.(FloatVector)
				want := tt.attr.Value.(FloatVector)
				if len(got) != len(want) {
					t.Errorf("FloatVector len = %d, want %d", len(got), len(want))
				} else {
					for i := range got {
						if got[i] != want[i] {
							t.Errorf("FloatVector[%d] = %v, want %v", i, got[i], want[i])
						}
					}
				}
			}
		})
	}
}

func TestAttributeStringVector(t *testing.T) {
	original := &Attribute{
		Name:  "multiView",
		Type:  AttrTypeStringVector,
		Value: []string{"left", "right"},
	}

	w := xdr.NewBufferWriter(256)
	if err := WriteAttribute(w, original); err != nil {
		t.Fatalf("WriteAttribute() error = %v", err)
	}
	w.WriteByte(0)

	r := xdr.NewReader(w.Bytes())
	result, err := ReadAttribute(r)
	if err != nil {
		t.Fatalf("ReadAttribute() error = %v", err)
	}

	strs := result.Value.([]string)
	if len(strs) != 2 {
		t.Fatalf("StringVector len = %d, want 2", len(strs))
	}
	if strs[0] != "left" || strs[1] != "right" {
		t.Errorf("StringVector = %v, want [left right]", strs)
	}
}

func TestAttributeEmptyStringVector(t *testing.T) {
	original := &Attribute{
		Name:  "empty",
		Type:  AttrTypeStringVector,
		Value: []string{},
	}

	w := xdr.NewBufferWriter(256)
	if err := WriteAttribute(w, original); err != nil {
		t.Fatalf("WriteAttribute() error = %v", err)
	}
	w.WriteByte(0)

	r := xdr.NewReader(w.Bytes())
	result, err := ReadAttribute(r)
	if err != nil {
		t.Fatalf("ReadAttribute() error = %v", err)
	}

	strs := result.Value.([]string)
	if len(strs) != 0 {
		t.Errorf("Empty StringVector len = %d, want 0", len(strs))
	}
}

func TestAttributeChannelList(t *testing.T) {
	cl := NewChannelList()
	cl.Add(NewChannel("R", PixelTypeHalf))
	cl.Add(NewChannel("G", PixelTypeHalf))
	cl.Add(NewChannel("B", PixelTypeHalf))

	original := &Attribute{
		Name:  "channels",
		Type:  AttrTypeChlist,
		Value: cl,
	}

	w := xdr.NewBufferWriter(256)
	if err := WriteAttribute(w, original); err != nil {
		t.Fatalf("WriteAttribute() error = %v", err)
	}
	w.WriteByte(0)

	r := xdr.NewReader(w.Bytes())
	result, err := ReadAttribute(r)
	if err != nil {
		t.Fatalf("ReadAttribute() error = %v", err)
	}

	resultCL := result.Value.(*ChannelList)
	if resultCL.Len() != 3 {
		t.Errorf("ChannelList len = %d, want 3", resultCL.Len())
	}
}

func TestAttributePreview(t *testing.T) {
	original := &Attribute{
		Name: "preview",
		Type: AttrTypePreview,
		Value: Preview{
			Width:  2,
			Height: 2,
			Pixels: []byte{255, 0, 0, 255, 0, 255, 0, 255, 0, 0, 255, 255, 255, 255, 255, 255},
		},
	}

	w := xdr.NewBufferWriter(256)
	if err := WriteAttribute(w, original); err != nil {
		t.Fatalf("WriteAttribute() error = %v", err)
	}
	w.WriteByte(0)

	r := xdr.NewReader(w.Bytes())
	result, err := ReadAttribute(r)
	if err != nil {
		t.Fatalf("ReadAttribute() error = %v", err)
	}

	preview := result.Value.(Preview)
	if preview.Width != 2 || preview.Height != 2 {
		t.Errorf("Preview size = %dx%d, want 2x2", preview.Width, preview.Height)
	}
	if len(preview.Pixels) != 16 {
		t.Errorf("Preview pixels len = %d, want 16", len(preview.Pixels))
	}
}

func TestAttributeUnknownType(t *testing.T) {
	// Write an attribute with an unknown type
	w := xdr.NewBufferWriter(64)
	w.WriteString("customAttr")      // name
	w.WriteString("customtype")      // type
	w.WriteInt32(4)                  // size
	w.WriteBytes([]byte{1, 2, 3, 4}) // raw data
	w.WriteByte(0)                   // header terminator

	r := xdr.NewReader(w.Bytes())
	attr, err := ReadAttribute(r)
	if err != nil {
		t.Fatalf("ReadAttribute() error = %v", err)
	}

	if attr.Name != "customAttr" {
		t.Errorf("Name = %q, want %q", attr.Name, "customAttr")
	}
	if attr.Type != "customtype" {
		t.Errorf("Type = %q, want %q", attr.Type, "customtype")
	}

	rawBytes, ok := attr.Value.([]byte)
	if !ok {
		t.Fatal("Value should be []byte for unknown type")
	}
	if len(rawBytes) != 4 {
		t.Errorf("Raw bytes len = %d, want 4", len(rawBytes))
	}
}

func TestAttributeWriteUnknownType(t *testing.T) {
	// Write raw bytes for unknown type
	attr := &Attribute{
		Name:  "custom",
		Type:  "unknowntype",
		Value: []byte{1, 2, 3, 4},
	}

	w := xdr.NewBufferWriter(64)
	err := WriteAttribute(w, attr)
	if err != nil {
		t.Fatalf("WriteAttribute() error = %v", err)
	}
}

func TestAttributeWriteInvalidUnknown(t *testing.T) {
	// Try to write non-[]byte value for unknown type
	attr := &Attribute{
		Name:  "invalid",
		Type:  "unknowntype",
		Value: "not bytes",
	}

	w := xdr.NewBufferWriter(64)
	err := WriteAttribute(w, attr)
	if err == nil {
		t.Error("WriteAttribute should fail for non-[]byte unknown type")
	}
}

func TestReadAttributeHeaderEnd(t *testing.T) {
	// Empty name signals end of header
	w := xdr.NewBufferWriter(4)
	w.WriteByte(0) // empty name

	r := xdr.NewReader(w.Bytes())
	attr, err := ReadAttribute(r)
	if err != nil {
		t.Fatalf("ReadAttribute() error = %v", err)
	}
	if attr != nil {
		t.Error("ReadAttribute should return nil for header terminator")
	}
}

func TestReadAttributeError(t *testing.T) {
	// Test reading with insufficient data
	r := xdr.NewReader([]byte{'t', 'e', 's', 't', 0}) // just name, no type
	_, err := ReadAttribute(r)
	if err == nil {
		t.Error("ReadAttribute with insufficient data should error")
	}
}

func TestTileDescription(t *testing.T) {
	td := TileDescription{
		XSize:        64,
		YSize:        64,
		Mode:         LevelModeRipmap,
		RoundingMode: LevelRoundUp,
	}

	w := xdr.NewBufferWriter(16)
	writeTileDescription(w, td)

	r := xdr.NewReader(w.Bytes())
	result, err := readTileDescription(r)
	if err != nil {
		t.Fatalf("readTileDescription() error = %v", err)
	}

	if result.XSize != td.XSize {
		t.Errorf("XSize = %d, want %d", result.XSize, td.XSize)
	}
	if result.YSize != td.YSize {
		t.Errorf("YSize = %d, want %d", result.YSize, td.YSize)
	}
	if result.Mode != td.Mode {
		t.Errorf("Mode = %d, want %d", result.Mode, td.Mode)
	}
	if result.RoundingMode != td.RoundingMode {
		t.Errorf("RoundingMode = %d, want %d", result.RoundingMode, td.RoundingMode)
	}
}

func TestReadTileDescriptionErrorXSize(t *testing.T) {
	// Empty reader - should fail on XSize
	r := xdr.NewReader([]byte{})
	_, err := readTileDescription(r)
	if err == nil {
		t.Error("readTileDescription with empty data should error")
	}
}

func TestReadTileDescriptionErrorYSize(t *testing.T) {
	// Only XSize present - should fail on YSize
	r := xdr.NewReader([]byte{64, 0, 0, 0})
	_, err := readTileDescription(r)
	if err == nil {
		t.Error("readTileDescription with missing YSize should error")
	}
}

func TestReadTileDescriptionErrorMode(t *testing.T) {
	// XSize and YSize present but no mode byte
	r := xdr.NewReader([]byte{64, 0, 0, 0, 64, 0, 0, 0})
	_, err := readTileDescription(r)
	if err == nil {
		t.Error("readTileDescription with missing mode should error")
	}
}

func TestReadStringVectorErrorReadInt(t *testing.T) {
	// Test readStringVector when string length read fails
	// Size says 10 bytes but only 2 present (not enough for a full int32)
	data := []byte{1, 2}
	r := xdr.NewReader(data)
	_, err := readStringVector(r, 10)
	if err == nil {
		t.Error("readStringVector with insufficient data for strLen should error")
	}
}

func TestReadStringVectorErrorReadBytes(t *testing.T) {
	// Test readStringVector when string bytes read fails
	// Length says 100 but only 4 bytes present
	data := []byte{100, 0, 0, 0, 'a', 'b'} // length=100 but only 2 bytes of data
	r := xdr.NewReader(data)
	_, err := readStringVector(r, len(data))
	if err == nil {
		t.Error("readStringVector with insufficient string data should error")
	}
}

func TestWriteAttributeEdgeCases(t *testing.T) {
	// Create an attribute with V3i value
	attr := &Attribute{
		Name:  "testV3i",
		Type:  AttrTypeV3i,
		Value: V3i{1, 2, 3},
	}
	bw := xdr.NewBufferWriter(256)
	WriteAttribute(bw, attr)
	if bw.Len() == 0 {
		t.Error("WriteAttribute(V3i) should write data")
	}

	// Create an attribute with V3f value
	attr2 := &Attribute{
		Name:  "testV3f",
		Type:  AttrTypeV3f,
		Value: V3f{1.0, 2.0, 3.0},
	}
	bw2 := xdr.NewBufferWriter(256)
	WriteAttribute(bw2, attr2)
	if bw2.Len() == 0 {
		t.Error("WriteAttribute(V3f) should write data")
	}

	// Create an attribute with Chromaticities value
	attr3 := &Attribute{
		Name: "chromaticities",
		Type: AttrTypeChromaticities,
		Value: Chromaticities{
			RedX: 0.64, RedY: 0.33,
			GreenX: 0.30, GreenY: 0.60,
			BlueX: 0.15, BlueY: 0.06,
			WhiteX: 0.31, WhiteY: 0.33,
		},
	}
	bw3 := xdr.NewBufferWriter(256)
	WriteAttribute(bw3, attr3)
	if bw3.Len() == 0 {
		t.Error("WriteAttribute(Chromaticities) should write data")
	}
}
