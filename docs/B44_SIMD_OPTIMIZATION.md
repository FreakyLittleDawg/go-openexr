# B44 SIMD Optimization Plan

## Objective

Implement high-performance SIMD optimizations for B44 compression across all major instruction sets to achieve near-C++ performance.

## Current State (Before Optimization)

| Architecture | B44 Status            | Performance vs C++ |
| ------------ | --------------------- | ------------------ |
| AMD64 SSE2   | Partial (3 functions) | ~6x slower         |
| AMD64 AVX2   | None                  | N/A                |
| ARM64 NEON   | **None** (pure Go)    | ~6x slower         |
| Generic      | Pure Go fallback      | Baseline           |

## Target Instruction Sets

### 1. ARM64 NEON (Priority: HIGH)

- Apple Silicon, AWS Graviton, ARM servers
- 128-bit vectors (8x uint16 or 16x uint8)
- Most impactful since current ARM64 has no SIMD

### 2. AMD64 AVX2 (Priority: MEDIUM)

- Modern Intel/AMD processors (2013+)
- 256-bit vectors (16x uint16)
- 2x throughput over existing SSE2

### 3. AMD64 SSE2 (Priority: LOW)

- Already implemented for core functions
- Expand to cover more of packB44()

---

## Implementation Phases

### Phase 1: ARM64 NEON Assembly

**Files:**

- `compression/b44_arm64.s` (new)
- `compression/b44_arm64.go` (convert to assembly declarations)

**Functions:**

| Function          | Status | Description                                      |
| ----------------- | ------ | ------------------------------------------------ |
| `toOrderedSIMD`   | Done   | Convert sign-magnitude to ordered representation |
| `findMaxSIMD`     | Done   | Horizontal maximum reduction                     |
| `fromOrderedSIMD` | Done   | Inverse conversion                               |

### Phase 2: AMD64 AVX2 Assembly

**Files:**

- `compression/b44_amd64_avx2.s` (new)
- `compression/b44_amd64_avx2.go` (new - declarations + CPU detection)

**Functions:**

| Function          | Status  | Description           |
| ----------------- | ------- | --------------------- |
| `hasAVX2`         | Pending | CPU feature detection |
| `toOrderedAVX2`   | Pending | 256-bit version       |
| `findMaxAVX2`     | Pending | 256-bit version       |
| `fromOrderedAVX2` | Pending | 256-bit version       |

### Phase 3: packB44 Optimizations

**Target:** Inner shift-and-round loop (data-dependent shifts)

| Optimization                  | Status  | Description           |
| ----------------------------- | ------- | --------------------- |
| Shift-and-round vectorization | Pending | ARM64 + AVX2          |
| Running differences           | Pending | Partial vectorization |

### Phase 4: Testing & Benchmarking

| Test                      | Status  | Description             |
| ------------------------- | ------- | ----------------------- |
| SIMD correctness tests    | Pending | All platforms           |
| Performance benchmarks    | Pending | Compare to C++          |
| Windows/AMD64 validation  | Pending | Via windows-test-runner |
| Cross-platform round-trip | Pending | Identical output        |

---

## File Structure

```
compression/
├── b44.go                  # Core algorithm (unchanged)
├── b44_amd64.go            # SSE2 declarations (existing)
├── b44_amd64.s             # SSE2 assembly (existing)
├── b44_amd64_avx2.go       # AVX2 declarations + CPU detection (new)
├── b44_amd64_avx2.s        # AVX2 assembly (new)
├── b44_arm64.go            # NEON declarations (modified)
├── b44_arm64.s             # NEON assembly (new)
├── b44_generic.go          # Pure Go fallback (existing)
├── b44_simd_test.go        # SIMD correctness tests (new)
└── b44_bench_test.go       # Performance benchmarks (new)
```

---

## Expected Performance

| Platform              | Before     | After Phase 1     | After Phase 2     |
| --------------------- | ---------- | ----------------- | ----------------- |
| ARM64 (Apple Silicon) | 6x slower  | **1.5-2x slower** | 1.5-2x slower     |
| AMD64 AVX2            | 6x slower  | 6x slower         | **1.5-2x slower** |
| AMD64 SSE2-only       | ~3x slower | ~3x slower        | ~3x slower        |

---

## Verification

### Local (ARM64 macOS)

```bash
go test ./compression/... -v -run B44
go test ./compression/... -bench=B44 -count=5
./exrmetrics -m --bench --csv --passes 5 -z b44,b44a exr/testdata/Flowers.exr
```

### Windows/AMD64

- Use windows-test-runner agent
- Run full test suite
- Verify SSE2/AVX2 code paths

### Cross-Platform

- Same input produces same compressed output
- Round-trip tests pass on all platforms
