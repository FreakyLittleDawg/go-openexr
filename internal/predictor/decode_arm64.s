//go:build arm64

#include "textflag.h"

// func decodeASM(data []byte)
// Performs predictor decode (prefix sum) using ARM NEON SIMD.
// Each byte becomes the sum of itself and all previous bytes.
TEXT ·decodeASM(SB), NOSPLIT, $0-24
    MOVD data_base+0(FP), R0    // R0 = &data[0]
    MOVD data_len+8(FP), R1     // R1 = len(data)

    CMP  $2, R1
    BLT  done                   // if len < 2, nothing to do

    CMP  $16, R1
    BLT  scalar                 // if len < 16, use scalar path

    // Initialize carry register to zero
    VEOR V0.B16, V0.B16, V0.B16 // V0 = carry (all zeros)
    VEOR V3.B16, V3.B16, V3.B16 // V3 = zero for VEXT

    // Calculate number of 16-byte chunks
    LSR  $4, R1, R2             // R2 = len / 16

    CBZ  R2, remainder

simd_loop:
    // Load 16 bytes
    VLD1 (R0), [V1.B16]         // V1 = data[i:i+16]

    // Parallel prefix sum within 16 bytes using VEXT and VADD
    // VEXT shifts right by (16-n) bytes = shifts left by n bytes

    // Step 1: Add with 1-byte left shift
    VEXT $15, V1.B16, V3.B16, V2.B16  // V2 = V1 shifted left by 1
    VADD V2.B16, V1.B16, V1.B16       // V1 += shifted

    // Step 2: Add with 2-byte left shift
    VEXT $14, V1.B16, V3.B16, V2.B16  // V2 = V1 shifted left by 2
    VADD V2.B16, V1.B16, V1.B16

    // Step 3: Add with 4-byte left shift
    VEXT $12, V1.B16, V3.B16, V2.B16  // V2 = V1 shifted left by 4
    VADD V2.B16, V1.B16, V1.B16

    // Step 4: Add with 8-byte left shift
    VEXT $8, V1.B16, V3.B16, V2.B16   // V2 = V1 shifted left by 8
    VADD V2.B16, V1.B16, V1.B16

    // Add carry from previous chunk
    VADD V0.B16, V1.B16, V1.B16

    // Store result
    VST1 [V1.B16], (R0)

    // Extract last byte and broadcast as new carry
    // Use VDUP to broadcast lane 15 to all lanes
    VDUP V1.B[15], V0.B16

    ADD  $16, R0
    SUB  $1, R2
    CBNZ R2, simd_loop

remainder:
    // Handle remaining bytes (< 16)
    AND  $15, R1, R2            // R2 = len % 16
    CBZ  R2, done

    // Get carry byte from V0 (all lanes have same value)
    VMOV V0.B[0], R3            // R3 = carry byte

remainder_loop:
    MOVBU (R0), R4
    ADD  R3, R4, R4
    MOVB R4, (R0)
    MOVD R4, R3                 // Update carry
    ADD  $1, R0
    SUB  $1, R2
    CBNZ R2, remainder_loop
    B    done

scalar:
    // Scalar path for small arrays (len < 16)
    MOVBU (R0), R3              // R3 = data[0] (carry)
    ADD  $1, R0
    SUB  $1, R1

scalar_loop:
    CBZ  R1, done
    MOVBU (R0), R4
    ADD  R3, R4, R4
    MOVB R4, (R0)
    MOVD R4, R3
    ADD  $1, R0
    SUB  $1, R1
    B    scalar_loop

done:
    RET
