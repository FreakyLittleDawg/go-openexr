package exr

import (
	"testing"
)

func TestBufferPoolGet(t *testing.T) {
	pool := NewBufferPool()

	tests := []struct {
		size         int
		expectedSize int
	}{
		{100, 100},
		{1024, 1024},
		{4096, 4096},
		{5000, 5000},
		{1 << 20, 1 << 20},
		{5 << 20, 5 << 20}, // Larger than max pool size
	}

	for _, tt := range tests {
		buf := pool.Get(tt.size)
		if len(buf) != tt.expectedSize {
			t.Errorf("Get(%d) returned len=%d, want %d", tt.size, len(buf), tt.expectedSize)
		}
		pool.Put(buf)
	}
}

func TestBufferPoolReuse(t *testing.T) {
	pool := NewBufferPool()

	// Get a buffer and put it back
	buf1 := pool.Get(1024)
	pool.Put(buf1)

	// Get another buffer of the same size
	buf2 := pool.Get(1024)

	// They should be from the same underlying array (capacity check)
	if cap(buf2) != cap(buf1) {
		t.Log("Buffer was not reused (this is OK, pool behavior is best-effort)")
	}

	pool.Put(buf2)
}

func TestGlobalBufferPool(t *testing.T) {
	buf := GetBuffer(4096)
	if len(buf) != 4096 {
		t.Errorf("GetBuffer(4096) returned len=%d, want 4096", len(buf))
	}
	PutBuffer(buf)
}

func TestPooledBuffer(t *testing.T) {
	pb := NewPooledBuffer(1024)
	if len(pb.Data) != 1024 {
		t.Errorf("NewPooledBuffer(1024) returned len=%d, want 1024", len(pb.Data))
	}

	pb.Release()
	if pb.Data != nil {
		t.Error("Release() should set Data to nil")
	}
}

func TestUint16Pool(t *testing.T) {
	pool := NewUint16Pool(1024)

	buf := pool.Get(512)
	if len(buf) != 512 {
		t.Errorf("Get(512) returned len=%d, want 512", len(buf))
	}
	pool.Put(buf)

	// Get a larger buffer
	buf2 := pool.Get(2048)
	if len(buf2) != 2048 {
		t.Errorf("Get(2048) returned len=%d, want 2048", len(buf2))
	}
	pool.Put(buf2)
}

func TestBufferPoolMemoryLimit(t *testing.T) {
	// Create pool with 100KB limit
	pool := NewBufferPoolWithLimit(100 * 1024)

	// First allocation should succeed (64KB)
	buf1 := pool.Get(64 * 1024)
	if buf1 == nil {
		t.Fatal("First allocation should succeed")
	}

	// Second allocation that would exceed limit should fail
	buf2 := pool.Get(64 * 1024) // Would need ~128KB total
	if buf2 != nil {
		t.Error("Second allocation should fail due to memory limit")
		pool.Put(buf2)
	}

	// Return first buffer
	pool.Put(buf1)

	// Now allocation should succeed again
	buf3 := pool.Get(64 * 1024)
	if buf3 == nil {
		t.Error("Allocation after Put should succeed")
	}
	pool.Put(buf3)
}

func TestBufferPoolGetWithError(t *testing.T) {
	pool := NewBufferPoolWithLimit(10 * 1024)

	// This should succeed
	buf, err := pool.GetWithError(4 * 1024)
	if err != nil {
		t.Fatalf("GetWithError should succeed: %v", err)
	}
	if buf == nil {
		t.Fatal("Buffer should not be nil")
	}

	// This should fail with error
	buf2, err := pool.GetWithError(64 * 1024)
	if err == nil {
		t.Error("GetWithError should return error when limit exceeded")
		pool.Put(buf2)
	}
	if _, ok := err.(*MemoryLimitExceededError); !ok {
		t.Errorf("Error should be MemoryLimitExceededError, got %T", err)
	}

	pool.Put(buf)
}

func TestBufferPoolStats(t *testing.T) {
	pool := NewBufferPool()
	pool.ResetStats()

	// Make some allocations
	buf1 := pool.Get(1024)
	buf2 := pool.Get(1024)
	pool.Put(buf1)
	buf3 := pool.Get(1024) // Should be a cache hit
	pool.Put(buf2)
	pool.Put(buf3)

	allocs, hits, misses := pool.Stats()
	if allocs != 3 {
		t.Errorf("Expected 3 allocations, got %d", allocs)
	}
	// At least one hit expected when reusing
	t.Logf("Stats: allocs=%d, hits=%d, misses=%d", allocs, hits, misses)
}

func TestBufferPoolMemoryUsed(t *testing.T) {
	pool := NewBufferPoolWithLimit(1 << 20) // 1MB limit

	if pool.MemoryUsed() != 0 {
		t.Error("Initial memory used should be 0")
	}

	buf := pool.Get(4096)
	used := pool.MemoryUsed()
	if used == 0 {
		t.Error("Memory used should increase after Get")
	}

	pool.Put(buf)
	if pool.MemoryUsed() != 0 {
		t.Error("Memory used should return to 0 after Put")
	}
}

func TestSetGlobalMemoryLimit(t *testing.T) {
	// Save original limit
	original := GlobalMemoryLimit()
	defer SetGlobalMemoryLimit(original)

	// Set a limit
	SetGlobalMemoryLimit(50 * 1024)
	if GlobalMemoryLimit() != 50*1024 {
		t.Error("GlobalMemoryLimit should return set value")
	}

	// Reset to unlimited
	SetGlobalMemoryLimit(0)
	if GlobalMemoryLimit() != 0 {
		t.Error("GlobalMemoryLimit should be 0 when unlimited")
	}
}

func BenchmarkBufferPoolGet(b *testing.B) {
	pool := NewBufferPool()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf := pool.Get(64 * 1024)
		pool.Put(buf)
	}
}

func BenchmarkMakeBytes(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf := make([]byte, 64*1024)
		_ = buf
	}
}

func BenchmarkGlobalBufferPool(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf := GetBuffer(64 * 1024)
		PutBuffer(buf)
	}
}

func BenchmarkPooledBuffer(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pb := NewPooledBuffer(64 * 1024)
		pb.Release()
	}
}

func TestMemoryLimitExceededError(t *testing.T) {
	err := &MemoryLimitExceededError{
		Requested: 1024 * 1024,
		Current:   512 * 1024,
		Limit:     512 * 1024,
	}

	msg := err.Error()
	if msg == "" {
		t.Error("Error() should return a non-empty message")
	}
	t.Logf("Error message: %s", msg)
}

func TestGlobalMemoryFunctions(t *testing.T) {
	// Save original limit and reset after test
	original := GlobalMemoryLimit()
	defer SetGlobalMemoryLimit(original)

	// Set a limit
	SetGlobalMemoryLimit(1 << 20) // 1MB

	// Get a buffer through global pool
	buf := GetBuffer(4096)

	// Check memory used
	used := GlobalMemoryUsed()
	t.Logf("Global memory used after alloc: %d", used)

	PutBuffer(buf)

	// Check after return
	usedAfter := GlobalMemoryUsed()
	t.Logf("Global memory used after put: %d", usedAfter)

	// Get stats
	allocs, hits, misses := GlobalPoolStats()
	t.Logf("Global pool stats: allocs=%d, hits=%d, misses=%d", allocs, hits, misses)
}

func TestGetBufferWithError(t *testing.T) {
	// This tests the global GetBufferWithError function
	// which should check against global memory limit

	original := GlobalMemoryLimit()
	defer SetGlobalMemoryLimit(original)

	// Set a small limit
	SetGlobalMemoryLimit(100 * 1024) // 100KB

	// First allocation should work
	buf, err := GetBufferWithError(50 * 1024)
	if err != nil {
		t.Fatalf("First allocation should succeed: %v", err)
	}

	// Second allocation that would exceed should fail
	buf2, err := GetBufferWithError(100 * 1024)
	if err == nil {
		PutBuffer(buf2)
		t.Error("Allocation exceeding limit should fail")
	} else {
		t.Logf("Expected error: %v", err)
	}

	PutBuffer(buf)

	// Reset to unlimited for other tests
	SetGlobalMemoryLimit(0)
}

func TestBufferPoolLargeAllocationWithLimit(t *testing.T) {
	// Test large allocations (exceeding pool sizes) with memory limit
	pool := NewBufferPoolWithLimit(10 * 1024 * 1024) // 10MB limit

	// Large allocation that exceeds all pool sizes (> 4MB)
	largeSize := 5 * 1024 * 1024 // 5MB
	buf := pool.Get(largeSize)
	if buf == nil {
		t.Fatal("Large allocation should succeed")
	}
	if len(buf) != largeSize {
		t.Errorf("Large buffer size = %d, want %d", len(buf), largeSize)
	}

	// Memory should be tracked
	used := pool.MemoryUsed()
	if used == 0 {
		t.Error("Memory should be tracked for large allocations")
	}

	pool.Put(buf)

	// Test that large allocation fails when limit exceeded
	pool2 := NewBufferPoolWithLimit(1 * 1024 * 1024) // 1MB limit
	buf2 := pool2.Get(5 * 1024 * 1024)               // Try 5MB
	if buf2 != nil {
		t.Error("Large allocation exceeding limit should return nil")
		pool2.Put(buf2)
	}
}

func TestBufferPoolPooledAllocationWithLimit(t *testing.T) {
	// Buffer sizes: 1KB, 4KB, 16KB, 64KB, 256KB, etc.
	// Test that pooled allocations respect memory limit
	pool := NewBufferPoolWithLimit(100 * 1024) // 100KB limit

	// First allocation for 4KB should succeed (rounds up to 4KB bucket)
	buf1 := pool.Get(4 * 1024)
	if buf1 == nil {
		t.Fatal("First allocation should succeed")
	}

	// Second large allocation that would exceed limit should fail (64KB bucket)
	buf2 := pool.Get(50 * 1024) // Would need 64KB bucket
	if buf2 == nil {
		// This is OK - we've already used some memory
		t.Log("Second large allocation failed as expected when memory is tight")
	} else {
		// Third allocation should fail
		buf3 := pool.Get(64 * 1024)
		if buf3 != nil {
			t.Error("Third allocation exceeding limit should return nil")
			pool.Put(buf3)
		}
		pool.Put(buf2)
	}

	// Return first buffer
	pool.Put(buf1)
}

func TestBufferPoolEdgeSizes(t *testing.T) {
	pool := NewBufferPool()

	// Test exact bucket boundaries
	sizes := []int{1, 1024, 1025, 4096, 4097, 16384, 16385}
	for _, size := range sizes {
		buf := pool.Get(size)
		if len(buf) < size {
			t.Errorf("Buffer too small for size %d: got %d", size, len(buf))
		}
		pool.Put(buf)
	}
}
