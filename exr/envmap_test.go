package exr

import (
	"math"
	"testing"
)

const (
	epsilon = 1e-5
)

func floatEquals(a, b, eps float32) bool {
	diff := a - b
	if diff < 0 {
		diff = -diff
	}
	return diff < eps
}

func v3fEquals(a, b V3f, eps float32) bool {
	return floatEquals(a.X, b.X, eps) &&
		floatEquals(a.Y, b.Y, eps) &&
		floatEquals(a.Z, b.Z, eps)
}

// normalizeV3f normalizes a V3f vector.
func normalizeV3f(v V3f) V3f {
	length := float32(math.Sqrt(float64(v.X*v.X + v.Y*v.Y + v.Z*v.Z)))
	if length == 0 {
		return V3f{}
	}
	return V3f{X: v.X / length, Y: v.Y / length, Z: v.Z / length}
}

func TestEnvMapString(t *testing.T) {
	tests := []struct {
		e    EnvMap
		want string
	}{
		{EnvMapLatLong, "latlong"},
		{EnvMapCube, "cube"},
		{EnvMap(99), "unknown"},
	}

	for _, tt := range tests {
		got := tt.e.String()
		if got != tt.want {
			t.Errorf("EnvMap(%d).String() = %q, want %q", tt.e, got, tt.want)
		}
	}
}

func TestLatLongFromDirection(t *testing.T) {
	pi := float32(math.Pi)

	tests := []struct {
		name      string
		dir       V3f
		wantLat   float32
		wantLong  float32
		tolerance float32
	}{
		// Cardinal directions
		{"positive Z", V3f{0, 0, 1}, 0, 0, epsilon},
		{"negative Z", V3f{0, 0, -1}, 0, pi, epsilon},
		{"positive X", V3f{1, 0, 0}, 0, pi / 2, epsilon},
		{"negative X", V3f{-1, 0, 0}, 0, -pi / 2, epsilon},
		{"positive Y", V3f{0, 1, 0}, pi / 2, 0, epsilon},
		{"negative Y", V3f{0, -1, 0}, -pi / 2, 0, epsilon},

		// Diagonal directions
		{"XZ diagonal", V3f{1, 0, 1}, 0, pi / 4, epsilon},
		{"YZ diagonal", V3f{0, 1, 1}, pi / 4, 0, epsilon},

		// Zero direction
		{"zero vector", V3f{0, 0, 0}, 0, 0, epsilon},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lat, lon := LatLongFromDirection(tt.dir)
			if !floatEquals(lat, tt.wantLat, tt.tolerance) {
				t.Errorf("LatLongFromDirection(%v) latitude = %v, want %v", tt.dir, lat, tt.wantLat)
			}
			if !floatEquals(lon, tt.wantLong, tt.tolerance) {
				t.Errorf("LatLongFromDirection(%v) longitude = %v, want %v", tt.dir, lon, tt.wantLong)
			}
		})
	}
}

func TestDirectionFromLatLong(t *testing.T) {
	pi := float32(math.Pi)

	tests := []struct {
		name    string
		lat     float32
		lon     float32
		wantDir V3f
	}{
		{"origin", 0, 0, V3f{0, 0, 1}},
		{"longitude pi/2", 0, pi / 2, V3f{1, 0, 0}},
		{"longitude -pi/2", 0, -pi / 2, V3f{-1, 0, 0}},
		{"longitude pi", 0, pi, V3f{0, 0, -1}},
		{"latitude pi/2", pi / 2, 0, V3f{0, 1, 0}},
		{"latitude -pi/2", -pi / 2, 0, V3f{0, -1, 0}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DirectionFromLatLong(tt.lat, tt.lon)
			if !v3fEquals(got, tt.wantDir, epsilon) {
				t.Errorf("DirectionFromLatLong(%v, %v) = %v, want %v", tt.lat, tt.lon, got, tt.wantDir)
			}
		})
	}
}

func TestLatLongRoundTrip(t *testing.T) {
	// Test that direction -> lat/long -> direction gives the same direction
	testDirs := []V3f{
		{1, 0, 0},
		{0, 1, 0},
		{0, 0, 1},
		{-1, 0, 0},
		{0, -1, 0},
		{0, 0, -1},
		{1, 1, 0},
		{1, 0, 1},
		{0, 1, 1},
		{1, 1, 1},
		{-1, -1, -1},
		{0.5, 0.3, 0.8},
	}

	for _, dir := range testDirs {
		normalized := normalizeV3f(dir)
		lat, lon := LatLongFromDirection(dir)
		recovered := DirectionFromLatLong(lat, lon)

		if !v3fEquals(normalized, recovered, epsilon) {
			t.Errorf("Round trip failed for %v: got %v", dir, recovered)
		}
	}
}

func TestLatLongPixelConversion(t *testing.T) {
	// Standard latlong image: 2*N by N
	dataWindow := Box2i{
		Min: V2i{0, 0},
		Max: V2i{511, 255}, // 512 x 256
	}

	// Test center of image -> direction -> pixel should give center back
	centerX := float32(256)
	centerY := float32(128)

	dir := DirectionFromLatLongPixel(dataWindow, centerX, centerY)
	px, py := LatLongPixelFromDirection(dataWindow, dir)

	if !floatEquals(px, centerX, 0.5) || !floatEquals(py, centerY, 0.5) {
		t.Errorf("Center pixel round trip: got (%v, %v), want (%v, %v)", px, py, centerX, centerY)
	}

	// Test corners
	// Top-left: latitude +pi/2, longitude +pi
	topLeftDir := DirectionFromLatLongPixel(dataWindow, 0, 0)
	if topLeftDir.Y <= 0 {
		t.Errorf("Top-left direction should have positive Y (up), got %v", topLeftDir)
	}

	// Bottom-right: latitude -pi/2, longitude -pi
	bottomRightDir := DirectionFromLatLongPixel(dataWindow, float32(dataWindow.Max.X), float32(dataWindow.Max.Y))
	if bottomRightDir.Y >= 0 {
		t.Errorf("Bottom-right direction should have negative Y (down), got %v", bottomRightDir)
	}
}

func TestLatLongPixel(t *testing.T) {
	dataWindow := Box2i{
		Min: V2i{0, 0},
		Max: V2i{511, 255},
	}

	// Test integer pixel function
	dir := V3f{0, 0, 1} // Should be near center
	x, y := LatLongPixel(dataWindow, dir)

	// Center should be around (256, 128)
	if x < 200 || x > 312 || y < 100 || y > 156 {
		t.Errorf("LatLongPixel for +Z direction: got (%d, %d), expected near center", x, y)
	}
}

func TestCubeSizeOfFace(t *testing.T) {
	tests := []struct {
		name       string
		dataWindow Box2i
		want       int
	}{
		{
			"standard 256x1536",
			Box2i{Min: V2i{0, 0}, Max: V2i{255, 1535}},
			256,
		},
		{
			"standard 64x384",
			Box2i{Min: V2i{0, 0}, Max: V2i{63, 383}},
			64,
		},
		{
			"width limited",
			Box2i{Min: V2i{0, 0}, Max: V2i{31, 383}},
			32,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CubeSizeOfFace(tt.dataWindow)
			if got != tt.want {
				t.Errorf("CubeSizeOfFace(%v) = %d, want %d", tt.dataWindow, got, tt.want)
			}
		})
	}
}

func TestCubeDataWindowForFace(t *testing.T) {
	dataWindow := Box2i{Min: V2i{0, 0}, Max: V2i{63, 383}} // 64x384

	tests := []struct {
		face int
		want Box2i
	}{
		{CubeFacePosX, Box2i{Min: V2i{0, 0}, Max: V2i{63, 63}}},
		{CubeFaceNegX, Box2i{Min: V2i{0, 64}, Max: V2i{63, 127}}},
		{CubeFacePosY, Box2i{Min: V2i{0, 128}, Max: V2i{63, 191}}},
		{CubeFaceNegY, Box2i{Min: V2i{0, 192}, Max: V2i{63, 255}}},
		{CubeFacePosZ, Box2i{Min: V2i{0, 256}, Max: V2i{63, 319}}},
		{CubeFaceNegZ, Box2i{Min: V2i{0, 320}, Max: V2i{63, 383}}},
	}

	for _, tt := range tests {
		t.Run(cubeFaceName(tt.face), func(t *testing.T) {
			got := CubeDataWindowForFace(tt.face, dataWindow)
			if got != tt.want {
				t.Errorf("CubeDataWindowForFace(%d, %v) = %v, want %v", tt.face, dataWindow, got, tt.want)
			}
		})
	}
}

func cubeFaceName(face int) string {
	switch face {
	case CubeFacePosX:
		return "+X"
	case CubeFaceNegX:
		return "-X"
	case CubeFacePosY:
		return "+Y"
	case CubeFaceNegY:
		return "-Y"
	case CubeFacePosZ:
		return "+Z"
	case CubeFaceNegZ:
		return "-Z"
	default:
		return "unknown"
	}
}

func TestCubeFaceAndPositionFromDirection(t *testing.T) {
	dataWindow := Box2i{Min: V2i{0, 0}, Max: V2i{63, 383}}

	tests := []struct {
		name     string
		dir      V3f
		wantFace int
	}{
		{"+X axis", V3f{1, 0, 0}, CubeFacePosX},
		{"-X axis", V3f{-1, 0, 0}, CubeFaceNegX},
		{"+Y axis", V3f{0, 1, 0}, CubeFacePosY},
		{"-Y axis", V3f{0, -1, 0}, CubeFaceNegY},
		{"+Z axis", V3f{0, 0, 1}, CubeFacePosZ},
		{"-Z axis", V3f{0, 0, -1}, CubeFaceNegZ},

		// Diagonal cases - largest component determines face
		{"+X dominant", V3f{1, 0.5, 0.3}, CubeFacePosX},
		{"-Y dominant", V3f{0.2, -1, 0.4}, CubeFaceNegY},
		{"+Z dominant", V3f{0.3, 0.4, 1}, CubeFacePosZ},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			face, _ := CubeFaceAndPositionFromDirection(tt.dir, dataWindow)
			if face != tt.wantFace {
				t.Errorf("CubeFaceAndPositionFromDirection(%v) face = %d (%s), want %d (%s)",
					tt.dir, face, cubeFaceName(face), tt.wantFace, cubeFaceName(tt.wantFace))
			}
		})
	}
}

func TestDirectionFromCubeFaceAndPosition(t *testing.T) {
	dataWindow := Box2i{Min: V2i{0, 0}, Max: V2i{63, 383}}
	sof := CubeSizeOfFace(dataWindow)
	center := float32(sof-1) / 2

	tests := []struct {
		name     string
		face     int
		pos      V2f
		wantDir  V3f // unnormalized expected direction
		checkDom string
	}{
		{"+X center", CubeFacePosX, V2f{center, center}, V3f{}, "X"},
		{"-X center", CubeFaceNegX, V2f{center, center}, V3f{}, "-X"},
		{"+Y center", CubeFacePosY, V2f{center, center}, V3f{}, "Y"},
		{"-Y center", CubeFaceNegY, V2f{center, center}, V3f{}, "-Y"},
		{"+Z center", CubeFacePosZ, V2f{center, center}, V3f{}, "Z"},
		{"-Z center", CubeFaceNegZ, V2f{center, center}, V3f{}, "-Z"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := DirectionFromCubeFaceAndPosition(tt.face, tt.pos, dataWindow)

			// Check that the dominant component is correct
			switch tt.checkDom {
			case "X":
				if dir.X <= 0 {
					t.Errorf("Expected positive X component, got %v", dir)
				}
			case "-X":
				if dir.X >= 0 {
					t.Errorf("Expected negative X component, got %v", dir)
				}
			case "Y":
				if dir.Y <= 0 {
					t.Errorf("Expected positive Y component, got %v", dir)
				}
			case "-Y":
				if dir.Y >= 0 {
					t.Errorf("Expected negative Y component, got %v", dir)
				}
			case "Z":
				if dir.Z <= 0 {
					t.Errorf("Expected positive Z component, got %v", dir)
				}
			case "-Z":
				if dir.Z >= 0 {
					t.Errorf("Expected negative Z component, got %v", dir)
				}
			}
		})
	}
}

func TestCubeRoundTrip(t *testing.T) {
	dataWindow := Box2i{Min: V2i{0, 0}, Max: V2i{63, 383}}

	// Test round trip for axis-aligned directions
	directions := []V3f{
		{1, 0, 0},
		{-1, 0, 0},
		{0, 1, 0},
		{0, -1, 0},
		{0, 0, 1},
		{0, 0, -1},
		{1, 0.5, 0.3},
		{-0.7, 0.8, 0.2},
		{0.3, -0.5, 0.9},
	}

	for _, dir := range directions {
		normalized := normalizeV3f(dir)

		face, pif := CubeFaceAndPositionFromDirection(dir, dataWindow)
		recovered := DirectionFromCubeFaceAndPosition(face, pif, dataWindow)
		recoveredNorm := normalizeV3f(recovered)

		if !v3fEquals(normalized, recoveredNorm, 0.1) {
			t.Errorf("Cube round trip failed for %v: got %v", dir, recoveredNorm)
		}
	}
}

func TestCubePixelFromDirection(t *testing.T) {
	dataWindow := Box2i{Min: V2i{0, 0}, Max: V2i{63, 383}}

	// Test that +X direction gives pixel in +X face region (first 64 rows)
	x, y := CubePixelFromDirection(dataWindow, V3f{1, 0, 0})
	if y < 0 || y >= 64 {
		t.Errorf("CubePixelFromDirection(+X) y=%v, expected in [0, 64)", y)
	}
	if x < 0 || x >= 64 {
		t.Errorf("CubePixelFromDirection(+X) x=%v, expected in [0, 64)", x)
	}

	// Test that +Z direction gives pixel in +Z face region (rows 256-319)
	_, y = CubePixelFromDirection(dataWindow, V3f{0, 0, 1})
	if y < 256 || y >= 320 {
		t.Errorf("CubePixelFromDirection(+Z) y=%v, expected in [256, 320)", y)
	}
}

func TestCubeDirectionPixel(t *testing.T) {
	dataWindow := Box2i{Min: V2i{0, 0}, Max: V2i{63, 383}}

	x, y := CubeDirectionPixel(dataWindow, V3f{1, 0, 0})

	// Should be within valid bounds
	if x < 0 || x > 63 || y < 0 || y > 383 {
		t.Errorf("CubeDirectionPixel out of bounds: (%d, %d)", x, y)
	}
}

func TestHeaderEnvmap(t *testing.T) {
	h := NewHeader()

	// Test default value
	if h.HasEnvmap() {
		t.Error("New header should not have envmap attribute")
	}

	// Test setting and getting latlong
	h.SetEnvmap(EnvMapLatLong)
	if !h.HasEnvmap() {
		t.Error("Header should have envmap after SetEnvmap")
	}
	if !h.IsEnvmap() {
		t.Error("IsEnvmap should return true after SetEnvmap")
	}
	if h.Envmap() != EnvMapLatLong {
		t.Errorf("Envmap() = %v, want %v", h.Envmap(), EnvMapLatLong)
	}

	// Test setting cube map
	h.SetEnvmap(EnvMapCube)
	if h.Envmap() != EnvMapCube {
		t.Errorf("Envmap() = %v, want %v", h.Envmap(), EnvMapCube)
	}

	// Test removing and default
	h.Remove(AttrNameEnvmap)
	if h.HasEnvmap() {
		t.Error("Header should not have envmap after removal")
	}
	// Default should be latlong
	if h.Envmap() != EnvMapLatLong {
		t.Errorf("Default Envmap() = %v, want %v", h.Envmap(), EnvMapLatLong)
	}
}

func TestHelperFunctions(t *testing.T) {
	// Test envmapSign function
	if envmapSign(5) != 1 {
		t.Error("envmapSign(5) should be 1")
	}
	if envmapSign(-5) != -1 {
		t.Error("envmapSign(-5) should be -1")
	}
	if envmapSign(0) != 0 {
		t.Error("envmapSign(0) should be 0")
	}

	// Test envmapAbs function
	if envmapAbs(5) != 5 {
		t.Error("envmapAbs(5) should be 5")
	}
	if envmapAbs(-5) != 5 {
		t.Error("envmapAbs(-5) should be 5")
	}
	if envmapAbs(0) != 0 {
		t.Error("envmapAbs(0) should be 0")
	}
}

func TestCubePixel(t *testing.T) {
	dataWindow := Box2i{Min: V2i{0, 0}, Max: V2i{63, 383}}
	sof := float32(CubeSizeOfFace(dataWindow) - 1)

	// Test center of +X face
	x, y := CubePixel(CubeFacePosX, dataWindow, V2f{sof / 2, sof / 2})
	if x < 0 || x > 63 || y < 0 || y > 63 {
		t.Errorf("CubePixel for +X face center: (%d, %d) out of expected range", x, y)
	}

	// Test center of +Z face
	_, y = CubePixel(CubeFacePosZ, dataWindow, V2f{sof / 2, sof / 2})
	if y < 256 || y > 319 {
		t.Errorf("CubePixel for +Z face center: y=%d not in expected range [256, 319]", y)
	}
}

func TestLatLongFromPixelEdgeCases(t *testing.T) {
	// Test with zero-size window
	zeroWidth := Box2i{Min: V2i{0, 0}, Max: V2i{0, 255}}
	_, lon := LatLongFromPixel(zeroWidth, 0, 128)
	if lon != 0 {
		t.Errorf("Zero width window should give longitude 0, got %v", lon)
	}

	zeroHeight := Box2i{Min: V2i{0, 0}, Max: V2i{255, 0}}
	lat, _ := LatLongFromPixel(zeroHeight, 128, 0)
	if lat != 0 {
		t.Errorf("Zero height window should give latitude 0, got %v", lat)
	}
}

func TestCubeZeroDirection(t *testing.T) {
	dataWindow := Box2i{Min: V2i{0, 0}, Max: V2i{63, 383}}

	// Zero direction should default to +X face
	face, pif := CubeFaceAndPositionFromDirection(V3f{0, 0, 0}, dataWindow)
	if face != CubeFacePosX {
		t.Errorf("Zero direction should map to +X face, got %s", cubeFaceName(face))
	}
	if pif.X != 0 || pif.Y != 0 {
		t.Errorf("Zero direction should give position (0,0), got %v", pif)
	}
}

func TestCubeZeroSizeWindow(t *testing.T) {
	// Edge case: very small window
	smallWindow := Box2i{Min: V2i{0, 0}, Max: V2i{0, 5}}

	sof := CubeSizeOfFace(smallWindow)
	if sof != 1 {
		t.Errorf("CubeSizeOfFace for minimal window = %d, want 1", sof)
	}

	// Direction should still work
	dir := DirectionFromCubeFaceAndPosition(CubeFacePosX, V2f{0, 0}, smallWindow)
	if dir.X != 1 {
		t.Errorf("Direction X should be 1 for +X face with sof=1, got %v", dir.X)
	}
}

func BenchmarkLatLongFromDirection(b *testing.B) {
	dir := V3f{0.5, 0.3, 0.8}
	for i := 0; i < b.N; i++ {
		LatLongFromDirection(dir)
	}
}

func BenchmarkDirectionFromLatLong(b *testing.B) {
	lat := float32(0.5)
	lon := float32(1.2)
	for i := 0; i < b.N; i++ {
		DirectionFromLatLong(lat, lon)
	}
}

func BenchmarkCubeFaceAndPositionFromDirection(b *testing.B) {
	dataWindow := Box2i{Min: V2i{0, 0}, Max: V2i{255, 1535}}
	dir := V3f{0.5, 0.3, 0.8}
	for i := 0; i < b.N; i++ {
		CubeFaceAndPositionFromDirection(dir, dataWindow)
	}
}

func BenchmarkDirectionFromCubeFaceAndPosition(b *testing.B) {
	dataWindow := Box2i{Min: V2i{0, 0}, Max: V2i{255, 1535}}
	pos := V2f{127, 127}
	for i := 0; i < b.N; i++ {
		DirectionFromCubeFaceAndPosition(CubeFacePosX, pos, dataWindow)
	}
}
