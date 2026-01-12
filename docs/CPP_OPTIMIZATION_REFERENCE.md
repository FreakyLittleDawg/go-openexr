# C++ OpenEXR Optimization Reference

This document captures the key optimizations used in the C++ OpenEXR implementation
to guide Go performance improvements. Refer to this instead of re-reading C++ source.

## B44 Compression (internal_b44.c)

### Architecture

- **No explicit SIMD** - relies on compiler auto-vectorization
- Zero heap allocations during compression (pre-allocated scratch buffers)
- All hot functions are `static inline`

### Key Optimizations

1. **In-place processing**: Data processed directly in scratch buffers without intermediate allocations

2. **Fast-path block assembly** (lines 450-470): Uses `memcpy` for 4 pixels at a time when no edge padding needed

   ```c
   // C++ copies 4 pixels at a time for interior blocks
   memcpy(&block[row*4], &channel[y*width + x], 4 * sizeof(uint16_t));
   ```

3. **Sign-magnitude conversion** (lines 115-123): Simple loop that compiler auto-vectorizes

   ```c
   for (int i = 0; i < 16; ++i) {
       if ((s[i] & 0x7c00) == 0x7c00) t[i] = 0x8000;
       else if (s[i] & 0x8000) t[i] = ~s[i];
       else t[i] = s[i] | 0x8000;
   }
   ```

4. **unpack14()** (lines 237-320): Fully unrolled bit extraction with no dependencies between operations

### Go Implementation Gaps

| Issue                               | Impact | Fix                             |
| ----------------------------------- | ------ | ------------------------------- |
| Per-channel slice allocations       | 30-40% | Use pre-allocated buffer pool   |
| Element-by-element block assembly   | 15-20% | Fast-path for interior blocks   |
| findMaxSIMD uses scalar PEXTRW loop | 10-15% | Use PMAXUW horizontal reduction |

---

## DWA/DWAB Compression (internal*dwa*\*.h)

### Architecture

- **Heavy SIMD usage**: SSE2, SSE4.1, AVX, AVX2, F16C, NEON
- DCT uses matrix decomposition for parallelism
- Multiple specialized code paths for different CPU features

### Key Optimizations

1. **DCT Implementation** (internal_dwa_simd.h:896-1283)
   - Reformulated as even/odd component computation
   - SSE2: Processes 4 values per instruction
   - AVX: Processes 8 values per instruction
   - Matrix coefficients stored as broadcast constants

   ```c
   // M1 handles [y0, y2, y4, y6], M2 handles [y1, y3, y5, y7]
   // Result: x_i = E_i + O_i (front), x_(7-i) = E_i - O_i (back)
   ```

2. **Zig-Zag + Half Conversion** (lines 607-787)
   - Combined in single pass using F16C `VCVTPH2PS`
   - NEON uses TBL/TBX for parallel reordering

   ```c
   vcvtph2ps %%xmm8, %%ymm8  // Convert 8 halfs to 8 floats in one instruction
   ```

3. **Color Space Conversion** (lines 117-164)
   - SSE2 vectorized, processes 4 pixels at once

   ```c
   r[i] = _mm_add_ps(r[i], _mm_mul_ps(src[2], c0));  // 4 pixels
   ```

4. **Sparse Block Detection** (internal_dwa_decoder.h:412-480)
   - Detects DC-only and sparse blocks
   - Uses specialized DCT variants that skip zero rows
   ```c
   if (lastNonZero == 0) dctInverse8x8DcOnly(data);
   else if (lastNonZero < 2) dctInverse8x8_7(data);  // Skip 7 rows
   ```

### Go Implementation Gaps

| Issue                     | Impact | Fix                           |
| ------------------------- | ------ | ----------------------------- |
| Naive O(N²) DCT           | 8-16x  | SIMD matrix decomposition     |
| Per-pixel half conversion | 4-8x   | Batch F16C/NEON conversion    |
| Sequential zigzag         | 2-3x   | SIMD shuffle tables           |
| No sparse detection       | 1.5-2x | DC-only and row-skip variants |

---

## PIZ Compression (internal_huf.c, internal_piz.c)

### Architecture

- **FastHufDecoder**: Custom Huffman implementation optimized for PIZ
- Left-justified 64-bit bit processing
- Double-buffering for latency hiding

### Key Optimizations

1. **Left-Justified Bit Buffer** (lines 1103-1130)
   - Codes stored "left justified" in 64-bit integers
   - Direct comparison without shifting for each symbol

   ```c
   // Table index = buffer >> 52 (no per-symbol shift calculation)
   if (tMin <= buffer) {  // Single comparison for fast path
       int tableIdx = buffer >> INDEX_BIT_SHIFT;  // 64-12=52
       codeLen = fhd->_tableCodeLen[tableIdx];
       symbol = fhd->_tableSymbol[tableIdx];
   }
   ```

2. **Double-Buffering** (lines 1233-1311)
   - Two 64-bit buffers, pre-loads next 64 bits
   - Refill transfers bits without memory access when possible

   ```c
   buffer = READ64(src);        // First 64 bits
   bufferBack = READ64(src+8);  // Pre-loaded!
   // Refill from bufferBack without memory access
   *buffer |= (*bufferBack) >> numBits;
   ```

3. **64-bit Block Reads** (lines 1062-1068)
   - Uses `__builtin_bswap64` for single-instruction big-endian reads

   ```c
   static inline uint64_t READ64(const uint8_t* c) {
       uint64_t x;
       memcpy(&x, c, sizeof(uint64_t));
       return __builtin_bswap64(x);
   }
   ```

4. **Separate Symbol/Length Tables** (lines 1127-1128)
   - Symbols (hot) and lengths in separate arrays for cache efficiency
   ```c
   int _tableSymbol[4096];   // 16KB - frequently accessed
   uint8_t _tableCodeLen[4096];  // 4KB - less frequently accessed
   ```

### Go Implementation Gaps

| Issue                        | Impact    | Fix                           |
| ---------------------------- | --------- | ----------------------------- |
| Byte-at-a-time refill        | 1.5-2x    | 64-bit block reads            |
| No double buffering          | 1.3-1.5x  | Pre-load next 64 bits         |
| Per-symbol shift calculation | 1.2-1.3x  | Left-justified buffers        |
| Packed table entries         | 1.05-1.1x | Separate symbol/length arrays |

---

## ZIP Compression (internal_zip.c)

### Architecture

- Uses libdeflate (not standard zlib)
- Fused reconstruct operation (predictor + interleave in one pass)
- SSE4.1 prefix sum for predictor

### Key Optimizations

1. **libdeflate** (compression.c:19-33)
   - Single-shot buffer decompression (not streaming)
   - Hand-tuned SIMD for x86 and ARM
   - No dynamic allocation during decompression

2. **Fused Reconstruct** (lines 233-238)
   - Single function does predictor decode and interleave
   - In-place predictor decode, then interleave to output

   ```c
   void internal_zip_reconstruct_bytes(uint8_t* out, uint8_t* src, uint64_t count) {
       reconstruct(src, count);      // In-place predictor
       interleave(out, src, count);  // To output
   }
   ```

3. **SSE4.1 Prefix Sum** (lines 34-79)
   - Parallel prefix algorithm with carry

   ```c
   d = _mm_add_epi8(d, _mm_slli_si128(d, 1));  // Add prev 1
   d = _mm_add_epi8(d, _mm_slli_si128(d, 2));  // Add prev 2
   d = _mm_add_epi8(d, _mm_slli_si128(d, 4));  // Add prev 4
   d = _mm_add_epi8(d, _mm_slli_si128(d, 8));  // Add prev 8
   d = _mm_add_epi8(d, vPrev);                  // Add carry
   vPrev = _mm_shuffle_epi8(d, shuffleMask);   // Broadcast last byte
   ```

4. **SSE2/NEON Interleave** (lines 146-175)
   - Uses PUNPCKLBW/PUNPCKHBW for byte interleaving
   ```c
   lo = _mm_unpacklo_epi8(a, b);  // Interleave low bytes
   hi = _mm_unpackhi_epi8(a, b);  // Interleave high bytes
   ```

### Go Implementation Gaps

| Issue                            | Impact | Fix                         |
| -------------------------------- | ------ | --------------------------- |
| klauspost/compress vs libdeflate | 2-3x   | Consider cgo libdeflate     |
| Separate operations              | 20-40% | Fuse deinterleave+predictor |
| Per-call allocations             | 15-25% | Buffer reuse/pooling        |
| SSE2-only predictor              | 10-20% | Add SSE4.1 PSHUFB path      |

---

## General C++ Patterns to Adopt

1. **Pre-allocated scratch buffers**: Reuse buffers across calls with sync.Pool
2. **Fast-paths for common cases**: Check for no-edge-padding blocks, DC-only DCT, etc.
3. **Bounds check elimination**: Use `_ = slice[len-1]` hints before loops
4. **SIMD assembly**: Use platform-specific assembly for hot paths
5. **Batch operations**: Process multiple items per function call to amortize overhead
6. **Inline small functions**: Keep hot functions small for inlining

---

## Quick Reference: Expected Gains

| Codec | Current Gap | After Optimization | Key Changes                      |
| ----- | ----------- | ------------------ | -------------------------------- |
| B44   | 23-33x      | 3-5x               | Buffer pooling, fast-path blocks |
| DWA   | 21-27x      | 4-8x               | SIMD DCT, batch conversions      |
| PIZ   | 5-7x        | 2-3x               | FastHufDecoder, 64-bit reads     |
| ZIP   | 5-6x        | 1.5-2x             | Buffer reuse, fused ops          |
