package openexr_test

import (
	"bytes"
	"fmt"
	"image"
	"os"

	"github.com/mrjoshuak/go-openexr/exr"
)

// Example_basicRead demonstrates reading an EXR file.
func Example_basicRead() {
	// Open an EXR file
	file, err := os.Open("image.exr")
	if err != nil {
		fmt.Println("Error opening file:", err)
		return
	}
	defer file.Close()

	// Get file size for OpenReader
	stat, _ := file.Stat()
	f, err := exr.OpenReader(file, stat.Size())
	if err != nil {
		fmt.Println("Error opening EXR:", err)
		return
	}

	// Read header information
	header := f.Header(0)
	fmt.Printf("Image size: %dx%d\n", header.DataWindow().Width(), header.DataWindow().Height())
	fmt.Printf("Compression: %s\n", header.Compression())

	// Create a scanline reader
	reader, err := exr.NewScanlineReader(f)
	if err != nil {
		fmt.Println("Error creating reader:", err)
		return
	}

	// Create a frame buffer
	dw := header.DataWindow()
	width := int(dw.Width())
	height := int(dw.Height())
	fb := exr.NewFrameBuffer()
	fb.Set("R", exr.NewSlice(exr.PixelTypeFloat, make([]byte, width*height*4), width, height))
	fb.Set("G", exr.NewSlice(exr.PixelTypeFloat, make([]byte, width*height*4), width, height))
	fb.Set("B", exr.NewSlice(exr.PixelTypeFloat, make([]byte, width*height*4), width, height))

	// Read all pixels
	reader.SetFrameBuffer(fb)
	err = reader.ReadPixels(int(dw.Min.Y), int(dw.Max.Y))
	if err != nil {
		fmt.Println("Error reading pixels:", err)
		return
	}

	fmt.Println("Successfully read image data")
}

// Example_basicWrite demonstrates writing an EXR file.
func Example_basicWrite() {
	width := 640
	height := 480

	// Create header
	header := exr.NewScanlineHeader(width, height)
	header.SetCompression(exr.CompressionZIP)

	// Create frame buffer with pixel data
	fb := exr.NewFrameBuffer()
	rData := make([]byte, width*height*4)
	gData := make([]byte, width*height*4)
	bData := make([]byte, width*height*4)

	fb.Set("R", exr.NewSlice(exr.PixelTypeFloat, rData, width, height))
	fb.Set("G", exr.NewSlice(exr.PixelTypeFloat, gData, width, height))
	fb.Set("B", exr.NewSlice(exr.PixelTypeFloat, bData, width, height))

	// Fill with gradient
	rSlice := fb.Get("R")
	gSlice := fb.Get("G")
	bSlice := fb.Get("B")

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			rSlice.SetFloat32(x, y, float32(x)/float32(width))
			gSlice.SetFloat32(x, y, float32(y)/float32(height))
			bSlice.SetFloat32(x, y, 0.5)
		}
	}

	// Create output buffer
	var buf bytes.Buffer
	writer, err := exr.NewScanlineWriter(&seekableBuffer{Buffer: buf}, header)
	if err != nil {
		fmt.Println("Error creating writer:", err)
		return
	}

	// Write pixels
	writer.SetFrameBuffer(fb)
	err = writer.WritePixels(0, height-1)
	if err != nil {
		fmt.Println("Error writing pixels:", err)
		return
	}

	err = writer.Close()
	if err != nil {
		fmt.Println("Error closing writer:", err)
		return
	}

	fmt.Println("Successfully wrote EXR data")
}

// Example_rgbaImage demonstrates the high-level RGBA API.
func Example_rgbaImage() {
	// Create an RGBA image
	width, height := 256, 256
	img := exr.NewRGBAImage(image.Rect(0, 0, width, height))

	// Fill with color
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.SetRGBA(x, y,
				float32(x)/float32(width),
				float32(y)/float32(height),
				0.5,
				1.0,
			)
		}
	}

	// Encode to buffer
	var buf bytes.Buffer
	err := exr.Encode(&seekableBuffer{Buffer: buf}, img)
	if err != nil {
		fmt.Println("Error encoding:", err)
		return
	}

	fmt.Printf("Encoded %dx%d image to %d bytes\n", width, height, buf.Len())
}

// Example_tiledImage demonstrates reading/writing tiled images.
func Example_tiledImage() {
	width := 512
	height := 512
	tileW := 64
	tileH := 64

	// Create tiled header
	header := exr.NewTiledHeader(width, height, tileW, tileH)
	header.SetCompression(exr.CompressionPIZ)

	fmt.Printf("Created %dx%d tiled image with %dx%d tiles\n", width, height, tileW, tileH)
}

// Example_multiPartFiles demonstrates working with multi-part EXR files.
func Example_multiPartFiles() {
	// Multi-part EXR files contain multiple independent images or layers
	// in a single file. Each part can have different dimensions, compression,
	// and channel configurations.

	fmt.Println("Multi-part EXR files can contain:")
	fmt.Println("- Multiple render passes (diffuse, specular, etc.)")
	fmt.Println("- Separate layers for compositing")
	fmt.Println("- Different image resolutions")

	// To read a multi-part file:
	// mpf, err := exr.OpenMultiPartFile("render.exr")
	// numParts := mpf.NumParts()
	// for i := 0; i < numParts; i++ {
	//     info := mpf.PartInfo(i)
	//     fmt.Printf("Part %d: %s\n", i, info.Name)
	// }
}

// Example_deepData demonstrates working with deep image data.
func Example_deepData() {
	// Deep images store multiple samples per pixel, useful for:
	// - Volume rendering
	// - Deep compositing
	// - Z-depth with transparency

	fmt.Println("Deep EXR features:")
	fmt.Println("- Variable samples per pixel")
	fmt.Println("- Z-depth with multiple layers")
	fmt.Println("- Perfect for volume rendering and compositing")

	// Deep data uses DeepFrameBuffer instead of FrameBuffer:
	// dfb := exr.NewDeepFrameBuffer()
	// dfb.SetSampleCounts(sampleCounts)
	// dfb.Set("Z", exr.NewDeepSlice(...))
}

// Example_compression demonstrates compression options.
func Example_compression() {
	// OpenEXR supports multiple compression methods, each optimized
	// for different use cases.

	compressionMethods := []struct {
		method string
		desc   string
	}{
		{"NONE", "No compression, fastest but largest files"},
		{"RLE", "Run-length encoding, fast, good for simple images"},
		{"ZIPS", "ZIP single scanline, good balance of speed and ratio"},
		{"ZIP", "ZIP 16 scanlines, better ratio than ZIPS"},
		{"PIZ", "Wavelet-based, best lossless ratio for most images"},
		{"PXR24", "Lossy 24-bit float, good for film production"},
		{"B44", "Lossy 4x4 block, fixed compression ratio"},
		{"B44A", "B44 with flat field detection"},
		{"DWAA", "Lossy DCT-based, excellent ratio for final images"},
		{"DWAB", "DWAA with larger blocks, better for large images"},
	}

	fmt.Println("OpenEXR Compression Methods:")
	for _, c := range compressionMethods {
		fmt.Printf("  %s: %s\n", c.method, c.desc)
	}
}

// Example_mipmaps demonstrates working with mipmap levels.
func Example_mipmaps() {
	// Mipmaps are pre-filtered versions of an image at decreasing resolutions.
	// They're essential for texture mapping to avoid aliasing.

	width, height := 1024, 1024
	header := exr.NewTiledHeader(width, height, 64, 64)
	header.SetTileDescription(exr.TileDescription{
		XSize:        64,
		YSize:        64,
		Mode:         exr.LevelModeMipmap,
		RoundingMode: exr.LevelRoundDown,
	})

	// Calculate number of mipmap levels
	// For a 1024x1024 image: 1024, 512, 256, 128, 64, 32, 16, 8, 4, 2, 1 = 11 levels
	numLevels := 0
	for s := width; s >= 1; s /= 2 {
		numLevels++
	}

	fmt.Printf("A %dx%d image has %d mipmap levels\n", width, height, numLevels)
}

// Example_memoryManagement demonstrates memory-efficient processing.
func Example_memoryManagement() {
	// For large images, memory management is important.
	// The library provides buffer pooling and memory limits.

	// Set a global memory limit (e.g., 512MB)
	exr.SetGlobalMemoryLimit(512 * 1024 * 1024)

	// Check current usage
	used := exr.GlobalMemoryUsed()
	limit := exr.GlobalMemoryLimit()
	allocs, hits, misses := exr.GlobalPoolStats()

	fmt.Printf("Memory: %d / %d bytes used\n", used, limit)
	fmt.Printf("Pool stats: %d allocations, %d hits, %d misses\n", allocs, hits, misses)

	// Reset limit to unlimited
	exr.SetGlobalMemoryLimit(0)
}

// Example_parallelProcessing demonstrates parallel reading/writing.
func Example_parallelProcessing() {
	// The library supports parallel processing for large images.

	// Configure worker pool
	config := exr.DefaultParallelConfig()
	config.NumWorkers = 4 // Use 4 workers
	config.GrainSize = 16 // Process 16 items per task

	fmt.Printf("Parallel config: %d workers, grain size %d\n",
		config.NumWorkers, config.GrainSize)

	// ParallelFor processes items in parallel:
	// exr.ParallelForWithError(0, height, func(y int) error {
	//     // Process scanline y
	//     return nil
	// })
}

// Example_halfPrecision demonstrates working with 16-bit floating point.
func Example_halfPrecision() {
	// OpenEXR uses 16-bit "half" precision floats for compact storage.
	// The half package provides conversion utilities.

	// Import: "github.com/mrjoshuak/go-openexr/half"

	// Convert float32 to half:
	// h := half.FromFloat32(1.5)

	// Convert back:
	// f := h.Float32()

	// Batch conversion for performance:
	// half.ConvertBatch32(dst []half.Half, src []float32)
	// half.ConvertBatchToFloat32(dst []float32, src []half.Half)

	fmt.Println("Half-precision float range: ~6x10^-8 to ~65504")
	fmt.Println("Precision: ~3 decimal digits")
}

// Example_hdrWorkflow demonstrates a typical HDR workflow.
func Example_hdrWorkflow() {
	// Typical HDR image processing workflow

	fmt.Println("HDR Workflow Steps:")
	fmt.Println("1. Read EXR file with Decode/DecodeFile")
	fmt.Println("2. Process pixel data (tone mapping, color grading)")
	fmt.Println("3. Write result with Encode/EncodeFile")
	fmt.Println("")
	fmt.Println("Example:")
	fmt.Println("  img, _ := exr.DecodeFile(\"input.exr\")")
	fmt.Println("  // Process img.Pix (float32 RGBA)")
	fmt.Println("  exr.EncodeFile(\"output.exr\", img)")
}

// Example_customChannels demonstrates working with custom channel configurations.
func Example_customChannels() {
	// EXR files can have any channel configuration, not just RGB/RGBA

	fmt.Println("Common channel configurations:")
	fmt.Println("  RGB - Color channels")
	fmt.Println("  RGBA - Color with alpha")
	fmt.Println("  Y - Luminance only")
	fmt.Println("  Z - Depth channel")
	fmt.Println("  N.x, N.y, N.z - Normal vectors")
	fmt.Println("  diffuse.R, diffuse.G, diffuse.B - Render passes")
	fmt.Println("")
	fmt.Println("Layer naming convention: layer.channel")
	fmt.Println("  Example: coat.R, coat.G, coat.B for coat layer")
}

// seekableBuffer wraps bytes.Buffer to implement io.WriteSeeker
type seekableBuffer struct {
	bytes.Buffer
	pos int64
}

func (s *seekableBuffer) Write(p []byte) (n int, err error) {
	for int(s.pos)+len(p) > s.Len() {
		s.Buffer.WriteByte(0)
	}
	data := s.Bytes()
	n = copy(data[s.pos:], p)
	s.pos += int64(n)
	if n < len(p) {
		m, err := s.Buffer.Write(p[n:])
		s.pos += int64(m)
		n += m
		if err != nil {
			return n, err
		}
	}
	return n, nil
}

func (s *seekableBuffer) Seek(offset int64, whence int) (int64, error) {
	switch whence {
	case 0:
		s.pos = offset
	case 1:
		s.pos += offset
	case 2:
		s.pos = int64(s.Len()) + offset
	}
	return s.pos, nil
}
