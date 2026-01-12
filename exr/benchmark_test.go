package exr

import (
	"bytes"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/mrjoshuak/go-openexr/half"
)

// BenchmarkConfig holds configuration for a benchmark run
type BenchmarkConfig struct {
	Width       int
	Height      int
	Compression Compression
	UseHalf     bool // true for half, false for float
}

// benchmarkWriteImage benchmarks writing an image with the given configuration
func benchmarkWriteImage(b *testing.B, cfg BenchmarkConfig) {
	b.Helper()

	// Create header with specified compression
	h := NewScanlineHeader(cfg.Width, cfg.Height)
	h.SetCompression(cfg.Compression)

	// Set pixel type for channels
	cl := NewChannelList()
	pixelType := PixelTypeFloat
	if cfg.UseHalf {
		pixelType = PixelTypeHalf
	}
	cl.Add(Channel{Name: "R", Type: pixelType, XSampling: 1, YSampling: 1})
	cl.Add(Channel{Name: "G", Type: pixelType, XSampling: 1, YSampling: 1})
	cl.Add(Channel{Name: "B", Type: pixelType, XSampling: 1, YSampling: 1})
	cl.Add(Channel{Name: "A", Type: pixelType, XSampling: 1, YSampling: 1})
	h.SetChannels(cl)

	// Create frame buffer with test pattern
	fb, _ := AllocateChannels(cl, h.DataWindow())
	for _, name := range []string{"R", "G", "B", "A"} {
		slice := fb.Get(name)
		for y := 0; y < cfg.Height; y++ {
			for x := 0; x < cfg.Width; x++ {
				val := float32(x+y) / float32(cfg.Width+cfg.Height)
				if cfg.UseHalf {
					slice.SetHalf(x, y, half.FromFloat32(val))
				} else {
					slice.SetFloat32(x, y, val)
				}
			}
		}
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		ws := newMockWriteSeeker()
		sw, err := NewScanlineWriter(ws, h)
		if err != nil {
			b.Fatalf("NewScanlineWriter() error = %v", err)
		}
		sw.SetFrameBuffer(fb)
		if err := sw.WritePixels(0, cfg.Height-1); err != nil {
			b.Fatalf("WritePixels() error = %v", err)
		}
		sw.Close()
	}
}

// benchmarkReadImage benchmarks reading an image with the given configuration
func benchmarkReadImage(b *testing.B, cfg BenchmarkConfig) {
	b.Helper()

	// First create the image to read
	h := NewScanlineHeader(cfg.Width, cfg.Height)
	h.SetCompression(cfg.Compression)

	cl := NewChannelList()
	pixelType := PixelTypeFloat
	if cfg.UseHalf {
		pixelType = PixelTypeHalf
	}
	cl.Add(Channel{Name: "R", Type: pixelType, XSampling: 1, YSampling: 1})
	cl.Add(Channel{Name: "G", Type: pixelType, XSampling: 1, YSampling: 1})
	cl.Add(Channel{Name: "B", Type: pixelType, XSampling: 1, YSampling: 1})
	cl.Add(Channel{Name: "A", Type: pixelType, XSampling: 1, YSampling: 1})
	h.SetChannels(cl)

	fb, _ := AllocateChannels(cl, h.DataWindow())
	for _, name := range []string{"R", "G", "B", "A"} {
		slice := fb.Get(name)
		for y := 0; y < cfg.Height; y++ {
			for x := 0; x < cfg.Width; x++ {
				val := float32(x+y) / float32(cfg.Width+cfg.Height)
				if cfg.UseHalf {
					slice.SetHalf(x, y, half.FromFloat32(val))
				} else {
					slice.SetFloat32(x, y, val)
				}
			}
		}
	}

	ws := newMockWriteSeeker()
	sw, _ := NewScanlineWriter(ws, h)
	sw.SetFrameBuffer(fb)
	sw.WritePixels(0, cfg.Height-1)
	sw.Close()

	data := ws.Bytes()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		reader := &readerAtWrapper{bytes.NewReader(data)}
		f, err := OpenReader(reader, int64(len(data)))
		if err != nil {
			b.Fatalf("OpenReader() error = %v", err)
		}

		sr, err := NewScanlineReader(f)
		if err != nil {
			b.Fatalf("NewScanlineReader() error = %v", err)
		}

		readFB, _ := AllocateChannels(sr.Header().Channels(), sr.DataWindow())
		sr.SetFrameBuffer(readFB)

		if err := sr.ReadPixels(0, cfg.Height-1); err != nil {
			b.Fatalf("ReadPixels() error = %v", err)
		}
	}
}

// Compression benchmarks matching C++ exrmetrics
// Image size: 720x576 (similar to Flowers.exr)

func BenchmarkWrite_None_Half(b *testing.B) {
	benchmarkWriteImage(b, BenchmarkConfig{Width: 720, Height: 576, Compression: CompressionNone, UseHalf: true})
}

func BenchmarkWrite_None_Float(b *testing.B) {
	benchmarkWriteImage(b, BenchmarkConfig{Width: 720, Height: 576, Compression: CompressionNone, UseHalf: false})
}

func BenchmarkWrite_RLE_Half(b *testing.B) {
	benchmarkWriteImage(b, BenchmarkConfig{Width: 720, Height: 576, Compression: CompressionRLE, UseHalf: true})
}

func BenchmarkWrite_RLE_Float(b *testing.B) {
	benchmarkWriteImage(b, BenchmarkConfig{Width: 720, Height: 576, Compression: CompressionRLE, UseHalf: false})
}

func BenchmarkWrite_ZIPS_Half(b *testing.B) {
	benchmarkWriteImage(b, BenchmarkConfig{Width: 720, Height: 576, Compression: CompressionZIPS, UseHalf: true})
}

func BenchmarkWrite_ZIPS_Float(b *testing.B) {
	benchmarkWriteImage(b, BenchmarkConfig{Width: 720, Height: 576, Compression: CompressionZIPS, UseHalf: false})
}

func BenchmarkWrite_ZIP_Half(b *testing.B) {
	benchmarkWriteImage(b, BenchmarkConfig{Width: 720, Height: 576, Compression: CompressionZIP, UseHalf: true})
}

func BenchmarkWrite_ZIP_Float(b *testing.B) {
	benchmarkWriteImage(b, BenchmarkConfig{Width: 720, Height: 576, Compression: CompressionZIP, UseHalf: false})
}

func BenchmarkWrite_PIZ_Half(b *testing.B) {
	benchmarkWriteImage(b, BenchmarkConfig{Width: 720, Height: 576, Compression: CompressionPIZ, UseHalf: true})
}

func BenchmarkWrite_PIZ_Float(b *testing.B) {
	benchmarkWriteImage(b, BenchmarkConfig{Width: 720, Height: 576, Compression: CompressionPIZ, UseHalf: false})
}

func BenchmarkWrite_PXR24_Half(b *testing.B) {
	benchmarkWriteImage(b, BenchmarkConfig{Width: 720, Height: 576, Compression: CompressionPXR24, UseHalf: true})
}

func BenchmarkWrite_PXR24_Float(b *testing.B) {
	benchmarkWriteImage(b, BenchmarkConfig{Width: 720, Height: 576, Compression: CompressionPXR24, UseHalf: false})
}

func BenchmarkWrite_B44_Half(b *testing.B) {
	benchmarkWriteImage(b, BenchmarkConfig{Width: 720, Height: 576, Compression: CompressionB44, UseHalf: true})
}

func BenchmarkWrite_B44_Float(b *testing.B) {
	benchmarkWriteImage(b, BenchmarkConfig{Width: 720, Height: 576, Compression: CompressionB44, UseHalf: false})
}

func BenchmarkWrite_B44A_Half(b *testing.B) {
	benchmarkWriteImage(b, BenchmarkConfig{Width: 720, Height: 576, Compression: CompressionB44A, UseHalf: true})
}

func BenchmarkWrite_B44A_Float(b *testing.B) {
	benchmarkWriteImage(b, BenchmarkConfig{Width: 720, Height: 576, Compression: CompressionB44A, UseHalf: false})
}

func BenchmarkWrite_DWAA_Half(b *testing.B) {
	benchmarkWriteImage(b, BenchmarkConfig{Width: 720, Height: 576, Compression: CompressionDWAA, UseHalf: true})
}

func BenchmarkWrite_DWAA_Float(b *testing.B) {
	benchmarkWriteImage(b, BenchmarkConfig{Width: 720, Height: 576, Compression: CompressionDWAA, UseHalf: false})
}

func BenchmarkWrite_DWAB_Half(b *testing.B) {
	benchmarkWriteImage(b, BenchmarkConfig{Width: 720, Height: 576, Compression: CompressionDWAB, UseHalf: true})
}

func BenchmarkWrite_DWAB_Float(b *testing.B) {
	benchmarkWriteImage(b, BenchmarkConfig{Width: 720, Height: 576, Compression: CompressionDWAB, UseHalf: false})
}

// Read benchmarks

func BenchmarkRead_None_Half(b *testing.B) {
	benchmarkReadImage(b, BenchmarkConfig{Width: 720, Height: 576, Compression: CompressionNone, UseHalf: true})
}

func BenchmarkRead_None_Float(b *testing.B) {
	benchmarkReadImage(b, BenchmarkConfig{Width: 720, Height: 576, Compression: CompressionNone, UseHalf: false})
}

func BenchmarkRead_RLE_Half(b *testing.B) {
	benchmarkReadImage(b, BenchmarkConfig{Width: 720, Height: 576, Compression: CompressionRLE, UseHalf: true})
}

func BenchmarkRead_RLE_Float(b *testing.B) {
	benchmarkReadImage(b, BenchmarkConfig{Width: 720, Height: 576, Compression: CompressionRLE, UseHalf: false})
}

func BenchmarkRead_ZIPS_Half(b *testing.B) {
	benchmarkReadImage(b, BenchmarkConfig{Width: 720, Height: 576, Compression: CompressionZIPS, UseHalf: true})
}

func BenchmarkRead_ZIPS_Float(b *testing.B) {
	benchmarkReadImage(b, BenchmarkConfig{Width: 720, Height: 576, Compression: CompressionZIPS, UseHalf: false})
}

func BenchmarkRead_ZIP_Half(b *testing.B) {
	benchmarkReadImage(b, BenchmarkConfig{Width: 720, Height: 576, Compression: CompressionZIP, UseHalf: true})
}

func BenchmarkRead_ZIP_Float(b *testing.B) {
	benchmarkReadImage(b, BenchmarkConfig{Width: 720, Height: 576, Compression: CompressionZIP, UseHalf: false})
}

func BenchmarkRead_PIZ_Half(b *testing.B) {
	benchmarkReadImage(b, BenchmarkConfig{Width: 720, Height: 576, Compression: CompressionPIZ, UseHalf: true})
}

func BenchmarkRead_PIZ_Float(b *testing.B) {
	benchmarkReadImage(b, BenchmarkConfig{Width: 720, Height: 576, Compression: CompressionPIZ, UseHalf: false})
}

func BenchmarkRead_PXR24_Half(b *testing.B) {
	benchmarkReadImage(b, BenchmarkConfig{Width: 720, Height: 576, Compression: CompressionPXR24, UseHalf: true})
}

func BenchmarkRead_PXR24_Float(b *testing.B) {
	benchmarkReadImage(b, BenchmarkConfig{Width: 720, Height: 576, Compression: CompressionPXR24, UseHalf: false})
}

func BenchmarkRead_B44_Half(b *testing.B) {
	benchmarkReadImage(b, BenchmarkConfig{Width: 720, Height: 576, Compression: CompressionB44, UseHalf: true})
}

func BenchmarkRead_B44_Float(b *testing.B) {
	benchmarkReadImage(b, BenchmarkConfig{Width: 720, Height: 576, Compression: CompressionB44, UseHalf: false})
}

func BenchmarkRead_B44A_Half(b *testing.B) {
	benchmarkReadImage(b, BenchmarkConfig{Width: 720, Height: 576, Compression: CompressionB44A, UseHalf: true})
}

func BenchmarkRead_B44A_Float(b *testing.B) {
	benchmarkReadImage(b, BenchmarkConfig{Width: 720, Height: 576, Compression: CompressionB44A, UseHalf: false})
}

func BenchmarkRead_DWAA_Half(b *testing.B) {
	benchmarkReadImage(b, BenchmarkConfig{Width: 720, Height: 576, Compression: CompressionDWAA, UseHalf: true})
}

func BenchmarkRead_DWAA_Float(b *testing.B) {
	benchmarkReadImage(b, BenchmarkConfig{Width: 720, Height: 576, Compression: CompressionDWAA, UseHalf: false})
}

func BenchmarkRead_DWAB_Half(b *testing.B) {
	benchmarkReadImage(b, BenchmarkConfig{Width: 720, Height: 576, Compression: CompressionDWAB, UseHalf: true})
}

func BenchmarkRead_DWAB_Float(b *testing.B) {
	benchmarkReadImage(b, BenchmarkConfig{Width: 720, Height: 576, Compression: CompressionDWAB, UseHalf: false})
}

// BenchmarkResult holds timing results for a single benchmark configuration
type BenchmarkResult struct {
	Compression string
	PixelMode   string
	WriteTime   time.Duration
	ReadTime    time.Duration
}

// RunMetrics runs the same benchmarks as C++ exrmetrics and prints results in CSV format
func RunMetrics(width, height, passes int) []BenchmarkResult {
	compressions := []struct {
		comp Compression
		name string
	}{
		{CompressionNone, "none"},
		{CompressionRLE, "rle"},
		{CompressionZIPS, "zips"},
		{CompressionZIP, "zip"},
		{CompressionPIZ, "piz"},
		{CompressionPXR24, "pxr24"},
		{CompressionB44, "b44"},
		{CompressionB44A, "b44a"},
		{CompressionDWAA, "dwaa"},
		{CompressionDWAB, "dwab"},
	}

	pixelModes := []struct {
		useHalf bool
		name    string
	}{
		{true, "half"},
		{false, "float"},
	}

	var results []BenchmarkResult

	for _, comp := range compressions {
		for _, pm := range pixelModes {
			cfg := BenchmarkConfig{
				Width:       width,
				Height:      height,
				Compression: comp.comp,
				UseHalf:     pm.useHalf,
			}

			// Measure write time
			var totalWriteTime time.Duration
			for p := 0; p < passes; p++ {
				start := time.Now()
				runWriteBenchmark(cfg)
				totalWriteTime += time.Since(start)
			}
			avgWriteTime := totalWriteTime / time.Duration(passes)

			// Measure read time
			var totalReadTime time.Duration
			data := createTestImage(cfg)
			for p := 0; p < passes; p++ {
				start := time.Now()
				runReadBenchmark(data, cfg.Height)
				totalReadTime += time.Since(start)
			}
			avgReadTime := totalReadTime / time.Duration(passes)

			results = append(results, BenchmarkResult{
				Compression: comp.name,
				PixelMode:   pm.name,
				WriteTime:   avgWriteTime,
				ReadTime:    avgReadTime,
			})
		}
	}

	return results
}

func runWriteBenchmark(cfg BenchmarkConfig) {
	h := NewScanlineHeader(cfg.Width, cfg.Height)
	h.SetCompression(cfg.Compression)

	cl := NewChannelList()
	pixelType := PixelTypeFloat
	if cfg.UseHalf {
		pixelType = PixelTypeHalf
	}
	cl.Add(Channel{Name: "R", Type: pixelType, XSampling: 1, YSampling: 1})
	cl.Add(Channel{Name: "G", Type: pixelType, XSampling: 1, YSampling: 1})
	cl.Add(Channel{Name: "B", Type: pixelType, XSampling: 1, YSampling: 1})
	cl.Add(Channel{Name: "A", Type: pixelType, XSampling: 1, YSampling: 1})
	h.SetChannels(cl)

	fb, _ := AllocateChannels(cl, h.DataWindow())
	for _, name := range []string{"R", "G", "B", "A"} {
		slice := fb.Get(name)
		for y := 0; y < cfg.Height; y++ {
			for x := 0; x < cfg.Width; x++ {
				val := float32(x+y) / float32(cfg.Width+cfg.Height)
				if cfg.UseHalf {
					slice.SetHalf(x, y, half.FromFloat32(val))
				} else {
					slice.SetFloat32(x, y, val)
				}
			}
		}
	}

	ws := newMockWriteSeeker()
	sw, _ := NewScanlineWriter(ws, h)
	sw.SetFrameBuffer(fb)
	sw.WritePixels(0, cfg.Height-1)
	sw.Close()
}

func createTestImage(cfg BenchmarkConfig) []byte {
	h := NewScanlineHeader(cfg.Width, cfg.Height)
	h.SetCompression(cfg.Compression)

	cl := NewChannelList()
	pixelType := PixelTypeFloat
	if cfg.UseHalf {
		pixelType = PixelTypeHalf
	}
	cl.Add(Channel{Name: "R", Type: pixelType, XSampling: 1, YSampling: 1})
	cl.Add(Channel{Name: "G", Type: pixelType, XSampling: 1, YSampling: 1})
	cl.Add(Channel{Name: "B", Type: pixelType, XSampling: 1, YSampling: 1})
	cl.Add(Channel{Name: "A", Type: pixelType, XSampling: 1, YSampling: 1})
	h.SetChannels(cl)

	fb, _ := AllocateChannels(cl, h.DataWindow())
	for _, name := range []string{"R", "G", "B", "A"} {
		slice := fb.Get(name)
		for y := 0; y < cfg.Height; y++ {
			for x := 0; x < cfg.Width; x++ {
				val := float32(x+y) / float32(cfg.Width+cfg.Height)
				if cfg.UseHalf {
					slice.SetHalf(x, y, half.FromFloat32(val))
				} else {
					slice.SetFloat32(x, y, val)
				}
			}
		}
	}

	ws := newMockWriteSeeker()
	sw, _ := NewScanlineWriter(ws, h)
	sw.SetFrameBuffer(fb)
	sw.WritePixels(0, cfg.Height-1)
	sw.Close()

	return ws.Bytes()
}

func runReadBenchmark(data []byte, height int) {
	reader := &readerAtWrapper{bytes.NewReader(data)}
	f, _ := OpenReader(reader, int64(len(data)))
	sr, _ := NewScanlineReader(f)
	readFB, _ := AllocateChannels(sr.Header().Channels(), sr.DataWindow())
	sr.SetFrameBuffer(readFB)
	sr.ReadPixels(0, height-1)
}

// TestRunMetricsCSV is a test that outputs results in CSV format like exrmetrics
func TestRunMetricsCSV(t *testing.T) {
	if os.Getenv("RUN_METRICS") != "1" {
		t.Skip("Set RUN_METRICS=1 to run metrics benchmark")
	}

	// Use same dimensions as Flowers.exr (720x576)
	results := RunMetrics(720, 576, 10)

	fmt.Println("compression,pixel mode,write time,read time")
	for _, r := range results {
		fmt.Printf("%s,%s,%.6f,%.6f\n",
			r.Compression, r.PixelMode,
			r.WriteTime.Seconds(), r.ReadTime.Seconds())
	}
}

// seekableBuffer is defined in deep_test.go
