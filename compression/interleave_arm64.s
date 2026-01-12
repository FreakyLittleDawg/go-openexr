//go:build arm64

#include "textflag.h"

// func deinterleaveASM(dst, src []byte)
// Deinterleaves bytes from split format to interleaved format.
// Input:  [a0,a1,a2,...,a(n/2-1) | b0,b1,b2,...,b(n/2-1)]
// Output: [a0,b0,a1,b1,a2,b2,...]
// Uses ARM NEON VZIP for efficient byte interleaving.
TEXT ·deinterleaveASM(SB), NOSPLIT, $0-48
    MOVD dst_base+0(FP), R0     // R0 = &dst[0]
    MOVD dst_len+8(FP), R1      // R1 = len(dst)
    MOVD src_base+24(FP), R2    // R2 = &src[0]
    MOVD src_len+32(FP), R3     // R3 = len(src)

    // Calculate half point
    ADD  $1, R3, R4
    LSR  $1, R4                 // R4 = (len+1)/2 = half

    MOVD R2, R5                 // R5 = &src[0] (first half)
    ADD  R4, R2, R6             // R6 = &src[half] (second half)

    // Calculate number of 16-byte chunks
    LSR  $4, R4, R7             // R7 = half / 16

    CBZ  R7, deint_remainder

deint_simd_loop:
    // Load 16 bytes from each half
    VLD1 (R5), [V0.B16]         // V0 = first half
    VLD1 (R6), [V1.B16]         // V1 = second half

    // Interleave using VZIP1 and VZIP2
    VZIP1 V1.B16, V0.B16, V2.B16  // V2 = interleaved low 16 bytes
    VZIP2 V1.B16, V0.B16, V3.B16  // V3 = interleaved high 16 bytes

    // Store 32 bytes of output
    VST1 [V2.B16, V3.B16], (R0)

    ADD  $16, R5
    ADD  $16, R6
    ADD  $32, R0
    SUB  $1, R7
    CBNZ R7, deint_simd_loop

deint_remainder:
    // Handle remaining bytes
    AND  $15, R4, R7            // R7 = half % 16
    CBZ  R7, deint_done

deint_remainder_loop:
    MOVBU (R5), R8
    MOVB  R8, (R0)
    ADD   $1, R0
    ADD   $1, R5

    // Check if second half has more bytes
    SUB   R2, R6, R8            // Current offset in src
    CMP   R3, R8
    BGE   deint_skip

    MOVBU (R6), R8
    MOVB  R8, (R0)
    ADD   $1, R0
    ADD   $1, R6

deint_skip:
    SUB   $1, R7
    CBNZ  R7, deint_remainder_loop

deint_done:
    RET


// func interleaveASM(dst, src []byte)
// Interleaves bytes from interleaved format to split format.
// Input:  [a0,b0,a1,b1,a2,b2,...]
// Output: [a0,a1,a2,...,a(n/2-1) | b0,b1,b2,...,b(n/2-1)]
// Uses ARM NEON VUZP for efficient byte de-interleaving.
TEXT ·interleaveASM(SB), NOSPLIT, $0-48
    MOVD dst_base+0(FP), R0     // R0 = &dst[0]
    MOVD dst_len+8(FP), R1      // R1 = len(dst)
    MOVD src_base+24(FP), R2    // R2 = &src[0]
    MOVD src_len+32(FP), R3     // R3 = len(src)

    // Calculate half point for output
    ADD  $1, R3, R4
    LSR  $1, R4                 // R4 = (len+1)/2 = half

    MOVD R0, R5                 // R5 = &dst[0] (first half output)
    ADD  R4, R0, R6             // R6 = &dst[half] (second half output)

    // Calculate number of 32-byte input chunks
    LSR  $5, R3, R7             // R7 = len / 32

    CBZ  R7, int_remainder

int_simd_loop:
    // Load 32 bytes of interleaved input
    VLD1.P 32(R2), [V0.B16, V1.B16]

    // De-interleave using VUZP1 and VUZP2
    // VUZP1 extracts even elements, VUZP2 extracts odd elements
    VUZP1 V1.B16, V0.B16, V2.B16  // V2 = even bytes (indices 0,2,4,...)
    VUZP2 V1.B16, V0.B16, V3.B16  // V3 = odd bytes (indices 1,3,5,...)

    // Store to split format
    VST1 [V2.B16], (R5)
    VST1 [V3.B16], (R6)

    ADD  $16, R5
    ADD  $16, R6
    SUB  $1, R7
    CBNZ R7, int_simd_loop

int_remainder:
    // Handle remaining bytes
    AND  $31, R3, R7            // R7 = len % 32
    CBZ  R7, int_done

int_remainder_loop:
    CBZ  R7, int_done

    // Even byte
    MOVBU (R2), R8
    MOVB  R8, (R5)
    ADD   $1, R5
    ADD   $1, R2
    SUB   $1, R7

    CBZ  R7, int_done

    // Odd byte
    MOVBU (R2), R8
    MOVB  R8, (R6)
    ADD   $1, R6
    ADD   $1, R2
    SUB   $1, R7
    B    int_remainder_loop

int_done:
    RET
