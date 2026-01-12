package half

import (
	"math"
)

// Batch processing constants.
const (
	// batchSize is the number of elements processed in each unrolled loop iteration.
	batchSize = 8
)

// ConvertBatch32 converts a slice of float32 to Half with loop unrolling.
// This is optimized for large arrays.
func ConvertBatch32(dst []Half, src []float32) {
	n := len(src)
	if len(dst) < n {
		panic("half: destination slice too small")
	}

	// Process in batches of 8
	i := 0
	for ; i+batchSize <= n; i += batchSize {
		dst[i] = FromFloat32(src[i])
		dst[i+1] = FromFloat32(src[i+1])
		dst[i+2] = FromFloat32(src[i+2])
		dst[i+3] = FromFloat32(src[i+3])
		dst[i+4] = FromFloat32(src[i+4])
		dst[i+5] = FromFloat32(src[i+5])
		dst[i+6] = FromFloat32(src[i+6])
		dst[i+7] = FromFloat32(src[i+7])
	}

	// Handle remainder
	for ; i < n; i++ {
		dst[i] = FromFloat32(src[i])
	}
}

// ConvertBatchToFloat32 converts a slice of Half to float32 with loop unrolling.
// This is optimized for large arrays.
func ConvertBatchToFloat32(dst []float32, src []Half) {
	n := len(src)
	if len(dst) < n {
		panic("half: destination slice too small")
	}

	// Process in batches of 8
	i := 0
	for ; i+batchSize <= n; i += batchSize {
		dst[i] = src[i].Float32()
		dst[i+1] = src[i+1].Float32()
		dst[i+2] = src[i+2].Float32()
		dst[i+3] = src[i+3].Float32()
		dst[i+4] = src[i+4].Float32()
		dst[i+5] = src[i+5].Float32()
		dst[i+6] = src[i+6].Float32()
		dst[i+7] = src[i+7].Float32()
	}

	// Handle remainder
	for ; i < n; i++ {
		dst[i] = src[i].Float32()
	}
}

// ConvertBytesToFloat32 converts bytes containing half-precision data to float32.
// Input bytes are in little-endian order (2 bytes per half).
func ConvertBytesToFloat32(dst []float32, src []byte) {
	n := len(src) / 2
	if len(dst) < n {
		panic("half: destination slice too small")
	}

	// Process in batches of 8
	i := 0
	for ; i+batchSize <= n; i += batchSize {
		j := i * 2
		dst[i] = FromBits(uint16(src[j]) | uint16(src[j+1])<<8).Float32()
		dst[i+1] = FromBits(uint16(src[j+2]) | uint16(src[j+3])<<8).Float32()
		dst[i+2] = FromBits(uint16(src[j+4]) | uint16(src[j+5])<<8).Float32()
		dst[i+3] = FromBits(uint16(src[j+6]) | uint16(src[j+7])<<8).Float32()
		dst[i+4] = FromBits(uint16(src[j+8]) | uint16(src[j+9])<<8).Float32()
		dst[i+5] = FromBits(uint16(src[j+10]) | uint16(src[j+11])<<8).Float32()
		dst[i+6] = FromBits(uint16(src[j+12]) | uint16(src[j+13])<<8).Float32()
		dst[i+7] = FromBits(uint16(src[j+14]) | uint16(src[j+15])<<8).Float32()
	}

	// Handle remainder
	for ; i < n; i++ {
		j := i * 2
		dst[i] = FromBits(uint16(src[j]) | uint16(src[j+1])<<8).Float32()
	}
}

// ConvertFloat32ToBytes converts float32 to bytes containing half-precision data.
// Output bytes are in little-endian order (2 bytes per half).
func ConvertFloat32ToBytes(dst []byte, src []float32) {
	n := len(src)
	if len(dst) < n*2 {
		panic("half: destination slice too small")
	}

	// Process in batches of 8
	i := 0
	for ; i+batchSize <= n; i += batchSize {
		j := i * 2
		h0 := FromFloat32(src[i]).Bits()
		h1 := FromFloat32(src[i+1]).Bits()
		h2 := FromFloat32(src[i+2]).Bits()
		h3 := FromFloat32(src[i+3]).Bits()
		h4 := FromFloat32(src[i+4]).Bits()
		h5 := FromFloat32(src[i+5]).Bits()
		h6 := FromFloat32(src[i+6]).Bits()
		h7 := FromFloat32(src[i+7]).Bits()

		dst[j] = byte(h0)
		dst[j+1] = byte(h0 >> 8)
		dst[j+2] = byte(h1)
		dst[j+3] = byte(h1 >> 8)
		dst[j+4] = byte(h2)
		dst[j+5] = byte(h2 >> 8)
		dst[j+6] = byte(h3)
		dst[j+7] = byte(h3 >> 8)
		dst[j+8] = byte(h4)
		dst[j+9] = byte(h4 >> 8)
		dst[j+10] = byte(h5)
		dst[j+11] = byte(h5 >> 8)
		dst[j+12] = byte(h6)
		dst[j+13] = byte(h6 >> 8)
		dst[j+14] = byte(h7)
		dst[j+15] = byte(h7 >> 8)
	}

	// Handle remainder
	for ; i < n; i++ {
		j := i * 2
		h := FromFloat32(src[i]).Bits()
		dst[j] = byte(h)
		dst[j+1] = byte(h >> 8)
	}
}

// ClampBatch clamps a slice of float32 values to half-precision range in-place.
// Values are clamped to [-65504, 65504] (half max range).
func ClampBatch(data []float32) {
	const maxVal = 65504.0
	const minVal = -65504.0

	n := len(data)
	i := 0
	for ; i+batchSize <= n; i += batchSize {
		data[i] = clampFloat32(data[i], minVal, maxVal)
		data[i+1] = clampFloat32(data[i+1], minVal, maxVal)
		data[i+2] = clampFloat32(data[i+2], minVal, maxVal)
		data[i+3] = clampFloat32(data[i+3], minVal, maxVal)
		data[i+4] = clampFloat32(data[i+4], minVal, maxVal)
		data[i+5] = clampFloat32(data[i+5], minVal, maxVal)
		data[i+6] = clampFloat32(data[i+6], minVal, maxVal)
		data[i+7] = clampFloat32(data[i+7], minVal, maxVal)
	}

	for ; i < n; i++ {
		data[i] = clampFloat32(data[i], minVal, maxVal)
	}
}

func clampFloat32(v, min, max float32) float32 {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	if math.IsNaN(float64(v)) {
		return 0
	}
	return v
}

// MultiplyBatch multiplies all elements in src by a scalar and stores the result in dst.
// Both slices must be the same length; dst must be at least as long as src.
func MultiplyBatch(dst, src []Half, scalar float32) {
	n := len(src)
	if len(dst) < n {
		panic("half: destination slice too small")
	}

	i := 0
	for ; i+batchSize <= n; i += batchSize {
		dst[i] = FromFloat32(src[i].Float32() * scalar)
		dst[i+1] = FromFloat32(src[i+1].Float32() * scalar)
		dst[i+2] = FromFloat32(src[i+2].Float32() * scalar)
		dst[i+3] = FromFloat32(src[i+3].Float32() * scalar)
		dst[i+4] = FromFloat32(src[i+4].Float32() * scalar)
		dst[i+5] = FromFloat32(src[i+5].Float32() * scalar)
		dst[i+6] = FromFloat32(src[i+6].Float32() * scalar)
		dst[i+7] = FromFloat32(src[i+7].Float32() * scalar)
	}

	for ; i < n; i++ {
		dst[i] = FromFloat32(src[i].Float32() * scalar)
	}
}

// AddBatch adds corresponding elements from slices a and b, storing the result in dst.
// All slices must be the same length; dst must be at least as long as a.
func AddBatch(dst, a, b []Half) {
	n := len(a)
	if len(b) < n || len(dst) < n {
		panic("half: slice size mismatch")
	}

	i := 0
	for ; i+batchSize <= n; i += batchSize {
		dst[i] = FromFloat32(a[i].Float32() + b[i].Float32())
		dst[i+1] = FromFloat32(a[i+1].Float32() + b[i+1].Float32())
		dst[i+2] = FromFloat32(a[i+2].Float32() + b[i+2].Float32())
		dst[i+3] = FromFloat32(a[i+3].Float32() + b[i+3].Float32())
		dst[i+4] = FromFloat32(a[i+4].Float32() + b[i+4].Float32())
		dst[i+5] = FromFloat32(a[i+5].Float32() + b[i+5].Float32())
		dst[i+6] = FromFloat32(a[i+6].Float32() + b[i+6].Float32())
		dst[i+7] = FromFloat32(a[i+7].Float32() + b[i+7].Float32())
	}

	for ; i < n; i++ {
		dst[i] = FromFloat32(a[i].Float32() + b[i].Float32())
	}
}

// LerpBatch performs linear interpolation between two slices.
// dst[i] = a[i] * (1-t) + b[i] * t
func LerpBatch(dst, a, b []Half, t float32) {
	n := len(a)
	if len(b) < n || len(dst) < n {
		panic("half: slice size mismatch")
	}

	oneMinusT := 1.0 - t

	i := 0
	for ; i+batchSize <= n; i += batchSize {
		dst[i] = FromFloat32(a[i].Float32()*oneMinusT + b[i].Float32()*t)
		dst[i+1] = FromFloat32(a[i+1].Float32()*oneMinusT + b[i+1].Float32()*t)
		dst[i+2] = FromFloat32(a[i+2].Float32()*oneMinusT + b[i+2].Float32()*t)
		dst[i+3] = FromFloat32(a[i+3].Float32()*oneMinusT + b[i+3].Float32()*t)
		dst[i+4] = FromFloat32(a[i+4].Float32()*oneMinusT + b[i+4].Float32()*t)
		dst[i+5] = FromFloat32(a[i+5].Float32()*oneMinusT + b[i+5].Float32()*t)
		dst[i+6] = FromFloat32(a[i+6].Float32()*oneMinusT + b[i+6].Float32()*t)
		dst[i+7] = FromFloat32(a[i+7].Float32()*oneMinusT + b[i+7].Float32()*t)
	}

	for ; i < n; i++ {
		dst[i] = FromFloat32(a[i].Float32()*oneMinusT + b[i].Float32()*t)
	}
}
