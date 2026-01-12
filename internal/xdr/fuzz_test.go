package xdr

import (
	"bytes"
	"testing"
)

// FuzzReaderReadString tests string reading with arbitrary data.
func FuzzReaderReadString(f *testing.F) {
	// Valid null-terminated strings
	f.Add([]byte("hello\x00"))
	f.Add([]byte("\x00")) // Empty string
	f.Add([]byte("test\x00more\x00"))

	// Malicious inputs
	f.Add([]byte{})                         // Empty - no null terminator
	f.Add(bytes.Repeat([]byte{'A'}, 10000)) // Long without null
	f.Add([]byte{0xff, 0xff, 0xff, 0xff})   // Binary garbage

	f.Fuzz(func(t *testing.T, data []byte) {
		r := NewReader(data)

		// Should not panic, may return error
		s, err := r.ReadString()
		if err != nil {
			return
		}

		// If successful, string should not contain null
		for i := 0; i < len(s); i++ {
			if s[i] == 0 {
				t.Errorf("string contains null byte at position %d", i)
			}
		}
	})
}

// FuzzReaderReadInt tests integer reading.
func FuzzReaderReadInt(f *testing.F) {
	f.Add([]byte{0x00, 0x00, 0x00, 0x00})
	f.Add([]byte{0xff, 0xff, 0xff, 0xff})
	f.Add([]byte{0x01, 0x00, 0x00, 0x00})
	f.Add([]byte{0x00, 0x00, 0x00, 0x80}) // Min int32

	f.Fuzz(func(t *testing.T, data []byte) {
		r := NewReader(data)

		// Try all integer read functions
		_, _ = r.ReadInt8()
		r.Reset()
		_, _ = r.ReadUint8()
		r.Reset()
		_, _ = r.ReadInt16()
		r.Reset()
		_, _ = r.ReadUint16()
		r.Reset()
		_, _ = r.ReadInt32()
		r.Reset()
		_, _ = r.ReadUint32()
		r.Reset()
		_, _ = r.ReadInt64()
		r.Reset()
		_, _ = r.ReadUint64()
	})
}

// FuzzReaderReadFloat tests float reading.
func FuzzReaderReadFloat(f *testing.F) {
	f.Add([]byte{0x00, 0x00, 0x00, 0x00})                         // 0.0f
	f.Add([]byte{0x00, 0x00, 0x80, 0x3f})                         // 1.0f
	f.Add([]byte{0x00, 0x00, 0x80, 0x7f})                         // +Inf
	f.Add([]byte{0x00, 0x00, 0xc0, 0x7f})                         // NaN
	f.Add([]byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0xf0, 0x3f}) // 1.0 double

	f.Fuzz(func(t *testing.T, data []byte) {
		r := NewReader(data)

		_, _ = r.ReadFloat32()
		r.Reset()
		_, _ = r.ReadFloat64()
	})
}

// FuzzReaderReadBytes tests byte slice reading.
func FuzzReaderReadBytes(f *testing.F) {
	f.Add([]byte{}, 0)
	f.Add([]byte{0x01, 0x02, 0x03}, 2)
	f.Add([]byte{0x01, 0x02, 0x03}, 100) // Request more than available
	f.Add(bytes.Repeat([]byte{0xaa}, 1000), 500)

	f.Fuzz(func(t *testing.T, data []byte, n int) {
		if n < 0 {
			n = 0
		}
		if n > 1000000 {
			n = 1000000 // Limit allocation
		}

		r := NewReader(data)
		_, _ = r.ReadBytes(n)
	})
}

// FuzzReaderPositioning tests seek/skip operations.
func FuzzReaderPositioning(f *testing.F) {
	f.Add([]byte{0x01, 0x02, 0x03, 0x04}, 0, 2)
	f.Add([]byte{0x01, 0x02, 0x03, 0x04}, 4, 0)
	f.Add([]byte{0x01, 0x02, 0x03, 0x04}, -1, 10) // Invalid positions

	f.Fuzz(func(t *testing.T, data []byte, pos, skip int) {
		r := NewReader(data)

		// SetPos should handle invalid positions gracefully
		_ = r.SetPos(pos)

		// Skip should handle invalid amounts gracefully
		_ = r.Skip(skip)

		// Reading after positioning should not panic
		_, _ = r.ReadByte()
	})
}

// FuzzWriterRoundtrip tests write/read roundtrip.
func FuzzWriterRoundtrip(f *testing.F) {
	f.Add(int32(0), uint32(0), float32(0), float64(0), "test")
	f.Add(int32(-1), uint32(0xffffffff), float32(1.5), float64(-2.5), "")
	f.Add(int32(0x7fffffff), uint32(0), float32(0), float64(0), "hello\x00world")

	f.Fuzz(func(t *testing.T, i32 int32, u32 uint32, f32 float32, f64 float64, str string) {
		// Remove null bytes from string for valid test
		cleanStr := ""
		for _, c := range str {
			if c != 0 {
				cleanStr += string(c)
			}
		}

		w := NewBufferWriter(256)

		// Write values
		w.WriteInt32(i32)
		w.WriteUint32(u32)
		w.WriteFloat32(f32)
		w.WriteFloat64(f64)
		w.WriteString(cleanStr)

		// Read back
		r := NewReader(w.Bytes())

		ri32, err := r.ReadInt32()
		if err != nil {
			t.Fatalf("ReadInt32 failed: %v", err)
		}
		if ri32 != i32 {
			t.Errorf("int32 mismatch: got %d, want %d", ri32, i32)
		}

		ru32, err := r.ReadUint32()
		if err != nil {
			t.Fatalf("ReadUint32 failed: %v", err)
		}
		if ru32 != u32 {
			t.Errorf("uint32 mismatch: got %d, want %d", ru32, u32)
		}

		rf32, err := r.ReadFloat32()
		if err != nil {
			t.Fatalf("ReadFloat32 failed: %v", err)
		}
		// Float comparison with NaN handling
		if rf32 != f32 && !(rf32 != rf32 && f32 != f32) {
			t.Errorf("float32 mismatch: got %v, want %v", rf32, f32)
		}

		rf64, err := r.ReadFloat64()
		if err != nil {
			t.Fatalf("ReadFloat64 failed: %v", err)
		}
		if rf64 != f64 && !(rf64 != rf64 && f64 != f64) {
			t.Errorf("float64 mismatch: got %v, want %v", rf64, f64)
		}

		rstr, err := r.ReadString()
		if err != nil {
			t.Fatalf("ReadString failed: %v", err)
		}
		if rstr != cleanStr {
			t.Errorf("string mismatch: got %q, want %q", rstr, cleanStr)
		}
	})
}

// FuzzReaderEdgeCases tests edge cases in reader.
func FuzzReaderEdgeCases(f *testing.F) {
	// Empty reader
	f.Add([]byte{})

	// Exactly sized buffers
	f.Add([]byte{0x00})
	f.Add([]byte{0x00, 0x00})
	f.Add([]byte{0x00, 0x00, 0x00, 0x00})
	f.Add([]byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00})

	f.Fuzz(func(t *testing.T, data []byte) {
		r := NewReader(data)

		// Multiple reads should eventually fail gracefully
		for i := 0; i < 100; i++ {
			_, err := r.ReadByte()
			if err != nil {
				break
			}
		}

		// Len should always be non-negative
		if r.Len() < 0 {
			t.Errorf("Len returned negative: %d", r.Len())
		}

		// Pos should be within bounds
		if r.Pos() < 0 || r.Pos() > len(data) {
			t.Errorf("Pos out of bounds: %d (data len: %d)", r.Pos(), len(data))
		}
	})
}
