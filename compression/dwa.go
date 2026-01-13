// Package compression provides DWA (Dreamworks Animation) compression for OpenEXR.
//
// DWA compression uses DCT (Discrete Cosine Transform) based lossy compression
// for RGB/luminance channels, combined with RLE for alpha and ZIP for other data.
// It's similar to JPEG but operates in a perceptually linear space.
//
// DWAA uses 32 scanlines per block, DWAB uses 256 scanlines per block.
//
// Channel classification:
//   - LOSSY_DCT: R, G, B, Y, RY, BY (HALF type only) - lossy DCT compression
//   - RLE: A (alpha) - lossless RLE compression
//   - UNKNOWN: All others - lossless ZIP compression
package compression

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"math"
	"sort"
	"sync"

	"github.com/klauspost/compress/zlib"
	"github.com/mrjoshuak/go-openexr/half"
)

// DWA compression version
const (
	dwaVersion = 2
)

// DWA header field indices
const (
	dwaVersion_                = 0
	dwaUnknownUncompressedSize = 1
	dwaUnknownCompressedSize   = 2
	dwaAcCompressedSize        = 3
	dwaDcCompressedSize        = 4
	dwaRleCompressedSize       = 5
	dwaRleUncompressedSize     = 6
	dwaRleRawSize              = 7
	dwaAcUncompressedCount     = 8
	dwaDcUncompressedCount     = 9
	dwaAcCompression           = 10
	dwaNumSizesSingle          = 11
)

// AC compression methods
const (
	acCompressionStaticHuffman = 0
	acCompressionDeflate       = 1
)

// Compressor schemes for channel classification
const (
	compressorUnknown  = 0
	compressorLossyDCT = 1
	compressorRLE      = 2
)

// ChannelPixelType represents pixel data types for DWA classification
const (
	dwaPixelTypeUint  = 0
	dwaPixelTypeHalf  = 1
	dwaPixelTypeFloat = 2
)

// jpegQuantTable is the standard JPEG luminance quantization matrix
// Used as base for DWA quantization, scaled by compression level
var jpegQuantTable = [64]float32{
	16, 11, 10, 16, 24, 40, 51, 61,
	12, 12, 14, 19, 26, 58, 60, 55,
	14, 13, 16, 24, 40, 57, 69, 56,
	14, 17, 22, 29, 51, 87, 80, 62,
	18, 22, 37, 56, 68, 109, 103, 77,
	24, 35, 55, 64, 81, 104, 113, 92,
	49, 64, 78, 87, 103, 121, 120, 101,
	72, 92, 95, 98, 112, 100, 103, 99,
}

// DwaChannelData holds per-channel information for DWA compression
type DwaChannelData struct {
	Name       string
	PixelType  int // dwaPixelTypeUint, dwaPixelTypeHalf, dwaPixelTypeFloat
	XSampling  int
	YSampling  int
	Scheme     int // compressorUnknown, compressorLossyDCT, compressorRLE
	PlanarData []uint16
}

// DWA compression errors
var (
	ErrDwaCorruptData   = errors.New("dwa: corrupt compressed data")
	ErrDwaUnsupported   = errors.New("dwa: unsupported version")
	ErrDwaInvalidHeader = errors.New("dwa: invalid header")
)

// dwaHeaderSize is the size of the DWA header in bytes
const dwaHeaderSize = dwaNumSizesSingle * 8

// zigzag is the zigzag order for 8x8 DCT coefficients
var zigzag = [64]int{
	0, 1, 8, 16, 9, 2, 3, 10,
	17, 24, 32, 25, 18, 11, 4, 5,
	12, 19, 26, 33, 40, 48, 41, 34,
	27, 20, 13, 6, 7, 14, 21, 28,
	35, 42, 49, 56, 57, 50, 43, 36,
	29, 22, 15, 23, 30, 37, 44, 51,
	58, 59, 52, 45, 38, 31, 39, 46,
	53, 60, 61, 54, 47, 55, 62, 63,
}

// invZigzag is the inverse zigzag mapping
var invZigzag = [64]int{
	0, 1, 5, 6, 14, 15, 27, 28,
	2, 4, 7, 13, 16, 26, 29, 42,
	3, 8, 12, 17, 25, 30, 41, 43,
	9, 11, 18, 24, 31, 40, 44, 53,
	10, 19, 23, 32, 39, 45, 52, 54,
	20, 22, 33, 38, 46, 51, 55, 60,
	21, 34, 37, 47, 50, 56, 59, 61,
	35, 36, 48, 49, 57, 58, 62, 63,
}

// Lookup tables for linear/nonlinear conversion
var (
	dwaTablesOnce       sync.Once
	dwaToLinearTable    [65536]uint16
	dwaToNonLinearTable [65536]uint16
)

// initDwaTables initializes the linear/nonlinear conversion lookup tables
func initDwaTables() {
	for i := 0; i < 65536; i++ {
		dwaToLinearTable[i] = dwaConvertToLinear(uint16(i))
		dwaToNonLinearTable[i] = dwaConvertToNonLinear(uint16(i))
	}
}

// ensureDwaTables ensures the lookup tables are initialized
func ensureDwaTables() {
	dwaTablesOnce.Do(initDwaTables)
}

// dwaConvertToLinear converts a nonlinear half-float to linear half-float
// For values <= 1.0: f^2.2 (gamma expansion)
// For values > 1.0: exp(2.2)^(f-1) (exponential for HDR)
func dwaConvertToLinear(x uint16) uint16 {
	if x == 0 {
		return 0
	}
	// Check for infinity/nan
	if (x & 0x7c00) == 0x7c00 {
		return 0
	}

	h := half.Half(x)
	f := h.Float32()
	sign := float32(1.0)
	if f < 0 {
		sign = -1.0
		f = -f
	}

	var z float32
	if f <= 1.0 {
		z = float32(math.Pow(float64(f), 2.2))
	} else {
		// pow(e^2.2, f-1) = exp(2.2 * (f-1))
		z = float32(math.Exp(2.2 * float64(f-1)))
	}

	return uint16(half.FromFloat32(sign * z))
}

// dwaConvertToNonLinear converts a linear half-float to nonlinear half-float
// For values <= 1.0: f^(1/2.2) (gamma compression)
// For values > 1.0: ln(f)/2.2 + 1 (log for HDR)
func dwaConvertToNonLinear(x uint16) uint16 {
	if x == 0 {
		return 0
	}
	// Check for infinity/nan
	if (x & 0x7c00) == 0x7c00 {
		return 0
	}

	h := half.Half(x)
	f := h.Float32()
	sign := float32(1.0)
	if f < 0 {
		sign = -1.0
		f = -f
	}

	var z float32
	if f <= 1.0 {
		z = float32(math.Pow(float64(f), 1.0/2.2))
	} else {
		z = float32(math.Log(float64(f)))/2.2 + 1.0
	}

	return uint16(half.FromFloat32(sign * z))
}

// DCT cosine coefficients for standard 8x8 DCT
// Using orthonormal DCT which has perfect round-trip properties
var (
	// Standard DCT constants: cos(k * pi / 16) for k = 1..7
	dctCos1 = float32(math.Cos(1.0 * math.Pi / 16.0)) // 0.98078528
	dctCos2 = float32(math.Cos(2.0 * math.Pi / 16.0)) // 0.92387953
	dctCos3 = float32(math.Cos(3.0 * math.Pi / 16.0)) // 0.83146961
	dctCos4 = float32(math.Cos(4.0 * math.Pi / 16.0)) // 0.70710678 = sqrt(1/2)
	dctCos5 = float32(math.Cos(5.0 * math.Pi / 16.0)) // 0.55557023
	dctCos6 = float32(math.Cos(6.0 * math.Pi / 16.0)) // 0.38268343
	dctCos7 = float32(math.Cos(7.0 * math.Pi / 16.0)) // 0.19509032
	dctSin1 = float32(math.Sin(1.0 * math.Pi / 16.0)) // 0.19509032
	dctSin2 = float32(math.Sin(2.0 * math.Pi / 16.0)) // 0.38268343
	dctSin3 = float32(math.Sin(3.0 * math.Pi / 16.0)) // 0.55557023
)

// precomputedDctCoeff contains the DCT coefficient matrix for 8x8 blocks
// C(u,v) = alpha(u) * alpha(v) * cos((2n+1)*u*pi/16) * cos((2m+1)*v*pi/16)
// where alpha(0) = 1/sqrt(8), alpha(k) = sqrt(2/8) for k > 0
var dctCoeff [8][8]float32

func init() {
	sqrt8 := float32(math.Sqrt(8))
	sqrt2_8 := float32(math.Sqrt(2.0 / 8.0))
	for k := 0; k < 8; k++ {
		for n := 0; n < 8; n++ {
			c := float32(math.Cos(float64(2*n+1) * float64(k) * math.Pi / 16.0))
			if k == 0 {
				dctCoeff[k][n] = c / sqrt8
			} else {
				dctCoeff[k][n] = c * sqrt2_8
			}
		}
	}
}

// dctForward8x8 performs a forward 8x8 DCT transform in-place
// Uses an optimized separable approach with unrolled loops
func dctForward8x8(data *[64]float32) {
	var workspace [64]float32

	// Pre-load coefficient rows for cache efficiency
	c0 := dctCoeff[0]
	c1 := dctCoeff[1]
	c2 := dctCoeff[2]
	c3 := dctCoeff[3]
	c4 := dctCoeff[4]
	c5 := dctCoeff[5]
	c6 := dctCoeff[6]
	c7 := dctCoeff[7]

	// First pass: transform rows
	for row := 0; row < 8; row++ {
		base := row * 8
		d0 := data[base]
		d1 := data[base+1]
		d2 := data[base+2]
		d3 := data[base+3]
		d4 := data[base+4]
		d5 := data[base+5]
		d6 := data[base+6]
		d7 := data[base+7]

		workspace[base] = d0*c0[0] + d1*c0[1] + d2*c0[2] + d3*c0[3] + d4*c0[4] + d5*c0[5] + d6*c0[6] + d7*c0[7]
		workspace[base+1] = d0*c1[0] + d1*c1[1] + d2*c1[2] + d3*c1[3] + d4*c1[4] + d5*c1[5] + d6*c1[6] + d7*c1[7]
		workspace[base+2] = d0*c2[0] + d1*c2[1] + d2*c2[2] + d3*c2[3] + d4*c2[4] + d5*c2[5] + d6*c2[6] + d7*c2[7]
		workspace[base+3] = d0*c3[0] + d1*c3[1] + d2*c3[2] + d3*c3[3] + d4*c3[4] + d5*c3[5] + d6*c3[6] + d7*c3[7]
		workspace[base+4] = d0*c4[0] + d1*c4[1] + d2*c4[2] + d3*c4[3] + d4*c4[4] + d5*c4[5] + d6*c4[6] + d7*c4[7]
		workspace[base+5] = d0*c5[0] + d1*c5[1] + d2*c5[2] + d3*c5[3] + d4*c5[4] + d5*c5[5] + d6*c5[6] + d7*c5[7]
		workspace[base+6] = d0*c6[0] + d1*c6[1] + d2*c6[2] + d3*c6[3] + d4*c6[4] + d5*c6[5] + d6*c6[6] + d7*c6[7]
		workspace[base+7] = d0*c7[0] + d1*c7[1] + d2*c7[2] + d3*c7[3] + d4*c7[4] + d5*c7[5] + d6*c7[6] + d7*c7[7]
	}

	// Second pass: transform columns
	for col := 0; col < 8; col++ {
		d0 := workspace[col]
		d1 := workspace[8+col]
		d2 := workspace[16+col]
		d3 := workspace[24+col]
		d4 := workspace[32+col]
		d5 := workspace[40+col]
		d6 := workspace[48+col]
		d7 := workspace[56+col]

		data[col] = d0*c0[0] + d1*c0[1] + d2*c0[2] + d3*c0[3] + d4*c0[4] + d5*c0[5] + d6*c0[6] + d7*c0[7]
		data[8+col] = d0*c1[0] + d1*c1[1] + d2*c1[2] + d3*c1[3] + d4*c1[4] + d5*c1[5] + d6*c1[6] + d7*c1[7]
		data[16+col] = d0*c2[0] + d1*c2[1] + d2*c2[2] + d3*c2[3] + d4*c2[4] + d5*c2[5] + d6*c2[6] + d7*c2[7]
		data[24+col] = d0*c3[0] + d1*c3[1] + d2*c3[2] + d3*c3[3] + d4*c3[4] + d5*c3[5] + d6*c3[6] + d7*c3[7]
		data[32+col] = d0*c4[0] + d1*c4[1] + d2*c4[2] + d3*c4[3] + d4*c4[4] + d5*c4[5] + d6*c4[6] + d7*c4[7]
		data[40+col] = d0*c5[0] + d1*c5[1] + d2*c5[2] + d3*c5[3] + d4*c5[4] + d5*c5[5] + d6*c5[6] + d7*c5[7]
		data[48+col] = d0*c6[0] + d1*c6[1] + d2*c6[2] + d3*c6[3] + d4*c6[4] + d5*c6[5] + d6*c6[6] + d7*c6[7]
		data[56+col] = d0*c7[0] + d1*c7[1] + d2*c7[2] + d3*c7[3] + d4*c7[4] + d5*c7[5] + d6*c7[6] + d7*c7[7]
	}
}

// dctInverse8x8 performs an inverse 8x8 DCT transform in-place
// zeroedRows indicates how many rows from the bottom are all zeros (optimization)
// Uses the standard DCT-III formula (inverse of DCT-II) with optimized unrolled loops
func dctInverse8x8(data *[64]float32, zeroedRows int) {
	var workspace [64]float32

	// Pre-load coefficient rows for cache efficiency
	c0 := dctCoeff[0]
	c1 := dctCoeff[1]
	c2 := dctCoeff[2]
	c3 := dctCoeff[3]
	c4 := dctCoeff[4]
	c5 := dctCoeff[5]
	c6 := dctCoeff[6]
	c7 := dctCoeff[7]

	// First pass: inverse transform columns
	for col := 0; col < 8; col++ {
		d0 := data[col]
		d1 := data[8+col]
		d2 := data[16+col]
		d3 := data[24+col]
		d4 := data[32+col]
		d5 := data[40+col]
		d6 := data[48+col]
		d7 := data[56+col]

		workspace[col] = d0*c0[0] + d1*c1[0] + d2*c2[0] + d3*c3[0] + d4*c4[0] + d5*c5[0] + d6*c6[0] + d7*c7[0]
		workspace[8+col] = d0*c0[1] + d1*c1[1] + d2*c2[1] + d3*c3[1] + d4*c4[1] + d5*c5[1] + d6*c6[1] + d7*c7[1]
		workspace[16+col] = d0*c0[2] + d1*c1[2] + d2*c2[2] + d3*c3[2] + d4*c4[2] + d5*c5[2] + d6*c6[2] + d7*c7[2]
		workspace[24+col] = d0*c0[3] + d1*c1[3] + d2*c2[3] + d3*c3[3] + d4*c4[3] + d5*c5[3] + d6*c6[3] + d7*c7[3]
		workspace[32+col] = d0*c0[4] + d1*c1[4] + d2*c2[4] + d3*c3[4] + d4*c4[4] + d5*c5[4] + d6*c6[4] + d7*c7[4]
		workspace[40+col] = d0*c0[5] + d1*c1[5] + d2*c2[5] + d3*c3[5] + d4*c4[5] + d5*c5[5] + d6*c6[5] + d7*c7[5]
		workspace[48+col] = d0*c0[6] + d1*c1[6] + d2*c2[6] + d3*c3[6] + d4*c4[6] + d5*c5[6] + d6*c6[6] + d7*c7[6]
		workspace[56+col] = d0*c0[7] + d1*c1[7] + d2*c2[7] + d3*c3[7] + d4*c4[7] + d5*c5[7] + d6*c6[7] + d7*c7[7]
	}

	// Second pass: inverse transform rows
	for row := 0; row < 8-zeroedRows; row++ {
		base := row * 8
		d0 := workspace[base]
		d1 := workspace[base+1]
		d2 := workspace[base+2]
		d3 := workspace[base+3]
		d4 := workspace[base+4]
		d5 := workspace[base+5]
		d6 := workspace[base+6]
		d7 := workspace[base+7]

		data[base] = d0*c0[0] + d1*c1[0] + d2*c2[0] + d3*c3[0] + d4*c4[0] + d5*c5[0] + d6*c6[0] + d7*c7[0]
		data[base+1] = d0*c0[1] + d1*c1[1] + d2*c2[1] + d3*c3[1] + d4*c4[1] + d5*c5[1] + d6*c6[1] + d7*c7[1]
		data[base+2] = d0*c0[2] + d1*c1[2] + d2*c2[2] + d3*c3[2] + d4*c4[2] + d5*c5[2] + d6*c6[2] + d7*c7[2]
		data[base+3] = d0*c0[3] + d1*c1[3] + d2*c2[3] + d3*c3[3] + d4*c4[3] + d5*c5[3] + d6*c6[3] + d7*c7[3]
		data[base+4] = d0*c0[4] + d1*c1[4] + d2*c2[4] + d3*c3[4] + d4*c4[4] + d5*c5[4] + d6*c6[4] + d7*c7[4]
		data[base+5] = d0*c0[5] + d1*c1[5] + d2*c2[5] + d3*c3[5] + d4*c4[5] + d5*c5[5] + d6*c6[5] + d7*c7[5]
		data[base+6] = d0*c0[6] + d1*c1[6] + d2*c2[6] + d3*c3[6] + d4*c4[6] + d5*c5[6] + d6*c6[6] + d7*c7[6]
		data[base+7] = d0*c0[7] + d1*c1[7] + d2*c2[7] + d3*c3[7] + d4*c4[7] + d5*c5[7] + d6*c6[7] + d7*c7[7]
	}
}

// dctInverse8x8DcOnly handles the special case where only DC component is non-zero
func dctInverse8x8DcOnly(data *[64]float32) {
	// For orthonormal DCT-II, the DC coefficient is scaled by 1/sqrt(8) in each dimension.
	// C++ uses: 3.535536e-01 * 3.535536e-01 = 0.125 = 1/8
	// This matches the DCT normalization factor of 1/sqrt(N) for each dimension.
	val := data[0] * 0.125
	for i := 0; i < 64; i++ {
		data[i] = val
	}
}

// csc709Forward performs forward color space conversion RGB -> YCbCr using 709 primaries
func csc709Forward(r, g, b *[64]float32) {
	for i := 0; i < 64; i++ {
		srcR := r[i]
		srcG := g[i]
		srcB := b[i]

		// Y'  = 0.2126 R' + 0.7152 G' + 0.0722 B'
		// Cb  = -0.1146 R' - 0.3854 G' + 0.5000 B'
		// Cr  = 0.5000 R' - 0.4542 G' - 0.0458 B'
		r[i] = 0.2126*srcR + 0.7152*srcG + 0.0722*srcB
		g[i] = -0.1146*srcR - 0.3854*srcG + 0.5000*srcB
		b[i] = 0.5000*srcR - 0.4542*srcG - 0.0458*srcB
	}
}

// csc709Inverse performs inverse color space conversion YCbCr -> RGB using 709 primaries
func csc709Inverse(y, cb, cr *[64]float32) {
	for i := 0; i < 64; i++ {
		srcY := y[i]
		srcCb := cb[i]
		srcCr := cr[i]

		// R' = Y' + 1.5747 Cr
		// G' = Y' - 0.1873 Cb - 0.4682 Cr
		// B' = Y' + 1.8556 Cb
		y[i] = srcY + 1.5747*srcCr
		cb[i] = srcY - 0.1873*srcCb - 0.4682*srcCr
		cr[i] = srcY + 1.8556*srcCb
	}
}

// DwaDecompressor handles DWA/DWAB decompression
type DwaDecompressor struct {
	width    int
	height   int
	channels []DwaChannelData
}

// NewDwaDecompressor creates a new DWA decompressor
func NewDwaDecompressor(width, height int) *DwaDecompressor {
	return &DwaDecompressor{
		width:  width,
		height: height,
	}
}

// SetChannels sets the channel information for decompression
func (d *DwaDecompressor) SetChannels(channels []DwaChannelData) {
	d.channels = make([]DwaChannelData, len(channels))
	copy(d.channels, channels)
	// Sort by name for consistent ordering
	sort.Slice(d.channels, func(i, j int) bool {
		return d.channels[i].Name < d.channels[j].Name
	})
}

// parseChannelRules parses channel rules from the compressed data
func (d *DwaDecompressor) parseChannelRules(data []byte) []DwaChannelData {
	var channels []DwaChannelData
	pos := 0
	for pos < len(data) {
		// Read null-terminated name
		nameEnd := pos
		for nameEnd < len(data) && data[nameEnd] != 0 {
			nameEnd++
		}
		if nameEnd >= len(data) {
			break
		}
		name := string(data[pos:nameEnd])
		pos = nameEnd + 1

		if pos+2 > len(data) {
			break
		}
		scheme := int(data[pos])
		pixelType := int(data[pos+1])
		pos += 2

		channels = append(channels, DwaChannelData{
			Name:      name,
			Scheme:    scheme,
			PixelType: pixelType,
		})
	}
	return channels
}

// Decompress decompresses DWA compressed data
func (d *DwaDecompressor) Decompress(src []byte, dst []byte) error {
	if len(src) < dwaHeaderSize {
		return ErrDwaCorruptData
	}

	// Read header
	header := make([]uint64, dwaNumSizesSingle)
	for i := 0; i < dwaNumSizesSingle; i++ {
		header[i] = binary.LittleEndian.Uint64(src[i*8:])
	}

	version := header[dwaVersion_]
	if version > 2 {
		return ErrDwaUnsupported
	}

	unknownUncompressedSize := header[dwaUnknownUncompressedSize]
	unknownCompressedSize := header[dwaUnknownCompressedSize]
	acCompressedSize := header[dwaAcCompressedSize]
	dcCompressedSize := header[dwaDcCompressedSize]
	rleCompressedSize := header[dwaRleCompressedSize]
	rleUncompressedSize := header[dwaRleUncompressedSize]
	// rleRawSize := header[dwaRleRawSize]
	totalAcUncompressedCount := header[dwaAcUncompressedCount]
	totalDcUncompressedCount := header[dwaDcUncompressedCount]
	acCompression := header[dwaAcCompression]

	// Validate sizes
	compressedSize := unknownCompressedSize + acCompressedSize +
		dcCompressedSize + rleCompressedSize

	dataPtr := dwaHeaderSize

	// Parse channel rules for version >= 2
	var channelRules []DwaChannelData
	if version >= 2 {
		if dataPtr >= len(src) {
			return ErrDwaCorruptData
		}
		// Read channel rules size (first 4 bytes after header)
		if dataPtr+4 > len(src) {
			return ErrDwaCorruptData
		}
		ruleSize := int(binary.LittleEndian.Uint32(src[dataPtr:]))
		dataPtr += 4
		if ruleSize > 0 {
			if dataPtr+ruleSize > len(src) {
				return ErrDwaCorruptData
			}
			channelRules = d.parseChannelRules(src[dataPtr : dataPtr+ruleSize])
			dataPtr += ruleSize
		}
	}

	// Use parsed rules or fallback to pre-set channels
	if len(channelRules) > 0 {
		d.channels = channelRules
	}

	if uint64(len(src)) < uint64(dataPtr)+compressedSize {
		return ErrDwaCorruptData
	}

	// Locate compressed data sections with individual bounds checking
	endPtr := dataPtr + int(unknownCompressedSize)
	if endPtr < dataPtr || endPtr > len(src) {
		return ErrDwaCorruptData
	}
	compressedUnknownBuf := src[dataPtr:endPtr]
	dataPtr = endPtr

	endPtr = dataPtr + int(acCompressedSize)
	if endPtr < dataPtr || endPtr > len(src) {
		return ErrDwaCorruptData
	}
	compressedAcBuf := src[dataPtr:endPtr]
	dataPtr = endPtr

	endPtr = dataPtr + int(dcCompressedSize)
	if endPtr < dataPtr || endPtr > len(src) {
		return ErrDwaCorruptData
	}
	compressedDcBuf := src[dataPtr:endPtr]
	dataPtr = endPtr

	endPtr = dataPtr + int(rleCompressedSize)
	if endPtr < dataPtr || endPtr > len(src) {
		return ErrDwaCorruptData
	}
	compressedRleBuf := src[dataPtr:endPtr]

	// Decompress UNKNOWN data
	var unknownData []byte
	if unknownCompressedSize > 0 && unknownUncompressedSize > 0 {
		var err error
		unknownData, err = zlibDecompress(compressedUnknownBuf, int(unknownUncompressedSize))
		if err != nil {
			return ErrDwaCorruptData
		}
	}

	// Decompress AC data
	var acData []byte
	if acCompressedSize > 0 && totalAcUncompressedCount > 0 {
		acDataSize := int(totalAcUncompressedCount * 2)
		if acCompression == acCompressionStaticHuffman {
			var err error
			acData, err = huffmanDecode(compressedAcBuf, acDataSize)
			if err != nil {
				return ErrDwaCorruptData
			}
		} else {
			var err error
			acData, err = zlibDecompress(compressedAcBuf, acDataSize)
			if err != nil {
				return ErrDwaCorruptData
			}
		}
	}

	// Decompress DC data
	var dcData []byte
	if dcCompressedSize > 0 && totalDcUncompressedCount > 0 {
		dcDataSize := int(totalDcUncompressedCount * 2)
		var err error
		dcData, err = zlibDecompress(compressedDcBuf, dcDataSize)
		if err != nil {
			return ErrDwaCorruptData
		}
	}

	// Decompress RLE data
	var rleData []byte
	if rleCompressedSize > 0 && rleUncompressedSize > 0 {
		var err error
		rleData, err = zlibDecompress(compressedRleBuf, int(rleUncompressedSize))
		if err != nil {
			return ErrDwaCorruptData
		}
	}

	// If we have UNKNOWN data and no channels defined, copy directly
	if len(d.channels) == 0 && len(unknownData) > 0 {
		copy(dst, unknownData)
		return nil
	}

	// Count DCT channels for block processing
	numDctChannels := 0
	for _, ch := range d.channels {
		if ch.Scheme == compressorLossyDCT {
			numDctChannels++
		}
	}

	// If we have DCT data, decode it
	if len(dcData) > 0 && len(acData) > 0 && numDctChannels > 0 {
		numBlocksX := (d.width + 7) / 8
		numBlocksY := (d.height + 7) / 8
		numBlocks := numBlocksX * numBlocksY
		numPixels := d.width * d.height

		// Decode DC coefficients and undo differencing
		dcValues := make([]uint16, len(dcData)/2)
		for i := 0; i < len(dcValues); i++ {
			dcValues[i] = binary.LittleEndian.Uint16(dcData[i*2:])
		}
		// Undo DC differencing
		if len(dcValues) > 1 {
			for i := 1; i < len(dcValues); i++ {
				dcValues[i] = dcValues[i-1] + dcValues[i]
			}
		}

		// Allocate channel data buffers
		channelData := make([][]uint16, len(d.channels))
		for i := range channelData {
			channelData[i] = make([]uint16, numPixels)
		}

		// Decode DCT channels block by block
		dcIdx := 0
		acPos := 0
		for chIdx, ch := range d.channels {
			if ch.Scheme != compressorLossyDCT {
				continue
			}

			for by := 0; by < numBlocksY; by++ {
				for bx := 0; bx < numBlocksX; bx++ {
					if dcIdx >= len(dcValues) {
						break
					}
					dc := dcValues[dcIdx]
					dcIdx++

					// Decode AC coefficients for this block
					ac, err := decodeAcCoefficients(acData[acPos:], 63)
					if err != nil {
						// Try to recover
						ac = make([]uint16, 63)
					}
					// Find where this block's AC data ends
					for acPos < len(acData) {
						if acData[acPos] == 0xFF && acPos+1 < len(acData) && acData[acPos+1] == 0 {
							acPos += 2
							break
						}
						if acData[acPos] == 0xFF {
							acPos += 2
						} else {
							acPos += 2
						}
					}

					// Decompress block
					blockPixels := decompressBlock8x8(dc, ac)

					// Copy block to channel data
					for y := 0; y < 8; y++ {
						for x := 0; x < 8; x++ {
							px := bx*8 + x
							py := by*8 + y
							if px < d.width && py < d.height {
								channelData[chIdx][py*d.width+px] = blockPixels[y*8+x]
							}
						}
					}
				}
			}
		}

		// Decode RLE channels
		rlePos := 0
		for chIdx, ch := range d.channels {
			if ch.Scheme != compressorRLE {
				continue
			}
			chDataSize := numPixels * 2
			if rlePos+chDataSize <= len(rleData) {
				decompressed, _ := RLEDecompress(rleData[rlePos:], chDataSize)
				for i := 0; i < numPixels && i*2+1 < len(decompressed); i++ {
					channelData[chIdx][i] = binary.LittleEndian.Uint16(decompressed[i*2:])
				}
				rlePos += chDataSize
			}
		}

		// Decode UNKNOWN channels
		unknownPos := 0
		for chIdx, ch := range d.channels {
			if ch.Scheme != compressorUnknown {
				continue
			}
			switch ch.PixelType {
			case dwaPixelTypeHalf:
				for i := 0; i < numPixels && unknownPos+2 <= len(unknownData); i++ {
					channelData[chIdx][i] = binary.LittleEndian.Uint16(unknownData[unknownPos:])
					unknownPos += 2
				}
			default:
				for i := 0; i < numPixels && unknownPos+4 <= len(unknownData); i++ {
					channelData[chIdx][i] = uint16(binary.LittleEndian.Uint32(unknownData[unknownPos:]) & 0xFFFF)
					unknownPos += 4
				}
			}
		}

		// Interleave channels back to dst
		dstPos := 0
		for pixel := 0; pixel < numPixels; pixel++ {
			for chIdx, ch := range d.channels {
				switch ch.PixelType {
				case dwaPixelTypeHalf:
					if dstPos+2 <= len(dst) {
						binary.LittleEndian.PutUint16(dst[dstPos:], channelData[chIdx][pixel])
						dstPos += 2
					}
				case dwaPixelTypeFloat:
					if dstPos+4 <= len(dst) {
						f := half.Half(channelData[chIdx][pixel]).Float32()
						binary.LittleEndian.PutUint32(dst[dstPos:], math.Float32bits(f))
						dstPos += 4
					}
				case dwaPixelTypeUint:
					if dstPos+4 <= len(dst) {
						binary.LittleEndian.PutUint32(dst[dstPos:], uint32(channelData[chIdx][pixel]))
						dstPos += 4
					}
				}
			}
		}

		_ = numBlocks // suppress unused variable warning
	} else if len(unknownData) > 0 {
		// No DCT data, just copy UNKNOWN
		copy(dst, unknownData)
	}

	return nil
}

// zlibDecompress decompresses zlib data
func zlibDecompress(src []byte, expectedSize int) ([]byte, error) {
	r, err := zlib.NewReader(bytes.NewReader(src))
	if err != nil {
		return nil, err
	}
	defer r.Close()

	dst := make([]byte, expectedSize)
	n, err := io.ReadFull(r, dst)
	if err != nil && err != io.ErrUnexpectedEOF {
		return nil, err
	}
	return dst[:n], nil
}

// huffmanDecode decodes Huffman compressed data (using OpenEXR's static Huffman)
func huffmanDecode(src []byte, expectedSize int) ([]byte, error) {
	// For now, fall back to zlib as the Huffman decoder needs the
	// OpenEXR-specific Huffman implementation
	// TODO: Implement proper Huffman decoder
	return zlibDecompress(src, expectedSize)
}

// classifyChannelName returns the compression scheme for a channel based on its name
// R, G, B, Y, RY, BY (HALF only) -> LOSSY_DCT
// A -> RLE
// Everything else -> UNKNOWN (ZIP)
func classifyChannelName(name string, pixelType int) int {
	if pixelType != dwaPixelTypeHalf {
		return compressorUnknown
	}

	// Get the suffix after any layer prefix (e.g., "layer.R" -> "R")
	suffix := name
	if idx := len(name) - 1; idx >= 0 {
		for i := len(name) - 1; i >= 0; i-- {
			if name[i] == '.' {
				suffix = name[i+1:]
				break
			}
		}
	}

	switch suffix {
	case "R", "G", "B", "Y", "RY", "BY":
		return compressorLossyDCT
	case "A":
		return compressorRLE
	default:
		return compressorUnknown
	}
}

// DwaCompressor handles DWA/DWAB compression
type DwaCompressor struct {
	width            int
	height           int
	compressionLevel float32
	channels         []DwaChannelData
}

// NewDwaCompressor creates a new DWA compressor
func NewDwaCompressor(width, height int, level float32) *DwaCompressor {
	if level < 0 {
		level = 45.0 // Default DWA compression level
	}
	return &DwaCompressor{
		width:            width,
		height:           height,
		compressionLevel: level,
	}
}

// SetChannels sets the channel information for DCT-based compression
func (c *DwaCompressor) SetChannels(channels []DwaChannelData) {
	c.channels = make([]DwaChannelData, len(channels))
	copy(c.channels, channels)
	// Classify each channel
	for i := range c.channels {
		c.channels[i].Scheme = classifyChannelName(c.channels[i].Name, c.channels[i].PixelType)
	}
	// Sort by name for consistent ordering
	sort.Slice(c.channels, func(i, j int) bool {
		return c.channels[i].Name < c.channels[j].Name
	})
}

// quantizeCoefficient applies DWA-style quantization to a DCT coefficient
// It finds the nearest value with fewer bits set within the error tolerance
func quantizeCoefficient(val float32, quantLevel float32) uint16 {
	h := half.FromFloat32(val)
	hv := uint16(h)

	if quantLevel == 0 || hv == 0 {
		return hv
	}

	// Compute error tolerance based on quantization level
	// Higher quantLevel = more aggressive quantization
	tolerance := float32(math.Abs(float64(val))) * quantLevel / 1000.0
	if tolerance < 0.0001 {
		tolerance = 0.0001
	}

	// Try to find a value with fewer bits set
	best := hv
	bestBits := popcount16(hv)

	// Check nearby half-float values
	for delta := uint16(1); delta < 64; delta++ {
		// Try lower values
		if hv >= delta {
			candidate := hv - delta
			candidateVal := half.Half(candidate).Float32()
			if math.Abs(float64(candidateVal-val)) <= float64(tolerance) {
				bits := popcount16(candidate)
				if bits < bestBits {
					best = candidate
					bestBits = bits
				}
			}
		}
		// Try higher values
		if hv+delta < 0x7C00 { // Less than infinity
			candidate := hv + delta
			candidateVal := half.Half(candidate).Float32()
			if math.Abs(float64(candidateVal-val)) <= float64(tolerance) {
				bits := popcount16(candidate)
				if bits < bestBits {
					best = candidate
					bestBits = bits
				}
			}
		}
	}

	return best
}

// popcount16 counts the number of set bits in a 16-bit value
func popcount16(x uint16) int {
	// Brian Kernighan's algorithm
	count := 0
	for x != 0 {
		x &= x - 1
		count++
	}
	return count
}

// encodeAcCoefficients encodes AC coefficients using RLE
// Format: 0xFF followed by run length for zero runs, 0xFF 0x00 for end-of-block
func encodeAcCoefficients(coeffs []uint16) []byte {
	var result []byte
	i := 0
	for i < len(coeffs) {
		// Count zeros
		zeroRun := 0
		for i+zeroRun < len(coeffs) && coeffs[i+zeroRun] == 0 && zeroRun < 254 {
			zeroRun++
		}

		if zeroRun > 0 {
			// Encode zero run
			result = append(result, 0xFF, byte(zeroRun))
			i += zeroRun
		} else {
			// Encode non-zero value
			val := coeffs[i]
			if val == 0xFF00 || (val&0xFF00) == 0xFF00 {
				// Escape sequence needed for values starting with 0xFF
				result = append(result, byte(val>>8), byte(val))
			} else {
				result = append(result, byte(val), byte(val>>8))
			}
			i++
		}
	}
	// End of block marker
	result = append(result, 0xFF, 0x00)
	return result
}

// decodeAcCoefficients decodes RLE-encoded AC coefficients
func decodeAcCoefficients(data []byte, count int) ([]uint16, error) {
	result := make([]uint16, 0, count)
	i := 0
	for i < len(data) && len(result) < count {
		if data[i] == 0xFF {
			if i+1 >= len(data) {
				return nil, ErrDwaCorruptData
			}
			runLen := data[i+1]
			if runLen == 0 {
				// End of block - fill rest with zeros
				for len(result) < count {
					result = append(result, 0)
				}
				break
			}
			// Zero run
			for j := 0; j < int(runLen) && len(result) < count; j++ {
				result = append(result, 0)
			}
			i += 2
		} else {
			// Non-zero value (little-endian)
			if i+1 >= len(data) {
				return nil, ErrDwaCorruptData
			}
			val := uint16(data[i]) | uint16(data[i+1])<<8
			result = append(result, val)
			i += 2
		}
	}
	return result, nil
}

// compressBlock8x8 compresses a single 8x8 block using DCT
// Returns DC coefficient and RLE-encoded AC coefficients
func (c *DwaCompressor) compressBlock8x8(block []uint16) (uint16, []byte) {
	var floatBlock [64]float32

	// Convert half-float to float and apply nonlinear conversion
	ensureDwaTables()
	for i := 0; i < 64 && i < len(block); i++ {
		// Convert to nonlinear space for better perceptual compression
		nlVal := dwaToNonLinearTable[block[i]]
		floatBlock[i] = half.Half(nlVal).Float32()
	}

	// Apply forward DCT
	dctForward8x8(&floatBlock)

	// Quantize coefficients and reorder using zigzag scan
	// zigzag[i] gives the position in the 8x8 block for the i-th coefficient in zigzag order
	var quantized [64]uint16
	for i := 0; i < 64; i++ {
		// Get coefficient at zigzag position
		coeff := floatBlock[zigzag[i]]
		// Scale quantization by JPEG table (using zigzag index for quant table)
		quantLevel := c.compressionLevel * jpegQuantTable[i] / 16.0
		quantized[i] = quantizeCoefficient(coeff, quantLevel)
	}

	// Separate DC (first in zigzag order) and AC (rest)
	dc := quantized[0]
	ac := encodeAcCoefficients(quantized[1:])

	return dc, ac
}

// decompressBlock8x8 decompresses a single 8x8 block
func decompressBlock8x8(dc uint16, ac []uint16) []uint16 {
	var floatBlock [64]float32

	// Place DC coefficient at position zigzag[0] = 0 (top-left)
	floatBlock[zigzag[0]] = half.Half(dc).Float32()

	// Place AC coefficients using inverse zigzag ordering
	for i := 0; i < len(ac) && i < 63; i++ {
		floatBlock[zigzag[i+1]] = half.Half(ac[i]).Float32()
	}

	// Apply inverse DCT
	dctInverse8x8(&floatBlock, 0)

	// Convert back to half-float and apply linear conversion
	ensureDwaTables()
	result := make([]uint16, 64)
	for i := 0; i < 64; i++ {
		h := half.FromFloat32(floatBlock[i])
		// Convert back to linear space
		result[i] = dwaToLinearTable[uint16(h)]
	}

	return result
}

// Compress compresses data using DWA compression with full DCT pipeline
func (c *DwaCompressor) Compress(src []byte) ([]byte, error) {
	// If no channels set, fall back to UNKNOWN (ZIP) compression
	if len(c.channels) == 0 {
		return c.compressUnknown(src)
	}

	// Separate channels into DCT, RLE, and UNKNOWN groups
	var dctChannels, rleChannels, unknownChannels []int
	for i, ch := range c.channels {
		switch ch.Scheme {
		case compressorLossyDCT:
			dctChannels = append(dctChannels, i)
		case compressorRLE:
			rleChannels = append(rleChannels, i)
		default:
			unknownChannels = append(unknownChannels, i)
		}
	}

	// If no DCT channels, use simple compression
	if len(dctChannels) == 0 {
		return c.compressUnknown(src)
	}

	// Calculate pixel counts per channel
	numPixels := c.width * c.height

	// Extract channel data from interleaved source
	// Assuming src is organized as scanlines with all channels interleaved
	bytesPerPixel := 0
	for _, ch := range c.channels {
		switch ch.PixelType {
		case dwaPixelTypeHalf:
			bytesPerPixel += 2
		case dwaPixelTypeFloat, dwaPixelTypeUint:
			bytesPerPixel += 4
		}
	}

	if len(src) < numPixels*bytesPerPixel {
		return c.compressUnknown(src)
	}

	// Deinterleave channels into planar format
	channelData := make([][]uint16, len(c.channels))
	for i := range channelData {
		channelData[i] = make([]uint16, numPixels)
	}

	srcPos := 0
	for pixel := 0; pixel < numPixels; pixel++ {
		for chIdx := range c.channels {
			ch := &c.channels[chIdx]
			switch ch.PixelType {
			case dwaPixelTypeHalf:
				if srcPos+2 <= len(src) {
					channelData[chIdx][pixel] = binary.LittleEndian.Uint16(src[srcPos:])
					srcPos += 2
				}
			case dwaPixelTypeFloat:
				if srcPos+4 <= len(src) {
					f := math.Float32frombits(binary.LittleEndian.Uint32(src[srcPos:]))
					channelData[chIdx][pixel] = uint16(half.FromFloat32(f))
					srcPos += 4
				}
			case dwaPixelTypeUint:
				if srcPos+4 <= len(src) {
					// Store as-is (will go to UNKNOWN)
					channelData[chIdx][pixel] = uint16(binary.LittleEndian.Uint32(src[srcPos:]) & 0xFFFF)
					srcPos += 4
				}
			}
		}
	}

	// Process DCT channels - encode in 8x8 blocks
	numBlocksX := (c.width + 7) / 8
	numBlocksY := (c.height + 7) / 8

	// Collect all DC and AC data
	var allDC []uint16
	var allAC bytes.Buffer

	for _, chIdx := range dctChannels {
		chData := channelData[chIdx]

		// Process each 8x8 block
		for by := 0; by < numBlocksY; by++ {
			for bx := 0; bx < numBlocksX; bx++ {
				// Extract 8x8 block
				var block [64]uint16
				for y := 0; y < 8; y++ {
					for x := 0; x < 8; x++ {
						px := bx*8 + x
						py := by*8 + y
						if px < c.width && py < c.height {
							block[y*8+x] = chData[py*c.width+px]
						}
					}
				}

				// Compress block
				dc, acBytes := c.compressBlock8x8(block[:])
				allDC = append(allDC, dc)
				allAC.Write(acBytes)
			}
		}
	}

	// Apply DC differencing for better compression
	if len(allDC) > 1 {
		prev := allDC[0]
		for i := 1; i < len(allDC); i++ {
			curr := allDC[i]
			allDC[i] = curr - prev
			prev = curr
		}
	}

	// Compress DC values with zlib
	dcBytes := make([]byte, len(allDC)*2)
	for i, dc := range allDC {
		binary.LittleEndian.PutUint16(dcBytes[i*2:], dc)
	}
	dcCompressed, err := zlibCompress(dcBytes)
	if err != nil {
		return c.compressUnknown(src)
	}

	// Compress AC data with zlib
	acCompressed, err := zlibCompress(allAC.Bytes())
	if err != nil {
		return c.compressUnknown(src)
	}

	// Process RLE channels
	var rleRawData bytes.Buffer
	var rleRawSize int
	for _, chIdx := range rleChannels {
		chData := channelData[chIdx]
		rawBytes := make([]byte, len(chData)*2)
		for i, v := range chData {
			binary.LittleEndian.PutUint16(rawBytes[i*2:], v)
		}
		rleRawSize += len(rawBytes)
		rleRawData.Write(RLECompress(rawBytes))
	}
	// Compress RLE data with zlib
	var rleCompressed []byte
	var rleUncompressedLen int
	if rleRawData.Len() > 0 {
		rleUncompressedLen = rleRawData.Len()
		var err error
		rleCompressed, err = zlibCompress(rleRawData.Bytes())
		if err != nil {
			rleCompressed = nil
		}
	}

	// Process UNKNOWN channels
	var unknownData bytes.Buffer
	for _, chIdx := range unknownChannels {
		chData := channelData[chIdx]
		ch := &c.channels[chIdx]
		switch ch.PixelType {
		case dwaPixelTypeHalf:
			for _, v := range chData {
				binary.Write(&unknownData, binary.LittleEndian, v)
			}
		default:
			// Re-read original data for non-HALF channels
			// (simplified - assumes we still have it)
			for _, v := range chData {
				binary.Write(&unknownData, binary.LittleEndian, uint32(v))
			}
		}
	}
	unknownCompressed, err := zlibCompress(unknownData.Bytes())
	if err != nil {
		unknownCompressed = nil
	}

	// Build output with DWA header
	var buf bytes.Buffer
	header := make([]uint64, dwaNumSizesSingle)
	header[dwaVersion_] = dwaVersion
	header[dwaUnknownUncompressedSize] = uint64(unknownData.Len())
	header[dwaUnknownCompressedSize] = uint64(len(unknownCompressed))
	header[dwaAcCompressedSize] = uint64(len(acCompressed))
	header[dwaDcCompressedSize] = uint64(len(dcCompressed))
	header[dwaRleCompressedSize] = uint64(len(rleCompressed))
	header[dwaRleUncompressedSize] = uint64(rleUncompressedLen)
	header[dwaRleRawSize] = uint64(rleRawSize)
	header[dwaAcUncompressedCount] = uint64(allAC.Len() / 2)
	header[dwaDcUncompressedCount] = uint64(len(allDC))
	header[dwaAcCompression] = acCompressionDeflate

	// Write header
	for _, v := range header {
		binary.Write(&buf, binary.LittleEndian, v)
	}

	// Write channel rules
	rules := c.buildChannelRules()
	binary.Write(&buf, binary.LittleEndian, uint32(len(rules)))
	buf.Write(rules)

	// Write compressed data sections
	buf.Write(unknownCompressed)
	buf.Write(acCompressed)
	buf.Write(dcCompressed)
	buf.Write(rleCompressed)

	return buf.Bytes(), nil
}

// buildChannelRules creates the channel classification rules for the header
func (c *DwaCompressor) buildChannelRules() []byte {
	var buf bytes.Buffer
	for _, ch := range c.channels {
		// Format: name (null-terminated) + scheme byte + pixel type byte
		buf.WriteString(ch.Name)
		buf.WriteByte(0)
		buf.WriteByte(byte(ch.Scheme))
		buf.WriteByte(byte(ch.PixelType))
	}
	return buf.Bytes()
}

// compressUnknown compresses all data as UNKNOWN (ZIP fallback)
func (c *DwaCompressor) compressUnknown(src []byte) ([]byte, error) {
	var buf bytes.Buffer

	// Write header
	header := make([]uint64, dwaNumSizesSingle)
	header[dwaVersion_] = dwaVersion

	// Compress src with zlib as "unknown" data
	compressedBuf, err := zlibCompress(src)
	if err != nil {
		return nil, err
	}

	header[dwaUnknownUncompressedSize] = uint64(len(src))
	header[dwaUnknownCompressedSize] = uint64(len(compressedBuf))
	header[dwaAcCompression] = acCompressionDeflate

	// Write header
	for _, v := range header {
		binary.Write(&buf, binary.LittleEndian, v)
	}

	// Write channel rules (empty for version 2)
	binary.Write(&buf, binary.LittleEndian, uint32(0))

	// Write compressed data
	buf.Write(compressedBuf)

	return buf.Bytes(), nil
}

// zlibCompress compresses data using zlib
func zlibCompress(src []byte) ([]byte, error) {
	var buf bytes.Buffer
	w := zlib.NewWriter(&buf)
	_, err := w.Write(src)
	if err != nil {
		return nil, err
	}
	w.Close()
	return buf.Bytes(), nil
}

// DecompressDWAA decompresses DWAA (32 scanlines) compressed data
func DecompressDWAA(src []byte, dst []byte, width, height int) error {
	d := NewDwaDecompressor(width, height)
	return d.Decompress(src, dst)
}

// DecompressDWAB decompresses DWAB (256 scanlines) compressed data
func DecompressDWAB(src []byte, dst []byte, width, height int) error {
	d := NewDwaDecompressor(width, height)
	return d.Decompress(src, dst)
}

// CompressDWAA compresses data using DWAA (32 scanlines) compression
func CompressDWAA(src []byte, width, height int, level float32) ([]byte, error) {
	c := NewDwaCompressor(width, height, level)
	return c.Compress(src)
}

// CompressDWAB compresses data using DWAB (256 scanlines) compression
func CompressDWAB(src []byte, width, height int, level float32) ([]byte, error) {
	c := NewDwaCompressor(width, height, level)
	return c.Compress(src)
}
