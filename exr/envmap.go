// Package exr provides reading and writing of OpenEXR image files.
//
// This file implements environment map support for OpenEXR files.
// Environment maps define a mapping from 3D directions to 2D pixel space
// locations and are typically used for effects such as approximating how
// shiny surfaces reflect their environment.

package exr

import (
	"math"
)

// Cube face constants for cube map environment maps.
const (
	// CubeFacePosX is the +X face of the cube map.
	CubeFacePosX = 0
	// CubeFaceNegX is the -X face of the cube map.
	CubeFaceNegX = 1
	// CubeFacePosY is the +Y face of the cube map.
	CubeFacePosY = 2
	// CubeFaceNegY is the -Y face of the cube map.
	CubeFaceNegY = 3
	// CubeFacePosZ is the +Z face of the cube map.
	CubeFacePosZ = 4
	// CubeFaceNegZ is the -Z face of the cube map.
	CubeFaceNegZ = 5
)

// String returns the name of the environment map type.
func (e EnvMap) String() string {
	switch e {
	case EnvMapLatLong:
		return "latlong"
	case EnvMapCube:
		return "cube"
	default:
		return "unknown"
	}
}

// LatLong Functions
//
// The latitude-longitude environment map projects the environment onto the
// image using polar coordinates (latitude and longitude). A pixel's x
// coordinate corresponds to its longitude, and the y coordinate corresponds
// to its latitude.
//
// Pixel (dataWindow.min.x, dataWindow.min.y) has latitude +pi/2 and longitude +pi.
// Pixel (dataWindow.max.x, dataWindow.max.y) has latitude -pi/2 and longitude -pi.
//
// In 3D space:
// - Latitudes -pi/2 and +pi/2 correspond to the negative and positive Y direction.
// - Latitude 0, longitude 0 points into the positive Z direction.
// - Latitude 0, longitude pi/2 points into the positive X direction.
//
// The size of the data window should be 2*N by N pixels (width by height).

// LatLongFromDirection converts a 3D direction to latitude and longitude.
// Returns (latitude, longitude) in radians.
// Latitude ranges from -pi/2 to +pi/2 (negative Y to positive Y).
// Longitude ranges from -pi to +pi.
func LatLongFromDirection(dir V3f) (latitude, longitude float32) {
	r := float32(math.Sqrt(float64(dir.Z*dir.Z + dir.X*dir.X)))

	absY := dir.Y
	if absY < 0 {
		absY = -absY
	}

	length := float32(math.Sqrt(float64(dir.X*dir.X + dir.Y*dir.Y + dir.Z*dir.Z)))
	if length == 0 {
		return 0, 0
	}

	if r < absY {
		// Near poles: use acos for better numerical stability
		latitude = float32(math.Acos(float64(r/length))) * envmapSign(dir.Y)
	} else {
		latitude = float32(math.Asin(float64(dir.Y / length)))
	}

	if dir.Z == 0 && dir.X == 0 {
		longitude = 0
	} else {
		longitude = float32(math.Atan2(float64(dir.X), float64(dir.Z)))
	}

	return latitude, longitude
}

// DirectionFromLatLong converts latitude and longitude to a 3D direction.
// The returned direction is normalized.
func DirectionFromLatLong(latitude, longitude float32) V3f {
	cosLat := float32(math.Cos(float64(latitude)))
	return V3f{
		X: float32(math.Sin(float64(longitude))) * cosLat,
		Y: float32(math.Sin(float64(latitude))),
		Z: float32(math.Cos(float64(longitude))) * cosLat,
	}
}

// LatLongPixelFromDirection converts a 3D direction to pixel coordinates
// in a latitude-longitude environment map.
func LatLongPixelFromDirection(dataWindow Box2i, dir V3f) (x, y float32) {
	lat, lon := LatLongFromDirection(dir)
	return LatLongPixelFromLatLong(dataWindow, lat, lon)
}

// LatLongPixelFromLatLong converts latitude and longitude to pixel coordinates
// in a latitude-longitude environment map.
func LatLongPixelFromLatLong(dataWindow Box2i, latitude, longitude float32) (x, y float32) {
	// longitude / (-2*pi) + 0.5 -> normalized x [0, 1]
	// latitude / (-pi) + 0.5 -> normalized y [0, 1]
	pi := float32(math.Pi)

	xNorm := longitude/(-2*pi) + 0.5
	yNorm := latitude/(-pi) + 0.5

	dwWidth := float32(dataWindow.Max.X - dataWindow.Min.X)
	dwHeight := float32(dataWindow.Max.Y - dataWindow.Min.Y)

	x = xNorm*dwWidth + float32(dataWindow.Min.X)
	y = yNorm*dwHeight + float32(dataWindow.Min.Y)

	return x, y
}

// DirectionFromLatLongPixel converts pixel coordinates in a latitude-longitude
// environment map to a 3D direction.
func DirectionFromLatLongPixel(dataWindow Box2i, x, y float32) V3f {
	lat, lon := LatLongFromPixel(dataWindow, x, y)
	return DirectionFromLatLong(lat, lon)
}

// LatLongFromPixel converts pixel coordinates in a latitude-longitude
// environment map to latitude and longitude.
func LatLongFromPixel(dataWindow Box2i, x, y float32) (latitude, longitude float32) {
	pi := float32(math.Pi)

	dwWidth := float32(dataWindow.Max.X - dataWindow.Min.X)
	dwHeight := float32(dataWindow.Max.Y - dataWindow.Min.Y)

	if dwHeight > 0 {
		latitude = -pi * ((y-float32(dataWindow.Min.Y))/dwHeight - 0.5)
	} else {
		latitude = 0
	}

	if dwWidth > 0 {
		longitude = -2 * pi * ((x-float32(dataWindow.Min.X))/dwWidth - 0.5)
	} else {
		longitude = 0
	}

	return latitude, longitude
}

// LatLongPixel converts a 3D direction to integer pixel coordinates
// in a latitude-longitude environment map.
func LatLongPixel(dataWindow Box2i, dir V3f) (x, y int) {
	fx, fy := LatLongPixelFromDirection(dataWindow, dir)
	return int(fx + 0.5), int(fy + 0.5)
}

// Cube Map Functions
//
// The cube map environment projects the environment onto six faces of an
// axis-aligned cube. The faces are arranged vertically in the image.
//
// The size of the data window should be N by 6*N pixels (width by height).

// CubeSizeOfFace returns the width/height of a single cube face in pixels.
func CubeSizeOfFace(dataWindow Box2i) int {
	width := int(dataWindow.Max.X - dataWindow.Min.X + 1)
	height := int(dataWindow.Max.Y - dataWindow.Min.Y + 1)
	heightPerFace := height / 6

	if width < heightPerFace {
		return width
	}
	return heightPerFace
}

// CubeDataWindowForFace returns the data window for a specific cube face.
func CubeDataWindowForFace(face int, dataWindow Box2i) Box2i {
	sof := CubeSizeOfFace(dataWindow)
	return Box2i{
		Min: V2i{X: 0, Y: int32(face * sof)},
		Max: V2i{X: int32(sof - 1), Y: int32((face+1)*sof - 1)},
	}
}

// CubeFaceAndPositionFromDirection converts a 3D direction to a cube face
// and position within that face.
// The returned positionInFace is in the range [0, sizeOfFace-1].
func CubeFaceAndPositionFromDirection(dir V3f, dataWindow Box2i) (face int, positionInFace V2f) {
	sof := CubeSizeOfFace(dataWindow)
	if sof <= 0 {
		return CubeFacePosX, V2f{}
	}

	absX := envmapAbs(dir.X)
	absY := envmapAbs(dir.Y)
	absZ := envmapAbs(dir.Z)

	if absX >= absY && absX >= absZ {
		if absX == 0 {
			// Direction is (0, 0, 0) - special case
			return CubeFacePosX, V2f{}
		}
		positionInFace.X = (dir.Y/absX + 1) / 2 * float32(sof-1)
		positionInFace.Y = (dir.Z/absX + 1) / 2 * float32(sof-1)
		if dir.X > 0 {
			face = CubeFacePosX
		} else {
			face = CubeFaceNegX
		}
	} else if absY >= absZ {
		positionInFace.X = (dir.X/absY + 1) / 2 * float32(sof-1)
		positionInFace.Y = (dir.Z/absY + 1) / 2 * float32(sof-1)
		if dir.Y > 0 {
			face = CubeFacePosY
		} else {
			face = CubeFaceNegY
		}
	} else {
		positionInFace.X = (dir.X/absZ + 1) / 2 * float32(sof-1)
		positionInFace.Y = (dir.Y/absZ + 1) / 2 * float32(sof-1)
		if dir.Z > 0 {
			face = CubeFacePosZ
		} else {
			face = CubeFaceNegZ
		}
	}

	return face, positionInFace
}

// DirectionFromCubeFaceAndPosition converts a cube face and position within
// that face to a 3D direction.
func DirectionFromCubeFaceAndPosition(face int, positionInFace V2f, dataWindow Box2i) V3f {
	sof := CubeSizeOfFace(dataWindow)

	var pos V2f
	if sof > 1 {
		pos = V2f{
			X: positionInFace.X/float32(sof-1)*2 - 1,
			Y: positionInFace.Y/float32(sof-1)*2 - 1,
		}
	}

	var dir V3f
	switch face {
	case CubeFacePosX:
		dir = V3f{X: 1, Y: pos.X, Z: pos.Y}
	case CubeFaceNegX:
		dir = V3f{X: -1, Y: pos.X, Z: pos.Y}
	case CubeFacePosY:
		dir = V3f{X: pos.X, Y: 1, Z: pos.Y}
	case CubeFaceNegY:
		dir = V3f{X: pos.X, Y: -1, Z: pos.Y}
	case CubeFacePosZ:
		dir = V3f{X: pos.X, Y: pos.Y, Z: 1}
	case CubeFaceNegZ:
		dir = V3f{X: pos.X, Y: pos.Y, Z: -1}
	default:
		dir = V3f{X: 1, Y: 0, Z: 0}
	}

	return dir
}

// CubePixelPositionFromFacePosition converts a position within a face to
// pixel coordinates in the full cube map image.
func CubePixelPositionFromFacePosition(face int, positionInFace V2f, dataWindow Box2i) V2f {
	dwf := CubeDataWindowForFace(face, dataWindow)
	var pos V2f

	switch face {
	case CubeFacePosX:
		pos.X = float32(dwf.Min.X) + positionInFace.Y
		pos.Y = float32(dwf.Max.Y) - positionInFace.X
	case CubeFaceNegX:
		pos.X = float32(dwf.Max.X) - positionInFace.Y
		pos.Y = float32(dwf.Max.Y) - positionInFace.X
	case CubeFacePosY:
		pos.X = float32(dwf.Min.X) + positionInFace.X
		pos.Y = float32(dwf.Max.Y) - positionInFace.Y
	case CubeFaceNegY:
		pos.X = float32(dwf.Min.X) + positionInFace.X
		pos.Y = float32(dwf.Min.Y) + positionInFace.Y
	case CubeFacePosZ:
		pos.X = float32(dwf.Max.X) - positionInFace.X
		pos.Y = float32(dwf.Max.Y) - positionInFace.Y
	case CubeFaceNegZ:
		pos.X = float32(dwf.Min.X) + positionInFace.X
		pos.Y = float32(dwf.Max.Y) - positionInFace.Y
	}

	return pos
}

// CubePixelFromDirection converts a 3D direction to pixel coordinates
// in a cube map environment map.
func CubePixelFromDirection(dataWindow Box2i, dir V3f) (x, y float32) {
	face, pif := CubeFaceAndPositionFromDirection(dir, dataWindow)
	pos := CubePixelPositionFromFacePosition(face, pif, dataWindow)
	return pos.X, pos.Y
}

// CubePixel converts a 3D direction to integer pixel coordinates
// in a cube map environment map, also returning the face index.
func CubePixel(face int, dataWindow Box2i, positionInFace V2f) (x, y int) {
	pos := CubePixelPositionFromFacePosition(face, positionInFace, dataWindow)
	return int(pos.X + 0.5), int(pos.Y + 0.5)
}

// CubeDirectionPixel converts a 3D direction to integer pixel coordinates
// in a cube map environment map.
func CubeDirectionPixel(dataWindow Box2i, dir V3f) (x, y int) {
	fx, fy := CubePixelFromDirection(dataWindow, dir)
	return int(fx + 0.5), int(fy + 0.5)
}

// Header attribute support for environment maps

// AttrNameEnvmap is the standard attribute name for environment map type.
const AttrNameEnvmap = "envmap"

// Envmap returns the environment map type, or EnvMapLatLong if not set.
func (h *Header) Envmap() EnvMap {
	attr := h.attrs[AttrNameEnvmap]
	if attr == nil {
		return EnvMapLatLong
	}
	return attr.Value.(EnvMap)
}

// SetEnvmap sets the environment map type.
func (h *Header) SetEnvmap(e EnvMap) {
	h.Set(&Attribute{Name: AttrNameEnvmap, Type: AttrTypeEnvmap, Value: e})
}

// HasEnvmap returns true if the header has an envmap attribute.
func (h *Header) HasEnvmap() bool {
	return h.Has(AttrNameEnvmap)
}

// IsEnvmap returns true if this image is marked as an environment map.
func (h *Header) IsEnvmap() bool {
	return h.HasEnvmap()
}

// Helper functions

// envmapSign returns -1 for negative values, +1 for positive, 0 for zero.
func envmapSign(x float32) float32 {
	if x < 0 {
		return -1
	}
	if x > 0 {
		return 1
	}
	return 0
}

// envmapAbs returns the absolute value of x.
func envmapAbs(x float32) float32 {
	if x < 0 {
		return -x
	}
	return x
}

// RGBA represents an RGBA color with float32 components.
type RGBA struct {
	R, G, B, A float32
}

// Add returns the sum of two RGBA colors.
func (c RGBA) Add(other RGBA) RGBA {
	return RGBA{
		R: c.R + other.R,
		G: c.G + other.G,
		B: c.B + other.B,
		A: c.A + other.A,
	}
}

// Scale returns the RGBA color scaled by a factor.
func (c RGBA) Scale(s float32) RGBA {
	return RGBA{
		R: c.R * s,
		G: c.G * s,
		B: c.B * s,
		A: c.A * s,
	}
}

// EnvMapImage holds an environment map image with RGBA pixel data.
type EnvMapImage struct {
	Type   EnvMap
	Width  int
	Height int
	Pixels []RGBA // Row-major order, origin at top-left
}

// NewEnvMapImage creates a new environment map image.
func NewEnvMapImage(envType EnvMap, width, height int) *EnvMapImage {
	return &EnvMapImage{
		Type:   envType,
		Width:  width,
		Height: height,
		Pixels: make([]RGBA, width*height),
	}
}

// Clear sets all pixels to zero.
func (img *EnvMapImage) Clear() {
	for i := range img.Pixels {
		img.Pixels[i] = RGBA{}
	}
}

// At returns the pixel at (x, y).
func (img *EnvMapImage) At(x, y int) RGBA {
	if x < 0 || x >= img.Width || y < 0 || y >= img.Height {
		return RGBA{}
	}
	return img.Pixels[y*img.Width+x]
}

// Set sets the pixel at (x, y).
func (img *EnvMapImage) Set(x, y int, c RGBA) {
	if x < 0 || x >= img.Width || y < 0 || y >= img.Height {
		return
	}
	img.Pixels[y*img.Width+x] = c
}

// DataWindow returns the data window for the image.
func (img *EnvMapImage) DataWindow() Box2i {
	return Box2i{
		Min: V2i{X: 0, Y: 0},
		Max: V2i{X: int32(img.Width - 1), Y: int32(img.Height - 1)},
	}
}

// Sample performs bilinear interpolation at floating-point coordinates.
func (img *EnvMapImage) Sample(x, y float32) RGBA {
	x0 := int(math.Floor(float64(x)))
	y0 := int(math.Floor(float64(y)))
	x1 := x0 + 1
	y1 := y0 + 1

	fx := x - float32(x0)
	fy := y - float32(y0)

	// Clamp coordinates
	if x0 < 0 {
		x0 = 0
	}
	if x0 >= img.Width {
		x0 = img.Width - 1
	}
	if x1 < 0 {
		x1 = 0
	}
	if x1 >= img.Width {
		x1 = img.Width - 1
	}
	if y0 < 0 {
		y0 = 0
	}
	if y0 >= img.Height {
		y0 = img.Height - 1
	}
	if y1 < 0 {
		y1 = 0
	}
	if y1 >= img.Height {
		y1 = img.Height - 1
	}

	p00 := img.At(x0, y0)
	p10 := img.At(x1, y0)
	p01 := img.At(x0, y1)
	p11 := img.At(x1, y1)

	// Bilinear interpolation
	return RGBA{
		R: (1-fx)*(1-fy)*p00.R + fx*(1-fy)*p10.R + (1-fx)*fy*p01.R + fx*fy*p11.R,
		G: (1-fx)*(1-fy)*p00.G + fx*(1-fy)*p10.G + (1-fx)*fy*p01.G + fx*fy*p11.G,
		B: (1-fx)*(1-fy)*p00.B + fx*(1-fy)*p10.B + (1-fx)*fy*p01.B + fx*fy*p11.B,
		A: (1-fx)*(1-fy)*p00.A + fx*(1-fy)*p10.A + (1-fx)*fy*p01.A + fx*fy*p11.A,
	}
}

// FilteredLookup performs a filtered lookup of the environment map.
// dir is the lookup direction, radius is the filter radius in radians,
// and numSamples is the number of samples per axis (n x n grid).
func (img *EnvMapImage) FilteredLookup(dir V3f, radius float32, numSamples int) RGBA {
	// Normalize direction
	length := float32(math.Sqrt(float64(dir.X*dir.X + dir.Y*dir.Y + dir.Z*dir.Z)))
	if length == 0 {
		return RGBA{}
	}
	dir.X /= length
	dir.Y /= length
	dir.Z /= length

	// Pick two vectors dx and dy of length radius that are orthogonal
	// to dir and to each other
	var dx, dy V3f

	if envmapAbs(dir.X) > 0.707 {
		// Use cross with Y axis
		dx = V3f{Y: -dir.Z, Z: dir.Y}
	} else {
		// Use cross with X axis
		dx = V3f{X: 0, Y: dir.Z, Z: -dir.Y}
	}

	// Normalize and scale dx
	dxLen := float32(math.Sqrt(float64(dx.X*dx.X + dx.Y*dx.Y + dx.Z*dx.Z)))
	if dxLen > 0 {
		dx.X *= radius / dxLen
		dx.Y *= radius / dxLen
		dx.Z *= radius / dxLen
	}

	// dy = dir cross dx
	dy = V3f{
		X: dir.Y*dx.Z - dir.Z*dx.Y,
		Y: dir.Z*dx.X - dir.X*dx.Z,
		Z: dir.X*dx.Y - dir.Y*dx.X,
	}
	// Normalize and scale dy
	dyLen := float32(math.Sqrt(float64(dy.X*dy.X + dy.Y*dy.Y + dy.Z*dy.Z)))
	if dyLen > 0 {
		dy.X *= radius / dyLen
		dy.Y *= radius / dyLen
		dy.Z *= radius / dyLen
	}

	// Take n by n samples with tent filter weights
	var wt float32
	var result RGBA

	for j := 0; j < numSamples; j++ {
		ry := float32(2*j+2)/float32(numSamples+1) - 1
		wy := 1 - envmapAbs(ry)
		ddyX := ry * dy.X
		ddyY := ry * dy.Y
		ddyZ := ry * dy.Z

		for i := 0; i < numSamples; i++ {
			rx := float32(2*i+2)/float32(numSamples+1) - 1
			wx := 1 - envmapAbs(rx)
			ddxX := rx * dx.X
			ddxY := rx * dx.Y
			ddxZ := rx * dx.Z

			// Sample direction
			sampleDir := V3f{
				X: dir.X + ddxX + ddyX,
				Y: dir.Y + ddxY + ddyY,
				Z: dir.Z + ddxZ + ddyZ,
			}

			s := img.Lookup(sampleDir)

			w := wx * wy
			wt += w
			result.R += s.R * w
			result.G += s.G * w
			result.B += s.B * w
			result.A += s.A * w
		}
	}

	if wt > 0 {
		return result.Scale(1 / wt)
	}
	return RGBA{}
}

// Lookup performs a point-sample lookup of the environment map in the given direction.
func (img *EnvMapImage) Lookup(dir V3f) RGBA {
	dw := img.DataWindow()

	if img.Type == EnvMapLatLong {
		x, y := LatLongPixelFromDirection(dw, dir)
		return img.Sample(x, y)
	}

	// Cube map
	x, y := CubePixelFromDirection(dw, dir)
	return img.Sample(x, y)
}
