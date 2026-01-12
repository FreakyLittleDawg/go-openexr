// exrmultiview combines multiple single-view OpenEXR image files into a single
// multi-view image file.
//
// Usage:
//
//	exrmultiview [options] -o outfile viewname1=infile1 viewname2=infile2 ...
//
// Options:
//
//	-o <file>        output file (required)
//	-c <type>        compression for output (none, rle, zips, zip, piz, pxr24, b44, b44a, dwaa, dwab)
//	-v               verbose output
//	-multipart       output as multi-part file (one part per view)
//	-s               strict mode: require all inputs to have identical dimensions
//	-h, --help       print this message
//	--version        print version information
//
// The first view listed becomes the default/hero view. When input files have
// different data windows, the output uses the union (bounding box) of all input
// data windows. Pixels outside an input's original data window are filled with
// zeros. Use -s flag to require matching dimensions. Channels are renamed with
// view prefixes (e.g., left.R, right.R). Default view channels can keep their
// short names.
package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/mrjoshuak/go-openexr/exr"
)

const version = "0.1.0"

func main() {
	if len(os.Args) < 2 {
		usageMessage(os.Stderr, false)
		os.Exit(1)
	}

	// Parse command line arguments
	var outFile string
	var compression exr.Compression = exr.CompressionPIZ
	var verbose bool
	var multiPart bool
	var strict bool
	var viewPairs []viewPair

	i := 1
	for i < len(os.Args) {
		arg := os.Args[i]

		switch {
		case arg == "-h" || arg == "--help":
			usageMessage(os.Stdout, true)
			os.Exit(0)

		case arg == "--version":
			fmt.Printf("exrmultiview (go-openexr) %s\n", version)
			fmt.Println("https://github.com/mrjoshuak/go-openexr")
			os.Exit(0)

		case arg == "-o":
			if i+1 >= len(os.Args) {
				fmt.Fprintln(os.Stderr, "exrmultiview: missing output file with -o option")
				os.Exit(1)
			}
			outFile = os.Args[i+1]
			i += 2

		case arg == "-c":
			if i+1 >= len(os.Args) {
				fmt.Fprintln(os.Stderr, "exrmultiview: missing compression value with -c option")
				os.Exit(1)
			}
			comp, err := parseCompression(os.Args[i+1])
			if err != nil {
				fmt.Fprintf(os.Stderr, "exrmultiview: %v\n", err)
				os.Exit(1)
			}
			compression = comp
			i += 2

		case arg == "-v":
			verbose = true
			i++

		case arg == "-multipart":
			multiPart = true
			i++

		case arg == "-s":
			strict = true
			i++

		case strings.HasPrefix(arg, "-"):
			fmt.Fprintf(os.Stderr, "exrmultiview: unknown option: %s\n", arg)
			usageMessage(os.Stderr, false)
			os.Exit(1)

		default:
			// Parse viewname=filename
			if !strings.Contains(arg, "=") {
				fmt.Fprintf(os.Stderr, "exrmultiview: invalid argument: %s (expected viewname=filename)\n", arg)
				os.Exit(1)
			}
			parts := strings.SplitN(arg, "=", 2)
			if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
				fmt.Fprintf(os.Stderr, "exrmultiview: invalid view specification: %s\n", arg)
				os.Exit(1)
			}
			viewPairs = append(viewPairs, viewPair{name: parts[0], file: parts[1]})
			i++
		}
	}

	// Validate arguments
	if outFile == "" {
		fmt.Fprintln(os.Stderr, "exrmultiview: must specify an output file with -o")
		usageMessage(os.Stderr, false)
		os.Exit(1)
	}

	if len(viewPairs) < 2 {
		fmt.Fprintln(os.Stderr, "exrmultiview: must specify at least two views")
		os.Exit(1)
	}

	// Check for duplicate view names
	viewNames := make(map[string]bool)
	for _, vp := range viewPairs {
		if viewNames[vp.name] {
			fmt.Fprintf(os.Stderr, "exrmultiview: duplicate view name: %s\n", vp.name)
			os.Exit(1)
		}
		viewNames[vp.name] = true
	}

	// Create multi-view file
	if err := makeMultiView(viewPairs, outFile, compression, verbose, multiPart, strict); err != nil {
		fmt.Fprintf(os.Stderr, "exrmultiview: %v\n", err)
		os.Exit(1)
	}
}

type viewPair struct {
	name string
	file string
}

func usageMessage(w *os.File, verbose bool) {
	fmt.Fprintf(w, "Usage: exrmultiview [options] -o outfile viewname1=infile1 viewname2=infile2 ...\n")

	if verbose {
		fmt.Fprintln(w, "")
		fmt.Fprintln(w, "Combine two or more single-view OpenEXR image files into")
		fmt.Fprintln(w, "a single multi-view image file. On the command line,")
		fmt.Fprintln(w, "each single-view input image is specified together with")
		fmt.Fprintln(w, "a corresponding view name. The first view on the command")
		fmt.Fprintln(w, "line becomes the default view. Example:")
		fmt.Fprintln(w, "")
		fmt.Fprintln(w, "  exrmultiview -o imgLR.exr left=imgL.exr right=imgR.exr")
		fmt.Fprintln(w, "")
		fmt.Fprintln(w, "Here, imgL.exr and imgR.exr become the left and right")
		fmt.Fprintln(w, "views in output file imgLR.exr. The left view becomes")
		fmt.Fprintln(w, "the default view.")
		fmt.Fprintln(w, "")
		fmt.Fprintln(w, "Options:")
		fmt.Fprintln(w, "  -o <file>     output file (required)")
		fmt.Fprintln(w, "  -c <type>     sets the data compression method")
		fmt.Fprintln(w, "                (none, rle, zips, zip, piz, pxr24, b44, b44a, dwaa, dwab;")
		fmt.Fprintln(w, "                default is piz)")
		fmt.Fprintln(w, "  -v            verbose mode")
		fmt.Fprintln(w, "  -multipart    output as multi-part file (one part per view)")
		fmt.Fprintln(w, "  -s            strict mode: require all inputs to have identical dimensions")
		fmt.Fprintln(w, "  -h, --help    print this message")
		fmt.Fprintln(w, "      --version print version information")
		fmt.Fprintln(w, "")
		fmt.Fprintln(w, "Report bugs via https://github.com/mrjoshuak/go-openexr/issues")
	}
}

func parseCompression(s string) (exr.Compression, error) {
	switch strings.ToLower(s) {
	case "none", "no":
		return exr.CompressionNone, nil
	case "rle":
		return exr.CompressionRLE, nil
	case "zips":
		return exr.CompressionZIPS, nil
	case "zip":
		return exr.CompressionZIP, nil
	case "piz":
		return exr.CompressionPIZ, nil
	case "pxr24":
		return exr.CompressionPXR24, nil
	case "b44":
		return exr.CompressionB44, nil
	case "b44a":
		return exr.CompressionB44A, nil
	case "dwaa":
		return exr.CompressionDWAA, nil
	case "dwab":
		return exr.CompressionDWAB, nil
	default:
		return 0, fmt.Errorf("unknown compression method: %s", s)
	}
}

// insertViewName inserts a view name into a channel name.
// For the default view (index 0), channels without layers keep their short names.
// For non-default views or channels with layers, the view name is inserted
// as the penultimate component (before the base channel name).
//
// Examples:
//
//	insertViewName("R", views, 0) -> "R" (default view, no layer)
//	insertViewName("R", views, 1) -> "left.R" (non-default view)
//	insertViewName("layer.R", views, 0) -> "layer.right.R" (default view with layer)
//	insertViewName("layer.R", views, 1) -> "layer.left.R" (non-default view with layer)
func insertViewName(channel string, views []string, viewIndex int) string {
	parts := strings.Split(channel, ".")

	if len(parts) == 0 {
		return ""
	}

	// For default view (index 0) with no layers, keep the short name
	if len(parts) == 1 && viewIndex == 0 {
		return channel
	}

	// Insert view name as penultimate component
	// "R" -> "viewname.R"
	// "layer.R" -> "layer.viewname.R"
	// "a.b.R" -> "a.b.viewname.R"
	var newParts []string
	for i := 0; i < len(parts)-1; i++ {
		newParts = append(newParts, parts[i])
	}
	newParts = append(newParts, views[viewIndex])
	newParts = append(newParts, parts[len(parts)-1])

	return strings.Join(newParts, ".")
}

// channelData holds pixel data for a channel
type channelData struct {
	name    string
	pixType exr.PixelType
	data    []byte
}

// viewInfo holds information about a single view's source file
type viewInfo struct {
	dataWindow exr.Box2i
	header     *exr.Header
}

// minInt32 returns the minimum of two int32 values
func minInt32(a, b int32) int32 {
	if a < b {
		return a
	}
	return b
}

// maxInt32 returns the maximum of two int32 values
func maxInt32(a, b int32) int32 {
	if a > b {
		return a
	}
	return b
}

// unionBox2i calculates the union (bounding box) of two Box2i
func unionBox2i(a, b exr.Box2i) exr.Box2i {
	return exr.Box2i{
		Min: exr.V2i{X: minInt32(a.Min.X, b.Min.X), Y: minInt32(a.Min.Y, b.Min.Y)},
		Max: exr.V2i{X: maxInt32(a.Max.X, b.Max.X), Y: maxInt32(a.Max.Y, b.Max.Y)},
	}
}

func makeMultiView(viewPairs []viewPair, outFile string, compression exr.Compression, verbose, multiPart, strict bool) error {
	// Extract view names
	views := make([]string, len(viewPairs))
	for i, vp := range viewPairs {
		views[i] = vp.name
	}

	// First pass: validate all input files and collect data windows
	viewInfos := make([]viewInfo, len(viewPairs))
	var unionDW exr.Box2i
	var baseHeader *exr.Header

	for i, vp := range viewPairs {
		f, err := exr.OpenFile(vp.file)
		if err != nil {
			return fmt.Errorf("cannot open %s: %w", vp.file, err)
		}

		if verbose {
			fmt.Printf("reading file %s for %s view\n", vp.file, vp.name)
		}

		h := f.Header(0)
		if h == nil {
			return fmt.Errorf("%s: invalid header", vp.file)
		}

		// Check if already multi-view
		if h.HasMultiView() {
			return fmt.Errorf("%s is already a multi-view image; cannot combine multiple multi-view images", vp.file)
		}

		dw := h.DataWindow()
		viewInfos[i] = viewInfo{
			dataWindow: dw,
			header:     h,
		}

		if i == 0 {
			// First file sets the initial union and base header
			unionDW = dw
			baseHeader = h
		} else {
			// Check dimensions match in strict mode
			if strict {
				firstDW := viewInfos[0].dataWindow
				if dw.Width() != firstDW.Width() || dw.Height() != firstDW.Height() {
					return fmt.Errorf("%s: dimensions (%dx%d) do not match first file (%dx%d)",
						vp.file, dw.Width(), dw.Height(), firstDW.Width(), firstDW.Height())
				}
				if dw.Min.X != firstDW.Min.X || dw.Min.Y != firstDW.Min.Y {
					return fmt.Errorf("%s: data window origin (%d,%d) does not match first file (%d,%d)",
						vp.file, dw.Min.X, dw.Min.Y, firstDW.Min.X, firstDW.Min.Y)
				}
			}
			// Calculate union of all data windows
			unionDW = unionBox2i(unionDW, dw)
		}
	}

	if verbose && !strict {
		// Report if data windows differ
		allSame := true
		firstDW := viewInfos[0].dataWindow
		for i := 1; i < len(viewInfos); i++ {
			dw := viewInfos[i].dataWindow
			if dw.Min.X != firstDW.Min.X || dw.Min.Y != firstDW.Min.Y ||
				dw.Max.X != firstDW.Max.X || dw.Max.Y != firstDW.Max.Y {
				allSame = false
				break
			}
		}
		if !allSame {
			fmt.Printf("input data windows differ; using union: (%d,%d)-(%d,%d) size %dx%d\n",
				unionDW.Min.X, unionDW.Min.Y, unionDW.Max.X, unionDW.Max.Y,
				unionDW.Width(), unionDW.Height())
		}
	}

	width := int(unionDW.Width())
	height := int(unionDW.Height())

	if multiPart {
		return makeMultiViewMultiPart(viewPairs, views, viewInfos, unionDW, outFile, compression, verbose, width, height, baseHeader)
	}
	return makeMultiViewSinglePart(viewPairs, views, viewInfos, unionDW, outFile, compression, verbose, width, height, baseHeader)
}

func makeMultiViewSinglePart(viewPairs []viewPair, views []string, viewInfos []viewInfo, unionDW exr.Box2i, outFile string, compression exr.Compression, verbose bool, width, height int, baseHeader *exr.Header) error {
	// Build output header based on first input file
	outHeader := exr.NewScanlineHeader(width, height)
	outHeader.SetCompression(compression)
	outHeader.SetLineOrder(baseHeader.LineOrder())
	outHeader.SetPixelAspectRatio(baseHeader.PixelAspectRatio())
	outHeader.SetScreenWindowCenter(baseHeader.ScreenWindowCenter())
	outHeader.SetScreenWindowWidth(baseHeader.ScreenWindowWidth())

	// Set the output data window to the union
	outHeader.SetDataWindow(unionDW)

	// Clear default channels - we'll rebuild them
	outChannelList := exr.NewChannelList()

	// Map of output channel name -> channel data (in output coordinates)
	allChannelData := make(map[string]*channelData)

	// Read each input file and collect channel data
	for viewIdx, vp := range viewPairs {
		f, err := exr.OpenFile(vp.file)
		if err != nil {
			return fmt.Errorf("cannot open %s: %w", vp.file, err)
		}

		if verbose {
			fmt.Printf("reading file %s for %s view\n", vp.file, vp.name)
		}

		h := f.Header(0)
		inChannels := h.Channels()
		inputDW := viewInfos[viewIdx].dataWindow

		// Allocate frame buffer for reading (sized for input data window)
		fb, inputBuffers := exr.AllocateChannels(inChannels, inputDW)

		// Read the pixel data
		if h.IsTiled() {
			tr, err := exr.NewTiledReader(f)
			if err != nil {
				return fmt.Errorf("%s: %w", vp.file, err)
			}
			tr.SetFrameBuffer(fb)

			numXTiles := h.NumXTiles(0)
			numYTiles := h.NumYTiles(0)

			if err := tr.ReadTiles(0, 0, numXTiles-1, numYTiles-1); err != nil {
				return fmt.Errorf("%s: %w", vp.file, err)
			}
		} else {
			sr, err := exr.NewScanlineReader(f)
			if err != nil {
				return fmt.Errorf("%s: %w", vp.file, err)
			}
			sr.SetFrameBuffer(fb)

			if err := sr.ReadPixels(int(inputDW.Min.Y), int(inputDW.Max.Y)); err != nil {
				return fmt.Errorf("%s: %w", vp.file, err)
			}
		}

		// Calculate offset of input data window within output union
		xOffset := int(inputDW.Min.X - unionDW.Min.X)
		yOffset := int(inputDW.Min.Y - unionDW.Min.Y)
		inputWidth := int(inputDW.Width())
		inputHeight := int(inputDW.Height())

		// Rename channels and copy to output-sized buffers
		for i := 0; i < inChannels.Len(); i++ {
			ch := inChannels.At(i)
			outChanName := insertViewName(ch.Name, views, viewIdx)

			// Create output channel
			outCh := exr.NewChannel(outChanName, ch.Type)
			outCh.XSampling = ch.XSampling
			outCh.YSampling = ch.YSampling
			outCh.PLinear = ch.PLinear
			outChannelList.Add(outCh)

			// Allocate output buffer (sized for union, zero-initialized)
			pixelSize := ch.Type.Size()
			outBufSize := width * height * pixelSize
			outBuf := make([]byte, outBufSize)

			// Copy input pixels to correct position in output buffer
			inputBuf := inputBuffers[ch.Name]
			copyPixelsToUnion(inputBuf, outBuf, inputWidth, inputHeight, width, height, xOffset, yOffset, pixelSize)

			// Store the output channel data
			allChannelData[outChanName] = &channelData{
				name:    outChanName,
				pixType: ch.Type,
				data:    outBuf,
			}
		}
	}

	// Sort channels by name (required for EXR format)
	outChannelList.SortByName()
	outHeader.SetChannels(outChannelList)

	// Set the multiView attribute
	outHeader.SetMultiView(views)

	// Create output file
	outF, err := os.Create(outFile)
	if err != nil {
		return err
	}
	defer outF.Close()

	sw, err := exr.NewScanlineWriter(outF, outHeader)
	if err != nil {
		return err
	}

	// Build output frame buffer
	outFB := exr.NewFrameBuffer()
	for i := 0; i < outChannelList.Len(); i++ {
		ch := outChannelList.At(i)
		cd := allChannelData[ch.Name]
		if cd != nil {
			outFB.Set(ch.Name, exr.NewSlice(cd.pixType, cd.data, width, height))
		}
	}
	sw.SetFrameBuffer(outFB)

	if verbose {
		fmt.Printf("writing file %s\n", outFile)
	}

	// Write all scanlines
	dw := outHeader.DataWindow()
	if err := sw.WritePixels(int(dw.Min.Y), int(dw.Max.Y)); err != nil {
		return err
	}

	return sw.Close()
}

// copyPixelsToUnion copies pixels from an input buffer to an output buffer,
// placing them at the correct offset within the output union.
// Both buffers are row-major order, and pixels outside the input are left as zeros.
func copyPixelsToUnion(src, dst []byte, srcWidth, srcHeight, dstWidth, dstHeight, xOffset, yOffset, pixelSize int) {
	srcRowBytes := srcWidth * pixelSize
	dstRowBytes := dstWidth * pixelSize

	for y := 0; y < srcHeight; y++ {
		dstY := y + yOffset
		if dstY < 0 || dstY >= dstHeight {
			continue
		}

		srcRowStart := y * srcRowBytes
		dstRowStart := dstY*dstRowBytes + xOffset*pixelSize

		// Copy the row
		copy(dst[dstRowStart:dstRowStart+srcRowBytes], src[srcRowStart:srcRowStart+srcRowBytes])
	}
}

func makeMultiViewMultiPart(viewPairs []viewPair, views []string, viewInfos []viewInfo, unionDW exr.Box2i, outFile string, compression exr.Compression, verbose bool, width, height int, baseHeader *exr.Header) error {
	// Create headers for each view/part
	headers := make([]*exr.Header, len(viewPairs))
	allChannelData := make([]map[string]*channelData, len(viewPairs))

	for viewIdx, vp := range viewPairs {
		f, err := exr.OpenFile(vp.file)
		if err != nil {
			return fmt.Errorf("cannot open %s: %w", vp.file, err)
		}

		if verbose {
			fmt.Printf("reading file %s for %s view\n", vp.file, vp.name)
		}

		h := f.Header(0)
		inChannels := h.Channels()
		inputDW := viewInfos[viewIdx].dataWindow

		// Create header for this part (using union dimensions)
		partHeader := exr.NewScanlineHeader(width, height)
		partHeader.SetCompression(compression)
		partHeader.SetLineOrder(baseHeader.LineOrder())
		partHeader.SetPixelAspectRatio(baseHeader.PixelAspectRatio())
		partHeader.SetScreenWindowCenter(baseHeader.ScreenWindowCenter())
		partHeader.SetScreenWindowWidth(baseHeader.ScreenWindowWidth())

		// Set the data window to the union
		partHeader.SetDataWindow(unionDW)

		// Set multi-part required attributes
		partHeader.Set(&exr.Attribute{Name: exr.AttrNameName, Type: exr.AttrTypeString, Value: vp.name})
		partHeader.Set(&exr.Attribute{Name: exr.AttrNameType, Type: exr.AttrTypeString, Value: exr.PartTypeScanline})
		partHeader.SetView(vp.name)

		// Copy channels (keep original names for multi-part)
		partChannelList := exr.NewChannelList()
		for i := 0; i < inChannels.Len(); i++ {
			ch := inChannels.At(i)
			partChannelList.Add(ch)
		}
		partChannelList.SortByName()
		partHeader.SetChannels(partChannelList)

		headers[viewIdx] = partHeader

		// Allocate frame buffer for reading (sized for input data window)
		fb, inputBuffers := exr.AllocateChannels(inChannels, inputDW)

		// Read the pixel data
		if h.IsTiled() {
			tr, err := exr.NewTiledReader(f)
			if err != nil {
				return fmt.Errorf("%s: %w", vp.file, err)
			}
			tr.SetFrameBuffer(fb)

			numXTiles := h.NumXTiles(0)
			numYTiles := h.NumYTiles(0)

			if err := tr.ReadTiles(0, 0, numXTiles-1, numYTiles-1); err != nil {
				return fmt.Errorf("%s: %w", vp.file, err)
			}
		} else {
			sr, err := exr.NewScanlineReader(f)
			if err != nil {
				return fmt.Errorf("%s: %w", vp.file, err)
			}
			sr.SetFrameBuffer(fb)

			if err := sr.ReadPixels(int(inputDW.Min.Y), int(inputDW.Max.Y)); err != nil {
				return fmt.Errorf("%s: %w", vp.file, err)
			}
		}

		// Calculate offset of input data window within output union
		xOffset := int(inputDW.Min.X - unionDW.Min.X)
		yOffset := int(inputDW.Min.Y - unionDW.Min.Y)
		inputWidth := int(inputDW.Width())
		inputHeight := int(inputDW.Height())

		// Store channel data for this view (copied to union-sized buffers)
		viewChannelData := make(map[string]*channelData)
		for i := 0; i < inChannels.Len(); i++ {
			ch := inChannels.At(i)

			// Allocate output buffer (sized for union, zero-initialized)
			pixelSize := ch.Type.Size()
			outBufSize := width * height * pixelSize
			outBuf := make([]byte, outBufSize)

			// Copy input pixels to correct position in output buffer
			inputBuf := inputBuffers[ch.Name]
			copyPixelsToUnion(inputBuf, outBuf, inputWidth, inputHeight, width, height, xOffset, yOffset, pixelSize)

			viewChannelData[ch.Name] = &channelData{
				name:    ch.Name,
				pixType: ch.Type,
				data:    outBuf,
			}
		}
		allChannelData[viewIdx] = viewChannelData
	}

	// Create output file
	outF, err := os.Create(outFile)
	if err != nil {
		return err
	}
	defer outF.Close()

	mp, err := exr.NewMultiPartOutputFile(outF, headers)
	if err != nil {
		return err
	}

	if verbose {
		fmt.Printf("writing file %s\n", outFile)
	}

	// Write each part
	for viewIdx := range viewPairs {
		h := headers[viewIdx]
		channels := h.Channels()

		// Build frame buffer for this part
		fb := exr.NewFrameBuffer()
		for i := 0; i < channels.Len(); i++ {
			ch := channels.At(i)
			cd := allChannelData[viewIdx][ch.Name]
			if cd != nil {
				fb.Set(ch.Name, exr.NewSlice(cd.pixType, cd.data, width, height))
			}
		}

		if err := mp.SetFrameBuffer(viewIdx, fb); err != nil {
			return err
		}

		if err := mp.WritePixels(viewIdx, height); err != nil {
			return err
		}
	}

	return mp.Close()
}
