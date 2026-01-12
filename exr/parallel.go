package exr

import (
	"runtime"
	"sync"
)

// ParallelConfig configures parallel processing behavior.
type ParallelConfig struct {
	// NumWorkers is the number of worker goroutines. 0 means runtime.GOMAXPROCS(0).
	NumWorkers int

	// GrainSize is the minimum work items per worker before parallelization.
	// If total work items < GrainSize * NumWorkers, runs sequentially.
	GrainSize int
}

// DefaultParallelConfig returns the default parallel configuration.
func DefaultParallelConfig() ParallelConfig {
	return ParallelConfig{
		NumWorkers: 0, // Use all available CPUs
		GrainSize:  1, // At least 1 item per worker (parallelize aggressively)
	}
}

// parallelConfig is the global configuration.
var (
	parallelConfig   = DefaultParallelConfig()
	parallelConfigMu sync.RWMutex
)

// SetParallelConfig sets the global parallel configuration.
func SetParallelConfig(config ParallelConfig) {
	parallelConfigMu.Lock()
	defer parallelConfigMu.Unlock()
	parallelConfig = config
}

// GetParallelConfig returns the current parallel configuration.
func GetParallelConfig() ParallelConfig {
	parallelConfigMu.RLock()
	defer parallelConfigMu.RUnlock()
	return parallelConfig
}

// effectiveWorkers returns the number of workers to use.
func effectiveWorkers(config ParallelConfig) int {
	if config.NumWorkers <= 0 {
		return runtime.GOMAXPROCS(0)
	}
	return config.NumWorkers
}

// WorkerPool manages a pool of workers for parallel processing.
type WorkerPool struct {
	numWorkers int
	wg         sync.WaitGroup
	taskChan   chan func()
	once       sync.Once
}

// NewWorkerPool creates a new worker pool.
func NewWorkerPool(numWorkers int) *WorkerPool {
	if numWorkers <= 0 {
		numWorkers = runtime.GOMAXPROCS(0)
	}

	pool := &WorkerPool{
		numWorkers: numWorkers,
		taskChan:   make(chan func(), numWorkers*4),
	}

	// Start workers
	for i := 0; i < numWorkers; i++ {
		go pool.worker()
	}

	return pool
}

// worker is the main loop for a worker goroutine.
func (p *WorkerPool) worker() {
	for task := range p.taskChan {
		task()
		p.wg.Done()
	}
}

// Submit submits a task to the pool.
func (p *WorkerPool) Submit(task func()) {
	p.wg.Add(1)
	p.taskChan <- task
}

// Wait waits for all submitted tasks to complete.
func (p *WorkerPool) Wait() {
	p.wg.Wait()
}

// Close shuts down the worker pool.
func (p *WorkerPool) Close() {
	p.once.Do(func() {
		close(p.taskChan)
	})
}

// ParallelFor runs fn(i) for i in [0, n) in parallel.
// If n is small or there's only one worker, runs sequentially.
func ParallelFor(n int, fn func(i int)) {
	config := GetParallelConfig()
	numWorkers := effectiveWorkers(config)

	// Run sequentially if not worth parallelizing
	if n <= config.GrainSize*numWorkers || numWorkers == 1 {
		for i := 0; i < n; i++ {
			fn(i)
		}
		return
	}

	var wg sync.WaitGroup
	chunkSize := (n + numWorkers - 1) / numWorkers

	for w := 0; w < numWorkers; w++ {
		start := w * chunkSize
		end := start + chunkSize
		if end > n {
			end = n
		}
		if start >= end {
			break
		}

		wg.Add(1)
		go func(s, e int) {
			defer wg.Done()
			for i := s; i < e; i++ {
				fn(i)
			}
		}(start, end)
	}

	wg.Wait()
}

// ParallelForWithError runs fn(i) for i in [0, n) in parallel with error handling.
// Returns the first error encountered (order not guaranteed).
func ParallelForWithError(n int, fn func(i int) error) error {
	config := GetParallelConfig()
	numWorkers := effectiveWorkers(config)

	// Run sequentially if not worth parallelizing
	if n <= config.GrainSize*numWorkers || numWorkers == 1 {
		for i := 0; i < n; i++ {
			if err := fn(i); err != nil {
				return err
			}
		}
		return nil
	}

	var wg sync.WaitGroup
	var errOnce sync.Once
	var firstErr error
	chunkSize := (n + numWorkers - 1) / numWorkers

	for w := 0; w < numWorkers; w++ {
		start := w * chunkSize
		end := start + chunkSize
		if end > n {
			end = n
		}
		if start >= end {
			break
		}

		wg.Add(1)
		go func(s, e int) {
			defer wg.Done()
			for i := s; i < e; i++ {
				if err := fn(i); err != nil {
					errOnce.Do(func() {
						firstErr = err
					})
					return
				}
			}
		}(start, end)
	}

	wg.Wait()
	return firstErr
}

// ChunkResult holds the result of processing a chunk.
type ChunkResult struct {
	Index int
	Data  []byte
	Error error
}

// ParallelChunkProcess processes chunks in parallel.
// processor receives chunk index and returns processed data or error.
func ParallelChunkProcess(numChunks int, processor func(chunkIdx int) ([]byte, error)) ([][]byte, error) {
	results := make([][]byte, numChunks)

	err := ParallelForWithError(numChunks, func(i int) error {
		data, err := processor(i)
		if err != nil {
			return err
		}
		results[i] = data
		return nil
	})

	if err != nil {
		return nil, err
	}
	return results, nil
}
