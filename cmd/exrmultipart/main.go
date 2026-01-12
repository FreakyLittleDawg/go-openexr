// exrmultipart combines, separates, or converts multi-part OpenEXR files.
//
// Usage:
//
//	exrmultipart [options] -o outfile
//
//	Combine mode:
//	  exrmultipart -combine -o outfile infile1 infile2 ...
//	  exrmultipart -combine -o outfile file1.exr:0::diffuse file2.exr:1
//
//	Separate mode:
//	  exrmultipart -separate infile [-o outdir]
//
//	Convert mode:
//	  exrmultipart -convert -o outfile infile
//
// Options:
//
//	-combine       combine multiple files into one multi-part file
//	-separate      extract parts from multi-part file to separate files
//	-convert       convert between single/multi-part and tiled/scanline formats
//	-o <path>      output file (combine/convert) or directory (separate)
//	-part <n>      extract only part N (with -separate)
//	-tiled         output as tiled (with -convert)
//	-scanline      output as scanline (with -convert)
//	-tile-size <n> tile size for tiled output (default 64)
//	-v             verbose output
//	-h, --help     print this message
//	--version      print version information
//
// Extended input syntax (for -combine):
//
//	file.exr              use all parts from file
//	file.exr:N            use only part N from file
//	file.exr::name        rename the part to 'name'
//	file.exr:N::name      use part N and rename to 'name'
package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/mrjoshuak/go-openexr/exr"
)

const version = "0.2.0"

type options struct {
	combine  bool
	separate bool
	convert  bool
	output   string
	partNum  int
	tiled    bool
	scanline bool
	tileSize int
	verbose  bool
	inputs   []string
}

// inputSpec represents a parsed input file specification
type inputSpec struct {
	filePath string
	partNum  int    // -1 means all parts
	partName string // empty means use original name
}

func main() {
	if len(os.Args) < 2 {
		usageMessage(os.Stderr, false)
		os.Exit(1)
	}

	// Check for help or version flags before parsing
	for _, arg := range os.Args[1:] {
		if arg == "-h" || arg == "--help" {
			usageMessage(os.Stdout, true)
			os.Exit(0)
		}
		if arg == "--version" {
			fmt.Printf("exrmultipart (go-openexr) %s\n", version)
			fmt.Println("https://github.com/mrjoshuak/go-openexr")
			os.Exit(0)
		}
	}

	opts, err := parseArgs(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "exrmultipart: %v\n", err)
		usageMessage(os.Stderr, false)
		os.Exit(1)
	}

	if err := run(opts); err != nil {
		fmt.Fprintf(os.Stderr, "exrmultipart: %v\n", err)
		os.Exit(1)
	}
}

func parseArgs(args []string) (*options, error) {
	opts := &options{
		partNum:  -1, // -1 means all parts
		tileSize: 64, // default tile size
	}

	// Custom flag parsing to handle positional args after flags
	i := 0
	for i < len(args) {
		arg := args[i]

		switch arg {
		case "-combine":
			opts.combine = true
			i++
		case "-separate":
			opts.separate = true
			i++
		case "-convert":
			opts.convert = true
			i++
		case "-tiled":
			opts.tiled = true
			i++
		case "-scanline":
			opts.scanline = true
			i++
		case "-tile-size":
			if i+1 >= len(args) {
				return nil, fmt.Errorf("-tile-size requires an argument")
			}
			n, err := strconv.Atoi(args[i+1])
			if err != nil || n < 1 {
				return nil, fmt.Errorf("invalid tile size: %s", args[i+1])
			}
			opts.tileSize = n
			i += 2
		case "-o":
			if i+1 >= len(args) {
				return nil, fmt.Errorf("-o requires an argument")
			}
			opts.output = args[i+1]
			i += 2
		case "-part":
			if i+1 >= len(args) {
				return nil, fmt.Errorf("-part requires an argument")
			}
			n, err := strconv.Atoi(args[i+1])
			if err != nil {
				return nil, fmt.Errorf("invalid part number: %s", args[i+1])
			}
			if n < 0 {
				return nil, fmt.Errorf("part number must be non-negative")
			}
			opts.partNum = n
			i += 2
		case "-v":
			opts.verbose = true
			i++
		default:
			if strings.HasPrefix(arg, "-") {
				return nil, fmt.Errorf("unknown option: %s", arg)
			}
			// Input file
			opts.inputs = append(opts.inputs, arg)
			i++
		}
	}

	// Count modes
	modeCount := 0
	if opts.combine {
		modeCount++
	}
	if opts.separate {
		modeCount++
	}
	if opts.convert {
		modeCount++
	}

	// Validate options
	if modeCount > 1 {
		return nil, fmt.Errorf("cannot use multiple modes (-combine, -separate, -convert)")
	}

	if modeCount == 0 {
		return nil, fmt.Errorf("must specify -combine, -separate, or -convert")
	}

	if len(opts.inputs) == 0 {
		return nil, fmt.Errorf("no input files specified")
	}

	if (opts.combine || opts.convert) && opts.output == "" {
		return nil, fmt.Errorf("-combine and -convert require -o <output file>")
	}

	if opts.separate && len(opts.inputs) > 1 {
		return nil, fmt.Errorf("-separate takes only one input file")
	}

	if opts.convert && len(opts.inputs) > 1 {
		return nil, fmt.Errorf("-convert takes only one input file")
	}

	if (opts.combine || opts.separate) && opts.partNum >= 0 && !opts.separate {
		return nil, fmt.Errorf("-part can only be used with -separate")
	}

	if opts.tiled && opts.scanline {
		return nil, fmt.Errorf("cannot use both -tiled and -scanline")
	}

	if (opts.tiled || opts.scanline) && !opts.convert {
		return nil, fmt.Errorf("-tiled and -scanline can only be used with -convert")
	}

	return opts, nil
}

// parseInputSpec parses the extended input syntax: file:partnum::partname
func parseInputSpec(input string) (*inputSpec, error) {
	spec := &inputSpec{
		partNum: -1, // all parts
	}

	// Check for :: (part name separator)
	nameIdx := strings.Index(input, "::")
	if nameIdx >= 0 {
		spec.partName = input[nameIdx+2:]
		input = input[:nameIdx]
	}

	// Check for : (part number separator) - but not at start (Windows drive letter)
	colonIdx := -1
	for i := len(input) - 1; i > 0; i-- {
		if input[i] == ':' {
			colonIdx = i
			break
		}
	}

	if colonIdx > 0 {
		partStr := input[colonIdx+1:]
		if partStr != "" {
			n, err := strconv.Atoi(partStr)
			if err != nil {
				return nil, fmt.Errorf("invalid part number in '%s': %w", input, err)
			}
			if n < 0 {
				return nil, fmt.Errorf("part number must be non-negative")
			}
			spec.partNum = n
		}
		input = input[:colonIdx]
	}

	spec.filePath = input
	return spec, nil
}

func run(opts *options) error {
	if opts.combine {
		return combine(opts)
	}
	if opts.convert {
		return convert(opts)
	}
	return separate(opts)
}

// combine merges multiple single-part EXR files into one multi-part file.
func combine(opts *options) error {
	if opts.verbose {
		fmt.Println("Combining files into multi-part EXR:")
	}

	// Parse input specifications
	var specs []*inputSpec
	for _, input := range opts.inputs {
		spec, err := parseInputSpec(input)
		if err != nil {
			return err
		}
		specs = append(specs, spec)
	}

	// Validate input/output
	for _, spec := range specs {
		absInput, _ := filepath.Abs(spec.filePath)
		absOutput, _ := filepath.Abs(opts.output)
		if absInput == absOutput {
			return fmt.Errorf("input and output file names cannot be the same: %s", spec.filePath)
		}
		if _, err := os.Stat(spec.filePath); os.IsNotExist(err) {
			return fmt.Errorf("input file not found: %s", spec.filePath)
		}
	}

	// Collect header information from input files
	type inputPart struct {
		filePath  string
		partIndex int
		header    *exr.Header
		partName  string
	}

	var parts []inputPart
	partNames := make(map[string]bool)

	for _, spec := range specs {
		f, err := os.Open(spec.filePath)
		if err != nil {
			return fmt.Errorf("failed to open %s: %w", spec.filePath, err)
		}

		stat, err := f.Stat()
		if err != nil {
			f.Close()
			return fmt.Errorf("failed to stat %s: %w", spec.filePath, err)
		}

		exrFile, err := exr.OpenReader(f, stat.Size())
		if err != nil {
			f.Close()
			return fmt.Errorf("failed to read %s: %w", spec.filePath, err)
		}

		// Determine which parts to include
		startPart := 0
		endPart := exrFile.NumParts()
		if spec.partNum >= 0 {
			if spec.partNum >= exrFile.NumParts() {
				f.Close()
				return fmt.Errorf("part %d does not exist in %s (file has %d parts)",
					spec.partNum, spec.filePath, exrFile.NumParts())
			}
			startPart = spec.partNum
			endPart = spec.partNum + 1
		}

		// Collect specified parts from this file
		for p := startPart; p < endPart; p++ {
			h := exrFile.Header(p)
			if h == nil {
				continue
			}

			// Clone the header
			newHeader := cloneHeader(h)

			// Determine part name
			partName := spec.partName
			if partName == "" {
				partName = getPartName(newHeader)
			}
			if partName == "" {
				// Derive name from filename
				partName = derivePartName(spec.filePath)
				if exrFile.NumParts() > 1 || (startPart != endPart-1) {
					partName = fmt.Sprintf("%s_%d", partName, p)
				}
			}

			// Ensure unique name
			baseName := partName
			counter := 1
			for partNames[partName] {
				partName = fmt.Sprintf("%s_%d", baseName, counter)
				counter++
			}
			partNames[partName] = true
			newHeader.Set(&exr.Attribute{Name: exr.AttrNameName, Type: exr.AttrTypeString, Value: partName})

			// Ensure type attribute is set
			if !newHeader.Has(exr.AttrNameType) {
				if newHeader.IsTiled() {
					newHeader.Set(&exr.Attribute{Name: exr.AttrNameType, Type: exr.AttrTypeString, Value: exr.PartTypeTiled})
				} else {
					newHeader.Set(&exr.Attribute{Name: exr.AttrNameType, Type: exr.AttrTypeString, Value: exr.PartTypeScanline})
				}
			}

			parts = append(parts, inputPart{
				filePath:  spec.filePath,
				partIndex: p,
				header:    newHeader,
				partName:  partName,
			})

			if opts.verbose {
				partType := getPartType(newHeader)
				fmt.Printf("  input: %s part %d -> %s (%s)\n", spec.filePath, p, partName, partType)
			}
		}

		f.Close()
	}

	if len(parts) == 0 {
		return fmt.Errorf("no parts found in input files")
	}

	// Create headers slice for multi-part output
	headers := make([]*exr.Header, len(parts))
	for i, p := range parts {
		headers[i] = p.header
	}

	// Create output file
	outFile, err := os.Create(opts.output)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer outFile.Close()

	// Create multi-part output file
	mpOut, err := exr.NewMultiPartOutputFile(outFile, headers)
	if err != nil {
		return fmt.Errorf("failed to create multi-part file: %w", err)
	}

	// Copy pixel data from each input part to output
	// Re-open each file for reading to avoid seek issues
	for outPartIdx, part := range parts {
		f, err := os.Open(part.filePath)
		if err != nil {
			return fmt.Errorf("failed to reopen %s: %w", part.filePath, err)
		}

		stat, err := f.Stat()
		if err != nil {
			f.Close()
			return fmt.Errorf("failed to stat %s: %w", part.filePath, err)
		}

		exrFile, err := exr.OpenReader(f, stat.Size())
		if err != nil {
			f.Close()
			return fmt.Errorf("failed to read %s: %w", part.filePath, err)
		}

		if err := copyPartData(exrFile, part.partIndex, mpOut, outPartIdx, opts.output, opts.verbose); err != nil {
			f.Close()
			return fmt.Errorf("failed to copy part %d: %w", outPartIdx, err)
		}

		f.Close()
	}

	if err := mpOut.Close(); err != nil {
		return fmt.Errorf("failed to finalize output file: %w", err)
	}

	if opts.verbose {
		fmt.Printf("  output: %s\n", opts.output)
		fmt.Println("\nCombine Success")
	}

	return nil
}

// convert converts between single/multi-part and tiled/scanline formats.
func convert(opts *options) error {
	inputPath := opts.inputs[0]

	// Validate input exists
	if _, err := os.Stat(inputPath); os.IsNotExist(err) {
		return fmt.Errorf("input file not found: %s", inputPath)
	}

	// Check output is not same as input
	absInput, _ := filepath.Abs(inputPath)
	absOutput, _ := filepath.Abs(opts.output)
	if absInput == absOutput {
		return fmt.Errorf("input and output file names cannot be the same")
	}

	if opts.verbose {
		fmt.Printf("Converting EXR file:\n")
		fmt.Printf("  input: %s\n", inputPath)
		fmt.Printf("  output: %s\n", opts.output)
	}

	// Open input file
	f, err := os.Open(inputPath)
	if err != nil {
		return fmt.Errorf("failed to open %s: %w", inputPath, err)
	}
	defer f.Close()

	stat, err := f.Stat()
	if err != nil {
		return fmt.Errorf("failed to stat %s: %w", inputPath, err)
	}

	exrFile, err := exr.OpenReader(f, stat.Size())
	if err != nil {
		return fmt.Errorf("failed to read %s: %w", inputPath, err)
	}

	numParts := exrFile.NumParts()
	if opts.verbose {
		fmt.Printf("  parts: %d\n", numParts)
	}

	// For single-part files, use simple conversion
	if numParts == 1 {
		return convertSinglePart(exrFile, opts)
	}

	// Multi-part file handling
	// Collect all parts with their headers
	headers := make([]*exr.Header, numParts)
	for p := 0; p < numParts; p++ {
		h := exrFile.Header(p)
		if h == nil {
			return fmt.Errorf("invalid header for part %d", p)
		}
		headers[p] = cloneHeader(h)

		// Modify format if requested
		if opts.tiled && !h.IsTiled() {
			// Convert scanline to tiled
			td := exr.TileDescription{
				XSize:        uint32(opts.tileSize),
				YSize:        uint32(opts.tileSize),
				Mode:         exr.LevelModeOne,
				RoundingMode: exr.LevelRoundDown,
			}
			headers[p].SetTileDescription(td)
			headers[p].Set(&exr.Attribute{Name: exr.AttrNameType, Type: exr.AttrTypeString, Value: exr.PartTypeTiled})
			if opts.verbose {
				fmt.Printf("  part %d: converting to tiled (%dx%d)\n", p, opts.tileSize, opts.tileSize)
			}
		} else if opts.scanline && h.IsTiled() {
			// Convert tiled to scanline
			headers[p].Remove("tiles")
			headers[p].Set(&exr.Attribute{Name: exr.AttrNameType, Type: exr.AttrTypeString, Value: exr.PartTypeScanline})
			if opts.verbose {
				fmt.Printf("  part %d: converting to scanline\n", p)
			}
		}

		// Ensure name and type are set for multi-part
		if !headers[p].Has(exr.AttrNameName) {
			partName := fmt.Sprintf("part%d", p)
			headers[p].Set(&exr.Attribute{Name: exr.AttrNameName, Type: exr.AttrTypeString, Value: partName})
		}
		if !headers[p].Has(exr.AttrNameType) {
			if headers[p].IsTiled() {
				headers[p].Set(&exr.Attribute{Name: exr.AttrNameType, Type: exr.AttrTypeString, Value: exr.PartTypeTiled})
			} else {
				headers[p].Set(&exr.Attribute{Name: exr.AttrNameType, Type: exr.AttrTypeString, Value: exr.PartTypeScanline})
			}
		}
	}

	// Create output file
	outFile, err := os.Create(opts.output)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer outFile.Close()

	// Create multi-part output
	mpOut, err := exr.NewMultiPartOutputFile(outFile, headers)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}

	// Copy each part
	for p := 0; p < numParts; p++ {
		srcHeader := exrFile.Header(p)
		dstHeader := headers[p]

		partType := getPartType(srcHeader)
		isDeep := partType == exr.PartTypeDeepScanline || partType == exr.PartTypeDeepTiled

		if isDeep {
			// Deep data conversion
			if err := copyDeepPartData(exrFile, p, mpOut, p, srcHeader.IsTiled(), opts.output, opts.verbose); err != nil {
				return fmt.Errorf("failed to copy deep part %d: %w", p, err)
			}
		} else if opts.tiled && !srcHeader.IsTiled() {
			// Scanline to tiled conversion
			if err := convertScanlineToTiled(exrFile, p, mpOut, p, dstHeader, opts.verbose); err != nil {
				return fmt.Errorf("failed to convert part %d to tiled: %w", p, err)
			}
		} else if opts.scanline && srcHeader.IsTiled() {
			// Tiled to scanline conversion
			if err := convertTiledToScanline(exrFile, p, mpOut, p, dstHeader, opts.verbose); err != nil {
				return fmt.Errorf("failed to convert part %d to scanline: %w", p, err)
			}
		} else {
			// Same format, just copy
			if err := copyPartData(exrFile, p, mpOut, p, opts.output, opts.verbose); err != nil {
				return fmt.Errorf("failed to copy part %d: %w", p, err)
			}
		}
	}

	if err := mpOut.Close(); err != nil {
		return fmt.Errorf("failed to finalize output file: %w", err)
	}

	if opts.verbose {
		fmt.Println("\nConvert Success")
	}

	return nil
}

// convertSinglePart handles conversion for single-part files.
func convertSinglePart(inFile *exr.File, opts *options) error {
	h := inFile.Header(0)
	if h == nil {
		return fmt.Errorf("invalid header")
	}

	partType := getPartType(h)
	isDeep := partType == exr.PartTypeDeepScanline || partType == exr.PartTypeDeepTiled
	srcTiled := h.IsTiled()

	// Clone header and modify as needed
	outHeader := cloneHeader(h)
	outHeader.Remove(exr.AttrNameName) // Not needed for single-part
	outHeader.Remove(exr.AttrNameType) // Not needed for single-part

	wantTiled := opts.tiled || (srcTiled && !opts.scanline)

	if opts.tiled && !srcTiled {
		// Add tile description
		td := exr.TileDescription{
			XSize:        uint32(opts.tileSize),
			YSize:        uint32(opts.tileSize),
			Mode:         exr.LevelModeOne,
			RoundingMode: exr.LevelRoundDown,
		}
		outHeader.SetTileDescription(td)
		if opts.verbose {
			fmt.Printf("  converting to tiled (%dx%d)\n", opts.tileSize, opts.tileSize)
		}
	} else if opts.scanline && srcTiled {
		// Remove tile description
		outHeader.Remove("tiles")
		if opts.verbose {
			fmt.Printf("  converting to scanline\n")
		}
	}

	// Handle deep data separately
	if isDeep {
		return writeDeepPart(inFile, 0, opts.output, srcTiled)
	}

	dw := h.DataWindow()
	cl := h.Channels()
	if cl == nil {
		return fmt.Errorf("no channels")
	}

	// Read source data
	fb, _ := exr.AllocateChannels(cl, dw)

	if srcTiled {
		reader, err := exr.NewTiledReaderPart(inFile, 0)
		if err != nil {
			return fmt.Errorf("failed to create tiled reader: %w", err)
		}
		reader.SetFrameBuffer(fb)

		td := h.TileDescription()
		numTilesX := h.NumXTiles(0)
		numTilesY := h.NumYTiles(0)

		for ty := 0; ty < numTilesY; ty++ {
			for tx := 0; tx < numTilesX; tx++ {
				if err := reader.ReadTile(tx, ty); err != nil {
					return fmt.Errorf("failed to read tile (%d,%d): %w", tx, ty, err)
				}
			}
		}
		_ = td // used for reference
	} else {
		reader, err := exr.NewScanlineReaderPart(inFile, 0)
		if err != nil {
			return fmt.Errorf("failed to create scanline reader: %w", err)
		}
		reader.SetFrameBuffer(fb)

		if err := reader.ReadPixels(int(dw.Min.Y), int(dw.Max.Y)); err != nil {
			return fmt.Errorf("failed to read pixels: %w", err)
		}
	}

	// Create output file
	outFile, err := os.Create(opts.output)
	if err != nil {
		return fmt.Errorf("failed to create output: %w", err)
	}
	defer outFile.Close()

	// Write in target format
	if wantTiled {
		td := outHeader.TileDescription()
		if td == nil {
			return fmt.Errorf("no tile description for tiled output")
		}

		writer, err := exr.NewTiledWriter(outFile, outHeader)
		if err != nil {
			return fmt.Errorf("failed to create tiled writer: %w", err)
		}
		writer.SetFrameBuffer(fb)

		width := int(dw.Width())
		height := int(dw.Height())
		numTilesX := (width + int(td.XSize) - 1) / int(td.XSize)
		numTilesY := (height + int(td.YSize) - 1) / int(td.YSize)

		for ty := 0; ty < numTilesY; ty++ {
			for tx := 0; tx < numTilesX; tx++ {
				if err := writer.WriteTile(tx, ty); err != nil {
					return fmt.Errorf("failed to write tile (%d,%d): %w", tx, ty, err)
				}
			}
		}

		if err := writer.Close(); err != nil {
			return fmt.Errorf("failed to close writer: %w", err)
		}
	} else {
		writer, err := exr.NewScanlineWriter(outFile, outHeader)
		if err != nil {
			return fmt.Errorf("failed to create scanline writer: %w", err)
		}
		writer.SetFrameBuffer(fb)

		if err := writer.WritePixels(int(dw.Min.Y), int(dw.Max.Y)); err != nil {
			return fmt.Errorf("failed to write pixels: %w", err)
		}

		if err := writer.Close(); err != nil {
			return fmt.Errorf("failed to close writer: %w", err)
		}
	}

	if opts.verbose {
		fmt.Println("\nConvert Success")
	}

	return nil
}

// convertScanlineToTiled reads scanline data and writes as tiled
func convertScanlineToTiled(inFile *exr.File, inPart int, mpOut *exr.MultiPartOutputFile, outPart int, h *exr.Header, verbose bool) error {
	srcHeader := inFile.Header(inPart)
	dw := srcHeader.DataWindow()
	width := int(dw.Width())
	height := int(dw.Height())

	// Get channel list
	cl := srcHeader.Channels()
	if cl == nil {
		return fmt.Errorf("no channels in part")
	}

	// Create frame buffer and read all pixels
	fb, _ := exr.AllocateChannels(cl, dw)
	reader, err := exr.NewScanlineReaderPart(inFile, inPart)
	if err != nil {
		return fmt.Errorf("failed to create scanline reader: %w", err)
	}
	reader.SetFrameBuffer(fb)

	if err := reader.ReadPixels(int(dw.Min.Y), int(dw.Max.Y)); err != nil {
		return fmt.Errorf("failed to read pixels: %w", err)
	}

	// Set frame buffer and write as tiles
	if err := mpOut.SetFrameBuffer(outPart, fb); err != nil {
		return fmt.Errorf("failed to set frame buffer: %w", err)
	}

	td := h.TileDescription()
	if td == nil {
		return fmt.Errorf("no tile description in output header")
	}

	numTilesX := (width + int(td.XSize) - 1) / int(td.XSize)
	numTilesY := (height + int(td.YSize) - 1) / int(td.YSize)

	for ty := 0; ty < numTilesY; ty++ {
		for tx := 0; tx < numTilesX; tx++ {
			if err := mpOut.WriteTile(outPart, tx, ty); err != nil {
				return fmt.Errorf("failed to write tile (%d,%d): %w", tx, ty, err)
			}
		}
	}

	return nil
}

// convertTiledToScanline reads tiled data and writes as scanline
func convertTiledToScanline(inFile *exr.File, inPart int, mpOut *exr.MultiPartOutputFile, outPart int, h *exr.Header, verbose bool) error {
	srcHeader := inFile.Header(inPart)
	dw := srcHeader.DataWindow()
	td := srcHeader.TileDescription()
	if td == nil {
		return fmt.Errorf("no tile description in input")
	}

	// Get channel list
	cl := srcHeader.Channels()
	if cl == nil {
		return fmt.Errorf("no channels in part")
	}

	// Create frame buffer
	fb, _ := exr.AllocateChannels(cl, dw)

	// Read all tiles
	reader, err := exr.NewTiledReaderPart(inFile, inPart)
	if err != nil {
		return fmt.Errorf("failed to create tiled reader: %w", err)
	}
	reader.SetFrameBuffer(fb)

	numTilesX := srcHeader.NumXTiles(0)
	numTilesY := srcHeader.NumYTiles(0)

	for ty := 0; ty < numTilesY; ty++ {
		for tx := 0; tx < numTilesX; tx++ {
			if err := reader.ReadTile(tx, ty); err != nil {
				return fmt.Errorf("failed to read tile (%d,%d): %w", tx, ty, err)
			}
		}
	}

	// Write as scanlines
	if err := mpOut.SetFrameBuffer(outPart, fb); err != nil {
		return fmt.Errorf("failed to set frame buffer: %w", err)
	}

	numLines := int(dw.Max.Y) - int(dw.Min.Y) + 1
	if err := mpOut.WritePixels(outPart, numLines); err != nil {
		return fmt.Errorf("failed to write pixels: %w", err)
	}

	return nil
}

// separate extracts parts from a multi-part EXR file into separate files.
func separate(opts *options) error {
	inputPath := opts.inputs[0]

	// Validate input exists
	if _, err := os.Stat(inputPath); os.IsNotExist(err) {
		return fmt.Errorf("input file not found: %s", inputPath)
	}

	// Determine output directory
	outDir := opts.output
	if outDir == "" {
		outDir = "."
	}

	// Create output directory if needed
	if err := os.MkdirAll(outDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Open input file
	f, err := os.Open(inputPath)
	if err != nil {
		return fmt.Errorf("failed to open %s: %w", inputPath, err)
	}
	defer f.Close()

	stat, err := f.Stat()
	if err != nil {
		return fmt.Errorf("failed to stat %s: %w", inputPath, err)
	}

	exrFile, err := exr.OpenReader(f, stat.Size())
	if err != nil {
		return fmt.Errorf("failed to read %s: %w", inputPath, err)
	}

	numParts := exrFile.NumParts()
	if opts.verbose {
		fmt.Printf("Separating multi-part EXR:\n")
		fmt.Printf("  input: %s\n", inputPath)
		fmt.Printf("  numParts: %d\n", numParts)
	}

	// Validate part number if specified
	if opts.partNum >= 0 && opts.partNum >= numParts {
		return fmt.Errorf("part %d does not exist (file has %d parts)", opts.partNum, numParts)
	}

	// Base name for output files
	baseName := filepath.Base(inputPath)
	ext := filepath.Ext(baseName)
	baseName = strings.TrimSuffix(baseName, ext)

	// Extract each part (or just the specified one)
	startPart := 0
	endPart := numParts
	if opts.partNum >= 0 {
		startPart = opts.partNum
		endPart = opts.partNum + 1
	}

	for p := startPart; p < endPart; p++ {
		h := exrFile.Header(p)
		if h == nil {
			continue
		}

		// Determine output filename
		partName := getPartName(h)
		var outFileName string
		if partName != "" {
			outFileName = sanitizeFileName(partName) + ".exr"
		} else {
			outFileName = fmt.Sprintf("%s.%d.exr", baseName, p+1)
		}
		outPath := filepath.Join(outDir, outFileName)

		// Check that output is not same as input
		absInput, _ := filepath.Abs(inputPath)
		absOutput, _ := filepath.Abs(outPath)
		if absInput == absOutput {
			return fmt.Errorf("input and output file names cannot be the same: %s", outPath)
		}

		if opts.verbose {
			partType := getPartType(h)
			fmt.Printf("  output: %s (%s)\n", outPath, partType)
		}

		// Write this part to a separate file
		if err := writeSinglePart(exrFile, p, outPath); err != nil {
			return fmt.Errorf("failed to write part %d: %w", p, err)
		}
	}

	if opts.verbose {
		fmt.Println("\nSeparate Success")
	}

	return nil
}

// copyPartData copies pixel data from input part to output part.
func copyPartData(inFile *exr.File, inPart int, mpOut *exr.MultiPartOutputFile, outPart int, outPath string, verbose bool) error {
	h := inFile.Header(inPart)
	if h == nil {
		return fmt.Errorf("invalid input part")
	}

	partType := getPartType(h)
	isDeep := partType == exr.PartTypeDeepScanline || partType == exr.PartTypeDeepTiled
	isTiled := h.IsTiled()

	// For deep data, we need special handling
	if isDeep {
		return copyDeepPartData(inFile, inPart, mpOut, outPart, isTiled, outPath, verbose)
	}

	if isTiled {
		return copyTiledPartData(inFile, inPart, mpOut, outPart, verbose)
	}

	return copyScanlinePartData(inFile, inPart, mpOut, outPart, verbose)
}

// copyScanlinePartData copies scanline data by reading and re-encoding pixels.
func copyScanlinePartData(inFile *exr.File, inPart int, mpOut *exr.MultiPartOutputFile, outPart int, verbose bool) error {
	h := inFile.Header(inPart)
	dw := h.DataWindow()

	// Create a frame buffer with all channels
	cl := h.Channels()
	if cl == nil {
		return fmt.Errorf("no channels in part")
	}

	fb, _ := exr.AllocateChannels(cl, dw)

	// Create scanline reader
	reader, err := exr.NewScanlineReaderPart(inFile, inPart)
	if err != nil {
		return fmt.Errorf("failed to create scanline reader: %w", err)
	}
	reader.SetFrameBuffer(fb)

	// Read all pixels
	if err := reader.ReadPixels(int(dw.Min.Y), int(dw.Max.Y)); err != nil {
		return fmt.Errorf("failed to read pixels: %w", err)
	}

	// Set frame buffer on output and write
	if err := mpOut.SetFrameBuffer(outPart, fb); err != nil {
		return fmt.Errorf("failed to set frame buffer: %w", err)
	}

	numLines := int(dw.Max.Y) - int(dw.Min.Y) + 1
	if err := mpOut.WritePixels(outPart, numLines); err != nil {
		return fmt.Errorf("failed to write pixels: %w", err)
	}

	return nil
}

// copyTiledPartData copies tiled data.
func copyTiledPartData(inFile *exr.File, inPart int, mpOut *exr.MultiPartOutputFile, outPart int, verbose bool) error {
	h := inFile.Header(inPart)
	dw := h.DataWindow()
	td := h.TileDescription()
	if td == nil {
		return fmt.Errorf("no tile description")
	}

	// Create frame buffer for the full image
	cl := h.Channels()
	if cl == nil {
		return fmt.Errorf("no channels in part")
	}

	fb, _ := exr.AllocateChannels(cl, dw)

	// Create tiled reader
	reader, err := exr.NewTiledReaderPart(inFile, inPart)
	if err != nil {
		return fmt.Errorf("failed to create tiled reader: %w", err)
	}
	reader.SetFrameBuffer(fb)

	// Handle different level modes
	numXLevels := reader.NumXLevels()
	numYLevels := reader.NumYLevels()

	// For now, handle LevelModeOne (single level)
	for ly := 0; ly < numYLevels; ly++ {
		for lx := 0; lx < numXLevels; lx++ {
			// For mipmap mode, skip if lx != ly
			if td.Mode == exr.LevelModeMipmap && lx != ly {
				continue
			}

			numTilesX := h.NumXTiles(lx)
			numTilesY := h.NumYTiles(ly)

			// Read all tiles at this level
			for ty := 0; ty < numTilesY; ty++ {
				for tx := 0; tx < numTilesX; tx++ {
					if err := reader.ReadTileLevel(tx, ty, lx, ly); err != nil {
						return fmt.Errorf("failed to read tile (%d,%d) level (%d,%d): %w", tx, ty, lx, ly, err)
					}
				}
			}
		}
	}

	// Set frame buffer on output
	if err := mpOut.SetFrameBuffer(outPart, fb); err != nil {
		return fmt.Errorf("failed to set frame buffer: %w", err)
	}

	// Write all tiles
	for ly := 0; ly < numYLevels; ly++ {
		for lx := 0; lx < numXLevels; lx++ {
			if td.Mode == exr.LevelModeMipmap && lx != ly {
				continue
			}

			numTilesX := h.NumXTiles(lx)
			numTilesY := h.NumYTiles(ly)

			for ty := 0; ty < numTilesY; ty++ {
				for tx := 0; tx < numTilesX; tx++ {
					if err := mpOut.WriteTileLevel(outPart, tx, ty, lx, ly); err != nil {
						return fmt.Errorf("failed to write tile (%d,%d) level (%d,%d): %w", tx, ty, lx, ly, err)
					}
				}
			}
		}
	}

	return nil
}

// copyDeepPartData copies deep data by reading and writing through dedicated files.
// Deep data cannot be written through MultiPartOutputFile, so we write to a temp file
// and then copy to the final destination.
func copyDeepPartData(inFile *exr.File, inPart int, mpOut *exr.MultiPartOutputFile, outPart int, isTiled bool, outPath string, verbose bool) error {
	h := inFile.Header(inPart)
	dw := h.DataWindow()
	width := int(dw.Width())
	height := int(dw.Height())

	// Create deep frame buffer
	dfb := exr.NewDeepFrameBuffer(width, height)

	// Add slices for all channels
	cl := h.Channels()
	if cl == nil {
		return fmt.Errorf("no channels in part")
	}

	for i := 0; i < cl.Len(); i++ {
		ch := cl.At(i)
		dfb.Insert(ch.Name, ch.Type)
	}

	if isTiled {
		return copyDeepTiledPartDataDirect(inFile, inPart, dfb, h, outPath, verbose)
	}

	return copyDeepScanlinePartDataDirect(inFile, inPart, dfb, h, outPath, verbose)
}

// copyDeepScanlinePartDataDirect copies deep scanline data directly to a standalone file.
func copyDeepScanlinePartDataDirect(inFile *exr.File, inPart int, dfb *exr.DeepFrameBuffer, h *exr.Header, outPath string, verbose bool) error {
	dw := h.DataWindow()
	yMin := int(dw.Min.Y)
	yMax := int(dw.Max.Y)

	// Create deep scanline reader
	reader, err := exr.NewDeepScanlineReader(inFile)
	if err != nil {
		return fmt.Errorf("failed to create deep scanline reader: %w", err)
	}
	reader.SetFrameBuffer(dfb)

	// Read sample counts first
	if err := reader.ReadPixelSampleCounts(yMin, yMax); err != nil {
		return fmt.Errorf("failed to read sample counts: %w", err)
	}

	// Read pixel data
	if err := reader.ReadPixels(yMin, yMax); err != nil {
		return fmt.Errorf("failed to read deep pixels: %w", err)
	}

	// Create output file
	outFile, err := os.Create(outPath)
	if err != nil {
		return fmt.Errorf("failed to create output: %w", err)
	}
	defer outFile.Close()

	// Create deep scanline writer
	writer, err := exr.NewDeepScanlineWriter(outFile, dfb.Width, dfb.Height)
	if err != nil {
		return fmt.Errorf("failed to create deep writer: %w", err)
	}

	// Configure header
	writer.Header().SetCompression(h.Compression())

	// Copy channels from source header
	srcCl := h.Channels()
	if srcCl != nil {
		dstCl := exr.NewChannelList()
		for i := 0; i < srcCl.Len(); i++ {
			dstCl.Add(srcCl.At(i))
		}
		writer.Header().SetChannels(dstCl)
	}

	writer.SetFrameBuffer(dfb)

	// Write pixels
	numLines := yMax - yMin + 1
	if err := writer.WritePixels(numLines); err != nil {
		return fmt.Errorf("failed to write deep pixels: %w", err)
	}

	return writer.Finalize()
}

// copyDeepTiledPartDataDirect copies deep tiled data directly to a standalone file.
func copyDeepTiledPartDataDirect(inFile *exr.File, inPart int, dfb *exr.DeepFrameBuffer, h *exr.Header, outPath string, verbose bool) error {
	td := h.TileDescription()
	if td == nil {
		return fmt.Errorf("no tile description")
	}

	// Read from input
	reader, err := exr.NewDeepTiledReaderPart(inFile, inPart)
	if err != nil {
		return fmt.Errorf("failed to create deep tiled reader: %w", err)
	}
	reader.SetFrameBuffer(dfb)

	// Read all tiles
	numTilesX := reader.NumTilesX()
	numTilesY := reader.NumTilesY()

	for ty := 0; ty < numTilesY; ty++ {
		for tx := 0; tx < numTilesX; tx++ {
			if err := reader.ReadTile(tx, ty); err != nil {
				return fmt.Errorf("failed to read deep tile: %w", err)
			}
		}
	}

	// Create output file
	outFile, err := os.Create(outPath)
	if err != nil {
		return fmt.Errorf("failed to create output: %w", err)
	}
	defer outFile.Close()

	// Create deep tiled writer
	writer, err := exr.NewDeepTiledWriter(outFile, dfb.Width, dfb.Height, td.XSize, td.YSize)
	if err != nil {
		return fmt.Errorf("failed to create deep tiled writer: %w", err)
	}

	writer.Header().SetCompression(h.Compression())

	// Copy channels from source header
	srcCl := h.Channels()
	if srcCl != nil {
		dstCl := exr.NewChannelList()
		for i := 0; i < srcCl.Len(); i++ {
			dstCl.Add(srcCl.At(i))
		}
		writer.Header().SetChannels(dstCl)
	}

	writer.SetFrameBuffer(dfb)

	// Write all tiles
	for ty := 0; ty < numTilesY; ty++ {
		for tx := 0; tx < numTilesX; tx++ {
			if err := writer.WriteTile(tx, ty); err != nil {
				return fmt.Errorf("failed to write deep tile: %w", err)
			}
		}
	}

	return writer.Finalize()
}

// writeSinglePart writes a single part to a new file.
func writeSinglePart(inFile *exr.File, partIndex int, outPath string) error {
	h := inFile.Header(partIndex)
	if h == nil {
		return fmt.Errorf("invalid part index")
	}

	partType := getPartType(h)
	isDeep := partType == exr.PartTypeDeepScanline || partType == exr.PartTypeDeepTiled
	isTiled := h.IsTiled()

	// Deep data
	if isDeep {
		return writeDeepPart(inFile, partIndex, outPath, isTiled)
	}

	// Create output file
	outFile, err := os.Create(outPath)
	if err != nil {
		return fmt.Errorf("failed to create output: %w", err)
	}
	defer outFile.Close()

	// Clone header for output (remove multi-part specific attributes)
	outHeader := cloneHeader(h)
	outHeader.Remove(exr.AttrNameName) // Name not needed for single-part
	outHeader.Remove(exr.AttrNameType) // Type not needed for single-part

	if isTiled {
		return writeTiledPart(inFile, partIndex, outFile, outHeader)
	}

	return writeScanlinePart(inFile, partIndex, outFile, outHeader)
}

// writeScanlinePart writes a scanline part to a file.
func writeScanlinePart(inFile *exr.File, partIndex int, outFile *os.File, h *exr.Header) error {
	dw := h.DataWindow()

	// Create frame buffer
	cl := h.Channels()
	if cl == nil {
		return fmt.Errorf("no channels")
	}

	fb, _ := exr.AllocateChannels(cl, dw)

	// Read pixels from input
	reader, err := exr.NewScanlineReaderPart(inFile, partIndex)
	if err != nil {
		return fmt.Errorf("failed to create reader: %w", err)
	}
	reader.SetFrameBuffer(fb)

	if err := reader.ReadPixels(int(dw.Min.Y), int(dw.Max.Y)); err != nil {
		return fmt.Errorf("failed to read pixels: %w", err)
	}

	// Write to output
	writer, err := exr.NewScanlineWriter(outFile, h)
	if err != nil {
		return fmt.Errorf("failed to create writer: %w", err)
	}
	writer.SetFrameBuffer(fb)

	if err := writer.WritePixels(int(dw.Min.Y), int(dw.Max.Y)); err != nil {
		return fmt.Errorf("failed to write pixels: %w", err)
	}

	return writer.Close()
}

// writeTiledPart writes a tiled part to a file.
func writeTiledPart(inFile *exr.File, partIndex int, outFile *os.File, h *exr.Header) error {
	dw := h.DataWindow()
	td := h.TileDescription()
	if td == nil {
		return fmt.Errorf("no tile description")
	}

	// Create frame buffer
	cl := h.Channels()
	if cl == nil {
		return fmt.Errorf("no channels")
	}

	fb, _ := exr.AllocateChannels(cl, dw)

	// Read all tiles from input
	reader, err := exr.NewTiledReaderPart(inFile, partIndex)
	if err != nil {
		return fmt.Errorf("failed to create reader: %w", err)
	}
	reader.SetFrameBuffer(fb)

	// Handle levels
	numXLevels := reader.NumXLevels()
	numYLevels := reader.NumYLevels()

	for ly := 0; ly < numYLevels; ly++ {
		for lx := 0; lx < numXLevels; lx++ {
			if td.Mode == exr.LevelModeMipmap && lx != ly {
				continue
			}

			numTilesX := h.NumXTiles(lx)
			numTilesY := h.NumYTiles(ly)

			for ty := 0; ty < numTilesY; ty++ {
				for tx := 0; tx < numTilesX; tx++ {
					if err := reader.ReadTileLevel(tx, ty, lx, ly); err != nil {
						return fmt.Errorf("failed to read tile: %w", err)
					}
				}
			}
		}
	}

	// Write to output
	writer, err := exr.NewTiledWriter(outFile, h)
	if err != nil {
		return fmt.Errorf("failed to create writer: %w", err)
	}
	writer.SetFrameBuffer(fb)

	for ly := 0; ly < numYLevels; ly++ {
		for lx := 0; lx < numXLevels; lx++ {
			if td.Mode == exr.LevelModeMipmap && lx != ly {
				continue
			}

			numTilesX := h.NumXTiles(lx)
			numTilesY := h.NumYTiles(ly)

			for ty := 0; ty < numTilesY; ty++ {
				for tx := 0; tx < numTilesX; tx++ {
					if err := writer.WriteTileLevel(tx, ty, lx, ly); err != nil {
						return fmt.Errorf("failed to write tile: %w", err)
					}
				}
			}
		}
	}

	return writer.Close()
}

// writeDeepPart writes a deep part to a file.
func writeDeepPart(inFile *exr.File, partIndex int, outPath string, isTiled bool) error {
	h := inFile.Header(partIndex)
	dw := h.DataWindow()
	width := int(dw.Width())
	height := int(dw.Height())

	// Create deep frame buffer
	dfb := exr.NewDeepFrameBuffer(width, height)
	cl := h.Channels()
	if cl == nil {
		return fmt.Errorf("no channels")
	}

	for i := 0; i < cl.Len(); i++ {
		ch := cl.At(i)
		dfb.Insert(ch.Name, ch.Type)
	}

	if isTiled {
		return writeDeepTiledPart(inFile, partIndex, outPath, dfb, h)
	}

	return writeDeepScanlinePart(inFile, partIndex, outPath, dfb, h)
}

// writeDeepScanlinePart writes a deep scanline part.
func writeDeepScanlinePart(inFile *exr.File, partIndex int, outPath string, dfb *exr.DeepFrameBuffer, h *exr.Header) error {
	dw := h.DataWindow()
	yMin := int(dw.Min.Y)
	yMax := int(dw.Max.Y)

	// Read from input
	reader, err := exr.NewDeepScanlineReader(inFile)
	if err != nil {
		return fmt.Errorf("failed to create deep reader: %w", err)
	}
	reader.SetFrameBuffer(dfb)

	if err := reader.ReadPixelSampleCounts(yMin, yMax); err != nil {
		return fmt.Errorf("failed to read sample counts: %w", err)
	}

	if err := reader.ReadPixels(yMin, yMax); err != nil {
		return fmt.Errorf("failed to read deep pixels: %w", err)
	}

	// Create output file
	outFile, err := os.Create(outPath)
	if err != nil {
		return fmt.Errorf("failed to create output: %w", err)
	}
	defer outFile.Close()

	// Create deep scanline writer
	writer, err := exr.NewDeepScanlineWriter(outFile, dfb.Width, dfb.Height)
	if err != nil {
		return fmt.Errorf("failed to create deep writer: %w", err)
	}

	// Configure header
	writer.Header().SetCompression(h.Compression())

	writer.SetFrameBuffer(dfb)

	// Write pixels
	numLines := yMax - yMin + 1
	if err := writer.WritePixels(numLines); err != nil {
		return fmt.Errorf("failed to write deep pixels: %w", err)
	}

	return writer.Finalize()
}

// writeDeepTiledPart writes a deep tiled part.
func writeDeepTiledPart(inFile *exr.File, partIndex int, outPath string, dfb *exr.DeepFrameBuffer, h *exr.Header) error {
	td := h.TileDescription()
	if td == nil {
		return fmt.Errorf("no tile description")
	}

	// Read from input
	reader, err := exr.NewDeepTiledReaderPart(inFile, partIndex)
	if err != nil {
		return fmt.Errorf("failed to create deep tiled reader: %w", err)
	}
	reader.SetFrameBuffer(dfb)

	// Read all tiles
	numTilesX := reader.NumTilesX()
	numTilesY := reader.NumTilesY()

	for ty := 0; ty < numTilesY; ty++ {
		for tx := 0; tx < numTilesX; tx++ {
			if err := reader.ReadTile(tx, ty); err != nil {
				return fmt.Errorf("failed to read deep tile: %w", err)
			}
		}
	}

	// Create output file
	outFile, err := os.Create(outPath)
	if err != nil {
		return fmt.Errorf("failed to create output: %w", err)
	}
	defer outFile.Close()

	// Create deep tiled writer
	writer, err := exr.NewDeepTiledWriter(outFile, dfb.Width, dfb.Height, td.XSize, td.YSize)
	if err != nil {
		return fmt.Errorf("failed to create deep tiled writer: %w", err)
	}

	writer.Header().SetCompression(h.Compression())
	writer.SetFrameBuffer(dfb)

	// Write all tiles
	for ty := 0; ty < numTilesY; ty++ {
		for tx := 0; tx < numTilesX; tx++ {
			if err := writer.WriteTile(tx, ty); err != nil {
				return fmt.Errorf("failed to write deep tile: %w", err)
			}
		}
	}

	return writer.Finalize()
}

// cloneHeader creates a copy of a header with all attributes.
func cloneHeader(h *exr.Header) *exr.Header {
	newH := exr.NewHeader()
	for _, attr := range h.Attributes() {
		// Clone the attribute
		newAttr := &exr.Attribute{
			Name:  attr.Name,
			Type:  attr.Type,
			Value: attr.Value,
		}
		newH.Set(newAttr)
	}
	return newH
}

// getPartName returns the name attribute of a header, or empty string if not set.
func getPartName(h *exr.Header) string {
	attr := h.Get(exr.AttrNameName)
	if attr == nil {
		return ""
	}
	if name, ok := attr.Value.(string); ok {
		return name
	}
	return ""
}

// getPartType returns the type of the part.
func getPartType(h *exr.Header) string {
	attr := h.Get(exr.AttrNameType)
	if attr != nil {
		if t, ok := attr.Value.(string); ok {
			return t
		}
	}
	// Infer from header
	if h.IsTiled() {
		return exr.PartTypeTiled
	}
	return exr.PartTypeScanline
}

// derivePartName derives a part name from a filename.
func derivePartName(filename string) string {
	base := filepath.Base(filename)
	// Remove extension
	ext := filepath.Ext(base)
	name := strings.TrimSuffix(base, ext)

	// Remove frame number if present (e.g., "image.0001" -> "image")
	parts := strings.Split(name, ".")
	if len(parts) > 1 {
		lastPart := parts[len(parts)-1]
		if isNumber(lastPart) {
			name = strings.Join(parts[:len(parts)-1], ".")
		}
	}

	return name
}

// isNumber returns true if the string is a number.
func isNumber(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// sanitizeFileName makes a string safe for use as a filename.
func sanitizeFileName(name string) string {
	// Replace path separators and other problematic characters
	replacer := strings.NewReplacer(
		"/", "_",
		"\\", "_",
		":", "_",
		"*", "_",
		"?", "_",
		"\"", "_",
		"<", "_",
		">", "_",
		"|", "_",
	)
	return replacer.Replace(name)
}

func usageMessage(w io.Writer, verbose bool) {
	fmt.Fprintf(w, "Usage: exrmultipart [options] -o outfile\n\n")
	fmt.Fprintf(w, "Combine mode:\n")
	fmt.Fprintf(w, "  exrmultipart -combine -o outfile infile1 infile2 ...\n")
	fmt.Fprintf(w, "  exrmultipart -combine -o outfile file1.exr:0::diffuse file2.exr:1\n\n")
	fmt.Fprintf(w, "Separate mode:\n")
	fmt.Fprintf(w, "  exrmultipart -separate infile [-o outdir]\n\n")
	fmt.Fprintf(w, "Convert mode:\n")
	fmt.Fprintf(w, "  exrmultipart -convert -o outfile infile\n\n")

	if verbose {
		fmt.Fprintln(w, "Options:")
		fmt.Fprintln(w, "  -combine       combine multiple files into one multi-part file")
		fmt.Fprintln(w, "  -separate      extract parts from multi-part file to separate files")
		fmt.Fprintln(w, "  -convert       convert between single/multi-part and tiled/scanline formats")
		fmt.Fprintln(w, "  -o <path>      output file (combine/convert) or directory (separate)")
		fmt.Fprintln(w, "  -part <n>      extract only part N (with -separate), 0-indexed")
		fmt.Fprintln(w, "  -tiled         output as tiled (with -convert)")
		fmt.Fprintln(w, "  -scanline      output as scanline (with -convert)")
		fmt.Fprintln(w, "  -tile-size <n> tile size for tiled output (default 64)")
		fmt.Fprintln(w, "  -v             verbose output")
		fmt.Fprintln(w, "  -h, --help     print this message")
		fmt.Fprintln(w, "      --version  print version information")
		fmt.Fprintln(w, "")
		fmt.Fprintln(w, "Extended input syntax (for -combine):")
		fmt.Fprintln(w, "  file.exr              use all parts from file")
		fmt.Fprintln(w, "  file.exr:N            use only part N from file")
		fmt.Fprintln(w, "  file.exr::name        rename the part to 'name'")
		fmt.Fprintln(w, "  file.exr:N::name      use part N and rename to 'name'")
		fmt.Fprintln(w, "")
		fmt.Fprintln(w, "Examples:")
		fmt.Fprintln(w, "  # Combine multiple EXR files into one multi-part file")
		fmt.Fprintln(w, "  exrmultipart -combine -o combined.exr diffuse.exr specular.exr")
		fmt.Fprintln(w, "")
		fmt.Fprintln(w, "  # Combine specific parts with custom names")
		fmt.Fprintln(w, "  exrmultipart -combine -o out.exr file1.exr:0::diffuse file2.exr:1::specular")
		fmt.Fprintln(w, "")
		fmt.Fprintln(w, "  # Extract all parts from a multi-part file")
		fmt.Fprintln(w, "  exrmultipart -separate multipart.exr -o ./output/")
		fmt.Fprintln(w, "")
		fmt.Fprintln(w, "  # Extract only part 0 from a multi-part file")
		fmt.Fprintln(w, "  exrmultipart -separate multipart.exr -part 0 -o ./output/")
		fmt.Fprintln(w, "")
		fmt.Fprintln(w, "  # Convert scanline to tiled format")
		fmt.Fprintln(w, "  exrmultipart -convert -tiled -tile-size 128 -o tiled.exr scanline.exr")
		fmt.Fprintln(w, "")
		fmt.Fprintln(w, "  # Convert tiled to scanline format")
		fmt.Fprintln(w, "  exrmultipart -convert -scanline -o scanline.exr tiled.exr")
		fmt.Fprintln(w, "")
		fmt.Fprintln(w, "Report bugs via https://github.com/mrjoshuak/go-openexr/issues")
	}
}
