package exr

import (
	"image"
	"math"
)

// GeneratePreview creates a preview image from an RGBAImage.
// The preview will fit within maxWidth x maxHeight while preserving aspect ratio.
// The preview uses 8-bit RGBA with simple tone mapping and gamma correction.
func GeneratePreview(img *RGBAImage, maxWidth, maxHeight int) *Preview {
	srcWidth := img.Rect.Dx()
	srcHeight := img.Rect.Dy()

	if srcWidth <= 0 || srcHeight <= 0 {
		return nil
	}

	// Calculate preview dimensions while preserving aspect ratio
	previewWidth, previewHeight := calculatePreviewSize(srcWidth, srcHeight, maxWidth, maxHeight)

	if previewWidth <= 0 || previewHeight <= 0 {
		return nil
	}

	// Create preview pixel buffer (RGBA, 8-bit per channel)
	pixels := make([]byte, previewWidth*previewHeight*4)

	// Calculate scale factors
	scaleX := float64(srcWidth) / float64(previewWidth)
	scaleY := float64(srcHeight) / float64(previewHeight)

	// Generate preview using box filter
	for py := 0; py < previewHeight; py++ {
		for px := 0; px < previewWidth; px++ {
			// Source rectangle for this preview pixel
			srcX0 := float64(px) * scaleX
			srcY0 := float64(py) * scaleY
			srcX1 := float64(px+1) * scaleX
			srcY1 := float64(py+1) * scaleY

			// Box filter: average all source pixels in the rectangle
			var sumR, sumG, sumB, sumA float64
			var count float64

			for sy := int(srcY0); sy < int(math.Ceil(srcY1)); sy++ {
				for sx := int(srcX0); sx < int(math.Ceil(srcX1)); sx++ {
					if sx >= 0 && sx < srcWidth && sy >= 0 && sy < srcHeight {
						r, g, b, a := img.RGBA(sx+img.Rect.Min.X, sy+img.Rect.Min.Y)
						sumR += float64(r)
						sumG += float64(g)
						sumB += float64(b)
						sumA += float64(a)
						count++
					}
				}
			}

			if count > 0 {
				// Average
				avgR := float32(sumR / count)
				avgG := float32(sumG / count)
				avgB := float32(sumB / count)
				avgA := float32(sumA / count)

				// Tone mapping (simple Reinhard for HDR)
				avgR = toneMap(avgR)
				avgG = toneMap(avgG)
				avgB = toneMap(avgB)

				// Gamma correction (linear to sRGB)
				avgR = linearToSRGB(avgR)
				avgG = linearToSRGB(avgG)
				avgB = linearToSRGB(avgB)

				// Convert to 8-bit
				idx := (py*previewWidth + px) * 4
				pixels[idx+0] = floatToByte(avgR)
				pixels[idx+1] = floatToByte(avgG)
				pixels[idx+2] = floatToByte(avgB)
				pixels[idx+3] = floatToByte(avgA)
			}
		}
	}

	return &Preview{
		Width:  uint32(previewWidth),
		Height: uint32(previewHeight),
		Pixels: pixels,
	}
}

// calculatePreviewSize calculates the preview dimensions preserving aspect ratio.
func calculatePreviewSize(srcWidth, srcHeight, maxWidth, maxHeight int) (int, int) {
	if maxWidth <= 0 || maxHeight <= 0 {
		return 0, 0
	}

	// If source is smaller than max, use source size
	if srcWidth <= maxWidth && srcHeight <= maxHeight {
		return srcWidth, srcHeight
	}

	// Scale to fit within max dimensions
	scaleX := float64(maxWidth) / float64(srcWidth)
	scaleY := float64(maxHeight) / float64(srcHeight)
	scale := math.Min(scaleX, scaleY)

	previewWidth := int(float64(srcWidth) * scale)
	previewHeight := int(float64(srcHeight) * scale)

	// Ensure at least 1 pixel
	if previewWidth < 1 {
		previewWidth = 1
	}
	if previewHeight < 1 {
		previewHeight = 1
	}

	return previewWidth, previewHeight
}

// toneMap applies simple Reinhard tone mapping to compress HDR to [0,1].
func toneMap(v float32) float32 {
	if v <= 0 {
		return 0
	}
	// Reinhard: v / (1 + v)
	return v / (1.0 + v)
}

// linearToSRGB converts linear RGB to sRGB gamma space.
func linearToSRGB(v float32) float32 {
	if v <= 0 {
		return 0
	}
	if v >= 1 {
		return 1
	}
	// sRGB gamma curve
	if v <= 0.0031308 {
		return v * 12.92
	}
	return 1.055*float32(math.Pow(float64(v), 1.0/2.4)) - 0.055
}

// floatToByte converts a [0,1] float to a [0,255] byte.
func floatToByte(v float32) byte {
	if v <= 0 {
		return 0
	}
	if v >= 1 {
		return 255
	}
	return byte(v*255 + 0.5)
}

// ExtractPreview extracts the preview image from an EXR file without reading the full image.
// Returns nil if no preview exists.
func ExtractPreview(path string) (*Preview, error) {
	f, err := OpenFile(path)
	if err != nil {
		return nil, err
	}

	h := f.Header(0)
	if h == nil {
		f.Close()
		return nil, ErrInvalidHeader
	}

	// Get the preview from the header
	p := h.Preview()

	// Close the file immediately
	f.Close()

	if p == nil {
		return nil, nil
	}

	// Create a copy to ensure we don't hold references to the file's data
	// This is important on Windows where file handles may not be released immediately
	pixelsCopy := make([]byte, len(p.Pixels))
	copy(pixelsCopy, p.Pixels)

	return &Preview{
		Width:  p.Width,
		Height: p.Height,
		Pixels: pixelsCopy,
	}, nil
}

// PreviewToRGBA converts a Preview to an RGBAImage for display or processing.
func PreviewToRGBA(p *Preview) *RGBAImage {
	if p == nil || p.Width == 0 || p.Height == 0 {
		return nil
	}

	width := int(p.Width)
	height := int(p.Height)

	img := NewRGBAImage(RectFromSize(width, height))

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			idx := (y*width + x) * 4
			// Convert 8-bit sRGB to linear float32
			r := sRGBToLinear(float32(p.Pixels[idx+0]) / 255.0)
			g := sRGBToLinear(float32(p.Pixels[idx+1]) / 255.0)
			b := sRGBToLinear(float32(p.Pixels[idx+2]) / 255.0)
			a := float32(p.Pixels[idx+3]) / 255.0

			img.SetRGBA(x, y, r, g, b, a)
		}
	}

	return img
}

// sRGBToLinear converts sRGB gamma space to linear RGB.
func sRGBToLinear(v float32) float32 {
	if v <= 0 {
		return 0
	}
	if v >= 1 {
		return 1
	}
	// Inverse sRGB gamma curve
	if v <= 0.04045 {
		return v / 12.92
	}
	return float32(math.Pow(float64((v+0.055)/1.055), 2.4))
}

// RectFromSize creates an image.Rectangle from width and height.
func RectFromSize(width, height int) image.Rectangle {
	return image.Rectangle{
		Min: image.Point{0, 0},
		Max: image.Point{width, height},
	}
}
