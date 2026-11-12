package pool

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestNewWorkerPoolClamping(t *testing.T) {
	tests := []struct {
		name            string
		inputWorkers    int
		expectedWorkers int
	}{
		{name: "negative workers clamped to 4", inputWorkers: -5, expectedWorkers: 4},
		{name: "zero workers clamped to 4", inputWorkers: 0, expectedWorkers: 4},
		{name: "single worker preserved", inputWorkers: 1, expectedWorkers: 1},
		{name: "valid workers within range", inputWorkers: 8, expectedWorkers: 8},
		{name: "max workers upper bound preserved", inputWorkers: 16, expectedWorkers: 16},
		{name: "workers above 16 clamped to 16", inputWorkers: 17, expectedWorkers: 16},
		{name: "large workers clamped to 16", inputWorkers: 100, expectedWorkers: 16},
	}

	dummyFn := func(ctx context.Context, item int) (int, error) {
		return item, nil
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewWorkerPool(tt.inputWorkers, dummyFn)
			if p.Workers() != tt.expectedWorkers {
				t.Errorf("NewWorkerPool(%d).Workers() = %d; want %d", tt.inputWorkers, p.Workers(), tt.expectedWorkers)
			}
		})
	}
}

func TestSetFailFast(t *testing.T) {
	dummyFn := func(ctx context.Context, item int) (int, error) {
		return item, nil
	}
	p := NewWorkerPool(4, dummyFn)
	if p.FailFast() != false {
		t.Errorf("default FailFast() = %v; want false", p.FailFast())
	}

	ret := p.SetFailFast(true)
	if ret != p {
		t.Errorf("SetFailFast should return pointer to WorkerPool for chaining")
	}
	if p.FailFast() != true {
		t.Errorf("after SetFailFast(true), FailFast() = %v; want true", p.FailFast())
	}

	p.SetFailFast(false)
	if p.FailFast() != false {
		t.Errorf("after SetFailFast(false), FailFast() = %v; want false", p.FailFast())
	}
}

func TestBoundedConcurrency(t *testing.T) {
	const configuredWorkers = 4
	const numItems = 24

	var currentActive atomic.Int32
	var maxObserved atomic.Int32

	pool := NewWorkerPool(configuredWorkers, func(ctx context.Context, item int) (int, error) {
		active := currentActive.Add(1)
		for {
			max := maxObserved.Load()
			if active <= max || maxObserved.CompareAndSwap(max, active) {
				break
			}
		}

		time.Sleep(20 * time.Millisecond)

		currentActive.Add(-1)
		return item * 2, nil
	})

	items := make([]int, numItems)
	for i := 0; i < numItems; i++ {
		items[i] = i
	}

	results := pool.Run(context.Background(), items)

	if len(results) != numItems {
		t.Fatalf("expected %d results, got %d", numItems, len(results))
	}

	max := maxObserved.Load()
	if max > configuredWorkers {
		t.Errorf("concurrency exceeded limit: observed max %d workers, allowed %d", max, configuredWorkers)
	}
	if max < 2 {
		t.Errorf("expected concurrency > 1, observed max %d", max)
	}
}

func TestResultOrdering(t *testing.T) {
	const numItems = 50
	items := make([]int, numItems)
	for i := 0; i < numItems; i++ {
		items[i] = i
	}

	pool := NewWorkerPool(8, func(ctx context.Context, item int) (string, error) {
		// Introduce pseudo-random variable delay so workers complete out of order
		delay := time.Duration((item*7)%11) * time.Millisecond
		time.Sleep(delay)
		return fmt.Sprintf("val-%d", item), nil
	})

	results := pool.Run(context.Background(), items)

	if len(results) != numItems {
		t.Fatalf("expected %d results, got %d", numItems, len(results))
	}

	for i, res := range results {
		if res.Item != i {
			t.Errorf("at index %d: res.Item = %d; want %d", i, res.Item, i)
		}
		expectedVal := fmt.Sprintf("val-%d", i)
		if res.Value != expectedVal {
			t.Errorf("at index %d: res.Value = %q; want %q", i, res.Value, expectedVal)
		}
		if res.Err != nil {
			t.Errorf("at index %d: unexpected error: %v", i, res.Err)
		}
	}
}

func TestErrorHandlingWithoutFailFast(t *testing.T) {
	const numItems = 20
	items := make([]int, numItems)
	for i := 0; i < numItems; i++ {
		items[i] = i
	}

	var executionCount atomic.Int32

	pool := NewWorkerPool(4, func(ctx context.Context, item int) (int, error) {
		executionCount.Add(1)
		time.Sleep(5 * time.Millisecond)
		if item%3 == 0 {
			return 0, fmt.Errorf("error on item %d", item)
		}
		return item * 10, nil
	})
	pool.SetFailFast(false)

	results := pool.Run(context.Background(), items)

	if int(executionCount.Load()) != numItems {
		t.Errorf("expected all %d items to be executed, but got %d", numItems, executionCount.Load())
	}
	if len(results) != numItems {
		t.Fatalf("expected %d results, got %d", numItems, len(results))
	}

	for i, res := range results {
		if res.Item != i {
			t.Errorf("at index %d: res.Item = %d, want %d", i, res.Item, i)
		}
		if i%3 == 0 {
			if res.Err == nil {
				t.Errorf("at index %d: expected error, got nil", i)
			} else if res.Err.Error() != fmt.Sprintf("error on item %d", i) {
				t.Errorf("at index %d: got error %q, want %q", i, res.Err.Error(), fmt.Sprintf("error on item %d", i))
			}
		} else {
			if res.Err != nil {
				t.Errorf("at index %d: unexpected error: %v", i, res.Err)
			}
			if res.Value != i*10 {
				t.Errorf("at index %d: res.Value = %d, want %d", i, res.Value, i*10)
			}
		}
	}
}

func TestFailFastBehavior(t *testing.T) {
	const numItems = 30
	items := make([]int, numItems)
	for i := 0; i < numItems; i++ {
		items[i] = i
	}

	var tasksStarted atomic.Int32

	expectedErr := errors.New("deliberate failure on item 2")

	pool := NewWorkerPool(2, func(ctx context.Context, item int) (int, error) {
		tasksStarted.Add(1)
		if item == 2 {
			return 0, expectedErr
		}

		select {
		case <-ctx.Done():
			return 0, ctx.Err()
		case <-time.After(50 * time.Millisecond):
			return item * 10, nil
		}
	})
	pool.SetFailFast(true)

	results := pool.Run(context.Background(), items)

	if len(results) != numItems {
		t.Fatalf("expected %d results, got %d", numItems, len(results))
	}

	started := tasksStarted.Load()
	if started >= int32(numItems) {
		t.Errorf("failFast failed to prevent pending tasks: all %d tasks were started", started)
	}

	// Verify that item 2 has the expected deliberate error
	if results[2].Err == nil || results[2].Err.Error() != expectedErr.Error() {
		t.Errorf("item 2: expected deliberate failure, got %v", results[2].Err)
	}

	// Verify that at least some pending items have context.Canceled error
	cancelledCount := 0
	for i, res := range results {
		if res.Item != i {
			t.Errorf("at index %d: res.Item = %d, want %d", i, res.Item, i)
		}
		if errors.Is(res.Err, context.Canceled) {
			cancelledCount++
		}
	}
	if cancelledCount == 0 {
		t.Errorf("expected cancelled tasks to have context.Canceled error, got 0")
	}
}

func TestEdgeCases(t *testing.T) {
	dummyFn := func(ctx context.Context, item int) (int, error) {
		return item * 2, nil
	}

	t.Run("empty items slice", func(t *testing.T) {
		pool := NewWorkerPool(4, dummyFn)
		results := pool.Run(context.Background(), []int{})
		if results == nil {
			t.Errorf("expected non-nil empty slice, got nil")
		}
		if len(results) != 0 {
			t.Errorf("expected 0 results, got %d", len(results))
		}
	})

	t.Run("nil items slice", func(t *testing.T) {
		pool := NewWorkerPool(4, dummyFn)
		results := pool.Run(context.Background(), nil)
		if results == nil {
			t.Errorf("expected non-nil empty slice, got nil")
		}
		if len(results) != 0 {
			t.Errorf("expected 0 results, got %d", len(results))
		}
	})

	t.Run("items fewer than workers", func(t *testing.T) {
		var activeWorkers atomic.Int32
		var maxWorkers atomic.Int32

		pool := NewWorkerPool(8, func(ctx context.Context, item int) (int, error) {
			active := activeWorkers.Add(1)
			for {
				max := maxWorkers.Load()
				if active <= max || maxWorkers.CompareAndSwap(max, active) {
					break
				}
			}
			time.Sleep(20 * time.Millisecond)
			activeWorkers.Add(-1)
			return item * 3, nil
		})

		items := []int{10, 20}
		results := pool.Run(context.Background(), items)

		if len(results) != 2 {
			t.Fatalf("expected 2 results, got %d", len(results))
		}
		if results[0].Value != 30 || results[1].Value != 60 {
			t.Errorf("unexpected values: %v, %v", results[0].Value, results[1].Value)
		}
		if maxWorkers.Load() > 2 {
			t.Errorf("expected at most 2 concurrent workers for 2 items, got %d", maxWorkers.Load())
		}
	})

	t.Run("pre-cancelled context", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		var calls atomic.Int32
		pool := NewWorkerPool(4, func(c context.Context, item int) (int, error) {
			calls.Add(1)
			return item, nil
		})

		items := []int{1, 2, 3, 4, 5}
		results := pool.Run(ctx, items)

		if len(results) != 5 {
			t.Fatalf("expected 5 results, got %d", len(results))
		}
		if calls.Load() != 0 {
			t.Errorf("expected 0 calls with pre-cancelled context, got %d", calls.Load())
		}
		for i, res := range results {
			if !errors.Is(res.Err, context.Canceled) {
				t.Errorf("at index %d: expected context.Canceled, got %v", i, res.Err)
			}
			if res.Item != items[i] {
				t.Errorf("at index %d: expected Item %d, got %d", i, items[i], res.Item)
			}
		}
	})

	t.Run("nil context defaults to Background", func(t *testing.T) {
		pool := NewWorkerPool(2, dummyFn)
		// Passing nil context should not panic
		results := pool.Run(nil, []int{42})
		if len(results) != 1 || results[0].Value != 84 {
			t.Errorf("expected successful execution with nil context, got %+v", results)
		}
	})

	t.Run("nil task function returns errors gracefully", func(t *testing.T) {
		pool := &WorkerPool[int, int]{workers: 4, fn: nil}
		results := pool.Run(context.Background(), []int{1, 2})
		if len(results) != 2 {
			t.Fatalf("expected 2 results, got %d", len(results))
		}
		for i, res := range results {
			if res.Err == nil {
				t.Errorf("at index %d: expected error for nil task function, got nil", i)
			}
		}
	})
}

func TestSynchronizedLog(t *testing.T) {
	t.Run("nil function does not panic", func(t *testing.T) {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("SynchronizedLog(nil) panicked: %v", r)
			}
		}()
		SynchronizedLog(nil)
	})

	t.Run("concurrent log serialization", func(t *testing.T) {
		const goroutines = 10
		const iterations = 20

		var buf bytes.Buffer
		var wg sync.WaitGroup

		for g := 0; g < goroutines; g++ {
			wg.Add(1)
			goroutineID := g
			go func() {
				defer wg.Done()
				for i := 0; i < iterations; i++ {
					SynchronizedLog(func() {
						buf.WriteString(fmt.Sprintf("[%d:%d:START]", goroutineID, i))
						time.Sleep(50 * time.Microsecond)
						buf.WriteString(fmt.Sprintf("[%d:%d:END]\n", goroutineID, i))
					})
				}
			}()
		}

		wg.Wait()

		lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
		expectedTotalLines := goroutines * iterations
		if len(lines) != expectedTotalLines {
			t.Fatalf("expected %d log lines, got %d", expectedTotalLines, len(lines))
		}

		for lineNum, line := range lines {
			// Each line must be well-formed: [G:I:START][G:I:END]
			var g1, i1, g2, i2 int
			n, err := fmt.Sscanf(line, "[%d:%d:START][%d:%d:END]", &g1, &i1, &g2, &i2)
			if err != nil || n != 4 || g1 != g2 || i1 != i2 {
				t.Errorf("line %d interleaved or malformed: %q", lineNum, line)
			}
		}
	})
}

func TestWorkerPool_UnclampedDirectStruct(t *testing.T) {
	dummyFn := func(ctx context.Context, item int) (int, error) {
		return item, nil
	}

	t.Run("direct zero workers defaults to 4", func(t *testing.T) {
		p := &WorkerPool[int, int]{workers: 0, fn: dummyFn}
		results := p.Run(context.Background(), []int{1, 2})
		if len(results) != 2 || results[0].Value != 1 {
			t.Errorf("unexpected results: %+v", results)
		}
	})

	t.Run("direct excessive workers clamped to 16", func(t *testing.T) {
		p := &WorkerPool[int, int]{workers: 99, fn: dummyFn}
		results := p.Run(context.Background(), []int{1, 2})
		if len(results) != 2 || results[0].Value != 1 {
			t.Errorf("unexpected results: %+v", results)
		}
	})
}
