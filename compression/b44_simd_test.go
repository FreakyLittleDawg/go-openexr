package compression

import (
	"testing"
)

// TestToOrderedSIMD tests the SIMD conversion from sign-magnitude to ordered representation.
func TestToOrderedSIMD(t *testing.T) {
	tests := []struct {
		name  string
		input [16]uint16
		want  [16]uint16
	}{
		{
			name: "positive_values",
			input: [16]uint16{
				0x0000, 0x0001, 0x3c00, 0x4000, // 0, epsilon, 1.0, 2.0
				0x0100, 0x0200, 0x0400, 0x0800,
				0x1000, 0x2000, 0x3000, 0x3800, // 0.5
				0x3e00, 0x3f00, 0x3fff, 0x4200,
			},
			want: [16]uint16{
				0x8000, 0x8001, 0xbc00, 0xc000, // positive: v | 0x8000
				0x8100, 0x8200, 0x8400, 0x8800,
				0x9000, 0xa000, 0xb000, 0xb800,
				0xbe00, 0xbf00, 0xbfff, 0xc200,
			},
		},
		{
			name: "negative_values",
			input: [16]uint16{
				0x8000, 0x8001, 0xbc00, 0xc000, // -0, -epsilon, -1.0, -2.0
				0x8100, 0x8200, 0x8400, 0x8800,
				0x9000, 0xa000, 0xb000, 0xb800,
				0xbe00, 0xbf00, 0xbfff, 0xc200,
			},
			want: [16]uint16{
				0x7fff, 0x7ffe, 0x43ff, 0x3fff, // negative: ^v
				0x7eff, 0x7dff, 0x7bff, 0x77ff,
				0x6fff, 0x5fff, 0x4fff, 0x47ff,
				0x41ff, 0x40ff, 0x4000, 0x3dff,
			},
		},
		{
			name: "nan_inf_values",
			input: [16]uint16{
				0x7c00, 0x7c01, 0x7c80, 0x7fff, // +Inf, +NaN variants
				0xfc00, 0xfc01, 0xfc80, 0xffff, // -Inf, -NaN variants
				0x0000, 0x8000, 0x3c00, 0xbc00, // Mixed normal
				0x7c00, 0x7c00, 0x7c00, 0x7c00,
			},
			want: [16]uint16{
				0x8000, 0x8000, 0x8000, 0x8000, // NaN/Inf all become 0x8000
				0x8000, 0x8000, 0x8000, 0x8000,
				0x8000, 0x7fff, 0xbc00, 0x43ff, // +0->0x8000, -0->0x7fff
				0x8000, 0x8000, 0x8000, 0x8000,
			},
		},
		{
			name: "mixed_gradient",
			input: [16]uint16{
				0x0000, 0x2000, 0x3000, 0x3800, // Positive gradient
				0x3c00, 0x3e00, 0x4000, 0x4200,
				0x8000, 0xa000, 0xb000, 0xb800, // Negative gradient
				0xbc00, 0xbe00, 0xc000, 0xc200,
			},
			want: [16]uint16{
				0x8000, 0xa000, 0xb000, 0xb800, // positive: | 0x8000
				0xbc00, 0xbe00, 0xc000, 0xc200,
				0x7fff, 0x5fff, 0x4fff, 0x47ff, // negative: ^v
				0x43ff, 0x41ff, 0x3fff, 0x3dff,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var dst [16]uint16
			toOrderedSIMD(&dst, &tt.input)
			for i := 0; i < 16; i++ {
				if dst[i] != tt.want[i] {
					t.Errorf("index %d: got 0x%04x, want 0x%04x (input 0x%04x)",
						i, dst[i], tt.want[i], tt.input[i])
				}
			}
		})
	}
}

// TestFindMaxSIMD tests the SIMD maximum finding function.
func TestFindMaxSIMD(t *testing.T) {
	tests := []struct {
		name  string
		input [16]uint16
		want  uint16
	}{
		{
			name:  "all_zeros",
			input: [16]uint16{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
			want:  0,
		},
		{
			name:  "all_max",
			input: [16]uint16{0xffff, 0xffff, 0xffff, 0xffff, 0xffff, 0xffff, 0xffff, 0xffff, 0xffff, 0xffff, 0xffff, 0xffff, 0xffff, 0xffff, 0xffff, 0xffff},
			want:  0xffff,
		},
		{
			name:  "max_at_start",
			input: [16]uint16{0xffff, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
			want:  0xffff,
		},
		{
			name:  "max_at_end",
			input: [16]uint16{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0xffff},
			want:  0xffff,
		},
		{
			name:  "max_in_middle",
			input: [16]uint16{0, 0, 0, 0, 0, 0, 0, 0xabcd, 0, 0, 0, 0, 0, 0, 0, 0},
			want:  0xabcd,
		},
		{
			name:  "sequential",
			input: [16]uint16{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15},
			want:  15,
		},
		{
			name:  "reverse_sequential",
			input: [16]uint16{15, 14, 13, 12, 11, 10, 9, 8, 7, 6, 5, 4, 3, 2, 1, 0},
			want:  15,
		},
		{
			name:  "random_values",
			input: [16]uint16{0x1234, 0x5678, 0x9abc, 0xdef0, 0x4321, 0x8765, 0xcba9, 0x0fed, 0x2468, 0xace0, 0x1357, 0x9bdf, 0x0246, 0x8ace, 0xfdb9, 0x7531},
			want:  0xfdb9,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := findMaxSIMD(&tt.input)
			if got != tt.want {
				t.Errorf("got 0x%04x, want 0x%04x", got, tt.want)
			}
		})
	}
}

// TestFromOrderedSIMD tests the inverse SIMD conversion.
func TestFromOrderedSIMD(t *testing.T) {
	tests := []struct {
		name  string
		input [16]uint16
		want  [16]uint16
	}{
		{
			name: "ordered_positive", // High bit set = was positive
			input: [16]uint16{
				0x8000, 0x8001, 0xbc00, 0xc000,
				0x8100, 0x8200, 0x8400, 0x8800,
				0x9000, 0xa000, 0xb000, 0xb800,
				0xbe00, 0xbf00, 0xbfff, 0xc200,
			},
			want: [16]uint16{
				0x0000, 0x0001, 0x3c00, 0x4000, // result: v & 0x7fff
				0x0100, 0x0200, 0x0400, 0x0800,
				0x1000, 0x2000, 0x3000, 0x3800,
				0x3e00, 0x3f00, 0x3fff, 0x4200,
			},
		},
		{
			name: "ordered_negative", // High bit clear = was negative
			input: [16]uint16{
				0x7fff, 0x7ffe, 0x43ff, 0x3fff,
				0x7eff, 0x7dff, 0x7bff, 0x77ff,
				0x6fff, 0x5fff, 0x4fff, 0x47ff,
				0x41ff, 0x40ff, 0x4000, 0x3dff,
			},
			want: [16]uint16{
				0x8000, 0x8001, 0xbc00, 0xc000, // result: ^v
				0x8100, 0x8200, 0x8400, 0x8800,
				0x9000, 0xa000, 0xb000, 0xb800,
				0xbe00, 0xbf00, 0xbfff, 0xc200,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var dst [16]uint16
			fromOrderedSIMD(&dst, &tt.input)
			for i := 0; i < 16; i++ {
				if dst[i] != tt.want[i] {
					t.Errorf("index %d: got 0x%04x, want 0x%04x (input 0x%04x)",
						i, dst[i], tt.want[i], tt.input[i])
				}
			}
		})
	}
}

// TestOrderedRoundtrip tests that toOrdered and fromOrdered are inverses
// for normal (non-NaN/Inf) values.
func TestOrderedRoundtrip(t *testing.T) {
	// Test various values that should round-trip exactly
	input := [16]uint16{
		0x0000, 0x0001, 0x3c00, 0x4000, // Positive
		0x8000, 0x8001, 0xbc00, 0xc000, // Negative
		0x0100, 0x0200, 0x0400, 0x0800,
		0x1000, 0x2000, 0x3000, 0x3800,
	}

	var ordered [16]uint16
	var restored [16]uint16

	toOrderedSIMD(&ordered, &input)
	fromOrderedSIMD(&restored, &ordered)

	for i := 0; i < 16; i++ {
		// Skip NaN/Inf (they get mapped to 0x8000 which can't round-trip)
		if (input[i] & 0x7c00) == 0x7c00 {
			continue
		}
		if restored[i] != input[i] {
			t.Errorf("index %d: input 0x%04x -> ordered 0x%04x -> restored 0x%04x",
				i, input[i], ordered[i], restored[i])
		}
	}
}

// TestOrderedOrdering tests that the ordered representation maintains correct ordering.
func TestOrderedOrdering(t *testing.T) {
	// In ordered representation:
	// - Small positive numbers should be larger than large negative numbers
	// - Ordering should match numerical ordering for comparisons

	// -2.0, -1.0, -0.5, -0, +0, +0.5, +1.0, +2.0
	input := [16]uint16{
		0xc000, 0xbc00, 0xb800, 0x8000, // -2.0, -1.0, -0.5, -0
		0x0000, 0x3800, 0x3c00, 0x4000, // +0, +0.5, +1.0, +2.0
		0x0000, 0x0000, 0x0000, 0x0000,
		0x0000, 0x0000, 0x0000, 0x0000,
	}

	var ordered [16]uint16
	toOrderedSIMD(&ordered, &input)

	// Check that ordered values are in ascending order for the first 8 values
	// which represent -2.0, -1.0, -0.5, -0, +0, +0.5, +1.0, +2.0
	for i := 0; i < 7; i++ {
		if ordered[i] >= ordered[i+1] {
			t.Errorf("ordering violated: ordered[%d]=0x%04x >= ordered[%d]=0x%04x (inputs: 0x%04x, 0x%04x)",
				i, ordered[i], i+1, ordered[i+1], input[i], input[i+1])
		}
	}
}

// Benchmarks for SIMD functions

func BenchmarkToOrderedSIMD(b *testing.B) {
	var src [16]uint16
	for i := 0; i < 16; i++ {
		src[i] = uint16(i * 0x1000)
	}
	var dst [16]uint16

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		toOrderedSIMD(&dst, &src)
	}
}

func BenchmarkFindMaxSIMD(b *testing.B) {
	var src [16]uint16
	for i := 0; i < 16; i++ {
		src[i] = uint16(i * 0x1000)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = findMaxSIMD(&src)
	}
}

func BenchmarkFromOrderedSIMD(b *testing.B) {
	var src [16]uint16
	for i := 0; i < 16; i++ {
		src[i] = uint16(i*0x1000) | 0x8000
	}
	var dst [16]uint16

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		fromOrderedSIMD(&dst, &src)
	}
}
