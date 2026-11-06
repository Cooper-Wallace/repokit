package repokit

import (
	"context"
	"errors"
	"sync"
)

// TaskFunc defines the function signature for a task executed by the worker pool.
type TaskFunc[T any, R any] func(ctx context.Context, item T) (R, error)

// PoolResult represents the execution result of a single task in the worker pool.
type PoolResult[T any, R any] struct {
	Item  T
	Value R
	Err   error
}

// WorkerPool is a generic, type-safe, bounded worker pool for concurrent task execution.
type WorkerPool[T any, R any] struct {
	workers  int
	fn       TaskFunc[T, R]
	failFast bool
}

// indexedItem associates an item with its original position in the input slice.
type indexedItem[T any] struct {
	index int
	item  T
}

// indexedResult associates a task result with its original position.
type indexedResult[T any, R any] struct {
	index int
	res   PoolResult[T, R]
}

// NewWorkerPool creates a new generic WorkerPool with bounded concurrency.
// workers is clamped: if <= 0, defaults to 4. If > 16, clamped to 16.
func NewWorkerPool[T any, R any](workers int, fn TaskFunc[T, R]) *WorkerPool[T, R] {
	if workers <= 0 {
		workers = 4
	} else if workers > 16 {
		workers = 16
	}
	return &WorkerPool[T, R]{
		workers: workers,
		fn:      fn,
	}
}

// SetFailFast configures whether the worker pool cancels the execution context
// as soon as any worker returns a non-nil error, preventing unnecessary subsequent work.
func (p *WorkerPool[T, R]) SetFailFast(failFast bool) *WorkerPool[T, R] {
	p.failFast = failFast
	return p
}

// Workers returns the number of concurrent workers configured for the pool.
func (p *WorkerPool[T, R]) Workers() int {
	return p.workers
}

// FailFast returns whether failFast is enabled for the pool.
func (p *WorkerPool[T, R]) FailFast() bool {
	return p.failFast
}

// Run executes the pool's task function across all provided items using bounded concurrency.
// It returns a slice of PoolResult preserving the original order of items.
func (p *WorkerPool[T, R]) Run(ctx context.Context, items []T) []PoolResult[T, R] {
	if len(items) == 0 {
		return []PoolResult[T, R]{}
	}
	if ctx == nil {
		ctx = context.Background()
	}

	if p == nil || p.fn == nil {
		results := make([]PoolResult[T, R], len(items))
		for i, item := range items {
			results[i] = PoolResult[T, R]{
				Item: item,
				Err:  errors.New("repokit: worker pool task function is nil"),
			}
		}
		return results
	}

	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	numWorkers := p.workers
	if numWorkers <= 0 {
		numWorkers = 4
	} else if numWorkers > 16 {
		numWorkers = 16
	}
	if len(items) < numWorkers {
		numWorkers = len(items)
	}

	tasksChan := make(chan indexedItem[T], len(items))
	resultsChan := make(chan indexedResult[T, R], len(items))

	for i, item := range items {
		tasksChan <- indexedItem[T]{index: i, item: item}
	}
	close(tasksChan)

	var wg sync.WaitGroup
	for w := 0; w < numWorkers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for task := range tasksChan {
				// If context is already cancelled, skip calling the task function.
				if err := runCtx.Err(); err != nil {
					resultsChan <- indexedResult[T, R]{
						index: task.index,
						res: PoolResult[T, R]{
							Item: task.item,
							Err:  err,
						},
					}
					continue
				}

				val, err := p.fn(runCtx, task.item)
				if err != nil && p.failFast {
					cancel()
				}

				resultsChan <- indexedResult[T, R]{
					index: task.index,
					res: PoolResult[T, R]{
						Item:  task.item,
						Value: val,
						Err:   err,
					},
				}
			}
		}()
	}

	wg.Wait()
	close(resultsChan)

	results := make([]PoolResult[T, R], len(items))
	for r := range resultsChan {
		results[r.index] = r.res
	}

	return results
}

var logMutex sync.Mutex

// SynchronizedLog ensures multi-line or formatted console logs from concurrent workers do not interleave.
func SynchronizedLog(fn func()) {
	if fn == nil {
		return
	}
	logMutex.Lock()
	defer logMutex.Unlock()
	fn()
}
