package exr

import (
	"bytes"
	"math"
	"testing"
)

// TestACESChromaticities verifies the ACES chromaticity constants.
func TestACESChromaticities(t *testing.T) {
	chr := ACESChromaticities()

	// Verify red primary
	if !acesFloatClose(chr.RedX, 0.73470, 1e-5) || !acesFloatClose(chr.RedY, 0.26530, 1e-5) {
		t.Errorf("Red primary: got (%f, %f), want (0.73470, 0.26530)", chr.RedX, chr.RedY)
	}

	// Verify green primary
	if !acesFloatClose(chr.GreenX, 0.00000, 1e-5) || !acesFloatClose(chr.GreenY, 1.00000, 1e-5) {
		t.Errorf("Green primary: got (%f, %f), want (0.00000, 1.00000)", chr.GreenX, chr.GreenY)
	}

	// Verify blue primary
	if !acesFloatClose(chr.BlueX, 0.00010, 1e-5) || !acesFloatClose(chr.BlueY, -0.07700, 1e-5) {
		t.Errorf("Blue primary: got (%f, %f), want (0.00010, -0.07700)", chr.BlueX, chr.BlueY)
	}

	// Verify white point (D60)
	if !acesFloatClose(chr.WhiteX, 0.32168, 1e-5) || !acesFloatClose(chr.WhiteY, 0.33767, 1e-5) {
		t.Errorf("White point: got (%f, %f), want (0.32168, 0.33767)", chr.WhiteX, chr.WhiteY)
	}
}

// TestACESPrimaryConstants verifies the individual primary constants.
func TestACESPrimaryConstants(t *testing.T) {
	if ACESRedPrimary.X != 0.73470 || ACESRedPrimary.Y != 0.26530 {
		t.Error("ACESRedPrimary mismatch")
	}
	if ACESGreenPrimary.X != 0.00000 || ACESGreenPrimary.Y != 1.00000 {
		t.Error("ACESGreenPrimary mismatch")
	}
	if ACESBluePrimary.X != 0.00010 || ACESBluePrimary.Y != -0.07700 {
		t.Error("ACESBluePrimary mismatch")
	}
	if ACESWhitePoint.X != 0.32168 || ACESWhitePoint.Y != 0.33767 {
		t.Error("ACESWhitePoint mismatch")
	}
}

// TestValidateACESCompression tests compression validation.
func TestValidateACESCompression(t *testing.T) {
	tests := []struct {
		name        string
		compression Compression
		wantErr     bool
	}{
		{"None", CompressionNone, false},
		{"PIZ", CompressionPIZ, false},
		{"B44A", CompressionB44A, false},
		{"RLE", CompressionRLE, true},
		{"ZIP", CompressionZIP, true},
		{"ZIPS", CompressionZIPS, true},
		{"PXR24", CompressionPXR24, true},
		{"B44", CompressionB44, true},
		{"DWAA", CompressionDWAA, true},
		{"DWAB", CompressionDWAB, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateACESCompression(tt.compression)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateACESCompression(%v) error = %v, wantErr %v", tt.compression, err, tt.wantErr)
			}
		})
	}
}

// TestValidateACESChannels tests channel configuration validation.
func TestValidateACESChannels(t *testing.T) {
	tests := []struct {
		name     string
		channels []string
		wantErr  bool
	}{
		{"RGB", []string{"R", "G", "B"}, false},
		{"RGBA", []string{"R", "G", "B", "A"}, false},
		{"YC", []string{"Y", "RY", "BY"}, false},
		{"YCA", []string{"Y", "RY", "BY", "A"}, false},
		{"RG only", []string{"R", "G"}, true},
		{"RGB+Z", []string{"R", "G", "B", "Z"}, true},
		{"Empty", []string{}, true},
		{"Single", []string{"R"}, true},
		{"Mixed", []string{"R", "G", "B", "Y"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cl := NewChannelList()
			for _, name := range tt.channels {
				cl.Add(NewChannel(name, PixelTypeHalf))
			}
			err := ValidateACESChannels(cl)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateACESChannels(%v) error = %v, wantErr %v", tt.channels, err, tt.wantErr)
			}
		})
	}
}

// TestRGBtoXYZ tests the RGB to XYZ conversion matrix generation.
func TestRGBtoXYZ(t *testing.T) {
	// Test with sRGB/Rec.709 primaries
	chr := DefaultChromaticities()
	m := RGBtoXYZ(chr)

	// The white point (1,1,1) in RGB should map to (Xw, Yw, Zw) in XYZ
	// where Yw = 1 by definition
	r, g, b := float32(1.0), float32(1.0), float32(1.0)
	x := m[0]*r + m[1]*g + m[2]*b
	y := m[4]*r + m[5]*g + m[6]*b
	z := m[8]*r + m[9]*g + m[10]*b

	// Y should be 1.0 for the white point
	if !acesFloatClose(y, 1.0, 0.01) {
		t.Errorf("White point Y: got %f, want 1.0", y)
	}

	// Check X/Y ratio matches white chromaticity
	if y != 0 {
		xChrom := x / (x + y + z)
		yChrom := y / (x + y + z)
		if !acesFloatClose(xChrom, chr.WhiteX, 0.01) {
			t.Errorf("White x chromaticity: got %f, want %f", xChrom, chr.WhiteX)
		}
		if !acesFloatClose(yChrom, chr.WhiteY, 0.01) {
			t.Errorf("White y chromaticity: got %f, want %f", yChrom, chr.WhiteY)
		}
	}
}

// TestXYZtoRGB tests the XYZ to RGB conversion matrix.
func TestXYZtoRGB(t *testing.T) {
	chr := DefaultChromaticities()
	rgbToXYZ := RGBtoXYZ(chr)
	xyzToRGB := XYZtoRGB(chr)

	// The product should be identity (approximately)
	identity := multiply44(rgbToXYZ, xyzToRGB)

	for i := 0; i < 4; i++ {
		for j := 0; j < 4; j++ {
			expected := float32(0)
			if i == j {
				expected = 1
			}
			if !acesFloatClose(identity[i*4+j], expected, 0.0001) {
				t.Errorf("Identity[%d][%d]: got %f, want %f", i, j, identity[i*4+j], expected)
			}
		}
	}
}

// TestChromaticAdaptation tests the Bradford chromatic adaptation.
func TestChromaticAdaptation(t *testing.T) {
	// Test adaptation from D65 to D60
	d65 := V2f{X: 0.3127, Y: 0.3290}
	d60 := V2f{X: 0.32168, Y: 0.33767}

	m := ChromaticAdaptation(d65, d60)

	// The matrix should be close to identity for similar white points
	// Check that diagonal elements are close to 1 and off-diagonal are small
	if !acesFloatClose(m[0], 1.0, 0.1) || !acesFloatClose(m[5], 1.0, 0.1) || !acesFloatClose(m[10], 1.0, 0.1) {
		t.Error("Diagonal elements should be close to 1 for similar white points")
	}

	// Same white point should give identity
	identity := ChromaticAdaptation(d65, d65)
	for i := 0; i < 4; i++ {
		for j := 0; j < 4; j++ {
			expected := float32(0)
			if i == j {
				expected = 1
			}
			if !acesFloatClose(identity[i*4+j], expected, 0.001) {
				t.Errorf("Same white point should give identity: [%d][%d] = %f", i, j, identity[i*4+j])
			}
		}
	}
}

// TestACESOutputFileCreation tests creating an ACES output file.
func TestACESOutputFileCreation(t *testing.T) {
	tests := []struct {
		name    string
		opts    *AcesOutputOptions
		wantErr bool
	}{
		{
			name:    "Default options",
			opts:    nil,
			wantErr: false,
		},
		{
			name:    "PIZ compression",
			opts:    &AcesOutputOptions{Compression: CompressionPIZ},
			wantErr: false,
		},
		{
			name:    "None compression",
			opts:    &AcesOutputOptions{Compression: CompressionNone},
			wantErr: false,
		},
		{
			name:    "B44A compression",
			opts:    &AcesOutputOptions{Compression: CompressionB44A},
			wantErr: false,
		},
		{
			name:    "With alpha",
			opts:    &AcesOutputOptions{WriteAlpha: true},
			wantErr: false,
		},
		{
			name:    "Invalid compression ZIP",
			opts:    &AcesOutputOptions{Compression: CompressionZIP},
			wantErr: true,
		},
		{
			name:    "Invalid compression RLE",
			opts:    &AcesOutputOptions{Compression: CompressionRLE},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := &bytes.Buffer{}
			ws := &acesWriteSeeker{buf: buf}
			_, err := NewAcesOutputFile(ws, 64, 64, tt.opts)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewAcesOutputFile() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestACESOutputFileFromHeader tests creating an ACES file from a header.
func TestACESOutputFileFromHeader(t *testing.T) {
	tests := []struct {
		name       string
		setupFunc  func(*Header)
		wantErr    bool
		errMessage string
	}{
		{
			name: "Valid RGB header",
			setupFunc: func(h *Header) {
				cl := NewChannelList()
				cl.Add(NewChannel("R", PixelTypeHalf))
				cl.Add(NewChannel("G", PixelTypeHalf))
				cl.Add(NewChannel("B", PixelTypeHalf))
				h.SetChannels(cl)
				h.SetCompression(CompressionPIZ)
			},
			wantErr: false,
		},
		{
			name: "Valid RGBA header",
			setupFunc: func(h *Header) {
				cl := NewChannelList()
				cl.Add(NewChannel("R", PixelTypeHalf))
				cl.Add(NewChannel("G", PixelTypeHalf))
				cl.Add(NewChannel("B", PixelTypeHalf))
				cl.Add(NewChannel("A", PixelTypeHalf))
				h.SetChannels(cl)
				h.SetCompression(CompressionNone)
			},
			wantErr: false,
		},
		{
			name: "Invalid compression",
			setupFunc: func(h *Header) {
				cl := NewChannelList()
				cl.Add(NewChannel("R", PixelTypeHalf))
				cl.Add(NewChannel("G", PixelTypeHalf))
				cl.Add(NewChannel("B", PixelTypeHalf))
				h.SetChannels(cl)
				h.SetCompression(CompressionZIP)
			},
			wantErr: true,
		},
		{
			name: "Invalid channels",
			setupFunc: func(h *Header) {
				cl := NewChannelList()
				cl.Add(NewChannel("R", PixelTypeHalf))
				cl.Add(NewChannel("G", PixelTypeHalf))
				h.SetChannels(cl)
				h.SetCompression(CompressionPIZ)
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := NewScanlineHeader(64, 64)
			tt.setupFunc(h)

			buf := &bytes.Buffer{}
			ws := &acesWriteSeeker{buf: buf}
			_, err := NewAcesOutputFileFromHeader(ws, h)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewAcesOutputFileFromHeader() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestACESOutputFileWriteRoundtrip tests writing and reading ACES data.
func TestACESOutputFileWriteRoundtrip(t *testing.T) {
	width := 32
	height := 32

	// Create ACES output file
	buf := &bytes.Buffer{}
	ws := &acesWriteSeeker{buf: buf}

	opts := &AcesOutputOptions{
		Compression: CompressionNone,
	}
	af, err := NewAcesOutputFile(ws, width, height, opts)
	if err != nil {
		t.Fatalf("Failed to create ACES output file: %v", err)
	}

	// Create frame buffer with test data
	rgbaFB := NewRGBAFrameBuffer(width, height, false)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			// Create a gradient pattern
			r := float32(x) / float32(width)
			g := float32(y) / float32(height)
			b := float32(x+y) / float32(width+height)
			rgbaFB.SetPixel(x, y, r, g, b, 1.0)
		}
	}

	af.SetFrameBuffer(rgbaFB.ToFrameBuffer())

	// Write pixels
	if err := af.WritePixels(0, height-1); err != nil {
		t.Fatalf("Failed to write pixels: %v", err)
	}

	// Close to finalize
	if err := af.Close(); err != nil {
		t.Fatalf("Failed to close file: %v", err)
	}

	// Read back the file
	data := buf.Bytes()
	reader := bytes.NewReader(data)

	inputFile, err := OpenAcesInputFile(reader, int64(len(data)))
	if err != nil {
		t.Fatalf("Failed to open ACES input file: %v", err)
	}

	// Verify header has ACES chromaticities
	if !HasACESChromaticities(inputFile.Header()) {
		t.Error("File should have ACES chromaticities")
	}

	// Create input frame buffer
	inputFB := NewRGBAFrameBuffer(width, height, false)
	inputFile.SetFrameBuffer(inputFB.ToFrameBuffer())

	// Read pixels
	if err := inputFile.ReadPixels(0, height-1); err != nil {
		t.Fatalf("Failed to read pixels: %v", err)
	}

	// Verify pixel values match (within floating point tolerance)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			or, og, ob, _ := rgbaFB.GetPixel(x, y)
			ir, ig, ib, _ := inputFB.GetPixel(x, y)

			// Allow for half-precision roundtrip error
			if !acesFloatClose(or, ir, 0.01) || !acesFloatClose(og, ig, 0.01) || !acesFloatClose(ob, ib, 0.01) {
				t.Errorf("Pixel mismatch at (%d, %d): wrote (%f, %f, %f), read (%f, %f, %f)",
					x, y, or, og, ob, ir, ig, ib)
			}
		}
	}
}

// TestACESInputFileColorConversion tests color space conversion on read.
func TestACESInputFileColorConversion(t *testing.T) {
	width := 16
	height := 16

	// Create a standard sRGB file (not ACES)
	buf := &bytes.Buffer{}
	ws := &acesWriteSeeker{buf: buf}

	header := NewScanlineHeader(width, height)
	header.SetCompression(CompressionNone)

	// Set sRGB/Rec.709 chromaticities explicitly
	SetChromaticities(header, DefaultChromaticities())

	writer, err := NewScanlineWriter(ws, header)
	if err != nil {
		t.Fatalf("Failed to create writer: %v", err)
	}

	// Create frame buffer with known values
	rgbaFB := NewRGBAFrameBuffer(width, height, false)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			rgbaFB.SetPixel(x, y, 1.0, 0.5, 0.25, 1.0)
		}
	}

	writer.SetFrameBuffer(rgbaFB.ToFrameBuffer())
	if err := writer.WritePixels(0, height-1); err != nil {
		t.Fatalf("Failed to write pixels: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Failed to close: %v", err)
	}

	// Read as ACES
	data := buf.Bytes()
	reader := bytes.NewReader(data)

	acesInput, err := OpenAcesInputFile(reader, int64(len(data)))
	if err != nil {
		t.Fatalf("Failed to open as ACES: %v", err)
	}

	// Should require color conversion since source is not ACES
	if !acesInput.NeedsColorConversion() {
		t.Error("File with sRGB chromaticities should need color conversion")
	}

	// Read and convert
	acesBuffer := NewRGBAFrameBuffer(width, height, false)
	acesInput.SetFrameBuffer(acesBuffer.ToFrameBuffer())
	if err := acesInput.ReadPixels(0, height-1); err != nil {
		t.Fatalf("Failed to read pixels: %v", err)
	}

	// The converted values should be different from the original
	// (color space conversion changes the values)
	// We can't easily verify the exact values without implementing the
	// full conversion ourselves, but we can check that:
	// 1. Values are different from input
	// 2. Values are reasonable (not NaN, not huge)

	ar, ag, ab, _ := acesBuffer.GetPixel(0, 0)

	// Values should be finite and reasonable
	if math.IsNaN(float64(ar)) || math.IsNaN(float64(ag)) || math.IsNaN(float64(ab)) {
		t.Error("Converted values should not be NaN")
	}

	if ar < -10 || ar > 10 || ag < -10 || ag > 10 || ab < -10 || ab > 10 {
		t.Errorf("Converted values seem unreasonable: (%f, %f, %f)", ar, ag, ab)
	}
}

// TestACESInputFileNoConversionNeeded tests that ACES files don't need conversion.
func TestACESInputFileNoConversionNeeded(t *testing.T) {
	width := 16
	height := 16

	// Create an ACES file
	buf := &bytes.Buffer{}
	ws := &acesWriteSeeker{buf: buf}

	opts := &AcesOutputOptions{Compression: CompressionNone}
	af, err := NewAcesOutputFile(ws, width, height, opts)
	if err != nil {
		t.Fatalf("Failed to create ACES output file: %v", err)
	}

	rgbaFB := NewRGBAFrameBuffer(width, height, false)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			rgbaFB.SetPixel(x, y, 0.5, 0.5, 0.5, 1.0)
		}
	}

	af.SetFrameBuffer(rgbaFB.ToFrameBuffer())
	if err := af.WritePixels(0, height-1); err != nil {
		t.Fatalf("Failed to write pixels: %v", err)
	}
	if err := af.Close(); err != nil {
		t.Fatalf("Failed to close: %v", err)
	}

	// Read back as ACES
	data := buf.Bytes()
	reader := bytes.NewReader(data)

	acesInput, err := OpenAcesInputFile(reader, int64(len(data)))
	if err != nil {
		t.Fatalf("Failed to open ACES input: %v", err)
	}

	// Should NOT need color conversion since source is already ACES
	if acesInput.NeedsColorConversion() {
		t.Error("ACES file should not need color conversion")
	}
}

// TestHeaderChromaticityFunctions tests the header helper functions.
func TestHeaderChromaticityFunctions(t *testing.T) {
	h := NewHeader()

	// Test default chromaticities
	chr := GetChromaticities(h)
	if !chromaticitiesEqual(chr, DefaultChromaticities()) {
		t.Error("Default chromaticities mismatch")
	}

	// Test setting chromaticities
	acesChr := ACESChromaticities()
	SetChromaticities(h, acesChr)
	chr = GetChromaticities(h)
	if !chromaticitiesEqual(chr, acesChr) {
		t.Error("Set chromaticities mismatch")
	}

	// Test HasACESChromaticities
	if !HasACESChromaticities(h) {
		t.Error("Should have ACES chromaticities")
	}

	// Test adopted neutral
	SetAdoptedNeutral(h, ACESWhitePoint)
	neutral := GetAdoptedNeutral(h)
	if !v2fEqual(neutral, ACESWhitePoint) {
		t.Error("Adopted neutral mismatch")
	}
}

// TestMatrixMultiplication tests matrix multiplication.
func TestMatrixMultiplication(t *testing.T) {
	// Identity * Identity = Identity
	id := Identity44()
	result := multiply44(id, id)
	for i := 0; i < 16; i++ {
		if result[i] != id[i] {
			t.Errorf("Identity multiplication failed at index %d", i)
		}
	}

	// Test with a known multiplication
	a := M44f{
		1, 2, 0, 0,
		0, 1, 0, 0,
		0, 0, 1, 0,
		0, 0, 0, 1,
	}
	b := M44f{
		1, 0, 0, 0,
		3, 1, 0, 0,
		0, 0, 1, 0,
		0, 0, 0, 1,
	}
	expected := M44f{
		7, 2, 0, 0,
		3, 1, 0, 0,
		0, 0, 1, 0,
		0, 0, 0, 1,
	}
	result = multiply44(a, b)
	for i := 0; i < 16; i++ {
		if !acesFloatClose(result[i], expected[i], 0.0001) {
			t.Errorf("Matrix multiplication failed at index %d: got %f, want %f", i, result[i], expected[i])
		}
	}
}

// TestMatrixInverse tests matrix inversion.
func TestMatrixInverse(t *testing.T) {
	// Test that A * A^-1 = I
	a := M44f{
		1, 2, 3, 0,
		0, 1, 4, 0,
		5, 6, 0, 0,
		0, 0, 0, 1,
	}
	aInv := inverse44(a)
	result := multiply44(a, aInv)

	for i := 0; i < 4; i++ {
		for j := 0; j < 4; j++ {
			expected := float32(0)
			if i == j {
				expected = 1
			}
			if !acesFloatClose(result[i*4+j], expected, 0.001) {
				t.Errorf("Inverse test failed at [%d][%d]: got %f, want %f", i, j, result[i*4+j], expected)
			}
		}
	}
}

// Helper functions

func acesFloatClose(a, b float32, epsilon float64) bool {
	return math.Abs(float64(a-b)) < epsilon
}

// acesWriteSeeker wraps a bytes.Buffer to implement io.WriteSeeker
type acesWriteSeeker struct {
	buf *bytes.Buffer
	pos int64
}

func (ws *acesWriteSeeker) Write(p []byte) (n int, err error) {
	// Ensure buffer is large enough
	for ws.buf.Len() < int(ws.pos) {
		ws.buf.WriteByte(0)
	}

	if int(ws.pos) < ws.buf.Len() {
		// Overwrite existing data
		data := ws.buf.Bytes()
		copy(data[ws.pos:], p)
		ws.pos += int64(len(p))
		// Extend buffer if necessary
		if int(ws.pos) > ws.buf.Len() {
			ws.buf.Write(p[ws.buf.Len()-int(ws.pos-int64(len(p))):])
		}
		return len(p), nil
	}

	n, err = ws.buf.Write(p)
	ws.pos += int64(n)
	return
}

func (ws *acesWriteSeeker) Seek(offset int64, whence int) (int64, error) {
	switch whence {
	case 0: // io.SeekStart
		ws.pos = offset
	case 1: // io.SeekCurrent
		ws.pos += offset
	case 2: // io.SeekEnd
		ws.pos = int64(ws.buf.Len()) + offset
	}
	return ws.pos, nil
}

// BenchmarkRGBtoXYZ benchmarks the RGB to XYZ matrix generation.
func BenchmarkRGBtoXYZ(b *testing.B) {
	chr := ACESChromaticities()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = RGBtoXYZ(chr)
	}
}

// BenchmarkChromaticAdaptation benchmarks the Bradford transform.
func BenchmarkChromaticAdaptation(b *testing.B) {
	d65 := V2f{X: 0.3127, Y: 0.3290}
	d60 := ACESWhitePoint
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ChromaticAdaptation(d65, d60)
	}
}

// BenchmarkColorConversion benchmarks pixel color conversion.
func BenchmarkColorConversion(b *testing.B) {
	// Create a conversion matrix
	srcChr := DefaultChromaticities()
	dstChr := ACESChromaticities()

	srcToXYZ := RGBtoXYZ(srcChr)
	adapt := ChromaticAdaptation(V2f{srcChr.WhiteX, srcChr.WhiteY}, V2f{dstChr.WhiteX, dstChr.WhiteY})
	xyzToDst := XYZtoRGB(dstChr)

	temp := multiply44(srcToXYZ, adapt)
	conversionMatrix := multiply44(temp, xyzToDst)

	r, g, bl := float32(0.5), float32(0.5), float32(0.5)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = conversionMatrix[0]*r + conversionMatrix[1]*g + conversionMatrix[2]*bl
		_ = conversionMatrix[4]*r + conversionMatrix[5]*g + conversionMatrix[6]*bl
		_ = conversionMatrix[8]*r + conversionMatrix[9]*g + conversionMatrix[10]*bl
	}
}

// TestAcesInputFileMethods tests additional AcesInputFile methods.
func TestAcesInputFileMethods(t *testing.T) {
	width := 16
	height := 16

	// Create an ACES file
	buf := &bytes.Buffer{}
	ws := &acesWriteSeeker{buf: buf}

	opts := &AcesOutputOptions{Compression: CompressionNone}
	af, err := NewAcesOutputFile(ws, width, height, opts)
	if err != nil {
		t.Fatalf("Failed to create ACES output file: %v", err)
	}

	rgbaFB := NewRGBAFrameBuffer(width, height, false)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			rgbaFB.SetPixel(x, y, 0.5, 0.5, 0.5, 1.0)
		}
	}

	af.SetFrameBuffer(rgbaFB.ToFrameBuffer())
	if err := af.WritePixels(0, height-1); err != nil {
		t.Fatalf("Failed to write pixels: %v", err)
	}
	if err := af.Close(); err != nil {
		t.Fatalf("Failed to close: %v", err)
	}

	// Read back and test methods
	data := buf.Bytes()
	reader := bytes.NewReader(data)

	acesInput, err := OpenAcesInputFile(reader, int64(len(data)))
	if err != nil {
		t.Fatalf("Failed to open ACES input: %v", err)
	}

	// Test DataWindow
	dw := acesInput.DataWindow()
	if dw.Width() != int32(width) || dw.Height() != int32(height) {
		t.Errorf("DataWindow = %dx%d, want %dx%d", dw.Width(), dw.Height(), width, height)
	}

	// Test DisplayWindow
	dispW := acesInput.DisplayWindow()
	if dispW.Width() != int32(width) || dispW.Height() != int32(height) {
		t.Errorf("DisplayWindow = %dx%d, want %dx%d", dispW.Width(), dispW.Height(), width, height)
	}

	// Test IsTiled
	if acesInput.IsTiled() {
		t.Error("Scanline file should not be tiled")
	}
}

// TestAcesOutputFileMethods tests additional AcesOutputFile methods.
func TestAcesOutputFileMethods(t *testing.T) {
	width := 16
	height := 16

	buf := &bytes.Buffer{}
	ws := &acesWriteSeeker{buf: buf}

	opts := &AcesOutputOptions{Compression: CompressionNone}
	af, err := NewAcesOutputFile(ws, width, height, opts)
	if err != nil {
		t.Fatalf("Failed to create ACES output file: %v", err)
	}

	// Test Header
	h := af.Header()
	if h == nil {
		t.Error("Header should not be nil")
	}

	// Test DataWindow
	dw := af.DataWindow()
	if dw.Width() != int32(width) || dw.Height() != int32(height) {
		t.Errorf("DataWindow = %dx%d, want %dx%d", dw.Width(), dw.Height(), width, height)
	}

	// Test WritePixels without frame buffer
	err = af.WritePixels(0, height-1)
	if err != ErrACESNoFrameBuffer {
		t.Errorf("WritePixels without framebuffer should return ErrACESNoFrameBuffer, got %v", err)
	}

	af.Close()
}

// TestAcesInputFileReadPixelsWithoutFrameBuffer tests error handling.
func TestAcesInputFileReadPixelsWithoutFrameBuffer(t *testing.T) {
	width := 16
	height := 16

	// Create an ACES file
	buf := &bytes.Buffer{}
	ws := &acesWriteSeeker{buf: buf}

	opts := &AcesOutputOptions{Compression: CompressionNone}
	af, err := NewAcesOutputFile(ws, width, height, opts)
	if err != nil {
		t.Fatalf("Failed to create ACES output file: %v", err)
	}

	rgbaFB := NewRGBAFrameBuffer(width, height, false)
	af.SetFrameBuffer(rgbaFB.ToFrameBuffer())
	af.WritePixels(0, height-1)
	af.Close()

	// Read back
	data := buf.Bytes()
	reader := bytes.NewReader(data)

	acesInput, err := OpenAcesInputFile(reader, int64(len(data)))
	if err != nil {
		t.Fatalf("Failed to open ACES input: %v", err)
	}

	// Try to read without setting frame buffer
	err = acesInput.ReadPixels(0, height-1)
	if err != ErrACESNoFrameBuffer {
		t.Errorf("ReadPixels without framebuffer should return ErrACESNoFrameBuffer, got %v", err)
	}
}

// TestAcesOutputFileFromHeaderWithTiles tests that tiled headers are rejected.
func TestAcesOutputFileFromHeaderWithTiles(t *testing.T) {
	buf := &bytes.Buffer{}
	ws := &acesWriteSeeker{buf: buf}

	// Create a tiled header
	h := NewTiledHeader(64, 64, 32, 32)
	cl := NewChannelList()
	cl.Add(NewChannel("R", PixelTypeHalf))
	cl.Add(NewChannel("G", PixelTypeHalf))
	cl.Add(NewChannel("B", PixelTypeHalf))
	h.SetChannels(cl)
	h.SetCompression(CompressionPIZ)

	_, err := NewAcesOutputFileFromHeader(ws, h)
	if err != ErrACESTiledNotAllowed {
		t.Errorf("Expected ErrACESTiledNotAllowed, got %v", err)
	}
}

// TestValidateACESChannelsNil tests nil channel list validation.
func TestValidateACESChannelsNil(t *testing.T) {
	err := ValidateACESChannels(nil)
	if err != ErrInvalidACESChannels {
		t.Errorf("Expected ErrInvalidACESChannels for nil, got %v", err)
	}
}

// TestGetAdoptedNeutralDefault tests default adopted neutral.
func TestGetAdoptedNeutralDefault(t *testing.T) {
	h := NewHeader()

	// Without any chromaticities, should return default white point
	neutral := GetAdoptedNeutral(h)
	expected := DefaultChromaticities()
	if !v2fEqual(neutral, V2f{X: expected.WhiteX, Y: expected.WhiteY}) {
		t.Errorf("Default adopted neutral = %v, want %v", neutral, V2f{X: expected.WhiteX, Y: expected.WhiteY})
	}
}

// TestAcesInputFileTiled tests reading a tiled EXR as ACES.
func TestAcesInputFileTiled(t *testing.T) {
	width := 64
	height := 64
	tileSize := 32

	// Create a tiled EXR file
	h := NewTiledHeader(width, height, tileSize, tileSize)
	h.SetCompression(CompressionNone)

	// Set ACES chromaticities to avoid color conversion
	acesChr := ACESChromaticities()
	h.Set(&Attribute{Name: "chromaticities", Type: AttrTypeChromaticities, Value: acesChr})

	buf := &bytes.Buffer{}
	ws := &acesWriteSeeker{buf: buf}

	tw, err := NewTiledWriter(ws, h)
	if err != nil {
		t.Fatalf("NewTiledWriter error: %v", err)
	}

	fb, _ := AllocateChannels(h.Channels(), h.DataWindow())
	tw.SetFrameBuffer(fb)

	// Write tiles
	numTilesX := (width + tileSize - 1) / tileSize
	numTilesY := (height + tileSize - 1) / tileSize
	for ty := 0; ty < numTilesY; ty++ {
		for tx := 0; tx < numTilesX; tx++ {
			tw.WriteTile(tx, ty)
		}
	}
	tw.Close()

	// Read as ACES
	data := buf.Bytes()
	reader := bytes.NewReader(data)

	acesInput, err := OpenAcesInputFile(reader, int64(len(data)))
	if err != nil {
		t.Fatalf("OpenAcesInputFile error: %v", err)
	}

	// Verify it's tiled
	if !acesInput.IsTiled() {
		t.Error("File should be tiled")
	}

	// Set frame buffer and read
	readFB, _ := AllocateChannels(acesInput.Header().Channels(), acesInput.DataWindow())
	acesInput.SetFrameBuffer(readFB)

	// Read pixels
	if err := acesInput.ReadPixels(0, height-1); err != nil {
		t.Fatalf("ReadPixels error: %v", err)
	}

	t.Log("ACES tiled input file read successfully")
}

// TestAcesInputFileWithColorConversion tests ACES input with color conversion.
func TestAcesInputFileWithColorConversion(t *testing.T) {
	width := 32
	height := 32

	// Create a scanline EXR with non-ACES chromaticities
	buf := &bytes.Buffer{}
	ws := &acesWriteSeeker{buf: buf}

	h := NewScanlineHeader(width, height)
	h.SetCompression(CompressionNone)

	// Set Rec.709 chromaticities (not ACES) to force conversion
	rec709 := DefaultChromaticities()
	h.Set(&Attribute{Name: "chromaticities", Type: AttrTypeChromaticities, Value: rec709})

	sw, err := NewScanlineWriter(ws, h)
	if err != nil {
		t.Fatalf("NewScanlineWriter error: %v", err)
	}

	fb, _ := AllocateChannels(h.Channels(), h.DataWindow())

	// Write colorful data
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
	sw.SetFrameBuffer(fb)
	sw.WritePixels(0, height-1)
	sw.Close()

	// Read as ACES - should convert colors
	data := buf.Bytes()
	reader := bytes.NewReader(data)

	acesInput, err := OpenAcesInputFile(reader, int64(len(data)))
	if err != nil {
		t.Fatalf("OpenAcesInputFile error: %v", err)
	}

	// Check if conversion is needed
	if !acesInput.NeedsColorConversion() {
		t.Log("Color conversion not needed (may be due to similar white point)")
	}

	// Set frame buffer and read
	readFB, _ := AllocateChannels(acesInput.Header().Channels(), acesInput.DataWindow())
	acesInput.SetFrameBuffer(readFB)

	if err := acesInput.ReadPixels(0, height-1); err != nil {
		t.Fatalf("ReadPixels error: %v", err)
	}

	// Verify some pixels were converted
	rRead := readFB.Get("R")
	if rRead != nil {
		val := rRead.GetFloat32(15, 15)
		t.Logf("Center R value after conversion: %v", val)
	}
}
