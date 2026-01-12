//go:build amd64

#include "textflag.h"

// func shiftRoundSIMD(d *[16]uint16, t *[16]uint16, tMax uint16, shift uint)
// Computes d[i] = shiftAndRound(tMax - t[i], shift) for 16 values using SSE2.
//
// The algorithm for each value:
//   diff = tMax - t[i]
//   x = diff << 1
//   a = (1 << shift) - 1
//   shiftP1 = shift + 1
//   roundBit = (x >> shiftP1) & 1
//   d[i] = (x + a + roundBit) >> shiftP1
//
// SSE2 supports uniform shifts from a register: PSRLW xmm, xmm (count from low 64 bits)
TEXT ·shiftRoundSIMD(SB), NOSPLIT, $0-32
    MOVQ d+0(FP), DI            // DI = d pointer
    MOVQ t+8(FP), SI            // SI = t pointer
    MOVQ tMax+16(FP), AX        // AX = tMax
    MOVQ shift+24(FP), CX       // CX = shift

    // Broadcast tMax to all 16-bit lanes of X0
    MOVQ AX, X0                 // Move tMax to low 64 bits of X0
    PSHUFLW $0, X0, X0          // Broadcast to low 4 words
    PSHUFD $0, X0, X0           // Broadcast to all 8 words
    MOVO X0, X8                 // X8 = tMax broadcast (save for second batch)

    // Compute a = (1 << shift) - 1 and broadcast
    MOVQ $1, AX
    SHLQ CX, AX                 // AX = 1 << shift
    DECQ AX                     // AX = (1 << shift) - 1
    MOVQ AX, X1
    PSHUFLW $0, X1, X1
    PSHUFD $0, X1, X1           // X1 = 'a' broadcast
    MOVO X1, X9                 // X9 = 'a' broadcast (save for second batch)

    // Compute shiftP1 = shift + 1 for PSRLW
    ADDQ $1, CX                 // CX = shiftP1
    MOVQ CX, X2                 // X2 low bits = shiftP1 (for PSRLW)
    MOVO X2, X10                // X10 = shiftP1 (save for second batch)

    // Constant 1 for AND with roundBit
    MOVQ $0x0001000100010001, AX
    MOVQ AX, X3
    PSHUFD $0x44, X3, X3        // X3 = 1 broadcast to all words
    MOVO X3, X11                // X11 = 1 broadcast (save)

    // ============ Process first 8 values ============
    MOVOU 0(SI), X4             // X4 = t[0:7]

    // diff = tMax - t[i]
    MOVO X8, X5                 // X5 = tMax
    PSUBW X4, X5                // X5 = tMax - t[0:7] = diff

    // x = diff << 1 (via add to self)
    PADDW X5, X5                // X5 = diff * 2 = x

    // roundBit = (x >> shiftP1) & 1
    MOVO X5, X6                 // X6 = x
    PSRLW X10, X6               // X6 = x >> shiftP1 (uniform shift from register)
    PAND X11, X6                // X6 = roundBit

    // d = (x + a + roundBit) >> shiftP1
    PADDW X9, X5                // X5 = x + a
    PADDW X6, X5                // X5 = x + a + roundBit
    PSRLW X10, X5               // X5 = d[0:7]

    MOVOU X5, 0(DI)             // Store d[0:7]

    // ============ Process second 8 values ============
    MOVOU 16(SI), X4            // X4 = t[8:15]

    // diff = tMax - t[i]
    MOVO X8, X5                 // X5 = tMax
    PSUBW X4, X5                // X5 = tMax - t[8:15] = diff

    // x = diff << 1
    PADDW X5, X5                // X5 = x

    // roundBit = (x >> shiftP1) & 1
    MOVO X5, X6                 // X6 = x
    PSRLW X10, X6               // X6 = x >> shiftP1
    PAND X11, X6                // X6 = roundBit

    // d = (x + a + roundBit) >> shiftP1
    PADDW X9, X5                // X5 = x + a
    PADDW X6, X5                // X5 = x + a + roundBit
    PSRLW X10, X5               // X5 = d[8:15]

    MOVOU X5, 16(DI)            // Store d[8:15]

    RET
