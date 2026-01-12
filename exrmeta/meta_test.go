package exrmeta

import (
	"testing"

	"github.com/mrjoshuak/go-openexr/exr"
)

func TestProductionMetadata(t *testing.T) {
	h := exr.NewScanlineHeader(100, 100)

	// Test Owner
	SetOwner(h, "Test Studio")
	if got := Owner(h); got != "Test Studio" {
		t.Errorf("Owner() = %q, want %q", got, "Test Studio")
	}

	// Test Comments
	SetComments(h, "Test render")
	if got := Comments(h); got != "Test render" {
		t.Errorf("Comments() = %q, want %q", got, "Test render")
	}

	// Test CapDate
	SetCapDate(h, "2026-01-05T10:30:00Z")
	if got := CapDate(h); got != "2026-01-05T10:30:00Z" {
		t.Errorf("CapDate() = %q, want %q", got, "2026-01-05T10:30:00Z")
	}

	// Test UTCOffset
	SetUTCOffset(h, -28800) // -8 hours
	if got := UTCOffset(h); got != -28800 {
		t.Errorf("UTCOffset() = %f, want -28800", got)
	}

	// Test FramesPerSecond
	SetFramesPerSecond(h, exr.Rational{Num: 24, Denom: 1})
	fps := FramesPerSecond(h)
	if fps == nil {
		t.Fatal("FramesPerSecond() returned nil")
	}
	if fps.Num != 24 || fps.Denom != 1 {
		t.Errorf("FramesPerSecond() = %d/%d, want 24/1", fps.Num, fps.Denom)
	}

	// Test ReelName
	SetReelName(h, "A001")
	if got := ReelName(h); got != "A001" {
		t.Errorf("ReelName() = %q, want %q", got, "A001")
	}

	// Test ImageCounter
	SetImageCounter(h, "001234")
	if got := ImageCounter(h); got != "001234" {
		t.Errorf("ImageCounter() = %q, want %q", got, "001234")
	}
}

func TestEnvMap(t *testing.T) {
	h := exr.NewScanlineHeader(100, 100)

	// Default should return false for exists
	_, exists := GetEnvMap(h)
	if exists {
		t.Error("GetEnvMap() should return false for unset attribute")
	}

	// Test LatLong
	SetEnvMap(h, EnvMapLatLong)
	env, exists := GetEnvMap(h)
	if !exists {
		t.Error("GetEnvMap() should return true after setting")
	}
	if env != EnvMapLatLong {
		t.Errorf("GetEnvMap() = %d, want EnvMapLatLong", env)
	}

	// Test Cube
	SetEnvMap(h, EnvMapCube)
	env, _ = GetEnvMap(h)
	if env != EnvMapCube {
		t.Errorf("GetEnvMap() = %d, want EnvMapCube", env)
	}
}

func TestWrapModes(t *testing.T) {
	h := exr.NewScanlineHeader(100, 100)

	// Default should be nil
	if w := GetWrapModes(h); w != nil {
		t.Error("GetWrapModes() should return nil for unset attribute")
	}

	// Test setting wrap modes
	SetWrapModes(h, WrapModes{Horizontal: WrapRepeat, Vertical: WrapClamp})
	w := GetWrapModes(h)
	if w == nil {
		t.Fatal("GetWrapModes() returned nil after setting")
	}
	if w.Horizontal != WrapRepeat {
		t.Errorf("WrapModes.Horizontal = %d, want WrapRepeat", w.Horizontal)
	}
	if w.Vertical != WrapClamp {
		t.Errorf("WrapModes.Vertical = %d, want WrapClamp", w.Vertical)
	}

	// Test other modes
	SetWrapModes(h, WrapModes{Horizontal: WrapBlack, Vertical: WrapMirror})
	w = GetWrapModes(h)
	if w.Horizontal != WrapBlack {
		t.Errorf("WrapModes.Horizontal = %d, want WrapBlack", w.Horizontal)
	}
	if w.Vertical != WrapMirror {
		t.Errorf("WrapModes.Vertical = %d, want WrapMirror", w.Vertical)
	}
}

func TestCameraProperties(t *testing.T) {
	h := exr.NewScanlineHeader(100, 100)

	// Test Aperture
	SetAperture(h, 2.8)
	if got := Aperture(h); got != 2.8 {
		t.Errorf("Aperture() = %f, want 2.8", got)
	}

	// Test Focus
	SetFocus(h, 1.5) // 1.5 meters
	if got := Focus(h); got != 1.5 {
		t.Errorf("Focus() = %f, want 1.5", got)
	}

	// Test ISOSpeed
	SetISOSpeed(h, 800)
	if got := ISOSpeed(h); got != 800 {
		t.Errorf("ISOSpeed() = %f, want 800", got)
	}

	// Test ExpTime
	SetExpTime(h, 0.02) // 1/50th second
	if got := ExpTime(h); got != 0.02 {
		t.Errorf("ExpTime() = %f, want 0.02", got)
	}

	// Test ShutterAngle
	SetShutterAngle(h, 180)
	if got := ShutterAngle(h); got != 180 {
		t.Errorf("ShutterAngle() = %f, want 180", got)
	}

	// Test TStop
	SetTStop(h, 4.0)
	if got := TStop(h); got != 4.0 {
		t.Errorf("TStop() = %f, want 4.0", got)
	}
}

func TestLensProperties(t *testing.T) {
	h := exr.NewScanlineHeader(100, 100)

	// Test NominalFocalLength
	SetNominalFocalLength(h, 50)
	if got := NominalFocalLength(h); got != 50 {
		t.Errorf("NominalFocalLength() = %f, want 50", got)
	}

	// Test EffectiveFocalLength
	SetEffectiveFocalLength(h, 52.5)
	if got := EffectiveFocalLength(h); got != 52.5 {
		t.Errorf("EffectiveFocalLength() = %f, want 52.5", got)
	}

	// Test PinholeFocalLength
	SetPinholeFocalLength(h, 51.2)
	if got := PinholeFocalLength(h); got != 51.2 {
		t.Errorf("PinholeFocalLength() = %f, want 51.2", got)
	}
}

func TestCameraInfo(t *testing.T) {
	h := exr.NewScanlineHeader(100, 100)

	// Set camera info
	info := CameraInfo{
		Make:            "ARRI",
		Model:           "ALEXA 35",
		SerialNumber:    "K1.0012345",
		FirmwareVersion: "SUP 1.2.3",
		UUID:            "abc-123-def",
		Label:           "A-Cam",
		CCTSetting:      5600,
		TintSetting:     0.5,
		ColorBalance:    exr.V2f{X: 0.3127, Y: 0.329},
	}
	SetCameraInfo(h, info)

	// Get camera info
	got := GetCameraInfo(h)
	if got.Make != info.Make {
		t.Errorf("CameraInfo.Make = %q, want %q", got.Make, info.Make)
	}
	if got.Model != info.Model {
		t.Errorf("CameraInfo.Model = %q, want %q", got.Model, info.Model)
	}
	if got.SerialNumber != info.SerialNumber {
		t.Errorf("CameraInfo.SerialNumber = %q, want %q", got.SerialNumber, info.SerialNumber)
	}
	if got.FirmwareVersion != info.FirmwareVersion {
		t.Errorf("CameraInfo.FirmwareVersion = %q, want %q", got.FirmwareVersion, info.FirmwareVersion)
	}
	if got.UUID != info.UUID {
		t.Errorf("CameraInfo.UUID = %q, want %q", got.UUID, info.UUID)
	}
	if got.Label != info.Label {
		t.Errorf("CameraInfo.Label = %q, want %q", got.Label, info.Label)
	}
	if got.CCTSetting != info.CCTSetting {
		t.Errorf("CameraInfo.CCTSetting = %f, want %f", got.CCTSetting, info.CCTSetting)
	}
	if got.TintSetting != info.TintSetting {
		t.Errorf("CameraInfo.TintSetting = %f, want %f", got.TintSetting, info.TintSetting)
	}
}

func TestLensInfo(t *testing.T) {
	h := exr.NewScanlineHeader(100, 100)

	// Set lens info
	info := LensInfo{
		Make:            "Zeiss",
		Model:           "Master Prime 50mm",
		SerialNumber:    "12345",
		FirmwareVersion: "1.0",
	}
	SetLensInfo(h, info)

	// Get lens info
	got := GetLensInfo(h)
	if got.Make != info.Make {
		t.Errorf("LensInfo.Make = %q, want %q", got.Make, info.Make)
	}
	if got.Model != info.Model {
		t.Errorf("LensInfo.Model = %q, want %q", got.Model, info.Model)
	}
	if got.SerialNumber != info.SerialNumber {
		t.Errorf("LensInfo.SerialNumber = %q, want %q", got.SerialNumber, info.SerialNumber)
	}
	if got.FirmwareVersion != info.FirmwareVersion {
		t.Errorf("LensInfo.FirmwareVersion = %q, want %q", got.FirmwareVersion, info.FirmwareVersion)
	}
}

func TestGeoLocation(t *testing.T) {
	h := exr.NewScanlineHeader(100, 100)

	// Default should be nil
	if loc := GetGeoLocation(h); loc != nil {
		t.Error("GetGeoLocation() should return nil for unset attributes")
	}

	// Set location
	loc := GeoLocation{
		Longitude: -122.4194,
		Latitude:  37.7749,
		Altitude:  10.5,
	}
	SetGeoLocation(h, loc)

	// Get location
	got := GetGeoLocation(h)
	if got == nil {
		t.Fatal("GetGeoLocation() returned nil after setting")
	}
	if got.Longitude != loc.Longitude {
		t.Errorf("GeoLocation.Longitude = %f, want %f", got.Longitude, loc.Longitude)
	}
	if got.Latitude != loc.Latitude {
		t.Errorf("GeoLocation.Latitude = %f, want %f", got.Latitude, loc.Latitude)
	}
	if got.Altitude != loc.Altitude {
		t.Errorf("GeoLocation.Altitude = %f, want %f", got.Altitude, loc.Altitude)
	}
}

func TestDisplayColor(t *testing.T) {
	h := exr.NewScanlineHeader(100, 100)

	// Test WhiteLuminance
	SetWhiteLuminance(h, 100) // 100 nits
	if got := WhiteLuminance(h); got != 100 {
		t.Errorf("WhiteLuminance() = %f, want 100", got)
	}

	// Test XDensity
	SetXDensity(h, 72) // 72 ppi
	if got := XDensity(h); got != 72 {
		t.Errorf("XDensity() = %f, want 72", got)
	}

	// Test AdoptedNeutral
	SetAdoptedNeutral(h, exr.V2f{X: 0.3127, Y: 0.329}) // D65
	an := AdoptedNeutral(h)
	if an == nil {
		t.Fatal("AdoptedNeutral() returned nil after setting")
	}
	if an.X != 0.3127 || an.Y != 0.329 {
		t.Errorf("AdoptedNeutral() = {%f, %f}, want {0.3127, 0.329}", an.X, an.Y)
	}

	// Test Chromaticities
	chrom := exr.Chromaticities{
		RedX: 0.64, RedY: 0.33,
		GreenX: 0.30, GreenY: 0.60,
		BlueX: 0.15, BlueY: 0.06,
		WhiteX: 0.3127, WhiteY: 0.329,
	}
	SetChromaticities(h, chrom)
	got := GetChromaticities(h)
	if got == nil {
		t.Fatal("GetChromaticities() returned nil after setting")
	}
	if got.RedX != chrom.RedX {
		t.Errorf("Chromaticities.RedX = %f, want %f", got.RedX, chrom.RedX)
	}
}

func TestTransforms(t *testing.T) {
	h := exr.NewScanlineHeader(100, 100)

	// Identity matrix
	identity := exr.Identity44()

	// Test WorldToCamera
	SetWorldToCamera(h, identity)
	wtc := WorldToCamera(h)
	if wtc == nil {
		t.Fatal("WorldToCamera() returned nil after setting")
	}
	// M44f is [16]float32 in row-major order: indices 0 and 15 are diagonal corners
	if (*wtc)[0] != 1 || (*wtc)[15] != 1 {
		t.Error("WorldToCamera() returned incorrect matrix")
	}

	// Test WorldToNDC
	SetWorldToNDC(h, identity)
	wtn := WorldToNDC(h)
	if wtn == nil {
		t.Fatal("WorldToNDC() returned nil after setting")
	}
	if (*wtn)[0] != 1 || (*wtn)[15] != 1 {
		t.Error("WorldToNDC() returned incorrect matrix")
	}
}

func TestSensorMetadata(t *testing.T) {
	h := exr.NewScanlineHeader(100, 100)

	// Test SensorCenterOffset
	SetSensorCenterOffset(h, exr.V2f{X: 0.1, Y: -0.05})
	offset := SensorCenterOffset(h)
	if offset == nil {
		t.Fatal("SensorCenterOffset() returned nil after setting")
	}
	if offset.X != 0.1 || offset.Y != -0.05 {
		t.Errorf("SensorCenterOffset() = {%f, %f}, want {0.1, -0.05}", offset.X, offset.Y)
	}

	// Test SensorOverallDimensions
	SetSensorOverallDimensions(h, exr.V2f{X: 36, Y: 24}) // Full frame
	dims := SensorOverallDimensions(h)
	if dims == nil {
		t.Fatal("SensorOverallDimensions() returned nil after setting")
	}
	if dims.X != 36 || dims.Y != 24 {
		t.Errorf("SensorOverallDimensions() = {%f, %f}, want {36, 24}", dims.X, dims.Y)
	}

	// Test SensorPhotositePitch
	SetSensorPhotositePitch(h, 0.00588) // ~6 microns
	if got := SensorPhotositePitch(h); got != 0.00588 {
		t.Errorf("SensorPhotositePitch() = %f, want 0.00588", got)
	}

	// Test SensorAcquisitionRectangle
	rect := exr.Box2i{Min: exr.V2i{X: 0, Y: 0}, Max: exr.V2i{X: 4096, Y: 2160}}
	SetSensorAcquisitionRectangle(h, rect)
	got := SensorAcquisitionRectangle(h)
	if got == nil {
		t.Fatal("SensorAcquisitionRectangle() returned nil after setting")
	}
	if got.Max.X != 4096 || got.Max.Y != 2160 {
		t.Errorf("SensorAcquisitionRectangle() = %v, want %v", got, rect)
	}
}

func TestFrameRateConstants(t *testing.T) {
	// Test that constants have expected values
	tests := []struct {
		name     string
		rate     exr.Rational
		expected float64
	}{
		{"FPS24", FPS24, 24.0},
		{"FPS23976", FPS23976, 23.976023976},
		{"FPS25", FPS25, 25.0},
		{"FPS2997", FPS2997, 29.97002997},
		{"FPS30", FPS30, 30.0},
		{"FPS48", FPS48, 48.0},
		{"FPS50", FPS50, 50.0},
		{"FPS5994", FPS5994, 59.94005994},
		{"FPS60", FPS60, 60.0},
		{"FPS120", FPS120, 120.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := RationalToFloat(tt.rate)
			diff := got - tt.expected
			if diff < 0 {
				diff = -diff
			}
			if diff > 0.0001 {
				t.Errorf("RationalToFloat(%v) = %f, want ~%f", tt.rate, got, tt.expected)
			}
		})
	}
}

func TestRationalToFloat(t *testing.T) {
	tests := []struct {
		input    exr.Rational
		expected float64
	}{
		{exr.Rational{Num: 24, Denom: 1}, 24.0},
		{exr.Rational{Num: 30000, Denom: 1001}, 29.97002997},
		{exr.Rational{Num: 1, Denom: 2}, 0.5},
		{exr.Rational{Num: 0, Denom: 1}, 0.0},
		{exr.Rational{Num: 100, Denom: 0}, 0.0}, // Division by zero protection
	}

	for _, tt := range tests {
		got := RationalToFloat(tt.input)
		diff := got - tt.expected
		if diff < 0 {
			diff = -diff
		}
		if diff > 0.0001 {
			t.Errorf("RationalToFloat(%v) = %f, want %f", tt.input, got, tt.expected)
		}
	}
}

func TestFloatToRational(t *testing.T) {
	tests := []struct {
		input    float64
		expected exr.Rational
	}{
		{24.0, FPS24},
		{23.976, FPS23976},
		{25.0, FPS25},
		{29.97, FPS2997},
		{30.0, FPS30},
		{48.0, FPS48},
		{50.0, FPS50},
		{59.94, FPS5994},
		{60.0, FPS60},
		{120.0, FPS120},
		{0.0, exr.Rational{Num: 0, Denom: 1}},
		{-1.0, exr.Rational{Num: 0, Denom: 1}},
	}

	for _, tt := range tests {
		got := FloatToRational(tt.input, 0)
		if got.Num != tt.expected.Num || got.Denom != tt.expected.Denom {
			t.Errorf("FloatToRational(%f) = %d/%d, want %d/%d",
				tt.input, got.Num, got.Denom, tt.expected.Num, tt.expected.Denom)
		}
	}
}

func TestFloatToRationalCustom(t *testing.T) {
	// Test with non-standard frame rate
	got := FloatToRational(15.0, 0)
	expected := 15.0
	actual := RationalToFloat(got)
	diff := actual - expected
	if diff < 0 {
		diff = -diff
	}
	if diff > 0.001 {
		t.Errorf("FloatToRational(15.0) converted to %f, want ~15.0", actual)
	}
}

func TestIsDropFrame(t *testing.T) {
	tests := []struct {
		rate     exr.Rational
		expected bool
	}{
		{FPS23976, true},
		{FPS2997, true},
		{FPS5994, true},
		{FPS24, false},
		{FPS25, false},
		{FPS30, false},
		{FPS60, false},
		{exr.Rational{Num: 12000, Denom: 1001}, false}, // Not a standard drop-frame rate
	}

	for _, tt := range tests {
		got := IsDropFrame(tt.rate)
		if got != tt.expected {
			t.Errorf("IsDropFrame(%v) = %v, want %v", tt.rate, got, tt.expected)
		}
	}
}

func TestFrameRateName(t *testing.T) {
	tests := []struct {
		rate     exr.Rational
		expected string
	}{
		{FPS24, "24 fps (Cinema)"},
		{FPS23976, "23.976 fps (NTSC Film)"},
		{FPS25, "25 fps (PAL)"},
		{FPS2997, "29.97 fps (NTSC)"},
		{FPS30, "30 fps"},
		{FPS48, "48 fps (HFR Cinema)"},
		{FPS50, "50 fps (PAL HFR)"},
		{FPS5994, "59.94 fps (NTSC HFR)"},
		{FPS60, "60 fps"},
		{FPS120, "120 fps"},
		{exr.Rational{Num: 15, Denom: 1}, ""}, // Non-standard
	}

	for _, tt := range tests {
		got := FrameRateName(tt.rate)
		if got != tt.expected {
			t.Errorf("FrameRateName(%v) = %q, want %q", tt.rate, got, tt.expected)
		}
	}
}

func TestSetFramesPerSecondWithConstants(t *testing.T) {
	h := exr.NewScanlineHeader(100, 100)

	// Test setting with constants
	SetFramesPerSecond(h, FPS24)
	fps := FramesPerSecond(h)
	if fps == nil {
		t.Fatal("FramesPerSecond() returned nil")
	}
	if fps.Num != 24 || fps.Denom != 1 {
		t.Errorf("FramesPerSecond() = %d/%d, want 24/1", fps.Num, fps.Denom)
	}

	// Test with drop-frame rate
	SetFramesPerSecond(h, FPS2997)
	fps = FramesPerSecond(h)
	if fps == nil {
		t.Fatal("FramesPerSecond() returned nil")
	}
	if fps.Num != 30000 || fps.Denom != 1001 {
		t.Errorf("FramesPerSecond() = %d/%d, want 30000/1001", fps.Num, fps.Denom)
	}
	if !IsDropFrame(*fps) {
		t.Error("IsDropFrame should return true for 29.97 fps")
	}
}

func TestDefaultValues(t *testing.T) {
	h := exr.NewScanlineHeader(100, 100)

	// String attributes should return empty string when not set
	if got := Owner(h); got != "" {
		t.Errorf("Owner() on empty header = %q, want empty string", got)
	}

	// Float attributes should return 0 when not set
	if got := Aperture(h); got != 0 {
		t.Errorf("Aperture() on empty header = %f, want 0", got)
	}

	// Pointer types should return nil when not set
	if got := FramesPerSecond(h); got != nil {
		t.Errorf("FramesPerSecond() on empty header = %v, want nil", got)
	}
	if got := AdoptedNeutral(h); got != nil {
		t.Errorf("AdoptedNeutral() on empty header = %v, want nil", got)
	}
	if got := WorldToCamera(h); got != nil {
		t.Errorf("WorldToCamera() on empty header = %v, want nil", got)
	}
}
