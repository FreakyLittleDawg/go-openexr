// Package exrutil provides EXR-specific utility functions.
//
// This package offers higher-level operations for working with OpenEXR files,
// including channel extraction, file information, validation, and conversion utilities.
//
// Example usage:
//
//	info, _ := exrutil.GetFileInfo("render.exr")
//	fmt.Printf("Size: %dx%d, Channels: %v\n", info.Width, info.Height, info.Channels)
//
//	depth, _ := exrutil.ExtractChannel(f, "Z")
package exrutil

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/mrjoshuak/go-openexr/exr"
	"github.com/mrjoshuak/go-openexr/half"
)

// ===========================================
// File Information
// ===========================================

// FileInfo provides a summary of an EXR file.
type FileInfo struct {
	Path        string
	Width       int
	Height      int
	Compression exr.Compression
	IsTiled     bool
	TileWidth   int
	TileHeight  int
	IsDeep      bool
	IsMultiPart bool
	NumParts    int
	Channels    []string
	HasPreview  bool
	FileSize    int64
}

// GetFileInfo returns summary information about an EXR file.
func GetFileInfo(path string) (*FileInfo, error) {
	// Get file size
	stat, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	f, err := exr.OpenFile(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	h := f.Header(0)
	info := &FileInfo{
		Path:        path,
		Width:       h.Width(),
		Height:      h.Height(),
		Compression: h.Compression(),
		IsTiled:     h.IsTiled(),
		IsDeep:      f.IsDeep(),
		IsMultiPart: f.IsMultiPart(),
		NumParts:    f.NumParts(),
		HasPreview:  h.HasPreview(),
		FileSize:    stat.Size(),
	}

	// Get channel names
	if cl := h.Channels(); cl != nil {
		for _, ch := range cl.Channels() {
			info.Channels = append(info.Channels, ch.Name)
		}
	}

	// Get tile info if tiled
	if info.IsTiled {
		if td := h.TileDescription(); td != nil {
			info.TileWidth = int(td.XSize)
			info.TileHeight = int(td.YSize)
		}
	}

	return info, nil
}

// ===========================================
// Channel Utilities
// ===========================================

// ExtractChannel extracts a single channel as a float32 slice.
// The channel data is converted to float32 regardless of storage type.
func ExtractChannel(f *exr.File, channelName string) ([]float32, error) {
	h := f.Header(0)
	width := h.Width()
	height := h.Height()

	// Find the channel
	cl := h.Channels()
	if cl == nil {
		return nil, fmt.Errorf("exrutil: no channels in file")
	}

	ch := cl.Get(channelName)
	if ch == nil {
		return nil, fmt.Errorf("exrutil: channel %q not found", channelName)
	}

	// Allocate output
	pixels := width * height
	result := make([]float32, pixels)

	// Create frame buffer with just this channel
	fb := exr.NewFrameBuffer()

	switch ch.Type {
	case exr.PixelTypeHalf:
		data := make([]half.Half, pixels)
		fb.Insert(channelName, exr.NewSliceFromHalf(data, width, height))

		if err := readPixels(f, fb); err != nil {
			return nil, err
		}

		// Convert to float32
		for i, hv := range data {
			result[i] = hv.Float32()
		}

	case exr.PixelTypeFloat:
		fb.Insert(channelName, exr.NewSliceFromFloat32(result, width, height))

		if err := readPixels(f, fb); err != nil {
			return nil, err
		}

	case exr.PixelTypeUint:
		data := make([]uint32, pixels)
		fb.Insert(channelName, exr.NewSliceFromUint32(data, width, height))

		if err := readPixels(f, fb); err != nil {
			return nil, err
		}

		// Convert to float32
		for i, u := range data {
			result[i] = float32(u)
		}
	}

	return result, nil
}

// ExtractChannels extracts multiple channels, returning a map of channel name to float32 slice.
func ExtractChannels(f *exr.File, channelNames ...string) (map[string][]float32, error) {
	result := make(map[string][]float32)

	for _, name := range channelNames {
		data, err := ExtractChannel(f, name)
		if err != nil {
			return nil, err
		}
		result[name] = data
	}

	return result, nil
}

// readPixels reads pixels using the appropriate reader type.
func readPixels(f *exr.File, fb *exr.FrameBuffer) error {
	h := f.Header(0)

	if h.IsTiled() {
		tr, err := exr.NewTiledReader(f)
		if err != nil {
			return err
		}
		tr.SetFrameBuffer(fb)
		// ReadTiles takes (tileX1, tileY1, tileX2, tileY2)
		return tr.ReadTiles(0, 0, h.NumXTiles(0)-1, h.NumYTiles(0)-1)
	}

	sr, err := exr.NewScanlineReader(f)
	if err != nil {
		return err
	}
	sr.SetFrameBuffer(fb)
	return sr.ReadPixels(0, h.Height()-1)
}

// SplitLayers returns channel names grouped by layer (dot-separated prefix).
// Channels without a layer prefix are grouped under an empty string key.
func SplitLayers(h *exr.Header) map[string][]string {
	layers := make(map[string][]string)

	cl := h.Channels()
	if cl == nil {
		return layers
	}

	for _, ch := range cl.Channels() {
		layer := ""
		name := ch.Name

		// Find the last dot to separate layer from channel
		if idx := strings.LastIndex(ch.Name, "."); idx >= 0 {
			layer = ch.Name[:idx]
			name = ch.Name[idx+1:]
		}

		layers[layer] = append(layers[layer], name)
	}

	return layers
}

// ListLayers returns a sorted list of layer names in the file.
// Returns an empty slice if there are no layers (all channels at root level).
func ListLayers(h *exr.Header) []string {
	layerMap := SplitLayers(h)

	var layers []string
	for layer := range layerMap {
		if layer != "" {
			layers = append(layers, layer)
		}
	}

	sort.Strings(layers)
	return layers
}

// ===========================================
// Validation
// ===========================================

// ValidationResult contains the results of file validation.
type ValidationResult struct {
	Valid    bool
	Warnings []string
	Errors   []string
}

// ValidateFile performs comprehensive validation of an EXR file.
func ValidateFile(path string) (*ValidationResult, error) {
	result := &ValidationResult{Valid: true}

	// Check file exists and is readable
	stat, err := os.Stat(path)
	if err != nil {
		result.Valid = false
		result.Errors = append(result.Errors, fmt.Sprintf("cannot access file: %v", err))
		return result, nil
	}

	if stat.Size() < 8 {
		result.Valid = false
		result.Errors = append(result.Errors, "file too small to be valid EXR")
		return result, nil
	}

	// Try to open the file
	f, err := exr.OpenFile(path)
	if err != nil {
		result.Valid = false
		result.Errors = append(result.Errors, fmt.Sprintf("cannot open file: %v", err))
		return result, nil
	}
	defer f.Close()

	h := f.Header(0)

	// Validate header
	if err := h.Validate(); err != nil {
		result.Valid = false
		result.Errors = append(result.Errors, fmt.Sprintf("header validation failed: %v", err))
	}

	// Check for empty image
	if h.Width() == 0 || h.Height() == 0 {
		result.Valid = false
		result.Errors = append(result.Errors, "image has zero dimensions")
	}

	// Check for channels
	cl := h.Channels()
	if cl == nil || cl.Len() == 0 {
		result.Valid = false
		result.Errors = append(result.Errors, "no channels defined")
	}

	// Warnings for unusual configurations
	if h.Width() > 32768 || h.Height() > 32768 {
		result.Warnings = append(result.Warnings, "very large image dimensions")
	}

	if cl != nil && cl.Len() > 100 {
		result.Warnings = append(result.Warnings, fmt.Sprintf("large number of channels: %d", cl.Len()))
	}

	// Check compression is valid
	comp := h.Compression()
	if comp > exr.CompressionDWAB {
		result.Warnings = append(result.Warnings, fmt.Sprintf("unknown compression type: %d", comp))
	}

	return result, nil
}

// ===========================================
// Comparison
// ===========================================

// CompareOptions configures file comparison behavior.
type CompareOptions struct {
	Tolerance      float32 // Maximum allowed difference for pixel values
	IgnoreMetadata bool    // If true, only compare pixel data
}

// CompareFiles checks if two EXR files have equivalent content.
// Returns true if files match within tolerance, along with any differences found.
func CompareFiles(path1, path2 string, opts CompareOptions) (bool, []string, error) {
	var diffs []string

	f1, err := exr.OpenFile(path1)
	if err != nil {
		return false, nil, fmt.Errorf("cannot open %s: %w", path1, err)
	}
	defer f1.Close()

	f2, err := exr.OpenFile(path2)
	if err != nil {
		return false, nil, fmt.Errorf("cannot open %s: %w", path2, err)
	}
	defer f2.Close()

	h1, h2 := f1.Header(0), f2.Header(0)

	// Compare dimensions
	if h1.Width() != h2.Width() || h1.Height() != h2.Height() {
		diffs = append(diffs, fmt.Sprintf("dimensions differ: %dx%d vs %dx%d",
			h1.Width(), h1.Height(), h2.Width(), h2.Height()))
		return false, diffs, nil
	}

	// Compare channels
	cl1, cl2 := h1.Channels(), h2.Channels()
	if cl1 == nil || cl2 == nil {
		if cl1 == nil && cl2 == nil {
			// Both have no channels - unusual but not a difference
			return len(diffs) == 0, diffs, nil
		}
		if cl1 == nil {
			diffs = append(diffs, "file1 has no channels, file2 has channels")
		} else {
			diffs = append(diffs, "file1 has channels, file2 has no channels")
		}
		return false, diffs, nil
	}
	if cl1.Len() != cl2.Len() {
		diffs = append(diffs, fmt.Sprintf("channel count differs: %d vs %d", cl1.Len(), cl2.Len()))
	}

	// Build channel name sets
	names1 := make(map[string]bool)
	for _, ch := range cl1.Channels() {
		names1[ch.Name] = true
	}

	for _, ch := range cl2.Channels() {
		if !names1[ch.Name] {
			diffs = append(diffs, fmt.Sprintf("channel %q in file2 but not file1", ch.Name))
		}
	}

	names2 := make(map[string]bool)
	for _, ch := range cl2.Channels() {
		names2[ch.Name] = true
	}

	for _, ch := range cl1.Channels() {
		if !names2[ch.Name] {
			diffs = append(diffs, fmt.Sprintf("channel %q in file1 but not file2", ch.Name))
		}
	}

	if !opts.IgnoreMetadata {
		// Compare compression
		if h1.Compression() != h2.Compression() {
			diffs = append(diffs, fmt.Sprintf("compression differs: %v vs %v",
				h1.Compression(), h2.Compression()))
		}
	}

	// Compare pixel data for common channels
	for _, ch := range cl1.Channels() {
		if !names2[ch.Name] {
			continue
		}

		data1, err := ExtractChannel(f1, ch.Name)
		if err != nil {
			return false, nil, fmt.Errorf("error reading channel %s from file1: %w", ch.Name, err)
		}

		data2, err := ExtractChannel(f2, ch.Name)
		if err != nil {
			return false, nil, fmt.Errorf("error reading channel %s from file2: %w", ch.Name, err)
		}

		maxDiff := float32(0)
		diffCount := 0

		for i := range data1 {
			diff := data1[i] - data2[i]
			if diff < 0 {
				diff = -diff
			}
			if diff > opts.Tolerance {
				diffCount++
				if diff > maxDiff {
					maxDiff = diff
				}
			}
		}

		if diffCount > 0 {
			diffs = append(diffs, fmt.Sprintf("channel %q: %d pixels differ (max diff: %f)",
				ch.Name, diffCount, maxDiff))
		}
	}

	return len(diffs) == 0, diffs, nil
}

// ===========================================
// Conversion Utilities
// ===========================================

// ConvertCompression reads an EXR file and writes it with a different compression.
//
// Deprecated: This function does not actually use the compression parameter.
// The exr.EncodeFile API currently hardcodes ZIP compression. To convert
// compression, use the low-level API (NewScanlineWriter) with header.SetCompression().
// This function remains for API compatibility but will always use ZIP compression.
func ConvertCompression(input, output string, compression exr.Compression) error {
	_ = compression // Unused - see deprecation notice

	// Read the input file
	img, err := exr.DecodeFile(input)
	if err != nil {
		return fmt.Errorf("failed to read input: %w", err)
	}

	// Note: EncodeFile hardcodes ZIP compression. The compression parameter
	// is ignored. See deprecation notice above.
	return exr.EncodeFile(output, img)
}

// CopyMetadata copies all metadata attributes from src to dst header.
// This excludes structural attributes like channels, dataWindow, etc.
func CopyMetadata(src, dst *exr.Header) {
	// List of structural attributes that shouldn't be copied
	structural := map[string]bool{
		"channels":           true,
		"compression":        true,
		"dataWindow":         true,
		"displayWindow":      true,
		"lineOrder":          true,
		"pixelAspectRatio":   true,
		"screenWindowCenter": true,
		"screenWindowWidth":  true,
		"tiles":              true,
		"type":               true,
		"name":               true,
		"version":            true,
		"chunkCount":         true,
	}

	for _, attr := range src.Attributes() {
		if !structural[attr.Name] {
			dst.Set(attr)
		}
	}
}
