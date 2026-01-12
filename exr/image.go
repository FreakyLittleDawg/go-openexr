package exr

import (
	"errors"
	"image"
	"image/color"
	"io"
	"math"
	"os"

	"github.com/mrjoshuak/go-openexr/half"
)

// High-level API errors
var (
	ErrUnsupportedFormat = errors.New("exr: unsupported image format")
)

// Open opens an EXR file from a reader.
// The size parameter is required for random access.
func Open(r io.ReaderAt, size ...int64) (*File, error) {
	if len(size) > 0 {
		return OpenReader(r, size[0])
	}
	// Try to determine size from Seeker
	if seeker, ok := r.(io.Seeker); ok {
		current, err := seeker.Seek(0, io.SeekCurrent)
		if err == nil {
			end, err := seeker.Seek(0, io.SeekEnd)
			if err == nil {
				seeker.Seek(current, io.SeekStart)
				return OpenReader(r, end)
			}
		}
	}
	return nil, errors.New("exr: cannot determine file size, use OpenReader instead")
}

// OpenFile opens an EXR file from the filesystem.
// The returned File must be closed to release the file handle.
func OpenFile(path string) (*File, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	info, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, err
	}
	file, err := OpenReader(f, info.Size())
	if err != nil {
		f.Close()
		return nil, err
	}
	file.closer = f
	return file, nil
}

// OpenFileMmap opens an EXR file using memory mapping for zero-copy access.
// This provides the best read performance for large files.
// The returned File must be closed to release the memory mapping.
func OpenFileMmap(path string) (*File, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	mmap, err := newMmapReader(f)
	if err != nil {
		f.Close()
		return nil, err
	}

	file, err := OpenReader(mmap, mmap.Size())
	if err != nil {
		mmap.Close()
		return nil, err
	}
	file.closer = mmap
	return file, nil
}

// RGBAImage represents an RGBA image loaded from an EXR file.
type RGBAImage struct {
	// Pix holds the image's pixels in RGBA order.
	// Stored as float32 values in [0,1] range (can exceed for HDR).
	Pix []float32
	// Stride is the pixel stride (4 for RGBA).
	Stride int
	// Rect is the image's bounds.
	Rect image.Rectangle
}

// NewRGBAImage creates a new RGBA image with the given bounds.
func NewRGBAImage(r image.Rectangle) *RGBAImage {
	w, h := r.Dx(), r.Dy()
	return &RGBAImage{
		Pix:    make([]float32, w*h*4),
		Stride: 4,
		Rect:   r,
	}
}

// Bounds returns the domain for which At can return non-zero color.
func (img *RGBAImage) Bounds() image.Rectangle {
	return img.Rect
}

// ColorModel returns the Image's color model.
func (img *RGBAImage) ColorModel() color.Model {
	return color.RGBAModel
}

// At returns the color of the pixel at (x, y).
func (img *RGBAImage) At(x, y int) color.Color {
	if !(image.Point{x, y}.In(img.Rect)) {
		return color.RGBA{}
	}
	i := img.PixOffset(x, y)
	r := clamp01(img.Pix[i+0])
	g := clamp01(img.Pix[i+1])
	b := clamp01(img.Pix[i+2])
	a := clamp01(img.Pix[i+3])
	return color.RGBA{
		R: uint8(r * 255),
		G: uint8(g * 255),
		B: uint8(b * 255),
		A: uint8(a * 255),
	}
}

// PixOffset returns the index of the first element of Pix for pixel (x, y).
func (img *RGBAImage) PixOffset(x, y int) int {
	return (y-img.Rect.Min.Y)*img.Rect.Dx()*img.Stride + (x-img.Rect.Min.X)*img.Stride
}

// SetRGBA sets the pixel at (x, y) to the given values.
func (img *RGBAImage) SetRGBA(x, y int, r, g, b, a float32) {
	if !(image.Point{x, y}.In(img.Rect)) {
		return
	}
	i := img.PixOffset(x, y)
	img.Pix[i+0] = r
	img.Pix[i+1] = g
	img.Pix[i+2] = b
	img.Pix[i+3] = a
}

// RGBA returns the RGBA values at (x, y).
func (img *RGBAImage) RGBA(x, y int) (r, g, b, a float32) {
	if !(image.Point{x, y}.In(img.Rect)) {
		return 0, 0, 0, 0
	}
	i := img.PixOffset(x, y)
	return img.Pix[i+0], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3]
}

// clamp01 clamps a float to [0, 1] range.
func clamp01(v float32) float32 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

// RGBAInputFile provides a simple interface for reading RGBA images.
type RGBAInputFile struct {
	file   *File
	header *Header
	dw     Box2i
}

// OpenRGBAInputFile opens an EXR file for reading RGBA data.
// The returned RGBAInputFile must be closed to release the file handle.
func OpenRGBAInputFile(path string) (*RGBAInputFile, error) {
	f, err := OpenFile(path)
	if err != nil {
		return nil, err
	}
	rgba, err := NewRGBAInputFile(f)
	if err != nil {
		f.Close()
		return nil, err
	}
	return rgba, nil
}

// NewRGBAInputFile creates an RGBA input file from an existing File.
func NewRGBAInputFile(f *File) (*RGBAInputFile, error) {
	if f == nil {
		return nil, ErrInvalidFile
	}
	h := f.Header(0)
	if h == nil {
		return nil, ErrInvalidHeader
	}
	return &RGBAInputFile{
		file:   f,
		header: h,
		dw:     h.DataWindow(),
	}, nil
}

// Header returns the file header.
func (r *RGBAInputFile) Header() *Header {
	return r.header
}

// Close closes the underlying file.
func (r *RGBAInputFile) Close() error {
	if r.file != nil {
		return r.file.Close()
	}
	return nil
}

// DataWindow returns the data window.
func (r *RGBAInputFile) DataWindow() Box2i {
	return r.dw
}

// DisplayWindow returns the display window.
func (r *RGBAInputFile) DisplayWindow() Box2i {
	return r.header.DisplayWindow()
}

// Width returns the image width.
func (r *RGBAInputFile) Width() int {
	return int(r.dw.Width())
}

// Height returns the image height.
func (r *RGBAInputFile) Height() int {
	return int(r.dw.Height())
}

// ReadRGBA reads the entire image into an RGBAImage.
func (r *RGBAInputFile) ReadRGBA() (*RGBAImage, error) {
	width := r.Width()
	height := r.Height()

	img := NewRGBAImage(image.Rect(0, 0, width, height))

	// Create frame buffer
	fb := NewFrameBuffer()

	// Try to find RGBA channels
	channels := r.header.Channels()
	if channels == nil {
		return nil, ErrInvalidHeader
	}

	// Map channel names (support common naming conventions)
	rChan := findChannel(channels, "R", "r", "red", "Red")
	gChan := findChannel(channels, "G", "g", "green", "Green")
	bChan := findChannel(channels, "B", "b", "blue", "Blue")
	aChan := findChannel(channels, "A", "a", "alpha", "Alpha")

	// Create slices for each channel
	rData := make([]byte, width*height*4) // Float32
	gData := make([]byte, width*height*4)
	bData := make([]byte, width*height*4)
	aData := make([]byte, width*height*4)

	// Fill alpha with 1.0 by default
	for i := 0; i < len(aData); i += 4 {
		bits := math.Float32bits(1.0)
		aData[i] = byte(bits)
		aData[i+1] = byte(bits >> 8)
		aData[i+2] = byte(bits >> 16)
		aData[i+3] = byte(bits >> 24)
	}

	if rChan != "" {
		fb.Set(rChan, NewSlice(PixelTypeFloat, rData, width, height))
	}
	if gChan != "" {
		fb.Set(gChan, NewSlice(PixelTypeFloat, gData, width, height))
	}
	if bChan != "" {
		fb.Set(bChan, NewSlice(PixelTypeFloat, bData, width, height))
	}
	if aChan != "" {
		fb.Set(aChan, NewSlice(PixelTypeFloat, aData, width, height))
	}

	// Read using appropriate reader
	if r.header.IsTiled() {
		tr, err := NewTiledReader(r.file)
		if err != nil {
			return nil, err
		}
		tr.SetFrameBuffer(fb)

		td := r.header.TileDescription()
		tilesX := (width + int(td.XSize) - 1) / int(td.XSize)
		tilesY := (height + int(td.YSize) - 1) / int(td.YSize)

		if err := tr.ReadTiles(0, 0, tilesX-1, tilesY-1); err != nil {
			return nil, err
		}
	} else {
		sr, err := NewScanlineReader(r.file)
		if err != nil {
			return nil, err
		}
		sr.SetFrameBuffer(fb)

		yMin := int(r.dw.Min.Y)
		yMax := int(r.dw.Max.Y)
		if err := sr.ReadPixels(yMin, yMax); err != nil {
			return nil, err
		}
	}

	// Convert frame buffer to RGBAImage
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			var rv, gv, bv, av float32

			if rChan != "" {
				if slice := fb.Get(rChan); slice != nil {
					rv = slice.GetFloat32(x, y)
				}
			}
			if gChan != "" {
				if slice := fb.Get(gChan); slice != nil {
					gv = slice.GetFloat32(x, y)
				}
			}
			if bChan != "" {
				if slice := fb.Get(bChan); slice != nil {
					bv = slice.GetFloat32(x, y)
				}
			}
			if aChan != "" {
				if slice := fb.Get(aChan); slice != nil {
					av = slice.GetFloat32(x, y)
				}
			} else {
				av = 1.0
			}

			img.SetRGBA(x, y, rv, gv, bv, av)
		}
	}

	return img, nil
}

// findChannel finds a channel by trying multiple names.
func findChannel(cl *ChannelList, names ...string) string {
	for _, name := range names {
		for i := 0; i < cl.Len(); i++ {
			if cl.At(i).Name == name {
				return name
			}
		}
	}
	return ""
}

// RGBAOutputFile provides a simple interface for writing RGBA images.
type RGBAOutputFile struct {
	path   string
	header *Header
	width  int
	height int
}

// NewRGBAOutputFile creates a new RGBA output file.
func NewRGBAOutputFile(path string, width, height int) (*RGBAOutputFile, error) {
	h := NewScanlineHeader(width, height)
	h.SetCompression(CompressionZIP)

	// Add RGBA channels
	channels := NewChannelList()
	channels.Add(Channel{Name: "R", Type: PixelTypeHalf, XSampling: 1, YSampling: 1})
	channels.Add(Channel{Name: "G", Type: PixelTypeHalf, XSampling: 1, YSampling: 1})
	channels.Add(Channel{Name: "B", Type: PixelTypeHalf, XSampling: 1, YSampling: 1})
	channels.Add(Channel{Name: "A", Type: PixelTypeHalf, XSampling: 1, YSampling: 1})
	h.SetChannels(channels)

	return &RGBAOutputFile{
		path:   path,
		header: h,
		width:  width,
		height: height,
	}, nil
}

// Header returns the header for configuration.
func (w *RGBAOutputFile) Header() *Header {
	return w.header
}

// WriteRGBA writes an RGBAImage to the file.
func (w *RGBAOutputFile) WriteRGBA(img *RGBAImage) error {
	f, err := os.Create(w.path)
	if err != nil {
		return err
	}
	defer f.Close()

	// Create frame buffer
	fb := NewFrameBuffer()
	rData := make([]byte, w.width*w.height*2) // Half
	gData := make([]byte, w.width*w.height*2)
	bData := make([]byte, w.width*w.height*2)
	aData := make([]byte, w.width*w.height*2)

	fb.Set("R", NewSlice(PixelTypeHalf, rData, w.width, w.height))
	fb.Set("G", NewSlice(PixelTypeHalf, gData, w.width, w.height))
	fb.Set("B", NewSlice(PixelTypeHalf, bData, w.width, w.height))
	fb.Set("A", NewSlice(PixelTypeHalf, aData, w.width, w.height))

	// Convert RGBAImage to frame buffer
	for y := 0; y < w.height; y++ {
		for x := 0; x < w.width; x++ {
			r, g, b, a := img.RGBA(x+img.Rect.Min.X, y+img.Rect.Min.Y)
			fb.Get("R").SetHalf(x, y, half.FromFloat32(r))
			fb.Get("G").SetHalf(x, y, half.FromFloat32(g))
			fb.Get("B").SetHalf(x, y, half.FromFloat32(b))
			fb.Get("A").SetHalf(x, y, half.FromFloat32(a))
		}
	}

	// Write file
	sw, err := NewScanlineWriter(f, w.header)
	if err != nil {
		return err
	}
	sw.SetFrameBuffer(fb)

	yMin := int(w.header.DataWindow().Min.Y)
	yMax := int(w.header.DataWindow().Max.Y)
	if err := sw.WritePixels(yMin, yMax); err != nil {
		return err
	}

	return sw.Close()
}

// Decode decodes an EXR image from a reader into an RGBAImage.
func Decode(r io.ReaderAt, size int64) (*RGBAImage, error) {
	f, err := OpenReader(r, size)
	if err != nil {
		return nil, err
	}
	rgba, err := NewRGBAInputFile(f)
	if err != nil {
		return nil, err
	}
	return rgba.ReadRGBA()
}

// DecodeFile decodes an EXR file from the filesystem.
func DecodeFile(path string) (*RGBAImage, error) {
	f, err := OpenFile(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	rgba, err := NewRGBAInputFile(f)
	if err != nil {
		return nil, err
	}
	return rgba.ReadRGBA()
}

// Encode encodes an RGBAImage to an EXR file.
func Encode(w io.WriteSeeker, img *RGBAImage) error {
	width := img.Rect.Dx()
	height := img.Rect.Dy()

	h := NewScanlineHeader(width, height)
	h.SetCompression(CompressionZIP)

	channels := NewChannelList()
	channels.Add(Channel{Name: "R", Type: PixelTypeHalf, XSampling: 1, YSampling: 1})
	channels.Add(Channel{Name: "G", Type: PixelTypeHalf, XSampling: 1, YSampling: 1})
	channels.Add(Channel{Name: "B", Type: PixelTypeHalf, XSampling: 1, YSampling: 1})
	channels.Add(Channel{Name: "A", Type: PixelTypeHalf, XSampling: 1, YSampling: 1})
	h.SetChannels(channels)

	// Create frame buffer
	fb := NewFrameBuffer()
	rData := make([]byte, width*height*2)
	gData := make([]byte, width*height*2)
	bData := make([]byte, width*height*2)
	aData := make([]byte, width*height*2)

	fb.Set("R", NewSlice(PixelTypeHalf, rData, width, height))
	fb.Set("G", NewSlice(PixelTypeHalf, gData, width, height))
	fb.Set("B", NewSlice(PixelTypeHalf, bData, width, height))
	fb.Set("A", NewSlice(PixelTypeHalf, aData, width, height))

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			r, g, b, a := img.RGBA(x+img.Rect.Min.X, y+img.Rect.Min.Y)
			fb.Get("R").SetHalf(x, y, half.FromFloat32(r))
			fb.Get("G").SetHalf(x, y, half.FromFloat32(g))
			fb.Get("B").SetHalf(x, y, half.FromFloat32(b))
			fb.Get("A").SetHalf(x, y, half.FromFloat32(a))
		}
	}

	sw, err := NewScanlineWriter(w, h)
	if err != nil {
		return err
	}
	sw.SetFrameBuffer(fb)

	yMin := int(h.DataWindow().Min.Y)
	yMax := int(h.DataWindow().Max.Y)
	if err := sw.WritePixels(yMin, yMax); err != nil {
		return err
	}

	return sw.Close()
}

// EncodeFile encodes an RGBAImage to an EXR file on disk.
func EncodeFile(path string, img *RGBAImage) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return Encode(f, img)
}
