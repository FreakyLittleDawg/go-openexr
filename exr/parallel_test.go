package exr

import (
	"errors"
	"sync/atomic"
	"testing"
)

func TestParallelFor(t *testing.T) {
	n := 1000

	// Test with a counter to verify all items are processed
	var count int64
	ParallelFor(n, func(i int) {
		atomic.AddInt64(&count, 1)
	})

	if count != int64(n) {
		t.Errorf("ParallelFor processed %d items, want %d", count, n)
	}
}

func TestParallelForSmall(t *testing.T) {
	// Test with small n that should run sequentially
	n := 4
	results := make([]int, n)

	ParallelFor(n, func(i int) {
		results[i] = i * 2
	})

	for i := 0; i < n; i++ {
		if results[i] != i*2 {
			t.Errorf("results[%d] = %d, want %d", i, results[i], i*2)
		}
	}
}

func TestParallelForWithError(t *testing.T) {
	n := 100

	// Test with no errors
	err := ParallelForWithError(n, func(i int) error {
		return nil
	})
	if err != nil {
		t.Errorf("ParallelForWithError returned error: %v", err)
	}

	// Test with error
	expectedErr := ErrNoFrameBuffer
	err = ParallelForWithError(n, func(i int) error {
		if i == 50 {
			return expectedErr
		}
		return nil
	})
	if err != expectedErr {
		t.Errorf("ParallelForWithError returned %v, want %v", err, expectedErr)
	}
}

func TestWorkerPool(t *testing.T) {
	pool := NewWorkerPool(4)
	defer pool.Close()

	var count int64
	n := 100

	for i := 0; i < n; i++ {
		pool.Submit(func() {
			atomic.AddInt64(&count, 1)
		})
	}

	pool.Wait()

	if count != int64(n) {
		t.Errorf("WorkerPool processed %d tasks, want %d", count, n)
	}
}

func TestParallelConfig(t *testing.T) {
	// Save original config
	original := GetParallelConfig()
	defer SetParallelConfig(original)

	// Test setting config
	config := ParallelConfig{
		NumWorkers: 8,
		GrainSize:  16,
	}
	SetParallelConfig(config)

	got := GetParallelConfig()
	if got.NumWorkers != 8 {
		t.Errorf("NumWorkers = %d, want 8", got.NumWorkers)
	}
	if got.GrainSize != 16 {
		t.Errorf("GrainSize = %d, want 16", got.GrainSize)
	}
}

func TestParallelChunkProcess(t *testing.T) {
	numChunks := 10
	results, err := ParallelChunkProcess(numChunks, func(idx int) ([]byte, error) {
		return []byte{byte(idx)}, nil
	})

	if err != nil {
		t.Errorf("ParallelChunkProcess returned error: %v", err)
	}

	if len(results) != numChunks {
		t.Errorf("Got %d results, want %d", len(results), numChunks)
	}

	for i, r := range results {
		if len(r) != 1 || r[0] != byte(i) {
			t.Errorf("results[%d] = %v, want [%d]", i, r, i)
		}
	}
}

func BenchmarkParallelFor(b *testing.B) {
	n := 10000
	data := make([]int, n)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ParallelFor(n, func(i int) {
			data[i] = i * 2
		})
	}
}

func BenchmarkSequentialFor(b *testing.B) {
	n := 10000
	data := make([]int, n)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for j := 0; j < n; j++ {
			data[j] = j * 2
		}
	}
}

func BenchmarkWorkerPool(b *testing.B) {
	pool := NewWorkerPool(0) // Use default workers
	defer pool.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for j := 0; j < 100; j++ {
			pool.Submit(func() {
				// Simulate some work
				x := 0
				for k := 0; k < 1000; k++ {
					x += k
				}
				_ = x
			})
		}
		pool.Wait()
	}
}

func TestParallelChunkProcessError(t *testing.T) {
	numChunks := 5
	testErr := errors.New("test error")

	results, err := ParallelChunkProcess(numChunks, func(idx int) ([]byte, error) {
		if idx == 2 {
			return nil, testErr
		}
		return []byte{byte(idx)}, nil
	})

	if err == nil {
		t.Error("ParallelChunkProcess should return error when processor fails")
	}
	if results != nil {
		t.Errorf("ParallelChunkProcess should return nil results on error, got %v", results)
	}
}
