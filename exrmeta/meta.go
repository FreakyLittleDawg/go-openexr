// Package exrmeta provides typed accessors for standard OpenEXR metadata attributes.
//
// This package offers a discoverable API for reading and writing standard
// OpenEXR attributes without bloating the core exr.Header type. All functions
// operate on *exr.Header.
//
// Example usage:
//
//	h := exr.NewScanlineHeader(1920, 1080)
//	exrmeta.SetOwner(h, "Studio XYZ")
//	exrmeta.SetFramesPerSecond(h, exr.Rational{Num: 24, Denom: 1})
//	exrmeta.SetISOSpeed(h, 800)
package exrmeta

import (
	"github.com/mrjoshuak/go-openexr/exr"
)

// Standard attribute names
const (
	// Production metadata
	AttrOwner           = "owner"
	AttrComments        = "comments"
	AttrCapDate         = "capDate"
	AttrUTCOffset       = "utcOffset"
	AttrFramesPerSecond = "framesPerSecond"
	AttrReelName        = "reelName"
	AttrImageCounter    = "imageCounter"

	// Environment/texture
	AttrEnvMap    = "envmap"
	AttrWrapModes = "wrapmodes"

	// Camera properties
	AttrAperture     = "aperture"
	AttrFocus        = "focus"
	AttrISOSpeed     = "isoSpeed"
	AttrExpTime      = "expTime"
	AttrShutterAngle = "shutterAngle"
	AttrTStop        = "tStop"

	// Lens properties
	AttrNominalFocalLength   = "nominalFocalLength"
	AttrEffectiveFocalLength = "effectiveFocalLength"
	AttrPinholeFocalLength   = "pinholeFocalLength"

	// Camera identification
	AttrCameraMake            = "cameraMake"
	AttrCameraModel           = "cameraModel"
	AttrCameraSerialNumber    = "cameraSerialNumber"
	AttrCameraFirmwareVersion = "cameraFirmwareVersion"
	AttrCameraUUID            = "cameraUuid"
	AttrCameraLabel           = "cameraLabel"
	AttrCameraCCTSetting      = "cameraCCTSetting"
	AttrCameraTintSetting     = "cameraTintSetting"
	AttrCameraColorBalance    = "cameraColorBalance"

	// Lens identification
	AttrLensMake            = "lensMake"
	AttrLensModel           = "lensModel"
	AttrLensSerialNumber    = "lensSerialNumber"
	AttrLensFirmwareVersion = "lensFirmwareVersion"

	// Geolocation
	AttrLongitude = "longitude"
	AttrLatitude  = "latitude"
	AttrAltitude  = "altitude"

	// Display/color
	AttrWhiteLuminance = "whiteLuminance"
	AttrXDensity       = "xDensity"
	AttrAdoptedNeutral = "adoptedNeutral"
	AttrChromaticities = "chromaticities"

	// 3D transforms
	AttrWorldToCamera = "worldToCamera"
	AttrWorldToNDC    = "worldToNDC"

	// Sensor metadata
	AttrSensorCenterOffset         = "sensorCenterOffset"
	AttrSensorOverallDimensions    = "sensorOverallDimensions"
	AttrSensorPhotositePitch       = "sensorPhotositePitch"
	AttrSensorAcquisitionRectangle = "sensorAcquisitionRectangle"
)

// ===========================================
// Environment Maps
// ===========================================

// EnvMap specifies the type of environment map.
type EnvMap uint8

const (
	// EnvMapLatLong is a latitude-longitude environment map.
	EnvMapLatLong EnvMap = 0
	// EnvMapCube is a cube-face environment map.
	EnvMapCube EnvMap = 1
)

// SetEnvMap sets the environment map type.
func SetEnvMap(h *exr.Header, e EnvMap) {
	h.Set(&exr.Attribute{Name: AttrEnvMap, Type: exr.AttrTypeEnvmap, Value: uint8(e)})
}

// GetEnvMap returns the environment map type.
// Returns EnvMapLatLong and false if not set.
func GetEnvMap(h *exr.Header) (EnvMap, bool) {
	attr := h.Get(AttrEnvMap)
	if attr == nil {
		return EnvMapLatLong, false
	}
	if v, ok := attr.Value.(uint8); ok {
		return EnvMap(v), true
	}
	return EnvMapLatLong, false
}

// ===========================================
// Wrap Modes
// ===========================================

// WrapMode specifies texture wrapping behavior.
type WrapMode uint8

const (
	WrapClamp  WrapMode = 0 // Clamp to edge
	WrapRepeat WrapMode = 1 // Tile/repeat
	WrapBlack  WrapMode = 2 // Black outside bounds
	WrapMirror WrapMode = 3 // Mirror at edges
)

// WrapModes specifies horizontal and vertical wrap modes.
type WrapModes struct {
	Horizontal WrapMode
	Vertical   WrapMode
}

// SetWrapModes sets the texture wrap modes.
func SetWrapModes(h *exr.Header, w WrapModes) {
	// Stored as string "horizontal,vertical" with mode names
	modeNames := []string{"clamp", "periodic", "black", "mirror"}
	hName := modeNames[w.Horizontal]
	vName := modeNames[w.Vertical]
	h.Set(&exr.Attribute{Name: AttrWrapModes, Type: exr.AttrTypeString, Value: hName + "," + vName})
}

// GetWrapModes returns the texture wrap modes.
// Returns nil if not set.
func GetWrapModes(h *exr.Header) *WrapModes {
	attr := h.Get(AttrWrapModes)
	if attr == nil {
		return nil
	}
	s, ok := attr.Value.(string)
	if !ok {
		return nil
	}
	// Parse "horizontal,vertical" format
	modes := parseWrapModes(s)
	return modes
}

func parseWrapModes(s string) *WrapModes {
	modeMap := map[string]WrapMode{
		"clamp":    WrapClamp,
		"periodic": WrapRepeat,
		"black":    WrapBlack,
		"mirror":   WrapMirror,
	}
	// Find comma
	comma := -1
	for i, c := range s {
		if c == ',' {
			comma = i
			break
		}
	}
	if comma < 0 {
		return nil
	}
	hName := s[:comma]
	vName := s[comma+1:]
	hMode, hOK := modeMap[hName]
	vMode, vOK := modeMap[vName]
	if !hOK || !vOK {
		return nil
	}
	return &WrapModes{Horizontal: hMode, Vertical: vMode}
}

// ===========================================
// Production Metadata
// ===========================================

// SetOwner sets the file owner/creator.
func SetOwner(h *exr.Header, owner string) {
	h.Set(&exr.Attribute{Name: AttrOwner, Type: exr.AttrTypeString, Value: owner})
}

// Owner returns the file owner/creator, or empty string if not set.
func Owner(h *exr.Header) string {
	return getString(h, AttrOwner)
}

// SetComments sets the file comments.
func SetComments(h *exr.Header, comments string) {
	h.Set(&exr.Attribute{Name: AttrComments, Type: exr.AttrTypeString, Value: comments})
}

// Comments returns the file comments, or empty string if not set.
func Comments(h *exr.Header) string {
	return getString(h, AttrComments)
}

// SetCapDate sets the capture date (ISO 8601 format recommended).
func SetCapDate(h *exr.Header, date string) {
	h.Set(&exr.Attribute{Name: AttrCapDate, Type: exr.AttrTypeString, Value: date})
}

// CapDate returns the capture date, or empty string if not set.
func CapDate(h *exr.Header) string {
	return getString(h, AttrCapDate)
}

// SetUTCOffset sets the UTC offset in seconds.
func SetUTCOffset(h *exr.Header, seconds float32) {
	h.Set(&exr.Attribute{Name: AttrUTCOffset, Type: exr.AttrTypeFloat, Value: seconds})
}

// UTCOffset returns the UTC offset in seconds, or 0 if not set.
func UTCOffset(h *exr.Header) float32 {
	return getFloat(h, AttrUTCOffset)
}

// SetFramesPerSecond sets the frame rate.
func SetFramesPerSecond(h *exr.Header, r exr.Rational) {
	h.Set(&exr.Attribute{Name: AttrFramesPerSecond, Type: exr.AttrTypeRational, Value: r})
}

// FramesPerSecond returns the frame rate, or nil if not set.
func FramesPerSecond(h *exr.Header) *exr.Rational {
	attr := h.Get(AttrFramesPerSecond)
	if attr == nil {
		return nil
	}
	if r, ok := attr.Value.(exr.Rational); ok {
		return &r
	}
	return nil
}

// ===========================================
// Standard Frame Rates
// ===========================================

// Standard frame rates as Rational values.
var (
	// Film frame rates
	FPS24    = exr.Rational{Num: 24, Denom: 1}       // 24 fps - Standard cinema
	FPS23976 = exr.Rational{Num: 24000, Denom: 1001} // 23.976 fps - NTSC film pulldown
	FPS48    = exr.Rational{Num: 48, Denom: 1}       // 48 fps - High frame rate cinema

	// PAL frame rates
	FPS25 = exr.Rational{Num: 25, Denom: 1} // 25 fps - PAL standard
	FPS50 = exr.Rational{Num: 50, Denom: 1} // 50 fps - PAL high frame rate

	// NTSC frame rates
	FPS2997 = exr.Rational{Num: 30000, Denom: 1001} // 29.97 fps - NTSC standard
	FPS30   = exr.Rational{Num: 30, Denom: 1}       // 30 fps - Non-drop NTSC
	FPS5994 = exr.Rational{Num: 60000, Denom: 1001} // 59.94 fps - NTSC high frame rate

	// High frame rates
	FPS60  = exr.Rational{Num: 60, Denom: 1}  // 60 fps - Gaming/HFR video
	FPS120 = exr.Rational{Num: 120, Denom: 1} // 120 fps - High frame rate gaming
)

// RationalToFloat converts a Rational to a float64.
func RationalToFloat(r exr.Rational) float64 {
	if r.Denom == 0 {
		return 0
	}
	return float64(r.Num) / float64(r.Denom)
}

// FloatToRational converts a float64 to a Rational.
// It attempts to find a good rational approximation using continued fractions.
// The maxDenom parameter limits the maximum denominator (use 0 for default of 1001).
func FloatToRational(f float64, maxDenom int32) exr.Rational {
	if maxDenom <= 0 {
		maxDenom = 1001 // Default max denominator (handles NTSC rates)
	}

	// Handle special cases
	if f <= 0 {
		return exr.Rational{Num: 0, Denom: 1}
	}

	// Check for common frame rates first (exact matches)
	commonRates := []exr.Rational{
		FPS24, FPS23976, FPS25, FPS2997, FPS30,
		FPS48, FPS50, FPS5994, FPS60, FPS120,
	}
	for _, r := range commonRates {
		rf := float64(r.Num) / float64(r.Denom)
		if abs(f-rf) < 0.0001 {
			return r
		}
	}

	// Use continued fraction approximation
	return continuedFraction(f, maxDenom)
}

// IsDropFrame returns true if the frame rate is a drop-frame rate.
// Drop-frame rates are NTSC-derived rates like 23.976, 29.97, and 59.94.
func IsDropFrame(r exr.Rational) bool {
	// Drop-frame rates use 1001 as denominator
	if r.Denom == 1001 {
		return r.Num == 24000 || r.Num == 30000 || r.Num == 60000
	}
	return false
}

// FrameRateName returns a human-readable name for common frame rates.
// Returns empty string for non-standard rates.
func FrameRateName(r exr.Rational) string {
	switch {
	case r.Num == 24 && r.Denom == 1:
		return "24 fps (Cinema)"
	case r.Num == 24000 && r.Denom == 1001:
		return "23.976 fps (NTSC Film)"
	case r.Num == 25 && r.Denom == 1:
		return "25 fps (PAL)"
	case r.Num == 30000 && r.Denom == 1001:
		return "29.97 fps (NTSC)"
	case r.Num == 30 && r.Denom == 1:
		return "30 fps"
	case r.Num == 48 && r.Denom == 1:
		return "48 fps (HFR Cinema)"
	case r.Num == 50 && r.Denom == 1:
		return "50 fps (PAL HFR)"
	case r.Num == 60000 && r.Denom == 1001:
		return "59.94 fps (NTSC HFR)"
	case r.Num == 60 && r.Denom == 1:
		return "60 fps"
	case r.Num == 120 && r.Denom == 1:
		return "120 fps"
	default:
		return ""
	}
}

// continuedFraction computes a rational approximation using continued fractions.
func continuedFraction(f float64, maxDenom int32) exr.Rational {
	// Simple continued fraction approximation
	var (
		n0, n1 int32 = 0, 1
		d0, d1 int32 = 1, 0
	)

	x := f
	for i := 0; i < 20; i++ { // Limit iterations
		a := int32(x)

		n := a*n1 + n0
		d := a*d1 + d0

		if d > maxDenom {
			break
		}

		n0, n1 = n1, n
		d0, d1 = d1, d

		frac := x - float64(a)
		if frac < 1e-10 {
			break
		}
		x = 1.0 / frac
	}

	return exr.Rational{Num: n1, Denom: uint32(d1)}
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

// SetReelName sets the film reel name.
func SetReelName(h *exr.Header, name string) {
	h.Set(&exr.Attribute{Name: AttrReelName, Type: exr.AttrTypeString, Value: name})
}

// ReelName returns the film reel name, or empty string if not set.
func ReelName(h *exr.Header) string {
	return getString(h, AttrReelName)
}

// SetImageCounter sets the frame/image counter.
func SetImageCounter(h *exr.Header, counter string) {
	h.Set(&exr.Attribute{Name: AttrImageCounter, Type: exr.AttrTypeString, Value: counter})
}

// ImageCounter returns the frame/image counter, or empty string if not set.
func ImageCounter(h *exr.Header) string {
	return getString(h, AttrImageCounter)
}

// ===========================================
// Camera Properties
// ===========================================

// SetAperture sets the lens aperture (f-number).
func SetAperture(h *exr.Header, fNumber float32) {
	h.Set(&exr.Attribute{Name: AttrAperture, Type: exr.AttrTypeFloat, Value: fNumber})
}

// Aperture returns the lens aperture (f-number), or 0 if not set.
func Aperture(h *exr.Header) float32 {
	return getFloat(h, AttrAperture)
}

// SetFocus sets the focus distance in meters.
func SetFocus(h *exr.Header, meters float32) {
	h.Set(&exr.Attribute{Name: AttrFocus, Type: exr.AttrTypeFloat, Value: meters})
}

// Focus returns the focus distance in meters, or 0 if not set.
func Focus(h *exr.Header) float32 {
	return getFloat(h, AttrFocus)
}

// SetISOSpeed sets the ISO sensitivity.
func SetISOSpeed(h *exr.Header, iso float32) {
	h.Set(&exr.Attribute{Name: AttrISOSpeed, Type: exr.AttrTypeFloat, Value: iso})
}

// ISOSpeed returns the ISO sensitivity, or 0 if not set.
func ISOSpeed(h *exr.Header) float32 {
	return getFloat(h, AttrISOSpeed)
}

// SetExpTime sets the exposure time in seconds.
func SetExpTime(h *exr.Header, seconds float32) {
	h.Set(&exr.Attribute{Name: AttrExpTime, Type: exr.AttrTypeFloat, Value: seconds})
}

// ExpTime returns the exposure time in seconds, or 0 if not set.
func ExpTime(h *exr.Header) float32 {
	return getFloat(h, AttrExpTime)
}

// SetShutterAngle sets the shutter angle in degrees (for motion blur).
func SetShutterAngle(h *exr.Header, degrees float32) {
	h.Set(&exr.Attribute{Name: AttrShutterAngle, Type: exr.AttrTypeFloat, Value: degrees})
}

// ShutterAngle returns the shutter angle in degrees, or 0 if not set.
func ShutterAngle(h *exr.Header) float32 {
	return getFloat(h, AttrShutterAngle)
}

// SetTStop sets the T-stop value.
func SetTStop(h *exr.Header, tStop float32) {
	h.Set(&exr.Attribute{Name: AttrTStop, Type: exr.AttrTypeFloat, Value: tStop})
}

// TStop returns the T-stop value, or 0 if not set.
func TStop(h *exr.Header) float32 {
	return getFloat(h, AttrTStop)
}

// ===========================================
// Lens Properties
// ===========================================

// SetNominalFocalLength sets the nominal focal length in mm.
func SetNominalFocalLength(h *exr.Header, mm float32) {
	h.Set(&exr.Attribute{Name: AttrNominalFocalLength, Type: exr.AttrTypeFloat, Value: mm})
}

// NominalFocalLength returns the nominal focal length in mm, or 0 if not set.
func NominalFocalLength(h *exr.Header) float32 {
	return getFloat(h, AttrNominalFocalLength)
}

// SetEffectiveFocalLength sets the effective focal length in mm.
func SetEffectiveFocalLength(h *exr.Header, mm float32) {
	h.Set(&exr.Attribute{Name: AttrEffectiveFocalLength, Type: exr.AttrTypeFloat, Value: mm})
}

// EffectiveFocalLength returns the effective focal length in mm, or 0 if not set.
func EffectiveFocalLength(h *exr.Header) float32 {
	return getFloat(h, AttrEffectiveFocalLength)
}

// SetPinholeFocalLength sets the pinhole focal length in mm.
func SetPinholeFocalLength(h *exr.Header, mm float32) {
	h.Set(&exr.Attribute{Name: AttrPinholeFocalLength, Type: exr.AttrTypeFloat, Value: mm})
}

// PinholeFocalLength returns the pinhole focal length in mm, or 0 if not set.
func PinholeFocalLength(h *exr.Header) float32 {
	return getFloat(h, AttrPinholeFocalLength)
}

// ===========================================
// Camera Identification
// ===========================================

// CameraInfo contains camera identification metadata.
type CameraInfo struct {
	Make            string
	Model           string
	SerialNumber    string
	FirmwareVersion string
	UUID            string
	Label           string
	CCTSetting      float32
	TintSetting     float32
	ColorBalance    exr.V2f
}

// SetCameraInfo sets all camera identification attributes.
func SetCameraInfo(h *exr.Header, info CameraInfo) {
	if info.Make != "" {
		h.Set(&exr.Attribute{Name: AttrCameraMake, Type: exr.AttrTypeString, Value: info.Make})
	}
	if info.Model != "" {
		h.Set(&exr.Attribute{Name: AttrCameraModel, Type: exr.AttrTypeString, Value: info.Model})
	}
	if info.SerialNumber != "" {
		h.Set(&exr.Attribute{Name: AttrCameraSerialNumber, Type: exr.AttrTypeString, Value: info.SerialNumber})
	}
	if info.FirmwareVersion != "" {
		h.Set(&exr.Attribute{Name: AttrCameraFirmwareVersion, Type: exr.AttrTypeString, Value: info.FirmwareVersion})
	}
	if info.UUID != "" {
		h.Set(&exr.Attribute{Name: AttrCameraUUID, Type: exr.AttrTypeString, Value: info.UUID})
	}
	if info.Label != "" {
		h.Set(&exr.Attribute{Name: AttrCameraLabel, Type: exr.AttrTypeString, Value: info.Label})
	}
	if info.CCTSetting != 0 {
		h.Set(&exr.Attribute{Name: AttrCameraCCTSetting, Type: exr.AttrTypeFloat, Value: info.CCTSetting})
	}
	if info.TintSetting != 0 {
		h.Set(&exr.Attribute{Name: AttrCameraTintSetting, Type: exr.AttrTypeFloat, Value: info.TintSetting})
	}
	if info.ColorBalance.X != 0 || info.ColorBalance.Y != 0 {
		h.Set(&exr.Attribute{Name: AttrCameraColorBalance, Type: exr.AttrTypeV2f, Value: info.ColorBalance})
	}
}

// GetCameraInfo retrieves all camera identification attributes.
func GetCameraInfo(h *exr.Header) CameraInfo {
	return CameraInfo{
		Make:            getString(h, AttrCameraMake),
		Model:           getString(h, AttrCameraModel),
		SerialNumber:    getString(h, AttrCameraSerialNumber),
		FirmwareVersion: getString(h, AttrCameraFirmwareVersion),
		UUID:            getString(h, AttrCameraUUID),
		Label:           getString(h, AttrCameraLabel),
		CCTSetting:      getFloat(h, AttrCameraCCTSetting),
		TintSetting:     getFloat(h, AttrCameraTintSetting),
		ColorBalance:    getV2f(h, AttrCameraColorBalance),
	}
}

// ===========================================
// Lens Identification
// ===========================================

// LensInfo contains lens identification metadata.
type LensInfo struct {
	Make            string
	Model           string
	SerialNumber    string
	FirmwareVersion string
}

// SetLensInfo sets all lens identification attributes.
func SetLensInfo(h *exr.Header, info LensInfo) {
	if info.Make != "" {
		h.Set(&exr.Attribute{Name: AttrLensMake, Type: exr.AttrTypeString, Value: info.Make})
	}
	if info.Model != "" {
		h.Set(&exr.Attribute{Name: AttrLensModel, Type: exr.AttrTypeString, Value: info.Model})
	}
	if info.SerialNumber != "" {
		h.Set(&exr.Attribute{Name: AttrLensSerialNumber, Type: exr.AttrTypeString, Value: info.SerialNumber})
	}
	if info.FirmwareVersion != "" {
		h.Set(&exr.Attribute{Name: AttrLensFirmwareVersion, Type: exr.AttrTypeString, Value: info.FirmwareVersion})
	}
}

// GetLensInfo retrieves all lens identification attributes.
func GetLensInfo(h *exr.Header) LensInfo {
	return LensInfo{
		Make:            getString(h, AttrLensMake),
		Model:           getString(h, AttrLensModel),
		SerialNumber:    getString(h, AttrLensSerialNumber),
		FirmwareVersion: getString(h, AttrLensFirmwareVersion),
	}
}

// ===========================================
// Geolocation
// ===========================================

// GeoLocation contains geographic coordinates.
type GeoLocation struct {
	Longitude float32 // degrees
	Latitude  float32 // degrees
	Altitude  float32 // meters
}

// SetGeoLocation sets the geographic location.
func SetGeoLocation(h *exr.Header, loc GeoLocation) {
	h.Set(&exr.Attribute{Name: AttrLongitude, Type: exr.AttrTypeFloat, Value: loc.Longitude})
	h.Set(&exr.Attribute{Name: AttrLatitude, Type: exr.AttrTypeFloat, Value: loc.Latitude})
	h.Set(&exr.Attribute{Name: AttrAltitude, Type: exr.AttrTypeFloat, Value: loc.Altitude})
}

// GetGeoLocation returns the geographic location, or nil if not set.
func GetGeoLocation(h *exr.Header) *GeoLocation {
	if !h.Has(AttrLatitude) && !h.Has(AttrLongitude) {
		return nil
	}
	return &GeoLocation{
		Longitude: getFloat(h, AttrLongitude),
		Latitude:  getFloat(h, AttrLatitude),
		Altitude:  getFloat(h, AttrAltitude),
	}
}

// ===========================================
// Display/Color
// ===========================================

// SetWhiteLuminance sets the white luminance in cd/m² (nits).
func SetWhiteLuminance(h *exr.Header, nits float32) {
	h.Set(&exr.Attribute{Name: AttrWhiteLuminance, Type: exr.AttrTypeFloat, Value: nits})
}

// WhiteLuminance returns the white luminance in cd/m², or 0 if not set.
func WhiteLuminance(h *exr.Header) float32 {
	return getFloat(h, AttrWhiteLuminance)
}

// SetXDensity sets the horizontal pixel density in pixels per inch.
func SetXDensity(h *exr.Header, ppi float32) {
	h.Set(&exr.Attribute{Name: AttrXDensity, Type: exr.AttrTypeFloat, Value: ppi})
}

// XDensity returns the horizontal pixel density in pixels per inch, or 0 if not set.
func XDensity(h *exr.Header) float32 {
	return getFloat(h, AttrXDensity)
}

// SetAdoptedNeutral sets the adopted neutral white point (CIE xy chromaticity).
func SetAdoptedNeutral(h *exr.Header, xy exr.V2f) {
	h.Set(&exr.Attribute{Name: AttrAdoptedNeutral, Type: exr.AttrTypeV2f, Value: xy})
}

// AdoptedNeutral returns the adopted neutral white point, or nil if not set.
func AdoptedNeutral(h *exr.Header) *exr.V2f {
	attr := h.Get(AttrAdoptedNeutral)
	if attr == nil {
		return nil
	}
	if v, ok := attr.Value.(exr.V2f); ok {
		return &v
	}
	return nil
}

// SetChromaticities sets the color primaries and white point.
func SetChromaticities(h *exr.Header, c exr.Chromaticities) {
	h.Set(&exr.Attribute{Name: AttrChromaticities, Type: exr.AttrTypeChromaticities, Value: c})
}

// GetChromaticities returns the color primaries and white point, or nil if not set.
func GetChromaticities(h *exr.Header) *exr.Chromaticities {
	attr := h.Get(AttrChromaticities)
	if attr == nil {
		return nil
	}
	if c, ok := attr.Value.(exr.Chromaticities); ok {
		return &c
	}
	return nil
}

// ===========================================
// 3D Transforms
// ===========================================

// SetWorldToCamera sets the world-to-camera transformation matrix.
func SetWorldToCamera(h *exr.Header, m exr.M44f) {
	h.Set(&exr.Attribute{Name: AttrWorldToCamera, Type: exr.AttrTypeM44f, Value: m})
}

// WorldToCamera returns the world-to-camera transformation matrix, or nil if not set.
func WorldToCamera(h *exr.Header) *exr.M44f {
	attr := h.Get(AttrWorldToCamera)
	if attr == nil {
		return nil
	}
	if m, ok := attr.Value.(exr.M44f); ok {
		return &m
	}
	return nil
}

// SetWorldToNDC sets the world-to-NDC (normalized device coordinates) transformation matrix.
func SetWorldToNDC(h *exr.Header, m exr.M44f) {
	h.Set(&exr.Attribute{Name: AttrWorldToNDC, Type: exr.AttrTypeM44f, Value: m})
}

// WorldToNDC returns the world-to-NDC transformation matrix, or nil if not set.
func WorldToNDC(h *exr.Header) *exr.M44f {
	attr := h.Get(AttrWorldToNDC)
	if attr == nil {
		return nil
	}
	if m, ok := attr.Value.(exr.M44f); ok {
		return &m
	}
	return nil
}

// ===========================================
// Sensor Metadata
// ===========================================

// SetSensorCenterOffset sets the sensor center offset in mm.
func SetSensorCenterOffset(h *exr.Header, offset exr.V2f) {
	h.Set(&exr.Attribute{Name: AttrSensorCenterOffset, Type: exr.AttrTypeV2f, Value: offset})
}

// SensorCenterOffset returns the sensor center offset, or nil if not set.
func SensorCenterOffset(h *exr.Header) *exr.V2f {
	attr := h.Get(AttrSensorCenterOffset)
	if attr == nil {
		return nil
	}
	if v, ok := attr.Value.(exr.V2f); ok {
		return &v
	}
	return nil
}

// SetSensorOverallDimensions sets the sensor overall dimensions in mm.
func SetSensorOverallDimensions(h *exr.Header, dims exr.V2f) {
	h.Set(&exr.Attribute{Name: AttrSensorOverallDimensions, Type: exr.AttrTypeV2f, Value: dims})
}

// SensorOverallDimensions returns the sensor overall dimensions, or nil if not set.
func SensorOverallDimensions(h *exr.Header) *exr.V2f {
	attr := h.Get(AttrSensorOverallDimensions)
	if attr == nil {
		return nil
	}
	if v, ok := attr.Value.(exr.V2f); ok {
		return &v
	}
	return nil
}

// SetSensorPhotositePitch sets the sensor photosite pitch in mm.
func SetSensorPhotositePitch(h *exr.Header, pitch float32) {
	h.Set(&exr.Attribute{Name: AttrSensorPhotositePitch, Type: exr.AttrTypeFloat, Value: pitch})
}

// SensorPhotositePitch returns the sensor photosite pitch, or 0 if not set.
func SensorPhotositePitch(h *exr.Header) float32 {
	return getFloat(h, AttrSensorPhotositePitch)
}

// SetSensorAcquisitionRectangle sets the sensor acquisition rectangle.
func SetSensorAcquisitionRectangle(h *exr.Header, rect exr.Box2i) {
	h.Set(&exr.Attribute{Name: AttrSensorAcquisitionRectangle, Type: exr.AttrTypeBox2i, Value: rect})
}

// SensorAcquisitionRectangle returns the sensor acquisition rectangle, or nil if not set.
func SensorAcquisitionRectangle(h *exr.Header) *exr.Box2i {
	attr := h.Get(AttrSensorAcquisitionRectangle)
	if attr == nil {
		return nil
	}
	if b, ok := attr.Value.(exr.Box2i); ok {
		return &b
	}
	return nil
}

// ===========================================
// Helper functions
// ===========================================

func getString(h *exr.Header, name string) string {
	attr := h.Get(name)
	if attr == nil {
		return ""
	}
	if s, ok := attr.Value.(string); ok {
		return s
	}
	return ""
}

func getFloat(h *exr.Header, name string) float32 {
	attr := h.Get(name)
	if attr == nil {
		return 0
	}
	if f, ok := attr.Value.(float32); ok {
		return f
	}
	return 0
}

func getV2f(h *exr.Header, name string) exr.V2f {
	attr := h.Get(name)
	if attr == nil {
		return exr.V2f{}
	}
	if v, ok := attr.Value.(exr.V2f); ok {
		return v
	}
	return exr.V2f{}
}
