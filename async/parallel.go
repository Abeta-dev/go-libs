// SPDX-License-Identifier: MIT

package async

import (
	"context"
	"fmt"
	"log/slog"
	"runtime/debug"
	"sync"
)

// Task represents a unit of work that can be executed concurrently in a goroutine.
type Task func(ctx context.Context) error

// Parallel executes all provided tasks concurrently in separate goroutines.
// It waits for all tasks to complete and returns the first error encountered (if any).
// Any panics within individual tasks are caught and recovered from safely.
// If the context is cancelled or times out, running tasks observe the cancellation.
func Parallel(ctx context.Context, tasks ...Task) error {
	if len(tasks) == 0 {
		return nil
	}

	var wg sync.WaitGroup
	var once sync.Once
	var firstErr error

	for _, task := range tasks {
		if task == nil {
			continue
		}
		wg.Add(1)
		go func(t Task) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					slog.Error("parallel task panic recovered",
						"panic", r,
						"stack", string(debug.Stack()),
					)
					once.Do(func() {
						firstErr = fmt.Errorf("task panicked: %v", r)
					})
				}
			}()

			select {
			case <-ctx.Done():
				once.Do(func() {
					firstErr = ctx.Err()
				})
				return
			default:
				if err := t(ctx); err != nil {
					once.Do(func() {
						firstErr = err
					})
				}
			}
		}(task)
	}

	wg.Wait()
	return firstErr
}

// ParallelLimit executes tasks concurrently with a maximum worker concurrency limit.
// This prevents resource or database connection pool exhaustion while executing batch tasks in parallel.
func ParallelLimit(ctx context.Context, concurrency int, tasks ...Task) error {
	if len(tasks) == 0 {
		return nil
	}
	if concurrency <= 0 {
		concurrency = 10
	}

	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup
	var once sync.Once
	var firstErr error

	for _, task := range tasks {
		if task == nil {
			continue
		}

		select {
		case <-ctx.Done():
			once.Do(func() {
				firstErr = ctx.Err()
			})
			return firstErr
		case sem <- struct{}{}:
		}

		wg.Add(1)
		go func(t Task) {
			defer wg.Done()
			defer func() { <-sem }()
			defer func() {
				if r := recover(); r != nil {
					slog.Error("parallel limit task panic recovered",
						"panic", r,
						"stack", string(debug.Stack()),
					)
					once.Do(func() {
						firstErr = fmt.Errorf("task panicked: %v", r)
					})
				}
			}()

			select {
			case <-ctx.Done():
				once.Do(func() {
					firstErr = ctx.Err()
				})
				return
			default:
				if err := t(ctx); err != nil {
					once.Do(func() {
						firstErr = err
					})
				}
			}
		}(task)
	}

	wg.Wait()
	return firstErr
}
