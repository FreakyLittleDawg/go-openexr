// exr2aces converts OpenEXR files to ACES (Academy Color Encoding System) format.
//
// The ACES image file format is a subset of the OpenEXR file format.
// ACES image files are restricted as follows:
//   - Images are stored as scanlines; tiles are not allowed.
//   - Images contain three color channels (R, G, B) or YC format (Y, RY, BY)
//   - Images may optionally contain an alpha channel.
//   - Only three compression types are allowed:
//     NO_COMPRESSION (file is not compressed)
//     PIZ_COMPRESSION (lossless)
//     B44A_COMPRESSION (lossy)
//   - The "chromaticities" header attribute must specify the ACES RGB primaries and white point.
//
// Usage:
//
//	exr2aces [options] infile outfile
//
// Options:
//
//	-v           verbose output
//	-c <type>    compression type (none, piz, b44a) - default: piz
//	-h, -help    show usage information
//	-version     show version information
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/mrjoshuak/go-openexr/exr"
)

const version = "1.0.0"

func main() {
	// Define flags
	verbose := flag.Bool("v", false, "verbose output")
	compressionStr := flag.String("c", "", "compression type (none, piz, b44a)")
	showVersion := flag.Bool("version", false, "show version information")

	// Custom usage function
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: exr2aces [options] infile outfile\n\n")
		fmt.Fprintf(os.Stderr, "Convert an OpenEXR file to ACES format.\n\n")
		fmt.Fprintf(os.Stderr, "The ACES image file format is a subset of the OpenEXR file format.\n")
		fmt.Fprintf(os.Stderr, "ACES image files are restricted as follows:\n")
		fmt.Fprintf(os.Stderr, "  * Images are stored as scanlines; tiles are not allowed.\n")
		fmt.Fprintf(os.Stderr, "  * Images contain three color channels, either:\n")
		fmt.Fprintf(os.Stderr, "      R, G, B (red, green, blue)\n")
		fmt.Fprintf(os.Stderr, "    or:\n")
		fmt.Fprintf(os.Stderr, "      Y, RY, BY (luminance, sub-sampled chroma)\n")
		fmt.Fprintf(os.Stderr, "  * Images may optionally contain an alpha channel.\n")
		fmt.Fprintf(os.Stderr, "  * Only three compression types are allowed:\n")
		fmt.Fprintf(os.Stderr, "      none (file is not compressed)\n")
		fmt.Fprintf(os.Stderr, "      piz  (lossless)\n")
		fmt.Fprintf(os.Stderr, "      b44a (lossy)\n")
		fmt.Fprintf(os.Stderr, "  * The \"chromaticities\" header attribute must specify\n")
		fmt.Fprintf(os.Stderr, "    the ACES RGB primaries and white point.\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		flag.PrintDefaults()
	}

	flag.Parse()

	// Handle version request
	if *showVersion {
		fmt.Printf("exr2aces version %s\n", version)
		fmt.Println("Part of go-openexr - https://github.com/mrjoshuak/go-openexr")
		os.Exit(0)
	}

	// Get positional arguments
	args := flag.Args()
	if len(args) != 2 {
		flag.Usage()
		os.Exit(1)
	}

	inFile := args[0]
	outFile := args[1]

	// Validate and parse compression
	var compression exr.Compression
	compressionSet := true
	switch *compressionStr {
	case "":
		// Default: let convert() decide based on input
		compressionSet = false
	case "piz":
		compression = exr.CompressionPIZ
	case "none":
		compression = exr.CompressionNone
	case "b44a":
		compression = exr.CompressionB44A
	default:
		fmt.Fprintf(os.Stderr, "Error: invalid compression type: %s\n", *compressionStr)
		fmt.Fprintf(os.Stderr, "Valid options are: none, piz, b44a\n")
		os.Exit(1)
	}

	// Run conversion
	if err := convert(inFile, outFile, compression, compressionSet, *verbose); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func convert(inFile, outFile string, compression exr.Compression, compressionSet bool, verbose bool) error {
	// Open input file
	if verbose {
		fmt.Printf("Reading file %s\n", inFile)
	}

	f, err := os.Open(inFile)
	if err != nil {
		return fmt.Errorf("cannot open input file: %w", err)
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return fmt.Errorf("cannot stat input file: %w", err)
	}

	// Open as ACES input file (handles color conversion automatically)
	acesIn, err := exr.OpenAcesInputFile(f, info.Size())
	if err != nil {
		return fmt.Errorf("cannot read input file: %w", err)
	}

	header := acesIn.Header()
	dw := acesIn.DataWindow()

	if verbose {
		// Print input file info
		fmt.Printf("  Data window: (%d, %d) - (%d, %d)\n",
			dw.Min.X, dw.Min.Y, dw.Max.X, dw.Max.Y)
		fmt.Printf("  Display window: (%d, %d) - (%d, %d)\n",
			header.DisplayWindow().Min.X, header.DisplayWindow().Min.Y,
			header.DisplayWindow().Max.X, header.DisplayWindow().Max.Y)
		fmt.Printf("  Compression: %s\n", header.Compression().String())

		// Check if file already has ACES chromaticities
		if exr.HasACESChromaticities(header) {
			fmt.Println("  Note: input file already has ACES chromaticities")
		} else {
			fmt.Println("  Converting color space to ACES")
		}

		// Report source chromaticities
		srcChr := exr.GetChromaticities(header)
		fmt.Printf("  Source chromaticities:\n")
		fmt.Printf("    Red:   (%0.4f, %0.4f)\n", srcChr.RedX, srcChr.RedY)
		fmt.Printf("    Green: (%0.4f, %0.4f)\n", srcChr.GreenX, srcChr.GreenY)
		fmt.Printf("    Blue:  (%0.4f, %0.4f)\n", srcChr.BlueX, srcChr.BlueY)
		fmt.Printf("    White: (%0.4f, %0.4f)\n", srcChr.WhiteX, srcChr.WhiteY)
	}

	// Determine output compression based on input if not specified
	outCompression := compression
	if !compressionSet {
		// Default behavior: try to preserve compression type
		inCompression := header.Compression()
		switch inCompression {
		case exr.CompressionNone:
			outCompression = exr.CompressionNone
		case exr.CompressionB44, exr.CompressionB44A:
			outCompression = exr.CompressionB44A
		default:
			// All other compression types map to PIZ
			outCompression = exr.CompressionPIZ
		}
	}

	width := int(dw.Width())
	height := int(dw.Height())

	// Check for alpha channel
	channels := header.Channels()
	hasAlpha := channels.Get("A") != nil

	// Allocate frame buffer for RGB(A) channels
	rData := make([]float32, width*height)
	gData := make([]float32, width*height)
	bData := make([]float32, width*height)
	var aData []float32
	if hasAlpha {
		aData = make([]float32, width*height)
	}

	fb := exr.NewFrameBuffer()
	fb.Set("R", exr.NewSliceFromFloat32(rData, width, height))
	fb.Set("G", exr.NewSliceFromFloat32(gData, width, height))
	fb.Set("B", exr.NewSliceFromFloat32(bData, width, height))
	if hasAlpha {
		fb.Set("A", exr.NewSliceFromFloat32(aData, width, height))
	}

	// Read pixels (this also performs color conversion to ACES)
	acesIn.SetFrameBuffer(fb)
	if err := acesIn.ReadPixels(int(dw.Min.Y), int(dw.Max.Y)); err != nil {
		return fmt.Errorf("cannot read pixels: %w", err)
	}

	// Create output file
	if verbose {
		fmt.Printf("Writing file %s\n", outFile)
		fmt.Printf("  Compression: %s\n", outCompression.String())
	}

	outF, err := os.Create(outFile)
	if err != nil {
		return fmt.Errorf("cannot create output file: %w", err)
	}
	defer outF.Close()

	// Create ACES output file with appropriate options
	opts := &exr.AcesOutputOptions{
		Compression:        outCompression,
		PixelAspectRatio:   header.PixelAspectRatio(),
		ScreenWindowCenter: header.ScreenWindowCenter(),
		ScreenWindowWidth:  header.ScreenWindowWidth(),
		LineOrder:          header.LineOrder(),
		WriteAlpha:         hasAlpha,
	}

	acesOut, err := exr.NewAcesOutputFile(outF, width, height, opts)
	if err != nil {
		return fmt.Errorf("cannot create output file: %w", err)
	}

	// Write pixels
	acesOut.SetFrameBuffer(fb)
	if err := acesOut.WritePixels(0, height-1); err != nil {
		return fmt.Errorf("cannot write pixels: %w", err)
	}

	if err := acesOut.Close(); err != nil {
		return fmt.Errorf("cannot close output file: %w", err)
	}

	if verbose {
		acesChr := exr.ACESChromaticities()
		fmt.Printf("  Output chromaticities (ACES):\n")
		fmt.Printf("    Red:   (%0.5f, %0.5f)\n", acesChr.RedX, acesChr.RedY)
		fmt.Printf("    Green: (%0.5f, %0.5f)\n", acesChr.GreenX, acesChr.GreenY)
		fmt.Printf("    Blue:  (%0.5f, %0.5f)\n", acesChr.BlueX, acesChr.BlueY)
		fmt.Printf("    White: (%0.5f, %0.5f)\n", acesChr.WhiteX, acesChr.WhiteY)
		fmt.Println("Conversion complete.")
	}

	return nil
}
