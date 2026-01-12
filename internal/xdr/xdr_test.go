package xdr

import (
	"bytes"
	"io"
	"math"
	"testing"
)

func TestReaderBasic(t *testing.T) {
	data := []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08}
	r := NewReader(data)

	if r.Len() != 8 {
		t.Errorf("Len() = %d, want 8", r.Len())
	}
	if r.Pos() != 0 {
		t.Errorf("Pos() = %d, want 0", r.Pos())
	}

	b, err := r.ReadByte()
	if err != nil {
		t.Errorf("ReadByte() error = %v", err)
	}
	if b != 0x01 {
		t.Errorf("ReadByte() = %d, want 1", b)
	}

	if r.Pos() != 1 {
		t.Errorf("Pos() after ReadByte = %d, want 1", r.Pos())
	}
}

func TestReaderIntegers(t *testing.T) {
	// Little-endian test data
	data := []byte{
		0x34, 0x12, // uint16: 0x1234
		0x78, 0x56, 0x34, 0x12, // uint32: 0x12345678
		0xEF, 0xCD, 0xAB, 0x89, 0x67, 0x45, 0x23, 0x01, // uint64: 0x0123456789ABCDEF
	}
	r := NewReader(data)

	u16, err := r.ReadUint16()
	if err != nil {
		t.Fatalf("ReadUint16() error = %v", err)
	}
	if u16 != 0x1234 {
		t.Errorf("ReadUint16() = 0x%04X, want 0x1234", u16)
	}

	u32, err := r.ReadUint32()
	if err != nil {
		t.Fatalf("ReadUint32() error = %v", err)
	}
	if u32 != 0x12345678 {
		t.Errorf("ReadUint32() = 0x%08X, want 0x12345678", u32)
	}

	u64, err := r.ReadUint64()
	if err != nil {
		t.Fatalf("ReadUint64() error = %v", err)
	}
	if u64 != 0x0123456789ABCDEF {
		t.Errorf("ReadUint64() = 0x%016X, want 0x0123456789ABCDEF", u64)
	}
}

func TestReaderSignedIntegers(t *testing.T) {
	// Test signed integers (negative values)
	data := []byte{
		0xFF,       // int8: -1
		0xFE, 0xFF, // int16: -2
		0xFD, 0xFF, 0xFF, 0xFF, // int32: -3
		0xFC, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, // int64: -4
	}
	r := NewReader(data)

	i8, _ := r.ReadInt8()
	if i8 != -1 {
		t.Errorf("ReadInt8() = %d, want -1", i8)
	}

	i16, _ := r.ReadInt16()
	if i16 != -2 {
		t.Errorf("ReadInt16() = %d, want -2", i16)
	}

	i32, _ := r.ReadInt32()
	if i32 != -3 {
		t.Errorf("ReadInt32() = %d, want -3", i32)
	}

	i64, _ := r.ReadInt64()
	if i64 != -4 {
		t.Errorf("ReadInt64() = %d, want -4", i64)
	}
}

func TestReaderFloats(t *testing.T) {
	// Create test data with known float values
	buf := make([]byte, 12)
	ByteOrder.PutUint32(buf[0:4], math.Float32bits(3.14))
	ByteOrder.PutUint64(buf[4:12], math.Float64bits(2.71828))

	r := NewReader(buf)

	f32, err := r.ReadFloat32()
	if err != nil {
		t.Fatalf("ReadFloat32() error = %v", err)
	}
	if f32 != 3.14 {
		t.Errorf("ReadFloat32() = %v, want 3.14", f32)
	}

	f64, err := r.ReadFloat64()
	if err != nil {
		t.Fatalf("ReadFloat64() error = %v", err)
	}
	if f64 != 2.71828 {
		t.Errorf("ReadFloat64() = %v, want 2.71828", f64)
	}
}

func TestReaderString(t *testing.T) {
	data := []byte{'h', 'e', 'l', 'l', 'o', 0, 'w', 'o', 'r', 'l', 'd', 0}
	r := NewReader(data)

	s1, err := r.ReadString()
	if err != nil {
		t.Fatalf("ReadString() error = %v", err)
	}
	if s1 != "hello" {
		t.Errorf("ReadString() = %q, want %q", s1, "hello")
	}

	s2, err := r.ReadString()
	if err != nil {
		t.Fatalf("ReadString() error = %v", err)
	}
	if s2 != "world" {
		t.Errorf("ReadString() = %q, want %q", s2, "world")
	}
}

func TestReaderStringN(t *testing.T) {
	// String with null terminator within n bytes
	data := []byte{'h', 'i', 0, 'x', 'y', 'z'}
	r := NewReader(data)

	s, err := r.ReadStringN(6)
	if err != nil {
		t.Fatalf("ReadStringN() error = %v", err)
	}
	if s != "hi" {
		t.Errorf("ReadStringN() = %q, want %q", s, "hi")
	}
	if r.Pos() != 6 {
		t.Errorf("Pos() after ReadStringN(6) = %d, want 6", r.Pos())
	}

	// String without null terminator
	r.Reset()
	data2 := []byte{'a', 'b', 'c', 'd'}
	r2 := NewReader(data2)
	s2, err := r2.ReadStringN(4)
	if err != nil {
		t.Fatalf("ReadStringN() error = %v", err)
	}
	if s2 != "abcd" {
		t.Errorf("ReadStringN() = %q, want %q", s2, "abcd")
	}
}

func TestReaderBytes(t *testing.T) {
	data := []byte{1, 2, 3, 4, 5}
	r := NewReader(data)

	b, err := r.ReadBytes(3)
	if err != nil {
		t.Fatalf("ReadBytes() error = %v", err)
	}
	if !bytes.Equal(b, []byte{1, 2, 3}) {
		t.Errorf("ReadBytes(3) = %v, want [1 2 3]", b)
	}

	// ReadBytesInto
	dst := make([]byte, 2)
	err = r.ReadBytesInto(dst)
	if err != nil {
		t.Fatalf("ReadBytesInto() error = %v", err)
	}
	if !bytes.Equal(dst, []byte{4, 5}) {
		t.Errorf("ReadBytesInto() = %v, want [4 5]", dst)
	}
}

func TestReaderErrors(t *testing.T) {
	r := NewReader([]byte{1, 2})

	// ReadUint32 on short buffer
	_, err := r.ReadUint32()
	if err != ErrShortBuffer {
		t.Errorf("ReadUint32() error = %v, want ErrShortBuffer", err)
	}

	// ReadBytes with negative size
	_, err = r.ReadBytes(-1)
	if err != ErrNegativeSize {
		t.Errorf("ReadBytes(-1) error = %v, want ErrNegativeSize", err)
	}

	// Skip with negative
	err = r.Skip(-1)
	if err != ErrNegativeSize {
		t.Errorf("Skip(-1) error = %v, want ErrNegativeSize", err)
	}

	// Skip past end
	err = r.Skip(100)
	if err != ErrShortBuffer {
		t.Errorf("Skip(100) error = %v, want ErrShortBuffer", err)
	}

	// SetPos out of bounds
	err = r.SetPos(-1)
	if err != ErrShortBuffer {
		t.Errorf("SetPos(-1) error = %v, want ErrShortBuffer", err)
	}
	err = r.SetPos(100)
	if err != ErrShortBuffer {
		t.Errorf("SetPos(100) error = %v, want ErrShortBuffer", err)
	}

	// ReadString without null terminator
	r2 := NewReader([]byte{'a', 'b', 'c'})
	_, err = r2.ReadString()
	if err != ErrShortBuffer {
		t.Errorf("ReadString() without null error = %v, want ErrShortBuffer", err)
	}

	// ReadStringN negative
	_, err = r2.ReadStringN(-1)
	if err != ErrNegativeSize {
		t.Errorf("ReadStringN(-1) error = %v, want ErrNegativeSize", err)
	}

	// ReadUint8 on empty reader
	r3 := NewReader([]byte{})
	_, err = r3.ReadUint8()
	if err != ErrShortBuffer {
		t.Errorf("ReadUint8() on empty error = %v, want ErrShortBuffer", err)
	}
}

func TestReaderReset(t *testing.T) {
	r := NewReader([]byte{1, 2, 3})
	r.ReadByte()
	r.ReadByte()
	if r.Pos() != 2 {
		t.Errorf("Pos() = %d, want 2", r.Pos())
	}
	r.Reset()
	if r.Pos() != 0 {
		t.Errorf("Pos() after Reset = %d, want 0", r.Pos())
	}
}

func TestWriterBasic(t *testing.T) {
	buf := make([]byte, 16)
	w := NewWriter(buf)

	if w.Len() != 16 {
		t.Errorf("Len() = %d, want 16", w.Len())
	}
	if w.Pos() != 0 {
		t.Errorf("Pos() = %d, want 0", w.Pos())
	}

	err := w.WriteByte(0x42)
	if err != nil {
		t.Errorf("WriteByte() error = %v", err)
	}
	if buf[0] != 0x42 {
		t.Errorf("buf[0] = %d, want 0x42", buf[0])
	}
}

func TestWriterIntegers(t *testing.T) {
	buf := make([]byte, 14)
	w := NewWriter(buf)

	w.WriteUint16(0x1234)
	w.WriteUint32(0x12345678)
	w.WriteUint64(0x0123456789ABCDEF)

	// Verify little-endian encoding
	expected := []byte{
		0x34, 0x12,
		0x78, 0x56, 0x34, 0x12,
		0xEF, 0xCD, 0xAB, 0x89, 0x67, 0x45, 0x23, 0x01,
	}
	if !bytes.Equal(buf, expected) {
		t.Errorf("buf = %v, want %v", buf, expected)
	}
}

func TestWriterSignedIntegers(t *testing.T) {
	buf := make([]byte, 15)
	w := NewWriter(buf)

	w.WriteInt8(-1)
	w.WriteInt16(-2)
	w.WriteInt32(-3)
	w.WriteInt64(-4)

	expected := []byte{
		0xFF,
		0xFE, 0xFF,
		0xFD, 0xFF, 0xFF, 0xFF,
		0xFC, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF,
	}
	if !bytes.Equal(buf, expected) {
		t.Errorf("buf = %v, want %v", buf, expected)
	}
}

func TestWriterFloats(t *testing.T) {
	buf := make([]byte, 12)
	w := NewWriter(buf)

	w.WriteFloat32(3.14)
	w.WriteFloat64(2.71828)

	// Read back and verify
	r := NewReader(buf)
	f32, _ := r.ReadFloat32()
	f64, _ := r.ReadFloat64()

	if f32 != 3.14 {
		t.Errorf("WriteFloat32/Read = %v, want 3.14", f32)
	}
	if f64 != 2.71828 {
		t.Errorf("WriteFloat64/Read = %v, want 2.71828", f64)
	}
}

func TestWriterString(t *testing.T) {
	buf := make([]byte, 12)
	w := NewWriter(buf)

	err := w.WriteString("hello")
	if err != nil {
		t.Fatalf("WriteString() error = %v", err)
	}

	expected := []byte{'h', 'e', 'l', 'l', 'o', 0}
	if !bytes.Equal(buf[:6], expected) {
		t.Errorf("buf = %v, want %v", buf[:6], expected)
	}
}

func TestWriterStringN(t *testing.T) {
	buf := make([]byte, 8)
	w := NewWriter(buf)

	// Short string padded with nulls
	err := w.WriteStringN("hi", 4)
	if err != nil {
		t.Fatalf("WriteStringN() error = %v", err)
	}
	if !bytes.Equal(buf[:4], []byte{'h', 'i', 0, 0}) {
		t.Errorf("buf[:4] = %v, want [h i 0 0]", buf[:4])
	}

	// Long string truncated
	err = w.WriteStringN("toolong", 4)
	if err != nil {
		t.Fatalf("WriteStringN() error = %v", err)
	}
	if !bytes.Equal(buf[4:8], []byte{'t', 'o', 'o', 'l'}) {
		t.Errorf("buf[4:8] = %v, want [t o o l]", buf[4:8])
	}
}

func TestWriterBytes(t *testing.T) {
	buf := make([]byte, 8)
	w := NewWriter(buf)

	err := w.WriteBytes([]byte{1, 2, 3, 4})
	if err != nil {
		t.Fatalf("WriteBytes() error = %v", err)
	}
	if !bytes.Equal(buf[:4], []byte{1, 2, 3, 4}) {
		t.Errorf("buf[:4] = %v, want [1 2 3 4]", buf[:4])
	}
}

func TestWriterErrors(t *testing.T) {
	w := NewWriter(make([]byte, 2))

	// Write too much data
	err := w.WriteUint32(0)
	if err != ErrShortBuffer {
		t.Errorf("WriteUint32() error = %v, want ErrShortBuffer", err)
	}

	// Skip with negative
	err = w.Skip(-1)
	if err != ErrNegativeSize {
		t.Errorf("Skip(-1) error = %v, want ErrNegativeSize", err)
	}

	// Skip past end
	err = w.Skip(100)
	if err != ErrShortBuffer {
		t.Errorf("Skip(100) error = %v, want ErrShortBuffer", err)
	}

	// SetPos out of bounds
	err = w.SetPos(-1)
	if err != ErrShortBuffer {
		t.Errorf("SetPos(-1) error = %v, want ErrShortBuffer", err)
	}

	// WriteStringN negative
	err = w.WriteStringN("x", -1)
	if err != ErrNegativeSize {
		t.Errorf("WriteStringN(-1) error = %v, want ErrNegativeSize", err)
	}

	// WriteByte when full
	w2 := NewWriter(make([]byte, 0))
	err = w2.WriteByte(1)
	if err != ErrShortBuffer {
		t.Errorf("WriteByte() when full error = %v, want ErrShortBuffer", err)
	}

	// WriteBytes when full
	w3 := NewWriter(make([]byte, 1))
	err = w3.WriteBytes([]byte{1, 2})
	if err != ErrShortBuffer {
		t.Errorf("WriteBytes() overflow error = %v, want ErrShortBuffer", err)
	}

	// WriteString too long
	w4 := NewWriter(make([]byte, 3))
	err = w4.WriteString("toolong")
	if err != ErrShortBuffer {
		t.Errorf("WriteString() overflow error = %v, want ErrShortBuffer", err)
	}

	// WriteStringN too long
	w5 := NewWriter(make([]byte, 2))
	err = w5.WriteStringN("x", 5)
	if err != ErrShortBuffer {
		t.Errorf("WriteStringN() overflow error = %v, want ErrShortBuffer", err)
	}
}

func TestWriterReset(t *testing.T) {
	buf := make([]byte, 8)
	w := NewWriter(buf)
	w.WriteUint32(0)
	if w.Pos() != 4 {
		t.Errorf("Pos() = %d, want 4", w.Pos())
	}
	w.Reset()
	if w.Pos() != 0 {
		t.Errorf("Pos() after Reset = %d, want 0", w.Pos())
	}
}

func TestBufferWriter(t *testing.T) {
	w := NewBufferWriter(16)

	if w.Len() != 0 {
		t.Errorf("Len() = %d, want 0", w.Len())
	}

	w.WriteUint32(0x12345678)
	w.WriteFloat32(3.14)
	w.WriteString("hi")

	if w.Len() != 4+4+3 {
		t.Errorf("Len() = %d, want 11", w.Len())
	}

	// Verify contents
	r := NewReader(w.Bytes())
	u32, _ := r.ReadUint32()
	f32, _ := r.ReadFloat32()
	s, _ := r.ReadString()

	if u32 != 0x12345678 {
		t.Errorf("ReadUint32() = 0x%08X, want 0x12345678", u32)
	}
	if f32 != 3.14 {
		t.Errorf("ReadFloat32() = %v, want 3.14", f32)
	}
	if s != "hi" {
		t.Errorf("ReadString() = %q, want %q", s, "hi")
	}

	// Test Reset
	w.Reset()
	if w.Len() != 0 {
		t.Errorf("Len() after Reset = %d, want 0", w.Len())
	}
}

func TestBufferWriterAllTypes(t *testing.T) {
	w := NewBufferWriter(64)

	w.WriteByte(1)
	w.WriteBytes([]byte{2, 3})
	w.WriteUint8(4)
	w.WriteInt8(-1)
	w.WriteUint16(0x1234)
	w.WriteInt16(-2)
	w.WriteUint32(0x12345678)
	w.WriteInt32(-3)
	w.WriteUint64(0x0123456789ABCDEF)
	w.WriteInt64(-4)
	w.WriteFloat32(3.14)
	w.WriteFloat64(2.718)
	w.WriteString("x")
	w.WriteStringN("ab", 4)

	// Verify round-trip
	r := NewReader(w.Bytes())

	b, _ := r.ReadByte()
	if b != 1 {
		t.Errorf("ReadByte() = %d, want 1", b)
	}

	bs, _ := r.ReadBytes(2)
	if !bytes.Equal(bs, []byte{2, 3}) {
		t.Errorf("ReadBytes() = %v, want [2 3]", bs)
	}

	u8, _ := r.ReadUint8()
	if u8 != 4 {
		t.Errorf("ReadUint8() = %d, want 4", u8)
	}

	i8, _ := r.ReadInt8()
	if i8 != -1 {
		t.Errorf("ReadInt8() = %d, want -1", i8)
	}

	u16, _ := r.ReadUint16()
	if u16 != 0x1234 {
		t.Errorf("ReadUint16() = 0x%04X, want 0x1234", u16)
	}

	i16, _ := r.ReadInt16()
	if i16 != -2 {
		t.Errorf("ReadInt16() = %d, want -2", i16)
	}

	u32, _ := r.ReadUint32()
	if u32 != 0x12345678 {
		t.Errorf("ReadUint32() = 0x%08X, want 0x12345678", u32)
	}

	i32, _ := r.ReadInt32()
	if i32 != -3 {
		t.Errorf("ReadInt32() = %d, want -3", i32)
	}

	u64, _ := r.ReadUint64()
	if u64 != 0x0123456789ABCDEF {
		t.Errorf("ReadUint64() = 0x%016X, want 0x0123456789ABCDEF", u64)
	}

	i64, _ := r.ReadInt64()
	if i64 != -4 {
		t.Errorf("ReadInt64() = %d, want -4", i64)
	}

	f32, _ := r.ReadFloat32()
	if f32 != 3.14 {
		t.Errorf("ReadFloat32() = %v, want 3.14", f32)
	}

	f64, _ := r.ReadFloat64()
	if f64 != 2.718 {
		t.Errorf("ReadFloat64() = %v, want 2.718", f64)
	}

	s, _ := r.ReadString()
	if s != "x" {
		t.Errorf("ReadString() = %q, want %q", s, "x")
	}

	sn, _ := r.ReadStringN(4)
	if sn != "ab" {
		t.Errorf("ReadStringN() = %q, want %q", sn, "ab")
	}
}

func TestStreamReader(t *testing.T) {
	data := []byte{
		0x34, 0x12, // uint16
		0x78, 0x56, 0x34, 0x12, // uint32
		0xEF, 0xCD, 0xAB, 0x89, 0x67, 0x45, 0x23, 0x01, // uint64
		0xFF,       // int8: -1
		0xFE, 0xFF, // int16: -2
		0xFD, 0xFF, 0xFF, 0xFF, // int32: -3
		0xFC, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, // int64: -4
	}

	r := NewStreamReader(bytes.NewReader(data))

	u16, _ := r.ReadUint16()
	if u16 != 0x1234 {
		t.Errorf("ReadUint16() = 0x%04X, want 0x1234", u16)
	}

	u32, _ := r.ReadUint32()
	if u32 != 0x12345678 {
		t.Errorf("ReadUint32() = 0x%08X, want 0x12345678", u32)
	}

	u64, _ := r.ReadUint64()
	if u64 != 0x0123456789ABCDEF {
		t.Errorf("ReadUint64() = 0x%016X, want 0x0123456789ABCDEF", u64)
	}

	i8, _ := r.ReadInt8()
	if i8 != -1 {
		t.Errorf("ReadInt8() = %d, want -1", i8)
	}

	i16, _ := r.ReadInt16()
	if i16 != -2 {
		t.Errorf("ReadInt16() = %d, want -2", i16)
	}

	i32, _ := r.ReadInt32()
	if i32 != -3 {
		t.Errorf("ReadInt32() = %d, want -3", i32)
	}

	i64, _ := r.ReadInt64()
	if i64 != -4 {
		t.Errorf("ReadInt64() = %d, want -4", i64)
	}
}

func TestStreamReaderFloats(t *testing.T) {
	buf := make([]byte, 12)
	ByteOrder.PutUint32(buf[0:4], math.Float32bits(3.14))
	ByteOrder.PutUint64(buf[4:12], math.Float64bits(2.71828))

	r := NewStreamReader(bytes.NewReader(buf))

	f32, _ := r.ReadFloat32()
	if f32 != 3.14 {
		t.Errorf("ReadFloat32() = %v, want 3.14", f32)
	}

	f64, _ := r.ReadFloat64()
	if f64 != 2.71828 {
		t.Errorf("ReadFloat64() = %v, want 2.71828", f64)
	}
}

func TestStreamReaderBytes(t *testing.T) {
	data := []byte{1, 2, 3, 4, 5}
	r := NewStreamReader(bytes.NewReader(data))

	b, _ := r.ReadByte()
	if b != 1 {
		t.Errorf("ReadByte() = %d, want 1", b)
	}

	u8, _ := r.ReadUint8()
	if u8 != 2 {
		t.Errorf("ReadUint8() = %d, want 2", u8)
	}

	bs, _ := r.ReadBytes(2)
	if !bytes.Equal(bs, []byte{3, 4}) {
		t.Errorf("ReadBytes() = %v, want [3 4]", bs)
	}

	dst := make([]byte, 1)
	r.ReadBytesInto(dst)
	if dst[0] != 5 {
		t.Errorf("ReadBytesInto() = %d, want 5", dst[0])
	}
}

func TestStreamReaderErrors(t *testing.T) {
	r := NewStreamReader(bytes.NewReader([]byte{}))

	_, err := r.ReadByte()
	if err != io.EOF {
		t.Errorf("ReadByte() error = %v, want EOF", err)
	}

	_, err = r.ReadBytes(-1)
	if err != ErrNegativeSize {
		t.Errorf("ReadBytes(-1) error = %v, want ErrNegativeSize", err)
	}
}

func TestStreamWriter(t *testing.T) {
	var buf bytes.Buffer
	w := NewStreamWriter(&buf)

	w.WriteUint16(0x1234)
	w.WriteUint32(0x12345678)
	w.WriteUint64(0x0123456789ABCDEF)
	w.WriteInt8(-1)
	w.WriteInt16(-2)
	w.WriteInt32(-3)
	w.WriteInt64(-4)
	w.WriteFloat32(3.14)
	w.WriteFloat64(2.718)
	w.WriteByte(0x42)
	w.WriteUint8(0x43)
	w.WriteBytes([]byte{0x44, 0x45})
	w.WriteString("hi")

	// Verify by reading back
	r := NewStreamReader(bytes.NewReader(buf.Bytes()))

	u16, _ := r.ReadUint16()
	if u16 != 0x1234 {
		t.Errorf("ReadUint16() = 0x%04X, want 0x1234", u16)
	}

	u32, _ := r.ReadUint32()
	if u32 != 0x12345678 {
		t.Errorf("ReadUint32() = 0x%08X, want 0x12345678", u32)
	}

	u64, _ := r.ReadUint64()
	if u64 != 0x0123456789ABCDEF {
		t.Errorf("ReadUint64() = 0x%016X, want 0x0123456789ABCDEF", u64)
	}

	i8, _ := r.ReadInt8()
	if i8 != -1 {
		t.Errorf("ReadInt8() = %d, want -1", i8)
	}

	i16, _ := r.ReadInt16()
	if i16 != -2 {
		t.Errorf("ReadInt16() = %d, want -2", i16)
	}

	i32, _ := r.ReadInt32()
	if i32 != -3 {
		t.Errorf("ReadInt32() = %d, want -3", i32)
	}

	i64, _ := r.ReadInt64()
	if i64 != -4 {
		t.Errorf("ReadInt64() = %d, want -4", i64)
	}

	f32, _ := r.ReadFloat32()
	if f32 != 3.14 {
		t.Errorf("ReadFloat32() = %v, want 3.14", f32)
	}

	f64, _ := r.ReadFloat64()
	if f64 != 2.718 {
		t.Errorf("ReadFloat64() = %v, want 2.718", f64)
	}

	b, _ := r.ReadByte()
	if b != 0x42 {
		t.Errorf("ReadByte() = 0x%02X, want 0x42", b)
	}

	u8, _ := r.ReadUint8()
	if u8 != 0x43 {
		t.Errorf("ReadUint8() = 0x%02X, want 0x43", u8)
	}

	bs, _ := r.ReadBytes(2)
	if !bytes.Equal(bs, []byte{0x44, 0x45}) {
		t.Errorf("ReadBytes() = %v, want [0x44 0x45]", bs)
	}
}

func TestRoundTrip(t *testing.T) {
	// Test that writing and reading produces the same values
	tests := []struct {
		name  string
		value interface{}
	}{
		{"uint16 max", uint16(0xFFFF)},
		{"uint32 max", uint32(0xFFFFFFFF)},
		{"uint64 max", uint64(0xFFFFFFFFFFFFFFFF)},
		{"int16 min", int16(-32768)},
		{"int32 min", int32(-2147483648)},
		{"int64 min", int64(-9223372036854775808)},
		{"float32 special", float32(math.Inf(1))},
		{"float64 special", math.Inf(-1)},
		{"float32 nan", float32(math.NaN())},
		{"float64 nan", math.NaN()},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := NewBufferWriter(16)
			var r *Reader
			var got interface{}

			switch v := tt.value.(type) {
			case uint16:
				w.WriteUint16(v)
				r = NewReader(w.Bytes())
				got, _ = r.ReadUint16()
			case uint32:
				w.WriteUint32(v)
				r = NewReader(w.Bytes())
				got, _ = r.ReadUint32()
			case uint64:
				w.WriteUint64(v)
				r = NewReader(w.Bytes())
				got, _ = r.ReadUint64()
			case int16:
				w.WriteInt16(v)
				r = NewReader(w.Bytes())
				got, _ = r.ReadInt16()
			case int32:
				w.WriteInt32(v)
				r = NewReader(w.Bytes())
				got, _ = r.ReadInt32()
			case int64:
				w.WriteInt64(v)
				r = NewReader(w.Bytes())
				got, _ = r.ReadInt64()
			case float32:
				w.WriteFloat32(v)
				r = NewReader(w.Bytes())
				got, _ = r.ReadFloat32()
			case float64:
				w.WriteFloat64(v)
				r = NewReader(w.Bytes())
				got, _ = r.ReadFloat64()
			}

			// Special handling for NaN
			switch v := tt.value.(type) {
			case float32:
				if math.IsNaN(float64(v)) {
					if !math.IsNaN(float64(got.(float32))) {
						t.Errorf("got %v, want NaN", got)
					}
					return
				}
			case float64:
				if math.IsNaN(v) {
					if !math.IsNaN(got.(float64)) {
						t.Errorf("got %v, want NaN", got)
					}
					return
				}
			}

			if got != tt.value {
				t.Errorf("got %v, want %v", got, tt.value)
			}
		})
	}
}

func TestReaderLen(t *testing.T) {
	r := NewReader([]byte{1, 2, 3})
	r.ReadByte()
	r.ReadByte()
	r.ReadByte()
	if r.Len() != 0 {
		t.Errorf("Len() at end = %d, want 0", r.Len())
	}
	// Read past end
	r.ReadByte()
	if r.Len() != 0 {
		t.Errorf("Len() past end = %d, want 0", r.Len())
	}
}

func TestWriterLen(t *testing.T) {
	w := NewWriter(make([]byte, 3))
	w.WriteByte(1)
	w.WriteByte(2)
	w.WriteByte(3)
	if w.Len() != 0 {
		t.Errorf("Len() at end = %d, want 0", w.Len())
	}
	// Write past end
	w.WriteByte(4)
	if w.Len() != 0 {
		t.Errorf("Len() past end = %d, want 0", w.Len())
	}
}

// Benchmarks

func BenchmarkReaderUint32(b *testing.B) {
	data := make([]byte, 4*b.N)
	r := NewReader(data)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r.ReadUint32()
	}
}

func BenchmarkWriterUint32(b *testing.B) {
	data := make([]byte, 4*b.N)
	w := NewWriter(data)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w.WriteUint32(uint32(i))
	}
}

func BenchmarkBufferWriterUint32(b *testing.B) {
	w := NewBufferWriter(4 * b.N)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w.WriteUint32(uint32(i))
	}
}

func BenchmarkStreamReaderUint32(b *testing.B) {
	data := make([]byte, 4*b.N)
	r := NewStreamReader(bytes.NewReader(data))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r.ReadUint32()
	}
}

func BenchmarkStreamWriterUint32(b *testing.B) {
	var buf bytes.Buffer
	buf.Grow(4 * b.N)
	w := NewStreamWriter(&buf)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w.WriteUint32(uint32(i))
	}
}

func TestWriterWriteUint8(t *testing.T) {
	data := make([]byte, 5)
	w := NewWriter(data)

	testValues := []uint8{0, 1, 127, 128, 255}
	for _, v := range testValues {
		err := w.WriteUint8(v)
		if err != nil {
			t.Fatalf("WriteUint8(%d) failed: %v", v, err)
		}
	}

	for i, v := range testValues {
		if data[i] != v {
			t.Errorf("byte[%d] = %d, want %d", i, data[i], v)
		}
	}
}

func TestBufferWriterWriteUint8(t *testing.T) {
	w := NewBufferWriter(5)

	testValues := []uint8{0, 1, 127, 128, 255}
	for _, v := range testValues {
		w.WriteUint8(v)
	}

	data := w.Bytes()
	if len(data) != len(testValues) {
		t.Fatalf("Expected %d bytes, got %d", len(testValues), len(data))
	}

	for i, v := range testValues {
		if data[i] != v {
			t.Errorf("byte[%d] = %d, want %d", i, data[i], v)
		}
	}
}

func TestStreamWriterWriteUint8(t *testing.T) {
	var buf bytes.Buffer
	w := NewStreamWriter(&buf)

	testValues := []uint8{0, 1, 127, 128, 255}
	for _, v := range testValues {
		err := w.WriteUint8(v)
		if err != nil {
			t.Fatalf("WriteUint8(%d) failed: %v", v, err)
		}
	}

	data := buf.Bytes()
	if len(data) != len(testValues) {
		t.Fatalf("Expected %d bytes, got %d", len(testValues), len(data))
	}

	for i, v := range testValues {
		if data[i] != v {
			t.Errorf("byte[%d] = %d, want %d", i, data[i], v)
		}
	}
}
