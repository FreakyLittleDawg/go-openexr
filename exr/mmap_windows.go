//go:build windows
// +build windows

package exr

import (
	"os"
	"syscall"
	"unsafe"
)

// mmapReader provides zero-copy file access via memory mapping.
type mmapReader struct {
	data   []byte
	file   *os.File
	handle syscall.Handle
}

// newMmapReader creates a memory-mapped reader for the given file.
func newMmapReader(f *os.File) (*mmapReader, error) {
	fi, err := f.Stat()
	if err != nil {
		return nil, err
	}

	size := fi.Size()
	if size == 0 {
		return &mmapReader{data: nil, file: f}, nil
	}

	// Create file mapping
	sizeLow := uint32(size)
	sizeHigh := uint32(size >> 32)
	handle, err := syscall.CreateFileMapping(syscall.Handle(f.Fd()), nil, syscall.PAGE_READONLY, sizeHigh, sizeLow, nil)
	if err != nil {
		return nil, err
	}

	// Map view of file
	ptr, err := syscall.MapViewOfFile(handle, syscall.FILE_MAP_READ, 0, 0, uintptr(size))
	if err != nil {
		syscall.CloseHandle(handle)
		return nil, err
	}

	// Create slice from pointer
	data := (*[1 << 30]byte)(unsafe.Pointer(ptr))[:size:size]

	return &mmapReader{data: data, file: f, handle: handle}, nil
}

// ReadAt implements io.ReaderAt with zero-copy access to mmap'd data.
func (m *mmapReader) ReadAt(p []byte, off int64) (int, error) {
	if off < 0 || off >= int64(len(m.data)) {
		return 0, syscall.EINVAL
	}
	n := copy(p, m.data[off:])
	return n, nil
}

// Slice returns a direct slice into the mmap'd data (zero-copy).
func (m *mmapReader) Slice(off, length int64) []byte {
	if off < 0 || off+length > int64(len(m.data)) {
		return nil
	}
	return m.data[off : off+length]
}

// Size returns the size of the mapped file.
func (m *mmapReader) Size() int64 {
	return int64(len(m.data))
}

// Close unmaps the file and closes the underlying file handle.
func (m *mmapReader) Close() error {
	if m.data != nil {
		syscall.UnmapViewOfFile(uintptr(unsafe.Pointer(&m.data[0])))
		m.data = nil
	}
	if m.handle != 0 {
		syscall.CloseHandle(m.handle)
		m.handle = 0
	}
	if m.file != nil {
		return m.file.Close()
	}
	return nil
}
