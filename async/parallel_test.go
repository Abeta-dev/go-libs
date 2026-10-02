// SPDX-License-Identifier: MIT

package async_test

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/umesh0492/go-libs/async"
)

func TestParallel_Success(t *testing.T) {
	var count atomic.Int32
	ctx := context.Background()

	err := async.Parallel(ctx,
		func(c context.Context) error {
			count.Add(1)
			return nil
		},
		func(c context.Context) error {
			count.Add(10)
			return nil
		},
		func(c context.Context) error {
			count.Add(100)
			return nil
		},
	)

	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}
	if got := count.Load(); got != 111 {
		t.Fatalf("expected count 111, got: %d", got)
	}
}

func TestParallel_ErrorPropagation(t *testing.T) {
	expectedErr := errors.New("query failed")
	ctx := context.Background()

	err := async.Parallel(ctx,
		func(c context.Context) error {
			return nil
		},
		func(c context.Context) error {
			return expectedErr
		},
		func(c context.Context) error {
			return nil
		},
	)

	if !errors.Is(err, expectedErr) {
		t.Fatalf("expected error %v, got: %v", expectedErr, err)
	}
}

func TestParallel_PanicRecovery(t *testing.T) {
	ctx := context.Background()

	err := async.Parallel(ctx,
		func(c context.Context) error {
			return nil
		},
		func(c context.Context) error {
			panic("unexpected nil pointer dereference")
		},
	)

	if err == nil {
		t.Fatal("expected error from recovered panic, got nil")
	}
}

func TestParallel_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := async.Parallel(ctx,
		func(c context.Context) error {
			time.Sleep(50 * time.Millisecond)
			return nil
		},
	)

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got: %v", err)
	}
}

func TestParallel_EmptyAndNil(t *testing.T) {
	ctx := context.Background()
	if err := async.Parallel(ctx); err != nil {
		t.Fatalf("expected nil for empty tasks, got: %v", err)
	}
	if err := async.Parallel(ctx, nil, nil); err != nil {
		t.Fatalf("expected nil for nil tasks, got: %v", err)
	}
}

func TestParallelLimit_BoundedConcurrency(t *testing.T) {
	var maxActive atomic.Int32
	var currentActive atomic.Int32
	concurrencyLimit := 3
	taskCount := 10

	tasks := make([]async.Task, taskCount)
	for i := 0; i < taskCount; i++ {
		tasks[i] = func(c context.Context) error {
			curr := currentActive.Add(1)
			defer currentActive.Add(-1)

			for {
				max := maxActive.Load()
				if curr <= max || maxActive.CompareAndSwap(max, curr) {
					break
				}
			}

			time.Sleep(20 * time.Millisecond)
			return nil
		}
	}

	ctx := context.Background()
	err := async.ParallelLimit(ctx, concurrencyLimit, tasks...)
	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}

	if peak := maxActive.Load(); peak > int32(concurrencyLimit) {
		t.Fatalf("peak concurrency %d exceeded limit %d", peak, concurrencyLimit)
	}
}

func TestParallelLimit_PanicRecovery(t *testing.T) {
	ctx := context.Background()
	err := async.ParallelLimit(ctx, 2,
		func(c context.Context) error {
			panic("limit worker crash")
		},
		func(c context.Context) error {
			return nil
		},
	)

	if err == nil {
		t.Fatal("expected error from recovered panic, got nil")
	}
}

func TestParallelLimit_ErrorPropagation(t *testing.T) {
	expectedErr := errors.New("limit task failed")
	ctx := context.Background()

	err := async.ParallelLimit(ctx, 2,
		func(c context.Context) error {
			return expectedErr
		},
		func(c context.Context) error {
			return nil
		},
	)

	if !errors.Is(err, expectedErr) {
		t.Fatalf("expected error %v, got: %v", expectedErr, err)
	}
}

func TestParallelLimit_EmptyAndNil(t *testing.T) {
	ctx := context.Background()
	if err := async.ParallelLimit(ctx, 0); err != nil {
		t.Fatalf("expected nil for empty tasks, got: %v", err)
	}
	if err := async.ParallelLimit(ctx, -1, nil, nil); err != nil {
		t.Fatalf("expected nil for nil tasks, got: %v", err)
	}
}

func TestParallelLimit_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := async.ParallelLimit(ctx, 2,
		func(c context.Context) error {
			return nil
		},
	)

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got: %v", err)
	}
}

func TestGo_NormalAndPanicRecovery(t *testing.T) {
	var wg sync.WaitGroup
	var ran atomic.Bool
	wg.Add(2)

	async.Go(func() {
		defer wg.Done()
		ran.Store(true)
	})

	async.Go(func() {
		defer wg.Done()
		panic("fire-and-forget worker panic")
	})

	wg.Wait()
	if !ran.Load() {
		t.Fatal("expected normal Go fn to execute")
	}
}

func TestGoCtx_NormalAndPanicRecovery(t *testing.T) {
	var wg sync.WaitGroup
	var ran atomic.Bool
	wg.Add(2)

	ctx := context.Background()
	async.GoCtx(ctx, func(c context.Context) {
		defer wg.Done()
		ran.Store(true)
	})

	async.GoCtx(ctx, func(c context.Context) {
		defer wg.Done()
		panic("fire-and-forget ctx worker panic")
	})

	wg.Wait()
	if !ran.Load() {
		t.Fatal("expected normal GoCtx fn to execute")
	}
}
