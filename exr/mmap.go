//go:build !windows
// +build !windows

package exr

import (
	"os"
	"syscall"
)

// mmapReader provides zero-copy file access via memory mapping.
type mmapReader struct {
	data []byte
	file *os.File
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

	data, err := syscall.Mmap(int(f.Fd()), 0, int(size), syscall.PROT_READ, syscall.MAP_SHARED)
	if err != nil {
		return nil, err
	}

	return &mmapReader{data: data, file: f}, nil
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
// The returned slice is only valid while the mmapReader is open.
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
		if err := syscall.Munmap(m.data); err != nil {
			return err
		}
		m.data = nil
	}
	if m.file != nil {
		return m.file.Close()
	}
	return nil
}
