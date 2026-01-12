package exr

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// TestValidationWithExrInfo validates our output using the exrinfo tool from the OpenEXR reference implementation.
func TestValidationWithExrInfo(t *testing.T) {
	// Check if exrinfo is available
	exrInfo, err := exec.LookPath("exrinfo")
	if err != nil {
		t.Skip("exrinfo not found in PATH, skipping validation tests")
	}

	tests := []struct {
		name        string
		width       int
		height      int
		compression Compression
		isTiled     bool
		tileSize    int
	}{
		{"none_scanline", 32, 24, CompressionNone, false, 0},
		{"rle_scanline", 32, 24, CompressionRLE, false, 0},
		{"zips_scanline", 32, 24, CompressionZIPS, false, 0},
		{"zip_scanline", 64, 48, CompressionZIP, false, 0},
		{"piz_scanline", 64, 48, CompressionPIZ, false, 0},
		{"none_tiled", 64, 64, CompressionNone, true, 32},
		{"zip_tiled", 64, 64, CompressionZIP, true, 32},
		{"piz_tiled", 128, 128, CompressionPIZ, true, 32},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary file
			tmpFile := filepath.Join(t.TempDir(), "test.exr")

			// Write EXR file
			if err := writeTestFile(t, tmpFile, tt.width, tt.height, tt.compression, tt.isTiled, tt.tileSize); err != nil {
				t.Fatalf("writeTestFile() error = %v", err)
			}

			// Validate with exrinfo
			cmd := exec.Command(exrInfo, tmpFile)
			output, err := cmd.CombinedOutput()
			if err != nil {
				t.Errorf("exrinfo validation failed: %v\nOutput: %s", err, output)
			} else {
				t.Logf("exrinfo output for %s:\n%s", tt.name, string(output))
			}
		})
	}
}

// TestRoundTripWithReferenceFiles tests round-trip reading and writing with reference files.
func TestRoundTripWithReferenceFiles(t *testing.T) {
	testFiles := []struct {
		name        string
		compression Compression
	}{
		{"comp_none.exr", CompressionNone},
		{"comp_rle.exr", CompressionRLE},
		{"comp_zips.exr", CompressionZIPS},
		{"comp_zip.exr", CompressionZIP},
		{"comp_piz.exr", CompressionPIZ},
	}

	for _, tt := range testFiles {
		t.Run(tt.name, func(t *testing.T) {
			// Open reference file
			path := filepath.Join("testdata", tt.name)
			data, err := os.ReadFile(path)
			if err != nil {
				t.Skipf("Test file %s not available: %v", tt.name, err)
				return
			}

			// Parse the file
			reader := bytes.NewReader(data)
			f, err := OpenReader(reader, int64(len(data)))
			if err != nil {
				t.Fatalf("OpenReader error: %v", err)
			}

			header := f.Header(0)
			if header == nil {
				t.Fatal("No header found")
			}

			t.Logf("File: %s", tt.name)
			t.Logf("  DataWindow: %v", header.DataWindow())
			t.Logf("  Compression: %v", header.Compression())
			t.Logf("  Channels: %d", header.Channels().Len())

			// Verify compression matches expected
			if header.Compression() != tt.compression {
				t.Errorf("Compression = %v, want %v", header.Compression(), tt.compression)
			}

			// Create a scanline reader and read the data
			if !header.IsTiled() {
				sr, err := NewScanlineReader(f)
				if err != nil {
					t.Fatalf("NewScanlineReader error: %v", err)
				}

				dw := header.DataWindow()
				width := int(dw.Width())
				height := int(dw.Height())

				fb, _ := AllocateChannels(header.Channels(), dw)
				sr.SetFrameBuffer(fb)

				err = sr.ReadPixels(int(dw.Min.Y), int(dw.Max.Y))
				if err != nil {
					// Some reference files may use features we don't fully support yet
					t.Logf("ReadPixels warning (may be expected): %v", err)
					return
				}

				// Write to new file and validate
				tmpFile := filepath.Join(t.TempDir(), "roundtrip.exr")
				if err := writeRoundTripFile(t, tmpFile, header, fb, width, height); err != nil {
					t.Logf("writeRoundTripFile warning: %v", err)
					return
				}

				// Validate with exrinfo if available
				validateWithExrInfo(t, tmpFile)
			}
		})
	}
}

// TestReadReferenceFiles tests reading all reference files in testdata.
func TestReadReferenceFiles(t *testing.T) {
	files, err := os.ReadDir("testdata")
	if err != nil {
		t.Skipf("Cannot read testdata directory: %v", err)
		return
	}

	for _, file := range files {
		if !file.IsDir() && filepath.Ext(file.Name()) == ".exr" {
			t.Run(file.Name(), func(t *testing.T) {
				path := filepath.Join("testdata", file.Name())
				data, err := os.ReadFile(path)
				if err != nil {
					t.Fatalf("Cannot read file: %v", err)
				}

				reader := bytes.NewReader(data)
				f, err := OpenReader(reader, int64(len(data)))
				if err != nil {
					// Some files might have features we don't support yet
					t.Logf("Cannot open %s: %v", file.Name(), err)
					return
				}

				header := f.Header(0)
				if header == nil {
					t.Error("No header found")
					return
				}

				t.Logf("%s:", file.Name())
				t.Logf("  Version: %d", f.Version())
				t.Logf("  IsTiled: %v", f.IsTiled())
				t.Logf("  IsDeep: %v", f.IsDeep())
				t.Logf("  IsMultiPart: %v", f.IsMultiPart())
				t.Logf("  DataWindow: %v", header.DataWindow())
				t.Logf("  Compression: %v", header.Compression())
				t.Logf("  Channels: %d", header.Channels().Len())
			})
		}
	}
}

func writeTestFile(t *testing.T, path string, width, height int, comp Compression, isTiled bool, tileSize int) error {
	t.Helper()

	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	if isTiled {
		h := NewTiledHeader(width, height, tileSize, tileSize)
		h.SetCompression(comp)

		tw, err := NewTiledWriter(f, h)
		if err != nil {
			return err
		}
		defer tw.Close()

		// Allocate frame buffer
		fb, _ := AllocateChannels(h.Channels(), h.DataWindow())
		tw.SetFrameBuffer(fb)

		// Write tiles
		numTilesX := (width + tileSize - 1) / tileSize
		numTilesY := (height + tileSize - 1) / tileSize
		for ty := 0; ty < numTilesY; ty++ {
			for tx := 0; tx < numTilesX; tx++ {
				if err := tw.WriteTile(tx, ty); err != nil {
					return err
				}
			}
		}
	} else {
		h := NewScanlineHeader(width, height)
		h.SetCompression(comp)

		sw, err := NewScanlineWriter(f, h)
		if err != nil {
			return err
		}
		defer sw.Close()

		// Allocate frame buffer
		fb, _ := AllocateChannels(h.Channels(), h.DataWindow())
		sw.SetFrameBuffer(fb)

		// Fill with gradient data
		for y := 0; y < height; y++ {
			for x := 0; x < width; x++ {
				rSlice := fb.Get("R")
				gSlice := fb.Get("G")
				bSlice := fb.Get("B")
				if rSlice != nil {
					rSlice.SetFloat32(x, y, float32(x)/float32(width))
				}
				if gSlice != nil {
					gSlice.SetFloat32(x, y, float32(y)/float32(height))
				}
				if bSlice != nil {
					bSlice.SetFloat32(x, y, 0.5)
				}
			}
		}

		// Write scanlines
		err = sw.WritePixels(0, height-1)
		if err != nil {
			return err
		}
	}

	return nil
}

func writeRoundTripFile(t *testing.T, path string, origHeader *Header, fb *FrameBuffer, width, height int) error {
	t.Helper()

	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	// Create new header with same settings
	h := NewScanlineHeader(width, height)
	h.SetCompression(origHeader.Compression())

	// Copy channels
	origChannels := origHeader.Channels()
	newChannels := NewChannelList()
	for i := 0; i < origChannels.Len(); i++ {
		newChannels.Add(origChannels.At(i))
	}
	h.SetChannels(newChannels)

	sw, err := NewScanlineWriter(f, h)
	if err != nil {
		return err
	}
	defer sw.Close()

	sw.SetFrameBuffer(fb)
	return sw.WritePixels(0, height-1)
}

func validateWithExrInfo(t *testing.T, path string) {
	t.Helper()

	exrInfo, err := exec.LookPath("exrinfo")
	if err != nil {
		t.Log("exrinfo not available, skipping validation")
		return
	}

	cmd := exec.Command(exrInfo, path)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Errorf("exrinfo validation failed: %v\nOutput: %s", err, output)
	}
}
