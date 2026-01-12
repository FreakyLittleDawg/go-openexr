package exr

import (
	"math"
	"testing"
)

// TestDeepSampleBasics tests the DeepSample type.
func TestDeepSampleBasics(t *testing.T) {
	// Test hard surface sample (Z == ZBack)
	t.Run("HardSurface", func(t *testing.T) {
		sample := DeepSample{
			Z:     1.0,
			ZBack: 1.0,
			A:     0.5,
			R:     0.25,
			G:     0.25,
			B:     0.25,
		}

		if sample.IsVolumetric() {
			t.Error("Hard surface sample should not be volumetric")
		}
		if sample.IsOpaque() {
			t.Error("Sample with alpha 0.5 should not be opaque")
		}
	})

	// Test volumetric sample (ZBack > Z)
	t.Run("Volumetric", func(t *testing.T) {
		sample := DeepSample{
			Z:     1.0,
			ZBack: 2.0,
			A:     0.3,
		}

		if !sample.IsVolumetric() {
			t.Error("Sample with ZBack != Z should be volumetric")
		}
	})

	// Test opaque sample
	t.Run("Opaque", func(t *testing.T) {
		sample := DeepSample{
			Z:     1.0,
			ZBack: 1.0,
			A:     1.0,
		}

		if !sample.IsOpaque() {
			t.Error("Sample with alpha 1.0 should be opaque")
		}
	})

	// Test channels map
	t.Run("Channels", func(t *testing.T) {
		sample := DeepSample{
			Z:        1.0,
			ZBack:    1.0,
			A:        0.5,
			Channels: make(map[string]float32),
		}
		sample.Channels["custom"] = 0.75

		if val, ok := sample.Channels["custom"]; !ok || val != 0.75 {
			t.Errorf("Expected custom channel value 0.75, got %v", val)
		}
	})
}

// TestDefaultDeepCompositingSortPixel tests sample sorting.
func TestDefaultDeepCompositingSortPixel(t *testing.T) {
	comp := NewDefaultDeepCompositing()

	t.Run("SortByZ", func(t *testing.T) {
		samples := []DeepSample{
			{Z: 3.0, ZBack: 3.0, A: 0.5, R: 0.3},
			{Z: 1.0, ZBack: 1.0, A: 0.5, R: 0.1},
			{Z: 2.0, ZBack: 2.0, A: 0.5, R: 0.2},
		}

		comp.SortPixel(samples)

		if samples[0].R != 0.1 || samples[1].R != 0.2 || samples[2].R != 0.3 {
			t.Error("Samples not sorted by Z depth")
		}
	})

	t.Run("SortByZBack", func(t *testing.T) {
		samples := []DeepSample{
			{Z: 1.0, ZBack: 3.0, A: 0.5, R: 0.1},
			{Z: 1.0, ZBack: 2.0, A: 0.5, R: 0.2},
			{Z: 1.0, ZBack: 1.0, A: 0.5, R: 0.3},
		}

		comp.SortPixel(samples)

		// Same Z should sort by ZBack
		if samples[0].R != 0.3 || samples[1].R != 0.2 || samples[2].R != 0.1 {
			t.Errorf("Samples with same Z not sorted by ZBack: %v", samples)
		}
	})

	t.Run("StableSort", func(t *testing.T) {
		// Identical samples should maintain order
		samples := []DeepSample{
			{Z: 1.0, ZBack: 1.0, A: 0.5, R: 0.1},
			{Z: 1.0, ZBack: 1.0, A: 0.5, R: 0.2},
		}

		comp.SortPixel(samples)

		if samples[0].R != 0.1 || samples[1].R != 0.2 {
			t.Error("Stable sort not preserved for identical depth samples")
		}
	})
}

// TestDefaultDeepCompositingCompositePixel tests the over operator.
func TestDefaultDeepCompositingCompositePixel(t *testing.T) {
	comp := NewDefaultDeepCompositing()

	t.Run("EmptySamples", func(t *testing.T) {
		r, g, b, a := comp.CompositePixel(nil)
		if r != 0 || g != 0 || b != 0 || a != 0 {
			t.Error("Empty samples should produce zero output")
		}
	})

	t.Run("SingleOpaqueSample", func(t *testing.T) {
		samples := []DeepSample{
			{Z: 1.0, ZBack: 1.0, A: 1.0, R: 1.0, G: 0.5, B: 0.25},
		}

		r, g, b, a := comp.CompositePixel(samples)

		if !floatsEqual(r, 1.0) || !floatsEqual(g, 0.5) || !floatsEqual(b, 0.25) || !floatsEqual(a, 1.0) {
			t.Errorf("Single opaque sample: got (%v, %v, %v, %v)", r, g, b, a)
		}
	})

	t.Run("SingleTransparentSample", func(t *testing.T) {
		samples := []DeepSample{
			{Z: 1.0, ZBack: 1.0, A: 0.5, R: 0.5, G: 0.25, B: 0.125},
		}

		r, g, b, a := comp.CompositePixel(samples)

		if !floatsEqual(r, 0.5) || !floatsEqual(g, 0.25) || !floatsEqual(b, 0.125) || !floatsEqual(a, 0.5) {
			t.Errorf("Single transparent sample: got (%v, %v, %v, %v)", r, g, b, a)
		}
	})

	t.Run("TwoSamplesOver", func(t *testing.T) {
		// Front sample: 50% alpha, red
		// Back sample: 100% alpha, green
		// Result should be front + (1-0.5) * back
		samples := []DeepSample{
			{Z: 1.0, ZBack: 1.0, A: 0.5, R: 0.5, G: 0.0, B: 0.0}, // Front (premultiplied)
			{Z: 2.0, ZBack: 2.0, A: 1.0, R: 0.0, G: 1.0, B: 0.0}, // Back (premultiplied)
		}

		r, g, b, a := comp.CompositePixel(samples)

		// R = 0.5 + 0.5 * 0.0 = 0.5
		// G = 0.0 + 0.5 * 1.0 = 0.5
		// A = 0.5 + 0.5 * 1.0 = 1.0
		if !floatsEqual(r, 0.5) || !floatsEqual(g, 0.5) || !floatsEqual(b, 0.0) || !floatsEqual(a, 1.0) {
			t.Errorf("Two samples over: got (%v, %v, %v, %v), expected (0.5, 0.5, 0, 1)", r, g, b, a)
		}
	})

	t.Run("EarlyTermination", func(t *testing.T) {
		samples := []DeepSample{
			{Z: 1.0, ZBack: 1.0, A: 1.0, R: 1.0, G: 0.0, B: 0.0}, // Opaque front
			{Z: 2.0, ZBack: 2.0, A: 1.0, R: 0.0, G: 1.0, B: 0.0}, // Should be ignored
		}

		r, g, b, a := comp.CompositePixel(samples)

		// Back sample should not contribute due to early termination
		if !floatsEqual(r, 1.0) || !floatsEqual(g, 0.0) || !floatsEqual(b, 0.0) || !floatsEqual(a, 1.0) {
			t.Errorf("Early termination: got (%v, %v, %v, %v), expected (1, 0, 0, 1)", r, g, b, a)
		}
	})

	t.Run("MultipleSemiTransparent", func(t *testing.T) {
		// Three 50% alpha samples
		samples := []DeepSample{
			{Z: 1.0, ZBack: 1.0, A: 0.5, R: 0.5, G: 0.0, B: 0.0}, // Red
			{Z: 2.0, ZBack: 2.0, A: 0.5, R: 0.0, G: 0.5, B: 0.0}, // Green
			{Z: 3.0, ZBack: 3.0, A: 0.5, R: 0.0, G: 0.0, B: 0.5}, // Blue
		}

		r, g, b, a := comp.CompositePixel(samples)

		// R = 0.5 + 0.5*0 + 0.25*0 = 0.5
		// G = 0 + 0.5*0.5 + 0.25*0 = 0.25
		// B = 0 + 0.5*0 + 0.25*0.5 = 0.125
		// A = 0.5 + 0.5*0.5 + 0.25*0.5 = 0.875
		expectedA := float32(0.5 + 0.5*0.5 + 0.25*0.5)
		if !floatsEqual(r, 0.5) || !floatsEqual(g, 0.25) || !floatsEqual(b, 0.125) || !floatsEqual(a, expectedA) {
			t.Errorf("Multiple semi-transparent: got (%v, %v, %v, %v), expected (0.5, 0.25, 0.125, %v)", r, g, b, a, expectedA)
		}
	})
}

// TestDefaultDeepCompositingAllChannels tests compositing with custom channels.
func TestDefaultDeepCompositingAllChannels(t *testing.T) {
	comp := NewDefaultDeepCompositing()

	t.Run("CustomChannel", func(t *testing.T) {
		samples := []DeepSample{
			{
				Z:     1.0,
				ZBack: 1.0,
				A:     0.5,
				R:     0.5,
				G:     0.25,
				B:     0.125,
				Channels: map[string]float32{
					"depth":    1.0,
					"velocity": 0.5,
				},
			},
			{
				Z:     2.0,
				ZBack: 2.0,
				A:     0.5,
				R:     0.0,
				G:     0.0,
				B:     0.0,
				Channels: map[string]float32{
					"depth":    2.0,
					"velocity": 1.0,
				},
			},
		}

		result := comp.CompositePixelAllChannels(samples, []string{"R", "G", "B", "A", "depth", "velocity"})

		// Verify standard channels
		if !floatsEqual(result["R"], 0.5) {
			t.Errorf("R channel: got %v, expected 0.5", result["R"])
		}

		// Verify custom channels
		// depth = 1.0 + 0.5*2.0 = 2.0
		if !floatsEqual(result["depth"], 2.0) {
			t.Errorf("depth channel: got %v, expected 2.0", result["depth"])
		}
	})
}

// TestCompositeDeepScanLineErrors tests error conditions.
func TestCompositeDeepScanLineErrors(t *testing.T) {
	t.Run("NoSources", func(t *testing.T) {
		comp := NewCompositeDeepScanLine()

		fb := NewFrameBuffer()
		comp.SetFrameBuffer(fb)

		err := comp.ReadPixels(0, 10)
		if err != ErrNoSources {
			t.Errorf("Expected ErrNoSources, got %v", err)
		}
	})

	t.Run("NoFrameBuffer", func(t *testing.T) {
		// This test would require a mock reader, so we just test the method exists
		comp := NewCompositeDeepScanLine()
		if comp.FrameBuffer() != nil {
			t.Error("FrameBuffer should be nil initially")
		}
	})

	t.Run("NilSource", func(t *testing.T) {
		comp := NewCompositeDeepScanLine()
		err := comp.AddSource(nil)
		if err == nil {
			t.Error("Expected error when adding nil source")
		}
	})
}

// TestCompositeDeepScanLineBasics tests basic compositor setup.
func TestCompositeDeepScanLineBasics(t *testing.T) {
	comp := NewCompositeDeepScanLine()

	t.Run("InitialState", func(t *testing.T) {
		if comp.Sources() != 0 {
			t.Error("Should start with zero sources")
		}
		if comp.FrameBuffer() != nil {
			t.Error("FrameBuffer should be nil initially")
		}
	})

	t.Run("SetFrameBuffer", func(t *testing.T) {
		fb := NewFrameBuffer()
		fb.Set("R", NewSlice(PixelTypeFloat, make([]byte, 400), 10, 10))

		comp.SetFrameBuffer(fb)

		if comp.FrameBuffer() != fb {
			t.Error("FrameBuffer not set correctly")
		}
	})

	t.Run("SetCompositing", func(t *testing.T) {
		customComp := NewVolumetricDeepCompositing()
		comp.SetCompositing(customComp)

		// Reset to default
		comp.SetCompositing(nil)
	})

	t.Run("MaxSampleCount", func(t *testing.T) {
		comp.SetMaximumSampleCount(10000)
		if comp.GetMaximumSampleCount() != 10000 {
			t.Error("MaximumSampleCount not set correctly")
		}

		comp.SetMaximumSampleCount(0)
		if comp.GetMaximumSampleCount() != 0 {
			t.Error("MaximumSampleCount should be 0")
		}
	})
}

// TestVolumetricDeepCompositing tests volumetric sample handling.
func TestVolumetricDeepCompositing(t *testing.T) {
	comp := NewVolumetricDeepCompositing()

	t.Run("VolumetricSamples", func(t *testing.T) {
		samples := []DeepSample{
			{Z: 1.0, ZBack: 2.0, A: 0.5, R: 0.5, G: 0.0, B: 0.0}, // Volumetric
			{Z: 3.0, ZBack: 3.0, A: 0.5, R: 0.0, G: 0.5, B: 0.0}, // Hard surface
		}

		r, g, b, a := comp.CompositePixel(samples)

		// Should still work with default over operation
		if r <= 0 || g <= 0 {
			t.Errorf("Volumetric compositing: got (%v, %v, %v, %v)", r, g, b, a)
		}
	})

	t.Run("SortVolumetric", func(t *testing.T) {
		samples := []DeepSample{
			{Z: 1.0, ZBack: 3.0, A: 0.5, R: 0.1}, // Long volume
			{Z: 1.0, ZBack: 2.0, A: 0.5, R: 0.2}, // Shorter volume
			{Z: 0.5, ZBack: 0.5, A: 0.5, R: 0.3}, // Hard surface in front
		}

		comp.SortPixel(samples)

		// Hard surface at Z=0.5 should come first
		if samples[0].R != 0.3 {
			t.Error("Hard surface should sort first")
		}
	})
}

// TestCompositeDeepTiledBasics tests basic tiled compositor setup.
func TestCompositeDeepTiledBasics(t *testing.T) {
	comp := NewCompositeDeepTiled()

	t.Run("InitialState", func(t *testing.T) {
		if comp.Sources() != 0 {
			t.Error("Should start with zero sources")
		}
		if comp.FrameBuffer() != nil {
			t.Error("FrameBuffer should be nil initially")
		}
	})

	t.Run("NilSource", func(t *testing.T) {
		err := comp.AddSource(nil)
		if err == nil {
			t.Error("Expected error when adding nil source")
		}
	})
}

// TestCompositeEdgeCases tests edge cases in compositing.
func TestCompositeEdgeCases(t *testing.T) {
	comp := NewDefaultDeepCompositing()

	t.Run("ZeroAlphaSamples", func(t *testing.T) {
		samples := []DeepSample{
			{Z: 1.0, ZBack: 1.0, A: 0.0, R: 1.0, G: 0.0, B: 0.0},
			{Z: 2.0, ZBack: 2.0, A: 0.0, R: 0.0, G: 1.0, B: 0.0},
		}

		r, g, b, a := comp.CompositePixel(samples)

		// Zero alpha samples should still accumulate color
		// (premultiplied, so color * alpha = 0)
		if a != 0 {
			t.Errorf("Zero alpha samples should give zero total alpha, got %v", a)
		}
		_ = r
		_ = g
		_ = b
	})

	t.Run("NegativeDepth", func(t *testing.T) {
		samples := []DeepSample{
			{Z: -2.0, ZBack: -2.0, A: 0.5, R: 0.5},
			{Z: -1.0, ZBack: -1.0, A: 0.5, R: 0.25},
			{Z: 0.0, ZBack: 0.0, A: 0.5, R: 0.125},
		}

		comp.SortPixel(samples)

		// Should sort negative depths correctly
		if samples[0].Z != -2.0 {
			t.Error("Negative depths should sort correctly")
		}
	})

	t.Run("InfiniteDepth", func(t *testing.T) {
		inf := float32(math.Inf(1))
		samples := []DeepSample{
			{Z: inf, ZBack: inf, A: 0.5, R: 0.5},
			{Z: 1.0, ZBack: 1.0, A: 0.5, R: 0.25},
		}

		comp.SortPixel(samples)

		// Infinite depth should sort to the back
		if samples[0].Z != 1.0 {
			t.Error("Infinite depth should sort last")
		}
	})

	t.Run("OverflowAlpha", func(t *testing.T) {
		// Many samples that could overflow alpha
		samples := make([]DeepSample, 100)
		for i := range samples {
			samples[i] = DeepSample{
				Z:     float32(i),
				ZBack: float32(i),
				A:     0.1,
				R:     0.1,
			}
		}

		_, _, _, a := comp.CompositePixel(samples)

		// Alpha should be clamped to 1.0
		if a > 1.0 {
			t.Error("Alpha should be clamped to 1.0")
		}
	})
}

// BenchmarkDeepCompositing benchmarks compositing performance.
func BenchmarkDeepCompositing(b *testing.B) {
	comp := NewDefaultDeepCompositing()

	b.Run("SmallSampleCount", func(b *testing.B) {
		samples := make([]DeepSample, 5)
		for i := range samples {
			samples[i] = DeepSample{
				Z:     float32(i),
				ZBack: float32(i),
				A:     0.5,
				R:     0.25,
				G:     0.25,
				B:     0.25,
			}
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			comp.SortPixel(samples)
			comp.CompositePixel(samples)
		}
	})

	b.Run("MediumSampleCount", func(b *testing.B) {
		samples := make([]DeepSample, 50)
		for i := range samples {
			samples[i] = DeepSample{
				Z:     float32(i),
				ZBack: float32(i),
				A:     0.1,
				R:     0.1,
				G:     0.1,
				B:     0.1,
			}
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			comp.SortPixel(samples)
			comp.CompositePixel(samples)
		}
	})

	b.Run("LargeSampleCount", func(b *testing.B) {
		samples := make([]DeepSample, 500)
		for i := range samples {
			samples[i] = DeepSample{
				Z:     float32(i),
				ZBack: float32(i),
				A:     0.05,
				R:     0.05,
				G:     0.05,
				B:     0.05,
			}
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			comp.SortPixel(samples)
			comp.CompositePixel(samples)
		}
	})

	b.Run("EarlyTermination", func(b *testing.B) {
		// First sample is opaque, rest should be skipped
		samples := make([]DeepSample, 100)
		samples[0] = DeepSample{Z: 0, ZBack: 0, A: 1.0, R: 1.0, G: 1.0, B: 1.0}
		for i := 1; i < len(samples); i++ {
			samples[i] = DeepSample{
				Z:     float32(i),
				ZBack: float32(i),
				A:     0.5,
				R:     0.5,
			}
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			comp.CompositePixel(samples)
		}
	})
}

// floatsEqual compares two float32 values with tolerance for compositing tests.
func floatsEqual(a, b float32) bool {
	const epsilon = 0.0001
	return math.Abs(float64(a-b)) < epsilon
}
