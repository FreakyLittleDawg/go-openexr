//go:build !windows
// +build !windows

package exr

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMmapReaderSlice(t *testing.T) {
	// Create a temp file with known content
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.bin")

	testData := []byte("Hello, World! This is test data for mmap slice testing.")
	if err := os.WriteFile(path, testData, 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	// Open the file with mmap
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("Failed to open file: %v", err)
	}
	defer f.Close()

	mr, err := newMmapReader(f)
	if err != nil {
		t.Fatalf("newMmapReader error: %v", err)
	}
	defer mr.Close()

	// Test Slice method
	t.Run("ValidSlice", func(t *testing.T) {
		slice := mr.Slice(0, 5)
		if slice == nil {
			t.Fatal("Slice returned nil")
		}
		if string(slice) != "Hello" {
			t.Errorf("Slice = %q, want %q", string(slice), "Hello")
		}
	})

	t.Run("SliceMiddle", func(t *testing.T) {
		slice := mr.Slice(7, 5)
		if slice == nil {
			t.Fatal("Slice returned nil")
		}
		if string(slice) != "World" {
			t.Errorf("Slice = %q, want %q", string(slice), "World")
		}
	})

	t.Run("SliceNegativeOffset", func(t *testing.T) {
		slice := mr.Slice(-1, 5)
		if slice != nil {
			t.Error("Slice with negative offset should return nil")
		}
	})

	t.Run("SlicePastEnd", func(t *testing.T) {
		slice := mr.Slice(int64(len(testData))-2, 10) // Would go past end
		if slice != nil {
			t.Error("Slice past end should return nil")
		}
	})

	t.Run("SliceExactEnd", func(t *testing.T) {
		slice := mr.Slice(int64(len(testData))-5, 5)
		if slice == nil {
			t.Fatal("Slice at exact end returned nil")
		}
		if string(slice) != "ting." {
			t.Errorf("Slice = %q, want %q", string(slice), "ting.")
		}
	})

	// Test Size method
	t.Run("Size", func(t *testing.T) {
		size := mr.Size()
		if size != int64(len(testData)) {
			t.Errorf("Size = %d, want %d", size, len(testData))
		}
	})
}

func TestMmapReaderReadAt(t *testing.T) {
	// Create a temp file with known content
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.bin")

	testData := []byte("0123456789ABCDEF")
	if err := os.WriteFile(path, testData, 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	// Open the file with mmap
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("Failed to open file: %v", err)
	}
	defer f.Close()

	mr, err := newMmapReader(f)
	if err != nil {
		t.Fatalf("newMmapReader error: %v", err)
	}
	defer mr.Close()

	t.Run("ReadAtStart", func(t *testing.T) {
		buf := make([]byte, 4)
		n, err := mr.ReadAt(buf, 0)
		if err != nil {
			t.Errorf("ReadAt error: %v", err)
		}
		if n != 4 {
			t.Errorf("ReadAt n = %d, want 4", n)
		}
		if string(buf) != "0123" {
			t.Errorf("ReadAt = %q, want %q", string(buf), "0123")
		}
	})

	t.Run("ReadAtMiddle", func(t *testing.T) {
		buf := make([]byte, 4)
		n, err := mr.ReadAt(buf, 10)
		if err != nil {
			t.Errorf("ReadAt error: %v", err)
		}
		if n != 4 {
			t.Errorf("ReadAt n = %d, want 4", n)
		}
		if string(buf) != "ABCD" {
			t.Errorf("ReadAt = %q, want %q", string(buf), "ABCD")
		}
	})

	t.Run("ReadAtNegativeOffset", func(t *testing.T) {
		buf := make([]byte, 4)
		_, err := mr.ReadAt(buf, -1)
		if err == nil {
			t.Error("ReadAt with negative offset should return error")
		}
	})

	t.Run("ReadAtPastEnd", func(t *testing.T) {
		buf := make([]byte, 4)
		_, err := mr.ReadAt(buf, int64(len(testData)))
		if err == nil {
			t.Error("ReadAt past end should return error")
		}
	})
}

func TestMmapReaderEmptyFile(t *testing.T) {
	// Create an empty temp file
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "empty.bin")

	if err := os.WriteFile(path, []byte{}, 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	// Open the file with mmap
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("Failed to open file: %v", err)
	}
	defer f.Close()

	mr, err := newMmapReader(f)
	if err != nil {
		t.Fatalf("newMmapReader error: %v", err)
	}
	defer mr.Close()

	// Size should be 0
	if mr.Size() != 0 {
		t.Errorf("Size = %d, want 0", mr.Size())
	}

	// Slice should return nil for any request
	slice := mr.Slice(0, 1)
	if slice != nil {
		t.Error("Slice on empty file should return nil")
	}
}

func TestMmapReaderClose(t *testing.T) {
	// Create a temp file
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.bin")

	if err := os.WriteFile(path, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	// Open the file with mmap
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("Failed to open file: %v", err)
	}

	mr, err := newMmapReader(f)
	if err != nil {
		t.Fatalf("newMmapReader error: %v", err)
	}

	// Close should succeed
	if err := mr.Close(); err != nil {
		t.Errorf("Close error: %v", err)
	}
}

func TestMmapReaderWithNilData(t *testing.T) {
	// Test the Close path when data is nil (empty file case)
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "empty.bin")

	if err := os.WriteFile(path, []byte{}, 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("Failed to open file: %v", err)
	}

	mr, err := newMmapReader(f)
	if err != nil {
		t.Fatalf("newMmapReader error: %v", err)
	}

	// The data should be nil for empty file
	if mr.data != nil {
		t.Error("data should be nil for empty file")
	}

	// Close should still work
	if err := mr.Close(); err != nil {
		t.Errorf("Close error: %v", err)
	}
}
