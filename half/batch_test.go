package half

import (
	"math"
	"testing"
)

func TestConvertBatch32(t *testing.T) {
	src := []float32{1.0, 2.0, 3.0, 4.0, 5.0, 6.0, 7.0, 8.0, 9.0, 10.0}
	dst := make([]Half, len(src))

	ConvertBatch32(dst, src)

	for i, s := range src {
		expected := FromFloat32(s)
		if dst[i] != expected {
			t.Errorf("ConvertBatch32[%d] = %v, want %v", i, dst[i], expected)
		}
	}
}

func TestConvertBatchToFloat32(t *testing.T) {
	src := []Half{
		FromFloat32(1.0), FromFloat32(2.0), FromFloat32(3.0),
		FromFloat32(4.0), FromFloat32(5.0), FromFloat32(6.0),
		FromFloat32(7.0), FromFloat32(8.0), FromFloat32(9.0),
	}
	dst := make([]float32, len(src))

	ConvertBatchToFloat32(dst, src)

	for i, s := range src {
		if dst[i] != s.Float32() {
			t.Errorf("ConvertBatchToFloat32[%d] = %v, want %v", i, dst[i], s.Float32())
		}
	}
}

func TestConvertBytesToFloat32(t *testing.T) {
	// Create some half values and convert to bytes
	halfs := []Half{FromFloat32(1.0), FromFloat32(2.0), FromFloat32(3.0), FromFloat32(4.0)}
	bytes := make([]byte, len(halfs)*2)
	for i, h := range halfs {
		bits := h.Bits()
		bytes[i*2] = byte(bits)
		bytes[i*2+1] = byte(bits >> 8)
	}

	dst := make([]float32, len(halfs))
	ConvertBytesToFloat32(dst, bytes)

	for i, h := range halfs {
		if dst[i] != h.Float32() {
			t.Errorf("ConvertBytesToFloat32[%d] = %v, want %v", i, dst[i], h.Float32())
		}
	}
}

func TestConvertFloat32ToBytes(t *testing.T) {
	src := []float32{1.0, 2.0, 3.0, 4.0, 5.0, 6.0, 7.0, 8.0}
	dst := make([]byte, len(src)*2)

	ConvertFloat32ToBytes(dst, src)

	// Verify by converting back
	result := make([]float32, len(src))
	ConvertBytesToFloat32(result, dst)

	for i := range src {
		// Compare with half-precision accuracy
		expected := FromFloat32(src[i]).Float32()
		if result[i] != expected {
			t.Errorf("Round-trip[%d] = %v, want %v", i, result[i], expected)
		}
	}
}

func TestClampBatch(t *testing.T) {
	data := []float32{
		-100000, -65504, -1, 0, 1, 65504, 100000,
		float32(math.Inf(1)), float32(math.Inf(-1)), float32(math.NaN()),
	}

	ClampBatch(data)

	expected := []float32{-65504, -65504, -1, 0, 1, 65504, 65504, 65504, -65504, 0}
	for i, v := range data {
		if v != expected[i] {
			t.Errorf("ClampBatch[%d] = %v, want %v", i, v, expected[i])
		}
	}
}

func TestMultiplyBatch(t *testing.T) {
	src := []Half{FromFloat32(1.0), FromFloat32(2.0), FromFloat32(3.0), FromFloat32(4.0)}
	dst := make([]Half, len(src))

	MultiplyBatch(dst, src, 2.0)

	for i, s := range src {
		expected := FromFloat32(s.Float32() * 2.0)
		if dst[i] != expected {
			t.Errorf("MultiplyBatch[%d] = %v, want %v", i, dst[i], expected)
		}
	}
}

func TestAddBatch(t *testing.T) {
	a := []Half{FromFloat32(1.0), FromFloat32(2.0), FromFloat32(3.0), FromFloat32(4.0)}
	b := []Half{FromFloat32(0.5), FromFloat32(1.5), FromFloat32(2.5), FromFloat32(3.5)}
	dst := make([]Half, len(a))

	AddBatch(dst, a, b)

	for i := range a {
		expected := FromFloat32(a[i].Float32() + b[i].Float32())
		if dst[i] != expected {
			t.Errorf("AddBatch[%d] = %v, want %v", i, dst[i], expected)
		}
	}
}

func TestLerpBatch(t *testing.T) {
	a := []Half{FromFloat32(0.0), FromFloat32(0.0), FromFloat32(0.0), FromFloat32(0.0)}
	b := []Half{FromFloat32(1.0), FromFloat32(2.0), FromFloat32(3.0), FromFloat32(4.0)}
	dst := make([]Half, len(a))

	LerpBatch(dst, a, b, 0.5)

	expected := []float32{0.5, 1.0, 1.5, 2.0}
	for i := range dst {
		if dst[i].Float32() != expected[i] {
			t.Errorf("LerpBatch[%d] = %v, want %v", i, dst[i].Float32(), expected[i])
		}
	}
}

func BenchmarkConvertBatch32(b *testing.B) {
	n := 1920 * 1080 // Full HD
	src := make([]float32, n)
	dst := make([]Half, n)

	for i := range src {
		src[i] = float32(i) / float32(n)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ConvertBatch32(dst, src)
	}
}

func BenchmarkConvertSlice32Baseline(b *testing.B) {
	n := 1920 * 1080 // Full HD
	src := make([]float32, n)
	dst := make([]Half, n)

	for i := range src {
		src[i] = float32(i) / float32(n)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ConvertSlice32(dst, src)
	}
}

func BenchmarkConvertBatchToFloat32(b *testing.B) {
	n := 1920 * 1080 // Full HD
	src := make([]Half, n)
	dst := make([]float32, n)

	for i := range src {
		src[i] = FromFloat32(float32(i) / float32(n))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ConvertBatchToFloat32(dst, src)
	}
}

func BenchmarkConvertSliceToFloat32Baseline(b *testing.B) {
	n := 1920 * 1080 // Full HD
	src := make([]Half, n)
	dst := make([]float32, n)

	for i := range src {
		src[i] = FromFloat32(float32(i) / float32(n))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ConvertSliceToFloat32(dst, src)
	}
}

func BenchmarkConvertBytesToFloat32(b *testing.B) {
	n := 1920 * 1080 * 2 // Full HD, bytes
	src := make([]byte, n)
	dst := make([]float32, n/2)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ConvertBytesToFloat32(dst, src)
	}
}

func BenchmarkMultiplyBatch(b *testing.B) {
	n := 1920 * 1080
	src := make([]Half, n)
	dst := make([]Half, n)

	for i := range src {
		src[i] = FromFloat32(float32(i) / float32(n))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		MultiplyBatch(dst, src, 1.5)
	}
}

func TestMultiplyBatchLarge(t *testing.T) {
	// Test with more than batchSize elements to hit the batched loop
	src := make([]Half, 16)
	dst := make([]Half, 16)
	for i := range src {
		src[i] = FromFloat32(float32(i + 1))
	}

	MultiplyBatch(dst, src, 2.0)

	for i, s := range src {
		expected := FromFloat32(s.Float32() * 2.0)
		if dst[i] != expected {
			t.Errorf("MultiplyBatch[%d] = %v, want %v", i, dst[i], expected)
		}
	}
}

func TestAddBatchLarge(t *testing.T) {
	// Test with more than batchSize elements to hit the batched loop
	a := make([]Half, 16)
	b := make([]Half, 16)
	dst := make([]Half, 16)
	for i := range a {
		a[i] = FromFloat32(float32(i))
		b[i] = FromFloat32(float32(i + 10))
	}

	AddBatch(dst, a, b)

	for i := range a {
		expected := FromFloat32(a[i].Float32() + b[i].Float32())
		if dst[i] != expected {
			t.Errorf("AddBatch[%d] = %v, want %v", i, dst[i], expected)
		}
	}
}

func TestLerpBatchLarge(t *testing.T) {
	// Test with more than batchSize elements to hit the batched loop
	a := make([]Half, 16)
	b := make([]Half, 16)
	dst := make([]Half, 16)
	for i := range a {
		a[i] = FromFloat32(0.0)
		b[i] = FromFloat32(float32(i + 1))
	}

	LerpBatch(dst, a, b, 0.5)

	for i := range dst {
		expected := float32(i+1) * 0.5
		if dst[i].Float32() != expected {
			t.Errorf("LerpBatch[%d] = %v, want %v", i, dst[i].Float32(), expected)
		}
	}
}
