package exr

import (
	"errors"
	"os"
	"strings"

	"github.com/mrjoshuak/go-openexr/half"
)

// Standard view names for stereo images
const (
	ViewLeft  = "left"
	ViewRight = "right"
)

// Multi-view errors
var (
	ErrViewNotFound    = errors.New("exr: view not found")
	ErrNotMultiView    = errors.New("exr: file is not a multi-view file")
	ErrNotStereo       = errors.New("exr: file is not a stereo file")
	ErrInvalidViewName = errors.New("exr: invalid view name")
)

// Header attribute names for multi-view
const (
	AttrNameMultiView = "multiView"
	AttrNameView      = "view"
)

// SetMultiView sets the multiView attribute for single-part multi-view files.
// The first view in the list is considered the default view.
func (h *Header) SetMultiView(views []string) {
	h.Set(&Attribute{Name: AttrNameMultiView, Type: AttrTypeStringVector, Value: views})
}

// MultiView returns the list of views in a single-part multi-view file.
// Returns nil if the file is not multi-view.
func (h *Header) MultiView() []string {
	attr := h.attrs[AttrNameMultiView]
	if attr == nil {
		return nil
	}
	return attr.Value.([]string)
}

// HasMultiView returns true if the header has a multiView attribute.
func (h *Header) HasMultiView() bool {
	return h.attrs[AttrNameMultiView] != nil
}

// SetView sets the view attribute for multi-part files.
func (h *Header) SetView(view string) {
	h.Set(&Attribute{Name: AttrNameView, Type: AttrTypeString, Value: view})
}

// View returns the view name for this part in a multi-part file.
// Returns empty string if not set.
func (h *Header) View() string {
	attr := h.attrs[AttrNameView]
	if attr == nil {
		return ""
	}
	return attr.Value.(string)
}

// HasView returns true if the header has a view attribute.
func (h *Header) HasView() bool {
	return h.attrs[AttrNameView] != nil
}

// IsMultiView returns true if the file contains multiple views.
// This checks for either multi-part with view attributes or single-part with multiView attribute.
func IsMultiView(f *File) bool {
	if f == nil {
		return false
	}

	// Check multi-part: look for view attributes
	if f.NumParts() > 1 {
		for i := 0; i < f.NumParts(); i++ {
			h := f.Header(i)
			if h != nil && h.HasView() {
				return true
			}
		}
	}

	// Check single-part: look for multiView attribute
	h := f.Header(0)
	if h != nil && h.HasMultiView() {
		return true
	}

	return false
}

// IsStereo returns true if the file contains stereo views (left and right).
func IsStereo(f *File) bool {
	views := GetViews(f)
	hasLeft := false
	hasRight := false
	for _, v := range views {
		if v == ViewLeft {
			hasLeft = true
		}
		if v == ViewRight {
			hasRight = true
		}
	}
	return hasLeft && hasRight
}

// GetViews returns all view names in the file.
func GetViews(f *File) []string {
	if f == nil {
		return nil
	}

	// Check multi-part first
	if f.NumParts() > 1 {
		viewSet := make(map[string]bool)
		for i := 0; i < f.NumParts(); i++ {
			h := f.Header(i)
			if h != nil {
				view := h.View()
				if view != "" {
					viewSet[view] = true
				}
			}
		}
		if len(viewSet) > 0 {
			views := make([]string, 0, len(viewSet))
			for v := range viewSet {
				views = append(views, v)
			}
			return views
		}
	}

	// Check single-part multiView attribute
	h := f.Header(0)
	if h != nil {
		if mv := h.MultiView(); mv != nil {
			return mv
		}
	}

	return nil
}

// GetDefaultView returns the default view name.
// For multi-part files, this is the view of the first part.
// For single-part with multiView, this is the first view in the list.
func GetDefaultView(f *File) string {
	if f == nil {
		return ""
	}

	// Check single-part multiView attribute
	h := f.Header(0)
	if h != nil {
		if mv := h.MultiView(); len(mv) > 0 {
			return mv[0]
		}
		// For multi-part, first part's view is default
		if h.HasView() {
			return h.View()
		}
	}

	return ""
}

// FindPartByView finds the part index for a given view in a multi-part file.
// Returns -1 if not found.
func FindPartByView(f *File, view string) int {
	if f == nil {
		return -1
	}

	for i := 0; i < f.NumParts(); i++ {
		h := f.Header(i)
		if h != nil && h.View() == view {
			return i
		}
	}

	return -1
}

// FindPartsByView finds all part indices for a given view.
// A file may have multiple parts per view (e.g., different layers).
func FindPartsByView(f *File, view string) []int {
	if f == nil {
		return nil
	}

	var parts []int
	for i := 0; i < f.NumParts(); i++ {
		h := f.Header(i)
		if h != nil && h.View() == view {
			parts = append(parts, i)
		}
	}

	return parts
}

// ViewChannelName represents a parsed channel name with view information.
type ViewChannelName struct {
	Layer   string // Layer name (empty for no layer)
	View    string // View name (empty for default view)
	Channel string // Base channel name (R, G, B, A, etc.)
}

// ParseViewChannelName parses a channel name in layer.view.channel format.
// Examples:
//   - "R" -> {Layer: "", View: "", Channel: "R"} (default view)
//   - "left.R" -> {Layer: "", View: "left", Channel: "R"}
//   - "diffuse.left.R" -> {Layer: "diffuse", View: "left", Channel: "R"}
func ParseViewChannelName(name string, views []string) ViewChannelName {
	parts := strings.Split(name, ".")

	if len(parts) == 1 {
		// Single component: just channel name (default view)
		return ViewChannelName{Channel: parts[0]}
	}

	if len(parts) == 2 {
		// Two components: could be layer.channel or view.channel
		// Check if first part is a known view
		for _, v := range views {
			if parts[0] == v {
				return ViewChannelName{View: parts[0], Channel: parts[1]}
			}
		}
		// Otherwise treat as layer.channel
		return ViewChannelName{Layer: parts[0], Channel: parts[1]}
	}

	// Three or more components: layer.view.channel
	// Last part is channel, second-to-last might be view
	channel := parts[len(parts)-1]
	potentialView := parts[len(parts)-2]

	for _, v := range views {
		if potentialView == v {
			// It's a view
			layer := strings.Join(parts[:len(parts)-2], ".")
			return ViewChannelName{Layer: layer, View: potentialView, Channel: channel}
		}
	}

	// No view found, treat all but last as layer
	layer := strings.Join(parts[:len(parts)-1], ".")
	return ViewChannelName{Layer: layer, Channel: channel}
}

// BuildViewChannelName constructs a channel name from components.
func BuildViewChannelName(layer, view, channel string) string {
	if layer == "" && view == "" {
		return channel
	}
	if layer == "" {
		return view + "." + channel
	}
	if view == "" {
		return layer + "." + channel
	}
	return layer + "." + view + "." + channel
}

// GetViewChannels returns all channels for a specific view in a single-part multi-view file.
func GetViewChannels(h *Header, view string) *ChannelList {
	if h == nil {
		return nil
	}

	views := h.MultiView()
	channels := h.Channels()
	if channels == nil {
		return nil
	}

	result := NewChannelList()
	defaultView := ""
	if len(views) > 0 {
		defaultView = views[0]
	}

	for i := 0; i < channels.Len(); i++ {
		ch := channels.At(i)
		parsed := ParseViewChannelName(ch.Name, views)

		// Match if view matches, or if this is default view and channel has no view prefix
		if parsed.View == view || (view == defaultView && parsed.View == "") {
			result.Add(ch)
		}
	}

	return result
}

// StereoInputFile provides a high-level interface for reading stereo images.
type StereoInputFile struct {
	file      *File
	leftPart  int
	rightPart int
}

// OpenStereoInputFile opens an EXR file for reading stereo data.
// The returned StereoInputFile must be closed to release the file handle.
func OpenStereoInputFile(path string) (*StereoInputFile, error) {
	f, err := OpenFile(path)
	if err != nil {
		return nil, err
	}
	stereo, err := NewStereoInputFile(f)
	if err != nil {
		f.Close()
		return nil, err
	}
	return stereo, nil
}

// NewStereoInputFile creates a stereo input file from an existing File.
func NewStereoInputFile(f *File) (*StereoInputFile, error) {
	if !IsStereo(f) {
		return nil, ErrNotStereo
	}

	leftPart := FindPartByView(f, ViewLeft)
	rightPart := FindPartByView(f, ViewRight)

	// For single-part stereo, both parts are 0
	if leftPart == -1 && rightPart == -1 {
		leftPart = 0
		rightPart = 0
	}

	return &StereoInputFile{
		file:      f,
		leftPart:  leftPart,
		rightPart: rightPart,
	}, nil
}

// File returns the underlying File.
func (s *StereoInputFile) File() *File {
	return s.file
}

// Close closes the underlying file.
func (s *StereoInputFile) Close() error {
	if s.file != nil {
		return s.file.Close()
	}
	return nil
}

// ReadLeftView reads the left eye view as RGBA.
func (s *StereoInputFile) ReadLeftView() (*RGBAImage, error) {
	return s.readView(ViewLeft)
}

// ReadRightView reads the right eye view as RGBA.
func (s *StereoInputFile) ReadRightView() (*RGBAImage, error) {
	return s.readView(ViewRight)
}

func (s *StereoInputFile) readView(view string) (*RGBAImage, error) {
	if s.file.NumParts() > 1 {
		// Multi-part: read from specific part
		partIdx := FindPartByView(s.file, view)
		if partIdx == -1 {
			return nil, ErrViewNotFound
		}
		return s.readPart(partIdx)
	}

	// Single-part: read channels with view prefix
	return s.readSinglePartView(view)
}

func (s *StereoInputFile) readPart(partIdx int) (*RGBAImage, error) {
	h := s.file.Header(partIdx)
	if h == nil {
		return nil, ErrInvalidHeader
	}

	dw := h.DataWindow()
	width := int(dw.Width())
	height := int(dw.Height())

	img := NewRGBAImage(RectFromSize(width, height))

	// Create frame buffer
	fb := NewFrameBuffer()
	channels := h.Channels()

	rChan := findChannel(channels, "R", "r")
	gChan := findChannel(channels, "G", "g")
	bChan := findChannel(channels, "B", "b")
	aChan := findChannel(channels, "A", "a")

	rData := make([]byte, width*height*4)
	gData := make([]byte, width*height*4)
	bData := make([]byte, width*height*4)
	aData := make([]byte, width*height*4)

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

	// Read using multipart reader
	mp := NewMultiPartInputFile(s.file)

	sr, err := mp.ScanlineReader(partIdx)
	if err != nil {
		return nil, err
	}
	sr.SetFrameBuffer(fb)

	yMin := int(dw.Min.Y)
	yMax := int(dw.Max.Y)
	if err := sr.ReadPixels(yMin, yMax); err != nil {
		return nil, err
	}

	// Convert to RGBAImage
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			var r, g, b, a float32
			if rChan != "" {
				r = fb.Get(rChan).GetFloat32(x, y)
			}
			if gChan != "" {
				g = fb.Get(gChan).GetFloat32(x, y)
			}
			if bChan != "" {
				b = fb.Get(bChan).GetFloat32(x, y)
			}
			if aChan != "" {
				a = fb.Get(aChan).GetFloat32(x, y)
			} else {
				a = 1.0
			}
			img.SetRGBA(x, y, r, g, b, a)
		}
	}

	return img, nil
}

func (s *StereoInputFile) readSinglePartView(view string) (*RGBAImage, error) {
	h := s.file.Header(0)
	if h == nil {
		return nil, ErrInvalidHeader
	}

	dw := h.DataWindow()
	width := int(dw.Width())
	height := int(dw.Height())

	img := NewRGBAImage(RectFromSize(width, height))
	fb := NewFrameBuffer()

	channels := h.Channels()
	views := h.MultiView()
	defaultView := ""
	if len(views) > 0 {
		defaultView = views[0]
	}

	// Find channels for this view
	var rChan, gChan, bChan, aChan string

	for i := 0; i < channels.Len(); i++ {
		ch := channels.At(i)
		parsed := ParseViewChannelName(ch.Name, views)

		// Match if view matches, or if this is default view and channel has no view prefix
		matches := parsed.View == view || (view == defaultView && parsed.View == "")
		if !matches {
			continue
		}

		switch parsed.Channel {
		case "R", "r":
			rChan = ch.Name
		case "G", "g":
			gChan = ch.Name
		case "B", "b":
			bChan = ch.Name
		case "A", "a":
			aChan = ch.Name
		}
	}

	rData := make([]byte, width*height*4)
	gData := make([]byte, width*height*4)
	bData := make([]byte, width*height*4)
	aData := make([]byte, width*height*4)

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

	sr, err := NewScanlineReader(s.file)
	if err != nil {
		return nil, err
	}
	sr.SetFrameBuffer(fb)

	yMin := int(dw.Min.Y)
	yMax := int(dw.Max.Y)
	if err := sr.ReadPixels(yMin, yMax); err != nil {
		return nil, err
	}

	// Convert to RGBAImage
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			var r, g, b, a float32
			if rChan != "" {
				r = fb.Get(rChan).GetFloat32(x, y)
			}
			if gChan != "" {
				g = fb.Get(gChan).GetFloat32(x, y)
			}
			if bChan != "" {
				b = fb.Get(bChan).GetFloat32(x, y)
			}
			if aChan != "" {
				a = fb.Get(aChan).GetFloat32(x, y)
			} else {
				a = 1.0
			}
			img.SetRGBA(x, y, r, g, b, a)
		}
	}

	return img, nil
}

// StereoOutputFile provides a high-level interface for writing stereo images.
type StereoOutputFile struct {
	path   string
	width  int
	height int
}

// NewStereoOutputFile creates a new stereo output file.
func NewStereoOutputFile(path string, width, height int) *StereoOutputFile {
	return &StereoOutputFile{
		path:   path,
		width:  width,
		height: height,
	}
}

// WriteStereo writes left and right views to a multi-part stereo file.
func (s *StereoOutputFile) WriteStereo(left, right *RGBAImage) error {
	return WriteStereoMultiPart(s.path, s.width, s.height, left, right, CompressionZIP)
}

// WriteStereoMultiPart writes stereo images as a multi-part EXR file.
func WriteStereoMultiPart(path string, width, height int, left, right *RGBAImage, compression Compression) error {
	// Create headers for each view
	leftHeader := NewScanlineHeader(width, height)
	leftHeader.SetCompression(compression)
	leftHeader.Set(&Attribute{Name: AttrNameName, Type: AttrTypeString, Value: "left"})
	leftHeader.Set(&Attribute{Name: AttrNameType, Type: AttrTypeString, Value: PartTypeScanline})
	leftHeader.SetView(ViewLeft)

	rightHeader := NewScanlineHeader(width, height)
	rightHeader.SetCompression(compression)
	rightHeader.Set(&Attribute{Name: AttrNameName, Type: AttrTypeString, Value: "right"})
	rightHeader.Set(&Attribute{Name: AttrNameType, Type: AttrTypeString, Value: PartTypeScanline})
	rightHeader.SetView(ViewRight)

	// Open file
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	// Use multi-part writer
	mp, err := NewMultiPartOutputFile(f, []*Header{leftHeader, rightHeader})
	if err != nil {
		return err
	}

	// Write left view
	if err := writeViewToMultiPart(mp, 0, width, height, left); err != nil {
		return err
	}

	// Write right view
	if err := writeViewToMultiPart(mp, 1, width, height, right); err != nil {
		return err
	}

	return mp.Close()
}

func writeViewToMultiPart(mp *MultiPartOutputFile, partIdx, width, height int, img *RGBAImage) error {
	fb := NewFrameBuffer()

	rData := make([]byte, width*height*2)
	gData := make([]byte, width*height*2)
	bData := make([]byte, width*height*2)

	fb.Set("R", NewSlice(PixelTypeHalf, rData, width, height))
	fb.Set("G", NewSlice(PixelTypeHalf, gData, width, height))
	fb.Set("B", NewSlice(PixelTypeHalf, bData, width, height))

	// Convert image data
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			r, g, b, _ := img.RGBA(x+img.Rect.Min.X, y+img.Rect.Min.Y)
			fb.Get("R").SetHalf(x, y, half.FromFloat32(r))
			fb.Get("G").SetHalf(x, y, half.FromFloat32(g))
			fb.Get("B").SetHalf(x, y, half.FromFloat32(b))
		}
	}

	// Set frame buffer and write
	if err := mp.SetFrameBuffer(partIdx, fb); err != nil {
		return err
	}

	return mp.WritePixels(partIdx, height)
}
