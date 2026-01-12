//go:build amd64

#include "textflag.h"

// func deinterleaveASM(dst, src []byte)
// Deinterleaves bytes from split format to interleaved format.
// Input:  [a0,a1,a2,...,a(n/2-1) | b0,b1,b2,...,b(n/2-1)]
// Output: [a0,b0,a1,b1,a2,b2,...]
// Uses SSE2 PUNPCKLBW/PUNPCKHBW for efficient byte interleaving.
TEXT ·deinterleaveASM(SB), NOSPLIT, $0-48
    MOVQ dst_base+0(FP), DI     // DI = &dst[0]
    MOVQ dst_len+8(FP), R8      // R8 = len(dst)
    MOVQ src_base+24(FP), SI    // SI = &src[0]
    MOVQ src_len+32(FP), CX     // CX = len(src)

    // Calculate half point
    MOVQ CX, DX
    ADDQ $1, DX
    SHRQ $1, DX                 // DX = (len+1)/2 = half

    MOVQ SI, R9                 // R9 = &src[0] (first half)
    MOVQ SI, R10
    ADDQ DX, R10                // R10 = &src[half] (second half)

    // Calculate number of 16-byte chunks (16 bytes from each half = 32 output bytes)
    MOVQ DX, R11
    SHRQ $4, R11                // R11 = half / 16

    CMPQ R11, $0
    JE   deint_remainder

deint_simd_loop:
    // Load 16 bytes from first half
    MOVOU (R9), X0              // X0 = src[0:16]
    // Load 16 bytes from second half
    MOVOU (R10), X1             // X1 = src[half:half+16]

    // Interleave low 8 bytes from each: a0,b0,a1,b1,a2,b2,a3,b3,a4,b4,a5,b5,a6,b6,a7,b7
    MOVOU X0, X2
    PUNPCKLBW X1, X2            // X2 = interleaved low bytes

    // Interleave high 8 bytes from each: a8,b8,a9,b9,...,a15,b15
    MOVOU X0, X3
    PUNPCKHBW X1, X3            // X3 = interleaved high bytes

    // Store 32 bytes of output
    MOVOU X2, (DI)
    MOVOU X3, 16(DI)

    ADDQ $16, R9
    ADDQ $16, R10
    ADDQ $32, DI
    DECQ R11
    JNZ  deint_simd_loop

deint_remainder:
    // Handle remaining bytes
    MOVQ DX, R11
    ANDQ $15, R11               // R11 = half % 16
    CMPQ R11, $0
    JE   deint_done

deint_remainder_loop:
    // Interleave one pair at a time
    MOVB (R9), AL
    MOVB AL, (DI)
    INCQ DI
    INCQ R9

    // Check if we have a second byte to pair
    // R10 points into src at offset (half + processed), check if < len
    MOVQ R10, R12
    SUBQ SI, R12                // R12 = current offset in src
    CMPQ R12, CX                // Compare with total length
    JGE  deint_skip_odd

    MOVB (R10), AL
    MOVB AL, (DI)
    INCQ DI
    INCQ R10

deint_skip_odd:
    DECQ R11
    JNZ  deint_remainder_loop

deint_done:
    RET


// func interleaveASM(dst, src []byte)
// Interleaves bytes from interleaved format to split format.
// Input:  [a0,b0,a1,b1,a2,b2,...]
// Output: [a0,a1,a2,...,a(n/2-1) | b0,b1,b2,...,b(n/2-1)]
// Uses SSE2 PSHUFB for efficient byte extraction.
TEXT ·interleaveASM(SB), NOSPLIT, $0-48
    MOVQ dst_base+0(FP), DI     // DI = &dst[0]
    MOVQ dst_len+8(FP), R8      // R8 = len(dst)
    MOVQ src_base+24(FP), SI    // SI = &src[0]
    MOVQ src_len+32(FP), CX     // CX = len(src)

    // Calculate half point for output
    MOVQ CX, DX
    ADDQ $1, DX
    SHRQ $1, DX                 // DX = (len+1)/2 = half

    MOVQ DI, R9                 // R9 = &dst[0] (first half output)
    MOVQ DI, R10
    ADDQ DX, R10                // R10 = &dst[half] (second half output)

    // Calculate number of 16-byte input chunks (32 input bytes -> 16+16 output)
    MOVQ CX, R11
    SHRQ $5, R11                // R11 = len / 32

    CMPQ R11, $0
    JE   int_remainder

    // Load shuffle masks for extracting even/odd bytes
    // Even bytes: 0,2,4,6,8,10,12,14,128,128,128,128,128,128,128,128
    // Odd bytes:  1,3,5,7,9,11,13,15,128,128,128,128,128,128,128,128
    // 128 = 0x80 means zero that byte
    MOVQ $0x0E0C0A0806040200, AX
    MOVQ AX, X4
    MOVQ $0x8080808080808080, AX
    PINSRQ $1, AX, X4           // X4 = even mask for low 16 bytes

    MOVQ $0x0F0D0B0907050301, AX
    MOVQ AX, X5
    MOVQ $0x8080808080808080, AX
    PINSRQ $1, AX, X5           // X5 = odd mask for low 16 bytes

    // Mask for high bytes (add 0 to low, keep high)
    MOVQ $0x8080808080808080, AX
    MOVQ AX, X6
    MOVQ $0x0E0C0A0806040200, AX
    PINSRQ $1, AX, X6           // X6 = even mask for high 16 bytes

    MOVQ $0x8080808080808080, AX
    MOVQ AX, X7
    MOVQ $0x0F0D0B0907050301, AX
    PINSRQ $1, AX, X7           // X7 = odd mask for high 16 bytes

int_simd_loop:
    // Load 32 bytes of interleaved input
    MOVOU (SI), X0              // X0 = src[0:16]
    MOVOU 16(SI), X1            // X1 = src[16:32]

    // Extract even bytes from both halves
    MOVOU X0, X2
    PSHUFB X4, X2               // X2 low = even bytes from X0
    MOVOU X1, X3
    PSHUFB X6, X3               // X3 high = even bytes from X1
    POR X3, X2                  // X2 = all 16 even bytes

    // Extract odd bytes from both halves
    PSHUFB X5, X0               // X0 low = odd bytes from original X0
    PSHUFB X7, X1               // X1 high = odd bytes from X1
    POR X1, X0                  // X0 = all 16 odd bytes

    // Store to split format
    MOVOU X2, (R9)              // First half (even bytes)
    MOVOU X0, (R10)             // Second half (odd bytes)

    ADDQ $32, SI
    ADDQ $16, R9
    ADDQ $16, R10
    DECQ R11
    JNZ  int_simd_loop

int_remainder:
    // Handle remaining bytes
    MOVQ CX, R11
    ANDQ $31, R11               // R11 = len % 32
    CMPQ R11, $0
    JE   int_done

    MOVQ $0, R12                // R12 = output index

int_remainder_loop:
    CMPQ R11, $0
    JE   int_done

    // Even byte
    MOVB (SI), AL
    MOVB AL, (R9)
    INCQ R9
    INCQ SI
    DECQ R11

    CMPQ R11, $0
    JE   int_done

    // Odd byte
    MOVB (SI), AL
    MOVB AL, (R10)
    INCQ R10
    INCQ SI
    DECQ R11
    JMP  int_remainder_loop

int_done:
    RET
