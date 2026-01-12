# GPU Processing for OpenEXR Files

## Overview

This document explores the potential for GPU acceleration in OpenEXR file processing, including which operations are suitable for GPU offload and which compression formats are most amenable to parallel processing.

## The EXR Processing Pipeline

A typical EXR processing pipeline involves these stages:

```
┌─────────────┐    ┌──────────────┐    ┌─────────────┐    ┌──────────────┐
│  File I/O   │ -> │ Decompress   │ -> │ Pixel Ops   │ -> │   Output     │
│  (Disk)     │    │ (CPU/GPU?)   │    │ (GPU ideal) │    │  (Display/   │
│             │    │              │    │             │    │   Encode)    │
└─────────────┘    └──────────────┘    └─────────────┘    └──────────────┘
```

### Bottleneck Analysis

| Stage            | Typical Bottleneck | GPU Suitability             |
| ---------------- | ------------------ | --------------------------- |
| File I/O         | Disk bandwidth     | N/A (no GPU benefit)        |
| Decompression    | CPU compute        | Mixed (algorithm-dependent) |
| Pixel Operations | Compute-bound      | Excellent                   |
| Color Transforms | Compute-bound      | Excellent                   |
| Tone Mapping     | Compute-bound      | Excellent                   |
| Display/Encode   | GPU already used   | Native                      |

## Compression Algorithm GPU Suitability

### Highly GPU-Parallelizable

#### B44/B44A

- **Structure:** Fixed 4x4 block transforms
- **GPU Suitability:** Excellent
- **Why:** Each 4x4 block is completely independent, enabling embarrassingly parallel processing. The sign-magnitude to ordered conversion and block packing can be vectorized across thousands of blocks simultaneously.
- **Potential Speedup:** 10-50x on modern GPUs

#### DWA/DWAB (DCT-based)

- **Structure:** Discrete Cosine Transform on 8x8 blocks
- **GPU Suitability:** Excellent
- **Why:** DCT is the same transform used in JPEG, which GPUs have been optimized for over decades. cuDCT and similar libraries achieve very high throughput.
- **Potential Speedup:** 20-100x on modern GPUs

#### HTJ2K (High-Throughput JPEG 2000)

- **Structure:** Block-based wavelet transform designed for parallelism
- **GPU Suitability:** Excellent (purpose-built for this)
- **Why:** HTJ2K was specifically designed with GPU acceleration in mind. NVIDIA's nvJPEG2000 library achieves multi-GB/s throughput.
- **Potential Speedup:** 50-200x on modern GPUs
- **Note:** This is the recommended format for throughput-critical pipelines

### Moderately GPU-Parallelizable

#### PIZ (Wavelet + Huffman)

- **Structure:** Wavelet transform followed by Huffman coding
- **GPU Suitability:** Moderate
- **Why:** The wavelet transform is GPU-friendly, but Huffman decoding has sequential dependencies. Modern approaches use table-based parallel Huffman decoders.
- **Potential Speedup:** 5-15x on modern GPUs

#### PXR24 (Predictor + ZIP)

- **Structure:** Differential predictor followed by zlib
- **GPU Suitability:** Moderate
- **Why:** The predictor stage has some sequential dependencies. DEFLATE decompression can be parallelized at the block level but not within blocks.
- **Potential Speedup:** 3-8x on modern GPUs

### Difficult to GPU-Parallelize

#### ZIP/ZIPS

- **Structure:** Standard DEFLATE compression
- **GPU Suitability:** Limited
- **Why:** DEFLATE has inherent sequential dependencies in both LZ77 matching and Huffman decoding. GPU implementations exist but don't achieve dramatic speedups.
- **Potential Speedup:** 2-4x on modern GPUs

#### RLE (Run-Length Encoding)

- **Structure:** Sequential run detection
- **GPU Suitability:** Poor for decode, moderate for encode
- **Why:** Decoding requires knowing where each run ends to process the next, creating serial dependencies. Encoding can use parallel prefix sums.
- **Potential Speedup:** 1-3x on modern GPUs

## Apple Silicon Unified Memory Advantage

Apple Silicon's unified memory architecture offers unique advantages for GPU-accelerated EXR processing:

### Traditional Architecture

```
┌──────────┐         ┌──────────┐
│   CPU    │ <-PCIe->│   GPU    │
│  Memory  │         │  Memory  │
│  (DDR5)  │         │  (GDDR6) │
└──────────┘         └──────────┘
     Data must be copied both ways
     PCIe bandwidth: ~32 GB/s (Gen4 x16)
```

### Apple Silicon Unified Memory

```
┌─────────────────────────────────┐
│      Unified Memory Pool        │
│         (LPDDR5X)               │
│   CPU and GPU share same RAM    │
│   Bandwidth: 400+ GB/s (M3 Max) │
└─────────────────────────────────┘
     Zero-copy data sharing
```

### Implications for EXR Processing

1. **Zero-Copy Pipeline:** Data can flow from disk -> CPU decompression -> GPU pixel ops -> display without memory copies
2. **Fine-Grained Offload:** Even small operations become worth offloading since there's no copy overhead
3. **Hybrid Processing:** CPU and GPU can work on the same buffer simultaneously for different stages

## Recommended Pipeline Architectures

### High-Throughput Ingest Pipeline

```
Disk -> CPU Parse Headers -> GPU Decompress (HTJ2K/B44) -> GPU Color Transform -> Display
```

### Real-Time Playback Pipeline

```
SSD RAID -> Memory Map -> GPU Direct Load (uncompressed) -> GPU Display
```

_For real-time playback, uncompressed or lightly compressed (RLE) formats avoid the decompression bottleneck entirely._

### Batch Processing Pipeline

```
Disk -> Parallel CPU Decompress -> GPU Pixel Operations -> Parallel CPU Compress -> Disk
```

## Metal Compute Shader Considerations

For implementing EXR processing on Apple Silicon with Metal:

### Good Candidates for Metal Compute

- Half-float to float conversion (trivial SIMD)
- Color space transforms (matrix multiply)
- Tone mapping (per-pixel operations)
- B44 block decode/encode
- DCT for DWA compression

### Implementation Notes

```metal
// Example: Half-to-float conversion kernel
kernel void halfToFloat(
    device const half* input [[buffer(0)]],
    device float* output [[buffer(1)]],
    uint id [[thread_position_in_grid]]
) {
    output[id] = float(input[id]);
}
```

### Memory Binding

- Use `MTLResourceStorageModeShared` for zero-copy CPU/GPU access
- Align buffers to page boundaries (16KB on Apple Silicon)
- Use triple-buffering for streaming pipelines

## Practical Recommendations

### For Maximum Throughput

1. Use **HTJ2K** compression with nvJPEG2000 (NVIDIA) or VideoToolbox (Apple)
2. Process multiple frames in parallel
3. Keep data on GPU for entire pixel processing pipeline

### For Balanced Performance

1. Use **B44** or **DWAB** compression
2. Decompress on CPU, pixel ops on GPU
3. Leverage unified memory on Apple Silicon

### For Compatibility

1. Use **ZIP** compression (universal support)
2. Process on CPU
3. GPU only for display/tone mapping

## Future Directions

1. **Hardware Decoders:** Future GPUs may include dedicated EXR/HTJ2K decode units (similar to video decode)
2. **DirectStorage/Metal 3:** Direct GPU loading from NVMe without CPU involvement
3. **Compression Coprocessors:** Dedicated silicon for entropy coding (already appearing in enterprise SSDs)

## References

- [NVIDIA nvJPEG2000](https://developer.nvidia.com/nvjpeg)
- [Apple Metal Best Practices](https://developer.apple.com/documentation/metal)
- [OpenEXR Technical Introduction](https://openexr.com/en/latest/TechnicalIntroduction.html)
- [HTJ2K Overview](https://htj2k.com/)
