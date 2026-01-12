package interleave

import (
	"bytes"
	"testing"
)

func TestInterleaveEmpty(t *testing.T) {
	data := []byte{}
	out := Interleave(data, 2, nil)
	if len(out) != 0 {
		t.Error("Empty input should produce empty output")
	}
}

func TestInterleaveStride1(t *testing.T) {
	data := []byte{1, 2, 3, 4}
	out := Interleave(data, 1, nil)
	if !bytes.Equal(out, data) {
		t.Errorf("Stride 1 should copy: got %v, want %v", out, data)
	}
}

func TestInterleaveStride2(t *testing.T) {
	// 4 half values: [a0,a1, b0,b1, c0,c1, d0,d1]
	data := []byte{0x10, 0x11, 0x20, 0x21, 0x30, 0x31, 0x40, 0x41}
	expected := []byte{0x10, 0x20, 0x30, 0x40, 0x11, 0x21, 0x31, 0x41}
	out := Interleave(data, 2, nil)
	if !bytes.Equal(out, expected) {
		t.Errorf("Interleave stride 2:\ngot  %v\nwant %v", out, expected)
	}
}

func TestDeinterleaveStride2(t *testing.T) {
	// Reverse of above
	data := []byte{0x10, 0x20, 0x30, 0x40, 0x11, 0x21, 0x31, 0x41}
	expected := []byte{0x10, 0x11, 0x20, 0x21, 0x30, 0x31, 0x40, 0x41}
	out := Deinterleave(data, 2, nil)
	if !bytes.Equal(out, expected) {
		t.Errorf("Deinterleave stride 2:\ngot  %v\nwant %v", out, expected)
	}
}

func TestInterleaveStride4(t *testing.T) {
	// 2 float values: [a0,a1,a2,a3, b0,b1,b2,b3]
	data := []byte{0x10, 0x11, 0x12, 0x13, 0x20, 0x21, 0x22, 0x23}
	expected := []byte{0x10, 0x20, 0x11, 0x21, 0x12, 0x22, 0x13, 0x23}
	out := Interleave(data, 4, nil)
	if !bytes.Equal(out, expected) {
		t.Errorf("Interleave stride 4:\ngot  %v\nwant %v", out, expected)
	}
}

func TestRoundTrip(t *testing.T) {
	original := []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12}

	for _, stride := range []int{2, 3, 4, 6} {
		data := make([]byte, len(original))
		copy(data, original)

		interleaved := Interleave(data, stride, nil)
		restored := Deinterleave(interleaved, stride, nil)

		if !bytes.Equal(restored, original) {
			t.Errorf("Round-trip failed for stride %d:\ngot  %v\nwant %v", stride, restored, original)
		}
	}
}

func TestInterleaveWithRemainder(t *testing.T) {
	// 5 bytes with stride 2: 2 full elements + 1 remainder
	data := []byte{1, 2, 3, 4, 5}
	out := Interleave(data, 2, nil)
	// Should interleave [1,2,3,4] -> [1,3,2,4] and copy remainder [5]
	expected := []byte{1, 3, 2, 4, 5}
	if !bytes.Equal(out, expected) {
		t.Errorf("Interleave with remainder:\ngot  %v\nwant %v", out, expected)
	}
}

func TestDeinterleaveWithRemainder(t *testing.T) {
	data := []byte{1, 3, 2, 4, 5}
	out := Deinterleave(data, 2, nil)
	expected := []byte{1, 2, 3, 4, 5}
	if !bytes.Equal(out, expected) {
		t.Errorf("Deinterleave with remainder:\ngot  %v\nwant %v", out, expected)
	}
}

func TestInterleaveInPlace(t *testing.T) {
	data := []byte{0x10, 0x11, 0x20, 0x21, 0x30, 0x31, 0x40, 0x41}
	expected := []byte{0x10, 0x20, 0x30, 0x40, 0x11, 0x21, 0x31, 0x41}

	InterleaveInPlace(data, 2)
	if !bytes.Equal(data, expected) {
		t.Errorf("InterleaveInPlace:\ngot  %v\nwant %v", data, expected)
	}
}

func TestDeinterleaveInPlace(t *testing.T) {
	data := []byte{0x10, 0x20, 0x30, 0x40, 0x11, 0x21, 0x31, 0x41}
	expected := []byte{0x10, 0x11, 0x20, 0x21, 0x30, 0x31, 0x40, 0x41}

	DeinterleaveInPlace(data, 2)
	if !bytes.Equal(data, expected) {
		t.Errorf("DeinterleaveInPlace:\ngot  %v\nwant %v", data, expected)
	}
}

func TestInPlaceEmpty(t *testing.T) {
	// Should not panic on empty data
	var data []byte
	InterleaveInPlace(data, 2)
	DeinterleaveInPlace(data, 2)
}

func TestInPlaceStride1(t *testing.T) {
	data := []byte{1, 2, 3, 4}
	original := make([]byte, len(data))
	copy(original, data)

	InterleaveInPlace(data, 1)
	if !bytes.Equal(data, original) {
		t.Error("InterleaveInPlace with stride 1 should not modify data")
	}

	DeinterleaveInPlace(data, 1)
	if !bytes.Equal(data, original) {
		t.Error("DeinterleaveInPlace with stride 1 should not modify data")
	}
}

func TestWithProvidedBuffer(t *testing.T) {
	data := []byte{0x10, 0x11, 0x20, 0x21}
	out := make([]byte, 4)

	result := Interleave(data, 2, out)
	if &result[0] != &out[0] {
		t.Error("Should use provided buffer")
	}
}

func BenchmarkInterleave(b *testing.B) {
	// Simulate 1920 pixels * 3 channels * 2 bytes (half) = 11520 bytes per scanline
	data := make([]byte, 1920*3*2)
	out := make([]byte, len(data))

	b.ResetTimer()
	b.SetBytes(int64(len(data)))

	for i := 0; i < b.N; i++ {
		Interleave(data, 2, out)
	}
}

func BenchmarkDeinterleave(b *testing.B) {
	data := make([]byte, 1920*3*2)
	out := make([]byte, len(data))

	b.ResetTimer()
	b.SetBytes(int64(len(data)))

	for i := 0; i < b.N; i++ {
		Deinterleave(data, 2, out)
	}
}
