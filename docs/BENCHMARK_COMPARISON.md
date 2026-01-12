# Go vs C++ OpenEXR Performance Comparison

**Test Date:** January 2026
**Test File:** Flowers.exr (multi-channel HDR image)
**Platform:** Apple M3 Max (ARM64)
**Passes:** 10 (averaged)

## Write Performance (Compression)

| Compression      | Go (ms)  | C++ (ms) | Ratio     | Notes              |
| ---------------- | -------- | -------- | --------- | ------------------ |
| none (half)      | 0.44     | 2.55     | **0.17x** | Go 5.8x faster     |
| none (float)     | 0.45     | 2.69     | **0.17x** | Go 6.0x faster     |
| rle (half)       | 2.63     | 2.42     | 1.09x     | Near parity        |
| rle (float)      | 2.87     | 2.94     | **0.98x** | Go slightly faster |
| zips (half)      | 5.87     | 3.32     | 1.77x     |                    |
| zips (float)     | 9.07     | 3.91     | 2.32x     |                    |
| zip (half)       | 3.22     | 1.79     | 1.80x     |                    |
| zip (float)      | 7.20     | 2.58     | 2.79x     |                    |
| piz (half)       | 5.95     | 1.35     | 4.41x     |                    |
| piz (float)      | 6.86     | 2.11     | 3.25x     |                    |
| pxr24 (half)     | 2.96     | 1.73     | 1.71x     |                    |
| pxr24 (float)    | 3.79     | 1.76     | 2.15x     |                    |
| **b44 (half)**   | **1.50** | **0.32** | **4.72x** | SIMD-optimized     |
| **b44 (float)**  | **0.71** | **0.23** | **3.15x** | SIMD-optimized     |
| **b44a (half)**  | **1.61** | **0.34** | **4.77x** | SIMD-optimized     |
| **b44a (float)** | **0.80** | **0.25** | **3.22x** | SIMD-optimized     |
| dwaa (half)      | 3.79     | 3.79     | **1.00x** | Parity             |
| dwaa (float)     | 5.38     | 3.60     | 1.49x     |                    |
| dwab (half)      | 25.84    | 7.09     | 3.64x     |                    |
| dwab (float)     | 31.70    | 7.34     | 4.32x     |                    |

## Read Performance (Decompression)

| Compression      | Go (ms)  | C++ (ms) | Ratio     | Notes          |
| ---------------- | -------- | -------- | --------- | -------------- |
| none (half)      | 0.43     | 2.65     | **0.16x** | Go 6.2x faster |
| none (float)     | 0.63     | 3.18     | **0.20x** | Go 5.0x faster |
| rle (half)       | 1.09     | 2.59     | **0.42x** | Go 2.4x faster |
| rle (float)      | 1.31     | 3.11     | **0.42x** | Go 2.4x faster |
| zips (half)      | 3.21     | 2.50     | 1.29x     |                |
| zips (float)     | 4.19     | 2.90     | 1.44x     |                |
| zip (half)       | 1.58     | 0.58     | 2.73x     |                |
| zip (float)      | 3.66     | 0.74     | 4.92x     |                |
| piz (half)       | 2.24     | 0.57     | 3.95x     |                |
| piz (float)      | 3.59     | 1.03     | 3.49x     |                |
| pxr24 (half)     | 1.65     | 0.55     | 3.00x     |                |
| pxr24 (float)    | 2.74     | 0.52     | 5.28x     |                |
| **b44 (half)**   | **1.24** | **0.27** | **4.56x** |                |
| **b44 (float)**  | **1.61** | **0.28** | **5.64x** |                |
| **b44a (half)**  | **1.20** | **0.24** | **4.96x** |                |
| **b44a (float)** | **1.12** | **0.29** | **3.86x** |                |
| dwaa (half)      | 1.55     | 0.68     | 2.28x     |                |
| dwaa (float)     | 3.30     | 0.73     | 4.52x     |                |
| dwab (half)      | 10.23    | 1.52     | 6.74x     |                |
| dwab (float)     | 15.68    | 1.83     | 8.59x     |                |

## Summary by Category

### Go is Faster (Ratio < 1.0x)

| Operation          | Best Ratio | Why                                           |
| ------------------ | ---------- | --------------------------------------------- |
| Uncompressed write | 0.17x      | Direct memory copy, Go memory model efficient |
| Uncompressed read  | 0.16x      | Direct memory copy                            |
| RLE write          | 0.98x      | Simple algorithm, near parity                 |
| RLE read           | 0.42x      | Go implementation is efficient                |

### Near Parity (1.0x - 2.0x)

| Operation         | Ratio | Notes                        |
| ----------------- | ----- | ---------------------------- |
| DWAA write (half) | 1.00x | DCT-based, complex algorithm |
| ZIPS write        | 1.77x | zlib overhead                |
| ZIP write         | 1.80x | zlib overhead                |
| PXR24 write       | 1.71x |                              |

### C++ is Significantly Faster (> 3.0x)

| Operation  | Ratio | Reason                          |
| ---------- | ----- | ------------------------------- |
| B44 write  | 3-5x  | Inner loop not fully vectorized |
| PIZ write  | 3-4x  | Complex wavelet + Huffman       |
| DWAB write | 3-4x  | Block-based DCT processing      |
| ZIP read   | 3-5x  | C zlib vs Go implementation     |

## B44 SIMD Optimization Impact

### Before SIMD (estimated from pure Go)

- B44 write: ~6x slower than C++
- B44 read: ~6x slower than C++

### After ARM64 NEON SIMD

- B44 write: 3-5x slower than C++
- B44 read: 4-6x slower than C++

**Improvement:** ~30-50% speedup from SIMD helper functions

### Remaining Gap Analysis

The remaining performance gap in B44 comes from:

1. **Inner pack loop (data-dependent shifts):** The shift amounts vary per-block based on the data range, preventing simple vectorization
2. **Memory access patterns:** C++ uses more cache-friendly layouts
3. **Compiler optimizations:** C++ benefits from more aggressive loop unrolling

## Recommendations

### For Go Applications

| Use Case                 | Recommended Compression |
| ------------------------ | ----------------------- |
| Maximum speed            | `none` or `rle`         |
| Good compression + speed | `dwaa` or `zip`         |
| Maximum compression      | `piz` or `zips`         |
| Lossy with good ratio    | `b44a`                  |

### When C++ Performance is Critical

If you need C++-level performance, consider:

1. Using CGO bindings to OpenEXR C++ library
2. Processing in batches with parallel workers
3. Using `none` compression for intermediate files

## Test Configuration

```
Platform: darwin/arm64
CPU: Apple M3 Max
Go Version: 1.25.5
C++ OpenEXR: 3.3.x (upstream build)
Test File: Flowers.exr
Passes: 10 (averaged)
Mode: Multithreaded (-m flag)
```
