# Deterministic Round-Trip Support

This document describes how to achieve deterministic file hashes when reading and writing OpenEXR files with go-openexr.

## Overview

For certification and verification workflows, it's often necessary to ensure that reading and writing an EXR file produces an identical file (byte-for-byte). This requires careful attention to several factors.

## What is Deterministic

The following are always deterministic in go-openexr:

1. **Header Serialization**: Attributes are written in alphabetical order by name
2. **Header.Attributes()**: Returns attributes sorted by name for deterministic iteration
3. **Compression Algorithms**: All compression algorithms produce identical output given identical input:
   - ZIP/ZIPS (zlib)
   - RLE
   - PIZ
   - B44/B44A
   - PXR24 (lossy, but deterministic)
   - DWAA/DWAB (lossy, but deterministic)
   - HTJ2K (via go-jpeg2000)
4. **Channel Order**: Channels are written in the order specified in the ChannelList
5. **ID Manifests** (exrid package):
   - Manifest entries are written in sorted order by ID
   - Cryptomatte JSON manifests have keys in alphabetical order

## Compression Level Detection and Preservation

ZIP/ZIPS compression uses zlib, which encodes a compression level category (FLEVEL) in the header:

| FLEVEL | Category | Levels     |
| ------ | -------- | ---------- |
| 0      | Fastest  | 0, 1       |
| 1      | Fast     | 2, 3, 4, 5 |
| 2      | Default  | 6 (or -1)  |
| 3      | Best     | 7, 8, 9    |

### Automatic Detection

When reading ZIP-compressed files, go-openexr automatically detects the FLEVEL from the compressed data and stores it in the header's `CompressionOptions`. This information is then used when writing to preserve the compression level category.

```go
// Reading a file automatically detects FLEVEL
file, err := exr.Open("input.exr")
header := file.Header(0)

// Check if FLEVEL was detected
flevel, detected := header.DetectedFLevel()
if detected {
    fmt.Printf("Detected FLEVEL: %d\n", flevel)
}
```

### Manual Level Configuration

You can manually set the compression level:

```go
header := exr.NewScanlineHeader(1920, 1080)
header.SetZIPLevel(compression.CompressionLevelBestSize) // Level 9
```

Available compression levels:

- `CompressionLevelHuffmanOnly` (-2): Huffman-only, fastest (klauspost extension)
- `CompressionLevelDefault` (-1): Default level (6)
- `CompressionLevelNone` (0): No compression
- `CompressionLevelBestSpeed` (1): Best speed
- `CompressionLevelBestSize` (9): Best compression

### Round-Trip Workflow

For identical file hashes:

```go
// 1. Read the source file
srcFile, err := exr.Open("source.exr")
if err != nil {
    return err
}
srcHeader := srcFile.Header(0)

// 2. Create output header, preserving compression options
dstHeader := exr.NewHeader()
// Copy attributes...
dstHeader.SetCompressionOptions(srcHeader.CompressionOptions())

// 3. Write with preserved settings
writer, err := exr.NewScanlineWriter(output, dstHeader)
// Write pixel data...
```

## Factors Affecting Determinism

### What CAN cause different output

1. **Different zlib implementations**: Go's zlib (via klauspost/compress) may produce different compressed bytes than C++ zlib, even at the same level. The decompressed data is identical, but compressed representation differs.

2. **Different compression levels**: Using a different compression level will produce different output.

3. **Lossy compression re-encoding**: Re-encoding with lossy compression (PXR24, B44, DWAA/DWAB) will compound losses.

4. **Attribute modifications**: Adding, removing, or modifying attributes changes the file.

### What is ALWAYS preserved

1. **Pixel data values**: For lossless compression (ZIP, ZIPS, RLE, PIZ), pixel values are exactly preserved.
2. **All attributes**: All standard and custom attributes are preserved with exact values.
3. **Channel definitions**: Channel names, types, and sampling rates are preserved.

## Achieving Byte-Exact Output

For byte-exact round-trip with the same library version:

1. **Use the same compression type and level**: The header automatically preserves detected FLEVEL.
2. **Preserve all attributes**: Copy all attributes from source to destination.
3. **Use the same channel order**: ChannelList preserves insertion order.
4. **Don't modify pixel data**: Any modification (color conversion, scaling) changes output.

```go
// Complete round-trip example
func copyExact(src, dst string) error {
    // Read source
    srcFile, err := exr.Open(src)
    if err != nil {
        return err
    }
    defer srcFile.Close()

    srcHeader := srcFile.Header(0)

    // Create destination with same compression options
    dstHeader := srcHeader.Clone()  // Clone preserves CompOpts

    // Create writer
    f, _ := os.Create(dst)
    defer f.Close()

    writer, _ := exr.NewScanlineWriter(f, dstHeader)

    // Read and write pixel data
    reader := srcFile.ScanlineReader(0)
    fb := exr.NewFrameBuffer()
    // ... setup frame buffer ...

    reader.ReadPixels(fb, minY, maxY)
    writer.WritePixels(fb, minY, maxY)

    return nil
}
```

## Cross-Implementation Compatibility

When round-tripping between different OpenEXR implementations (e.g., go-openexr and C++ OpenEXR), byte-exact output is generally not achievable due to:

1. Different zlib implementations producing different compressed bytes
2. Different default compression levels
3. Potential differences in floating-point handling

However, **semantic equivalence** is always preserved:

- All pixel values decompress to identical values
- All attributes have identical values
- The file is functionally identical

## Testing Determinism

```go
func TestDeterminism(t *testing.T) {
    data := createTestData()

    var hashes []string
    for i := 0; i < 10; i++ {
        output := &bytes.Buffer{}
        writeEXR(output, data)
        hash := sha512.Sum512(output.Bytes())
        hashes = append(hashes, hex.EncodeToString(hash[:]))
    }

    for i := 1; i < len(hashes); i++ {
        if hashes[i] != hashes[0] {
            t.Errorf("Non-deterministic output: run %d differs", i)
        }
    }
}
```

## API Reference

### compression.CompressionLevel

```go
type CompressionLevel int

const (
    CompressionLevelHuffmanOnly CompressionLevel = -2
    CompressionLevelDefault     CompressionLevel = -1
    CompressionLevelNone        CompressionLevel = 0
    CompressionLevelBestSpeed   CompressionLevel = 1
    CompressionLevelBestSize    CompressionLevel = 9
)
```

### compression.FLevel

```go
type FLevel int

const (
    FLevelFastest FLevel = 0 // Levels -2, 0, 1
    FLevelFast    FLevel = 1 // Levels 2, 3, 4, 5
    FLevelDefault FLevel = 2 // Levels 6, -1
    FLevelBest    FLevel = 3 // Levels 7, 8, 9
)

func DetectZlibFLevel(data []byte) (FLevel, bool)
func FLevelToLevel(fl FLevel) CompressionLevel
```

### exr.Header Compression Methods

```go
func (h *Header) SetZIPLevel(level compression.CompressionLevel)
func (h *Header) ZIPLevel() compression.CompressionLevel
func (h *Header) SetDetectedFLevel(flevel compression.FLevel)
func (h *Header) DetectedFLevel() (compression.FLevel, bool)
func (h *Header) CompressionOptions() CompressionOptions
func (h *Header) SetCompressionOptions(opts CompressionOptions)
```

## C++ OpenEXR Recommendations

Based on the determinism issues found and fixed in go-openexr, we have prepared a proposal for similar improvements to the C++ OpenEXR reference implementation.

See: `upstream/ASWF/proposal/DETERMINISM_IMPROVEMENTS.md`

This proposal covers:

- Header attribute ordering
- Compression level detection and preservation
- ID manifest serialization
- Recommended tests and API additions
