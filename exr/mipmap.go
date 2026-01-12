package exr

import (
	"math"

	"github.com/mrjoshuak/go-openexr/half"
)

// FilterType defines the type of filter used for mipmap generation.
type FilterType int

const (
	// FilterBox uses a 2x2 box filter (simple average of 4 pixels).
	FilterBox FilterType = iota
	// FilterTriangle uses a tent/triangle filter (weighted average).
	FilterTriangle
	// FilterLanczos uses a Lanczos-2 filter (high quality, slower).
	FilterLanczos
)

// MipmapGenerator generates mipmap levels for a frame buffer.
type MipmapGenerator struct {
	filter   FilterType
	clampNeg bool // Clamp negative values to 0 (useful for color channels)
}

// NewMipmapGenerator creates a new mipmap generator with the specified filter.
func NewMipmapGenerator(filter FilterType) *MipmapGenerator {
	return &MipmapGenerator{
		filter:   filter,
		clampNeg: false,
	}
}

// SetClampNegative enables or disables clamping of negative values.
func (g *MipmapGenerator) SetClampNegative(clamp bool) {
	g.clampNeg = clamp
}

// MipmapLevel represents a single mipmap level with its frame buffer and backing storage.
type MipmapLevel struct {
	FrameBuffer *FrameBuffer
	Width       int
	Height      int
	Buffers     map[string][]byte // Backing storage for slices
}

// GenerateLevels generates all mipmap levels from the source frame buffer.
// Returns a slice of MipmapLevel, one for each level (index 0 is the source).
// The header should have LevelModeMipmap set for the tile description.
func (g *MipmapGenerator) GenerateLevels(source *FrameBuffer, sourceWidth, sourceHeight int, header *Header) ([]*MipmapLevel, error) {
	numLevels := header.NumXLevels()
	if numLevels <= 0 {
		numLevels = 1
	}

	levels := make([]*MipmapLevel, numLevels)

	// Level 0 is the source
	levels[0] = &MipmapLevel{
		FrameBuffer: source,
		Width:       sourceWidth,
		Height:      sourceHeight,
	}

	for level := 1; level < numLevels; level++ {
		width := header.LevelWidth(level)
		height := header.LevelHeight(level)

		fb := NewFrameBuffer()
		buffers := make(map[string][]byte)

		// Create slices for each channel in the source
		for _, name := range source.Names() {
			srcSlice := source.Get(name)
			if srcSlice == nil {
				continue
			}

			pixelSize := srcSlice.Type.Size()
			bufSize := width * height * pixelSize
			buf := make([]byte, bufSize)
			buffers[name] = buf

			slice := NewSlice(srcSlice.Type, buf, width, height)
			fb.Set(name, slice)
		}

		// Downsample from previous level
		prevLevel := levels[level-1]
		for _, name := range source.Names() {
			srcSlice := prevLevel.FrameBuffer.Get(name)
			dstSlice := fb.Get(name)
			if srcSlice == nil || dstSlice == nil {
				continue
			}

			g.downsample(srcSlice, dstSlice, prevLevel.Width, prevLevel.Height, width, height)
		}

		levels[level] = &MipmapLevel{
			FrameBuffer: fb,
			Width:       width,
			Height:      height,
			Buffers:     buffers,
		}
	}

	return levels, nil
}

// downsample downsamples the source slice by 2x in each dimension.
func (g *MipmapGenerator) downsample(src, dst *Slice, srcW, srcH, dstW, dstH int) {
	switch g.filter {
	case FilterBox:
		g.downsampleBox(src, dst, srcW, srcH, dstW, dstH)
	case FilterTriangle:
		g.downsampleTriangle(src, dst, srcW, srcH, dstW, dstH)
	case FilterLanczos:
		g.downsampleLanczos(src, dst, srcW, srcH, dstW, dstH)
	}
}

// downsampleBox performs 2x2 box filter downsampling.
func (g *MipmapGenerator) downsampleBox(src, dst *Slice, srcW, srcH, dstW, dstH int) {
	for dstY := 0; dstY < dstH; dstY++ {
		srcY := dstY * 2
		for dstX := 0; dstX < dstW; dstX++ {
			srcX := dstX * 2

			// Sample 2x2 block
			var sum float64
			var count int

			for dy := 0; dy < 2; dy++ {
				sy := srcY + dy
				if sy >= srcH {
					continue
				}
				for dx := 0; dx < 2; dx++ {
					sx := srcX + dx
					if sx >= srcW {
						continue
					}
					sum += float64(src.GetFloat32(sx, sy))
					count++
				}
			}

			if count > 0 {
				value := float32(sum / float64(count))
				if g.clampNeg && value < 0 {
					value = 0
				}
				dst.SetFloat32(dstX, dstY, value)
			}
		}
	}
}

// downsampleTriangle performs tent filter downsampling with 3x3 kernel.
func (g *MipmapGenerator) downsampleTriangle(src, dst *Slice, srcW, srcH, dstW, dstH int) {
	// 3x3 tent filter weights for 2x downsampling
	// Applied around the center of each 2x2 source block
	for dstY := 0; dstY < dstH; dstY++ {
		srcY := dstY*2 + 1 // Center of 2x2 block
		for dstX := 0; dstX < dstW; dstX++ {
			srcX := dstX*2 + 1

			var sum float64
			var weightSum float64

			// 3x3 tent filter
			for dy := -1; dy <= 1; dy++ {
				sy := srcY + dy
				if sy < 0 || sy >= srcH {
					continue
				}
				wy := 1.0 - float64(abs(dy))*0.5 // Weight: 1.0, 0.5, 0.5

				for dx := -1; dx <= 1; dx++ {
					sx := srcX + dx
					if sx < 0 || sx >= srcW {
						continue
					}
					wx := 1.0 - float64(abs(dx))*0.5
					w := wx * wy

					sum += float64(src.GetFloat32(sx, sy)) * w
					weightSum += w
				}
			}

			if weightSum > 0 {
				value := float32(sum / weightSum)
				if g.clampNeg && value < 0 {
					value = 0
				}
				dst.SetFloat32(dstX, dstY, value)
			}
		}
	}
}

// lanczos computes the Lanczos kernel value.
func lanczos(x float64, a float64) float64 {
	if x == 0 {
		return 1.0
	}
	if x < -a || x > a {
		return 0.0
	}
	return (a * math.Sin(math.Pi*x) * math.Sin(math.Pi*x/a)) / (math.Pi * math.Pi * x * x)
}

// downsampleLanczos performs Lanczos-2 filter downsampling.
func (g *MipmapGenerator) downsampleLanczos(src, dst *Slice, srcW, srcH, dstW, dstH int) {
	const a = 2.0 // Lanczos-2

	for dstY := 0; dstY < dstH; dstY++ {
		srcCenterY := (float64(dstY) + 0.5) * 2.0
		for dstX := 0; dstX < dstW; dstX++ {
			srcCenterX := (float64(dstX) + 0.5) * 2.0

			var sum float64
			var weightSum float64

			// Sample a (2*a) x (2*a) region around the center
			yStart := int(math.Floor(srcCenterY - a))
			yEnd := int(math.Ceil(srcCenterY + a))
			xStart := int(math.Floor(srcCenterX - a))
			xEnd := int(math.Ceil(srcCenterX + a))

			for sy := yStart; sy <= yEnd; sy++ {
				if sy < 0 || sy >= srcH {
					continue
				}
				wy := lanczos(float64(sy)-srcCenterY+0.5, a)
				if wy == 0 {
					continue
				}

				for sx := xStart; sx <= xEnd; sx++ {
					if sx < 0 || sx >= srcW {
						continue
					}
					wx := lanczos(float64(sx)-srcCenterX+0.5, a)
					if wx == 0 {
						continue
					}

					w := wx * wy
					sum += float64(src.GetFloat32(sx, sy)) * w
					weightSum += w
				}
			}

			if weightSum > 0 {
				value := float32(sum / weightSum)
				if g.clampNeg && value < 0 {
					value = 0
				}
				dst.SetFloat32(dstX, dstY, value)
			}
		}
	}
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// GenerateMipmapsFromFrameBuffer is a convenience function that generates
// all mipmap levels for a frame buffer using the specified filter.
func GenerateMipmapsFromFrameBuffer(source *FrameBuffer, sourceWidth, sourceHeight int, header *Header, filter FilterType) ([]*MipmapLevel, error) {
	gen := NewMipmapGenerator(filter)
	return gen.GenerateLevels(source, sourceWidth, sourceHeight, header)
}

// RipmapLevel represents a single ripmap level with its frame buffer and backing storage.
type RipmapLevel struct {
	FrameBuffer *FrameBuffer
	Width       int
	Height      int
	Buffers     map[string][]byte
}

// RipmapGenerator generates ripmap levels for a frame buffer.
// Ripmaps have independent X and Y resolution levels.
type RipmapGenerator struct {
	filter   FilterType
	clampNeg bool
}

// NewRipmapGenerator creates a new ripmap generator with the specified filter.
func NewRipmapGenerator(filter FilterType) *RipmapGenerator {
	return &RipmapGenerator{
		filter:   filter,
		clampNeg: false,
	}
}

// SetClampNegative enables or disables clamping of negative values.
func (g *RipmapGenerator) SetClampNegative(clamp bool) {
	g.clampNeg = clamp
}

// GenerateLevels generates all ripmap levels from the source frame buffer.
// Returns a 2D slice of RipmapLevel indexed by [levelY][levelX].
// The header should have LevelModeRipmap set for the tile description.
func (g *RipmapGenerator) GenerateLevels(source *FrameBuffer, sourceWidth, sourceHeight int, header *Header) ([][]*RipmapLevel, error) {
	numXLevels := header.NumXLevels()
	numYLevels := header.NumYLevels()

	if numXLevels <= 0 {
		numXLevels = 1
	}
	if numYLevels <= 0 {
		numYLevels = 1
	}

	// Create 2D array of levels
	levels := make([][]*RipmapLevel, numYLevels)
	for ly := 0; ly < numYLevels; ly++ {
		levels[ly] = make([]*RipmapLevel, numXLevels)
	}

	// Level [0][0] is the source
	levels[0][0] = &RipmapLevel{
		FrameBuffer: source,
		Width:       sourceWidth,
		Height:      sourceHeight,
	}

	// First, fill in the first row (ly=0, varying lx) - X-only downsampling
	for lx := 1; lx < numXLevels; lx++ {
		width := header.LevelWidth(lx)
		height := sourceHeight

		fb := NewFrameBuffer()
		buffers := make(map[string][]byte)

		for _, name := range source.Names() {
			srcSlice := source.Get(name)
			if srcSlice == nil {
				continue
			}

			pixelSize := srcSlice.Type.Size()
			bufSize := width * height * pixelSize
			buf := make([]byte, bufSize)
			buffers[name] = buf

			slice := NewSlice(srcSlice.Type, buf, width, height)
			fb.Set(name, slice)
		}

		prevLevel := levels[0][lx-1]
		for _, name := range source.Names() {
			srcSlice := prevLevel.FrameBuffer.Get(name)
			dstSlice := fb.Get(name)
			if srcSlice == nil || dstSlice == nil {
				continue
			}

			g.downsampleX(srcSlice, dstSlice, prevLevel.Width, prevLevel.Height, width)
		}

		levels[0][lx] = &RipmapLevel{
			FrameBuffer: fb,
			Width:       width,
			Height:      height,
			Buffers:     buffers,
		}
	}

	// Then fill in each subsequent row by Y-downsampling from the row above
	for ly := 1; ly < numYLevels; ly++ {
		for lx := 0; lx < numXLevels; lx++ {
			width := header.LevelWidth(lx)
			height := header.LevelHeight(ly)

			fb := NewFrameBuffer()
			buffers := make(map[string][]byte)

			for _, name := range source.Names() {
				srcSlice := source.Get(name)
				if srcSlice == nil {
					continue
				}

				pixelSize := srcSlice.Type.Size()
				bufSize := width * height * pixelSize
				buf := make([]byte, bufSize)
				buffers[name] = buf

				slice := NewSlice(srcSlice.Type, buf, width, height)
				fb.Set(name, slice)
			}

			prevLevel := levels[ly-1][lx]
			for _, name := range source.Names() {
				srcSlice := prevLevel.FrameBuffer.Get(name)
				dstSlice := fb.Get(name)
				if srcSlice == nil || dstSlice == nil {
					continue
				}

				g.downsampleY(srcSlice, dstSlice, prevLevel.Width, prevLevel.Height, height)
			}

			levels[ly][lx] = &RipmapLevel{
				FrameBuffer: fb,
				Width:       width,
				Height:      height,
				Buffers:     buffers,
			}
		}
	}

	return levels, nil
}

// downsampleX performs 2x1 filter downsampling in X direction only.
func (g *RipmapGenerator) downsampleX(src, dst *Slice, srcW, srcH, dstW int) {
	for dstY := 0; dstY < srcH; dstY++ {
		for dstX := 0; dstX < dstW; dstX++ {
			srcX := dstX * 2

			var sum float64
			var count int

			for dx := 0; dx < 2; dx++ {
				sx := srcX + dx
				if sx >= srcW {
					continue
				}
				sum += float64(src.GetFloat32(sx, dstY))
				count++
			}

			if count > 0 {
				value := float32(sum / float64(count))
				if g.clampNeg && value < 0 {
					value = 0
				}
				dst.SetFloat32(dstX, dstY, value)
			}
		}
	}
}

// downsampleY performs 1x2 filter downsampling in Y direction only.
func (g *RipmapGenerator) downsampleY(src, dst *Slice, srcW, srcH, dstH int) {
	for dstY := 0; dstY < dstH; dstY++ {
		srcY := dstY * 2
		for dstX := 0; dstX < srcW; dstX++ {
			var sum float64
			var count int

			for dy := 0; dy < 2; dy++ {
				sy := srcY + dy
				if sy >= srcH {
					continue
				}
				sum += float64(src.GetFloat32(dstX, sy))
				count++
			}

			if count > 0 {
				value := float32(sum / float64(count))
				if g.clampNeg && value < 0 {
					value = 0
				}
				dst.SetFloat32(dstX, dstY, value)
			}
		}
	}
}

// GenerateRipmapsFromFrameBuffer is a convenience function that generates
// all ripmap levels for a frame buffer using the specified filter.
func GenerateRipmapsFromFrameBuffer(source *FrameBuffer, sourceWidth, sourceHeight int, header *Header, filter FilterType) ([][]*RipmapLevel, error) {
	gen := NewRipmapGenerator(filter)
	return gen.GenerateLevels(source, sourceWidth, sourceHeight, header)
}

// WriteMipmapTiledImage writes a complete mipmap tiled image with automatic level generation.
// It takes a source frame buffer and generates all mipmap levels using the specified filter.
func WriteMipmapTiledImage(w *TiledWriter, source *FrameBuffer, sourceWidth, sourceHeight int, filter FilterType) error {
	header := w.Header()

	// Generate mipmap levels
	levels, err := GenerateMipmapsFromFrameBuffer(source, sourceWidth, sourceHeight, header, filter)
	if err != nil {
		return err
	}

	// Write each level
	for level := 0; level < len(levels); level++ {
		levelData := levels[level]

		// Create a new frame buffer for this level's tiles
		w.SetFrameBuffer(levelData.FrameBuffer)

		// Write all tiles at this level
		numXTiles := w.NumXTilesAtLevel(level)
		numYTiles := w.NumYTilesAtLevel(level)

		for tileY := 0; tileY < numYTiles; tileY++ {
			for tileX := 0; tileX < numXTiles; tileX++ {
				if err := w.WriteTileLevel(tileX, tileY, level, level); err != nil {
					return err
				}
			}
		}
	}

	return nil
}

// WriteRipmapTiledImage writes a complete ripmap tiled image with automatic level generation.
// It takes a source frame buffer and generates all ripmap levels using the specified filter.
func WriteRipmapTiledImage(w *TiledWriter, source *FrameBuffer, sourceWidth, sourceHeight int, filter FilterType) error {
	header := w.Header()

	// Generate ripmap levels
	levels, err := GenerateRipmapsFromFrameBuffer(source, sourceWidth, sourceHeight, header, filter)
	if err != nil {
		return err
	}

	numXLevels := header.NumXLevels()
	numYLevels := header.NumYLevels()

	// Write each level
	for ly := 0; ly < numYLevels; ly++ {
		for lx := 0; lx < numXLevels; lx++ {
			levelData := levels[ly][lx]

			// Set the frame buffer for this level
			w.SetFrameBuffer(levelData.FrameBuffer)

			// Write all tiles at this level
			numXTiles := w.NumXTilesAtLevel(lx)
			numYTiles := w.NumYTilesAtLevel(ly)

			for tileY := 0; tileY < numYTiles; tileY++ {
				for tileX := 0; tileX < numXTiles; tileX++ {
					if err := w.WriteTileLevel(tileX, tileY, lx, ly); err != nil {
						return err
					}
				}
			}
		}
	}

	return nil
}

// HalfDownsampleBox performs efficient 2x2 box filter downsampling for half-precision data.
// This is optimized for the common case of HALF-type channels.
func HalfDownsampleBox(src []half.Half, srcW, srcH int, dst []half.Half, dstW, dstH int) {
	for dstY := 0; dstY < dstH; dstY++ {
		srcY := dstY * 2
		for dstX := 0; dstX < dstW; dstX++ {
			srcX := dstX * 2

			var sum float32
			var count int

			for dy := 0; dy < 2; dy++ {
				sy := srcY + dy
				if sy >= srcH {
					continue
				}
				for dx := 0; dx < 2; dx++ {
					sx := srcX + dx
					if sx >= srcW {
						continue
					}
					idx := sy*srcW + sx
					if idx < len(src) {
						sum += src[idx].Float32()
						count++
					}
				}
			}

			if count > 0 {
				dstIdx := dstY*dstW + dstX
				if dstIdx < len(dst) {
					dst[dstIdx] = half.FromFloat32(sum / float32(count))
				}
			}
		}
	}
}
