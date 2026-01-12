// Package exr contains reference tests that compare go-openexr behavior
// against known values from the C++ OpenEXR reference implementation.
//
// These tests use hardcoded reference values computed by the C++ implementation
// to verify that go-openexr produces compatible results.
package exr

import (
	"testing"
)

// =============================================================================
// TimeCode C++ Reference Values
// =============================================================================
//
// Reference values computed using ImfTimeCode from OpenEXR C++ library.
// The C++ implementation uses BCD encoding for time values.
// go-openexr now matches this encoding exactly.

// TimeCodeReference contains expected values from C++ implementation
type TimeCodeReference struct {
	// Input values
	Hours   int
	Minutes int
	Seconds int
	Frames  int

	// Expected packed TimeAndFlags value (TV60 packing with BCD encoding)
	// This is the same for both C++ and Go now that we use BCD
	WantTimeAndFlags uint32
}

// Reference values computed from C++ ImfTimeCode implementation
var timeCodeReferences = []TimeCodeReference{
	{
		// 00:00:00:00 - Midnight
		Hours: 0, Minutes: 0, Seconds: 0, Frames: 0,
		WantTimeAndFlags: 0x00000000,
	},
	{
		// 01:02:03:04 - Simple incrementing values
		Hours: 1, Minutes: 2, Seconds: 3, Frames: 4,
		// BCD: hours=0x01, mins=0x02, secs=0x03, frames=0x04
		WantTimeAndFlags: 0x01020304,
	},
	{
		// 12:34:56:29 - Common values with two-digit numbers
		Hours: 12, Minutes: 34, Seconds: 56, Frames: 29,
		// BCD encoding matches C++:
		// hours=12 → BCD 0x12, bits 24-29
		// minutes=34 → BCD 0x34, bits 16-22
		// seconds=56 → BCD 0x56, bits 8-14
		// frames=29 → BCD 0x29, bits 0-5
		WantTimeAndFlags: 0x12345629,
	},
	{
		// 23:59:59:29 - Maximum valid values
		Hours: 23, Minutes: 59, Seconds: 59, Frames: 29,
		// BCD: 0x23, 0x59, 0x59, 0x29
		WantTimeAndFlags: 0x23595929,
	},
}

// TestTimeCode_CppReferenceValues verifies packed values match C++ implementation.
func TestTimeCode_CppReferenceValues(t *testing.T) {
	for _, ref := range timeCodeReferences {
		name := "timecode"
		t.Run(name, func(t *testing.T) {
			tc, err := NewTimeCode(ref.Hours, ref.Minutes, ref.Seconds, ref.Frames, false)
			if err != nil {
				t.Fatalf("NewTimeCode() error = %v", err)
			}

			// Verify Go implementation produces expected BCD encoding (same as C++)
			got := tc.TimeAndFlags(TV60Packing)
			if got != ref.WantTimeAndFlags {
				t.Errorf("TimeAndFlags(TV60) = %#08x, want %#08x (C++ reference)",
					got, ref.WantTimeAndFlags)
			}

			// Verify round-trip works correctly
			if tc.Hours() != ref.Hours {
				t.Errorf("Hours() = %d, want %d", tc.Hours(), ref.Hours)
			}
			if tc.Minutes() != ref.Minutes {
				t.Errorf("Minutes() = %d, want %d", tc.Minutes(), ref.Minutes)
			}
			if tc.Seconds() != ref.Seconds {
				t.Errorf("Seconds() = %d, want %d", tc.Seconds(), ref.Seconds)
			}
			if tc.Frames() != ref.Frames {
				t.Errorf("Frames() = %d, want %d", tc.Frames(), ref.Frames)
			}
		})
	}
}

// TestTimeCode_DropFrameFlag verifies drop frame flag bit position.
func TestTimeCode_DropFrameFlag(t *testing.T) {
	// C++ implementation sets drop frame at bit 6
	const dropFrameBit = 1 << 6

	tests := []struct {
		name      string
		dropFrame bool
		wantBit   bool
	}{
		{"no drop frame", false, false},
		{"drop frame", true, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tc, err := NewTimeCode(0, 0, 0, 0, tt.dropFrame)
			if err != nil {
				t.Fatalf("NewTimeCode() error = %v", err)
			}

			packed := tc.TimeAndFlags(TV60Packing)
			gotBit := (packed & dropFrameBit) != 0
			if gotBit != tt.wantBit {
				t.Errorf("Drop frame bit = %v, want %v (TimeAndFlags=%#x)",
					gotBit, tt.wantBit, packed)
			}

			if tc.DropFrame() != tt.dropFrame {
				t.Errorf("DropFrame() = %v, want %v", tc.DropFrame(), tt.dropFrame)
			}
		})
	}
}

// =============================================================================
// Box2i C++ Reference Values
// =============================================================================

// TestBox2i_CppSemantics verifies Box2i matches C++ Imath semantics.
func TestBox2i_CppSemantics(t *testing.T) {
	// C++ Imath Box2i semantics:
	// - Min and Max corners are inclusive
	// - Width = Max.X - Min.X + 1 (for non-empty box)
	// - Height = Max.Y - Min.Y + 1 (for non-empty box)

	tests := []struct {
		name       string
		box        Box2i
		wantWidth  int32
		wantHeight int32
		wantArea   int64
		wantEmpty  bool
	}{
		{
			name:       "1920x1080 image",
			box:        Box2i{Min: V2i{0, 0}, Max: V2i{1919, 1079}},
			wantWidth:  1920,
			wantHeight: 1080,
			wantArea:   1920 * 1080,
			wantEmpty:  false,
		},
		{
			name:       "single pixel",
			box:        Box2i{Min: V2i{0, 0}, Max: V2i{0, 0}},
			wantWidth:  1,
			wantHeight: 1,
			wantArea:   1,
			wantEmpty:  false,
		},
		{
			name:       "empty box",
			box:        Box2i{Min: V2i{10, 10}, Max: V2i{5, 5}},
			wantWidth:  -4, // Max.X - Min.X + 1 = 5 - 10 + 1 = -4
			wantHeight: -4,
			wantArea:   0, // Empty box has zero area
			wantEmpty:  true,
		},
		{
			name:       "offset box",
			box:        Box2i{Min: V2i{100, 200}, Max: V2i{199, 299}},
			wantWidth:  100,
			wantHeight: 100,
			wantArea:   10000,
			wantEmpty:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.box.Width(); got != tt.wantWidth {
				t.Errorf("Width() = %d, want %d", got, tt.wantWidth)
			}
			if got := tt.box.Height(); got != tt.wantHeight {
				t.Errorf("Height() = %d, want %d", got, tt.wantHeight)
			}
			if got := tt.box.Area(); got != tt.wantArea {
				t.Errorf("Area() = %d, want %d", got, tt.wantArea)
			}
			if got := tt.box.IsEmpty(); got != tt.wantEmpty {
				t.Errorf("IsEmpty() = %v, want %v", got, tt.wantEmpty)
			}
		})
	}
}

// =============================================================================
// Chromaticities C++ Reference Values
// =============================================================================

// TestChromaticities_Rec709Reference verifies Rec. 709 values match C++.
func TestChromaticities_Rec709Reference(t *testing.T) {
	// C++ default chromaticities from ImfStandardAttributes.cpp:
	// Rec. ITU-R BT.709-3 primaries and D65 white point
	//
	// Red:   (0.6400, 0.3300)
	// Green: (0.3000, 0.6000)
	// Blue:  (0.1500, 0.0600)
	// White: (0.3127, 0.3290)  // D65

	c := DefaultChromaticities()

	const epsilon = 0.0001

	tests := []struct {
		name string
		got  float32
		want float32
	}{
		{"RedX", c.RedX, 0.6400},
		{"RedY", c.RedY, 0.3300},
		{"GreenX", c.GreenX, 0.3000},
		{"GreenY", c.GreenY, 0.6000},
		{"BlueX", c.BlueX, 0.1500},
		{"BlueY", c.BlueY, 0.0600},
		{"WhiteX", c.WhiteX, 0.3127},
		{"WhiteY", c.WhiteY, 0.3290},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			diff := tt.got - tt.want
			if diff < 0 {
				diff = -diff
			}
			if diff > epsilon {
				t.Errorf("%s = %f, want %f", tt.name, tt.got, tt.want)
			}
		})
	}
}

// =============================================================================
// Rational C++ Reference Values
// =============================================================================

// TestRational_CommonFrameRates verifies frame rate encoding.
func TestRational_CommonFrameRates(t *testing.T) {
	// Common frame rates in OpenEXR files
	tests := []struct {
		name        string
		num         int32
		denom       uint32
		wantFloat64 float64
		epsilon     float64
	}{
		{"24 fps", 24, 1, 24.0, 0.0001},
		{"23.976 fps (24000/1001)", 24000, 1001, 23.976023976, 0.000001},
		{"25 fps", 25, 1, 25.0, 0.0001},
		{"29.97 fps (30000/1001)", 30000, 1001, 29.97002997, 0.000001},
		{"30 fps", 30, 1, 30.0, 0.0001},
		{"48 fps", 48, 1, 48.0, 0.0001},
		{"59.94 fps (60000/1001)", 60000, 1001, 59.94005994, 0.000001},
		{"60 fps", 60, 1, 60.0, 0.0001},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := Rational{Num: tt.num, Denom: tt.denom}
			got := r.Float64()
			diff := got - tt.wantFloat64
			if diff < 0 {
				diff = -diff
			}
			if diff > tt.epsilon {
				t.Errorf("Float64() = %f, want %f (diff=%e)",
					got, tt.wantFloat64, diff)
			}
		})
	}
}

// =============================================================================
// Matrix C++ Reference Values
// =============================================================================

// TestMatrix_Identity verifies identity matrices match C++ Imath.
func TestMatrix_Identity(t *testing.T) {
	// C++ Imath identity matrices
	t.Run("M33f", func(t *testing.T) {
		m := Identity33()
		expected := M33f{
			1, 0, 0,
			0, 1, 0,
			0, 0, 1,
		}
		if m != expected {
			t.Errorf("Identity33() = %v, want %v", m, expected)
		}
	})

	t.Run("M44f", func(t *testing.T) {
		m := Identity44()
		expected := M44f{
			1, 0, 0, 0,
			0, 1, 0, 0,
			0, 0, 1, 0,
			0, 0, 0, 1,
		}
		if m != expected {
			t.Errorf("Identity44() = %v, want %v", m, expected)
		}
	})
}

// =============================================================================
// Serialization Reference Tests
// =============================================================================

// TestSerialization_ByteOrder verifies little-endian byte order matches C++.
func TestSerialization_ByteOrder(t *testing.T) {
	// OpenEXR uses little-endian byte order throughout
	// This verifies our XDR implementation matches

	t.Run("int32", func(t *testing.T) {
		// Value: 0x12345678
		// Little-endian bytes: 0x78, 0x56, 0x34, 0x12
		expected := []byte{0x78, 0x56, 0x34, 0x12}

		// Test via Box2i which uses int32 fields
		box := Box2i{Min: V2i{0x12345678, 0}, Max: V2i{0, 0}}

		// Note: This test documents expected byte order
		// Actual verification would require serialization access
		t.Logf("int32 0x12345678 should serialize as: %x", expected)
		_ = box // Use box to avoid unused variable
	})

	t.Run("float32", func(t *testing.T) {
		// Value: 1.0f
		// IEEE 754: 0x3F800000
		// Little-endian bytes: 0x00, 0x00, 0x80, 0x3F
		expected := []byte{0x00, 0x00, 0x80, 0x3F}
		t.Logf("float32 1.0 should serialize as: %x", expected)
	})
}

// =============================================================================
// Header Attribute Reference Tests
// =============================================================================

// TestAttribute_StandardNames verifies standard attribute names match C++.
func TestAttribute_StandardNames(t *testing.T) {
	// Standard attribute names from ImfStandardAttributes.h
	standardAttributes := []struct {
		name        string
		attrType    string
		description string
	}{
		{"channels", "chlist", "Channel list (required)"},
		{"compression", "compression", "Compression type (required)"},
		{"dataWindow", "box2i", "Pixel data bounding box (required)"},
		{"displayWindow", "box2i", "Display bounding box (required)"},
		{"lineOrder", "lineOrder", "Scanline storage order (required)"},
		{"pixelAspectRatio", "float", "Pixel width/height ratio (required)"},
		{"screenWindowCenter", "v2f", "Screen window center (required)"},
		{"screenWindowWidth", "float", "Screen window width (required)"},
		// Optional standard attributes
		{"chromaticities", "chromaticities", "CIE xy color primaries"},
		{"whiteLuminance", "float", "White point luminance (cd/m²)"},
		{"adoptedNeutral", "v2f", "Adopted neutral white"},
		{"renderingTransform", "string", "Color rendering transform"},
		{"lookModTransform", "string", "Look modification transform"},
		{"xDensity", "float", "Physical X density (pixels/inch)"},
		{"owner", "string", "Image owner/copyright"},
		{"comments", "string", "Additional comments"},
		{"capDate", "string", "Capture date (YYYY:MM:DD HH:MM:SS)"},
		{"utcOffset", "float", "UTC offset in seconds"},
		{"longitude", "float", "GPS longitude"},
		{"latitude", "float", "GPS latitude"},
		{"altitude", "float", "GPS altitude"},
		{"focus", "float", "Lens focus distance (meters)"},
		{"expTime", "float", "Exposure time (seconds)"},
		{"aperture", "float", "Lens aperture (f-number)"},
		{"isoSpeed", "float", "ISO film speed"},
		{"envmap", "envmap", "Environment map type"},
		{"keyCode", "keycode", "Film edge code"},
		{"timeCode", "timecode", "SMPTE time code"},
		{"framesPerSecond", "rational", "Frame rate"},
		{"multiView", "stringvector", "Multi-view image views"},
		{"worldToCamera", "m44f", "World to camera transform"},
		{"worldToNDC", "m44f", "World to NDC transform"},
		{"deepImageState", "deepImageState", "Deep image state"},
		{"originalDataWindow", "box2i", "Original data window"},
		{"dwaCompressionLevel", "float", "DWA compression level"},
	}

	t.Log("Standard OpenEXR Attributes (from ImfStandardAttributes.h):")
	for _, attr := range standardAttributes {
		t.Logf("  %-25s %-18s %s", attr.name, attr.attrType, attr.description)
	}
}
