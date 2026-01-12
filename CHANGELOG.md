# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [1.0.1] - 2026-01-11

### Fixed

- Corrected author name in NOTICE file

## [1.0.0] - 2026-01-11

### Added

- Pure Go implementation of OpenEXR file format (version 2.x)
- Complete format support: scanline, tiled, multipart, deep data
- All compression methods: None, RLE, ZIP, ZIPS, PIZ, PXR24, B44, B44A, DWAA, DWAB
- HTJ2K compression support via go-jpeg2000
- IEEE-754 half-precision float type with SIMD batch operations
- Multi-view stereo and environment map support
- Mipmap and ripmap tiled images
- Cryptomatte ID manifest parsing (exrid package)
- ACES color workflow utilities
- Zero-copy memory-mapped file reading
- SIMD-optimized paths for ARM64 NEON and AMD64 SSE2
- Parallel chunk processing with configurable grain size
- Command-line tools: exrinfo, exrheader, exrcheck, exrmaketiled, and more
- CODE_OF_CONDUCT.md, CONTRIBUTING.md, SECURITY.md documentation

### Performance

- ARM64 NEON SIMD for B44 compression primitives
- AMD64 SSE2 vectorized shift operations
- Parallel scanline/tile compression and decompression
- Object pooling for reduced allocations
- Optimized predictor and interleave operations
