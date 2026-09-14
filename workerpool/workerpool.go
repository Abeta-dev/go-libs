// SPDX-License-Identifier: MIT

// Package workerpool provides a bounded, panic-safe worker pool for concurrent background task execution.
package workerpool

import (
	"context"
	"errors"
	"log/slog"
	"runtime/debug"
	"sync"

	"github.com/umesh0492/go-libs/metrics"
)

// ErrPoolClosed is returned when submitting tasks to a closed pool.
var ErrPoolClosed = errors.New("workerpool: pool is closed")

// ErrQueueFull is returned when the task queue is at capacity and cannot accept new tasks.
var ErrQueueFull = errors.New("workerpool: queue full")

// Metrics holds optional metrics collectors for workerpool observability.
type Metrics struct {
	Submitted metrics.Counter
	Completed metrics.Counter
	Dropped   metrics.Counter
	Panics    metrics.Counter
	QueueSize metrics.Gauge
}

// Option configures a Pool.
type Option func(*Pool)

// WithLogger sets the structured logger used for error and panic logging.
func WithLogger(l *slog.Logger) Option {
	return func(p *Pool) {
		p.logger = l
	}
}

// WithPanicHandler registers a custom handler invoked when a worker panics.
func WithPanicHandler(fn func(r any, stack []byte)) Option {
	return func(p *Pool) {
		p.panicHandler = fn
	}
}

// WithMetrics attaches metrics collectors to the pool.
func WithMetrics(m Metrics) Option {
	return func(p *Pool) {
		p.metrics = m
	}
}

// Pool manages a fixed number of worker goroutines processing tasks from a buffered channel.
type Pool struct {
	workers      int
	tasks        chan func()
	wg           sync.WaitGroup
	mu           sync.RWMutex
	isClosed     bool
	closeOnce    sync.Once
	closed       chan struct{}
	logger       *slog.Logger
	panicHandler func(r any, stack []byte)
	metrics      Metrics
}

// New creates and starts a bounded worker pool with the specified number of workers,
// task queue capacity, and optional configuration options.
// If workers <= 0, it defaults to 1. If queueSize <= 0, it defaults to workers * 2.
func New(workers, queueSize int, opts ...Option) *Pool {
	if workers <= 0 {
		workers = 1
	}
	if queueSize <= 0 {
		queueSize = workers * 2
	}

	p := &Pool{
		workers: workers,
		tasks:   make(chan func(), queueSize),
		closed:  make(chan struct{}),
		logger:  slog.Default(),
		metrics: Metrics{
			Submitted: metrics.NoopCounter{},
			Completed: metrics.NoopCounter{},
			Dropped:   metrics.NoopCounter{},
			Panics:    metrics.NoopCounter{},
			QueueSize: metrics.NoopGauge{},
		},
	}

	for _, opt := range opts {
		opt(p)
	}
	if p.metrics.Submitted == nil {
		p.metrics.Submitted = metrics.NoopCounter{}
	}
	if p.metrics.Completed == nil {
		p.metrics.Completed = metrics.NoopCounter{}
	}
	if p.metrics.Dropped == nil {
		p.metrics.Dropped = metrics.NoopCounter{}
	}
	if p.metrics.Panics == nil {
		p.metrics.Panics = metrics.NoopCounter{}
	}
	if p.metrics.QueueSize == nil {
		p.metrics.QueueSize = metrics.NoopGauge{}
	}

	p.wg.Add(workers)
	for i := 0; i < workers; i++ {
		go p.worker()
	}

	return p
}

func (p *Pool) worker() {
	defer p.wg.Done()
	for task := range p.tasks {
		p.metrics.QueueSize.Set(float64(len(p.tasks)))
		p.executeSafely(task)
	}
}

func (p *Pool) executeSafely(task func()) {
	defer func() {
		if r := recover(); r != nil {
			p.metrics.Panics.Inc()
			stack := debug.Stack()
			if p.panicHandler != nil {
				func() {
					defer func() {
						if pr := recover(); pr != nil {
							p.logger.Error("workerpool panicHandler panicked",
								"panic", pr,
								"original_panic", r,
							)
						}
					}()
					p.panicHandler(r, stack)
				}()
			} else {
				p.logger.Error("workerpool worker panic recovered",
					"panic", r,
					"stack", string(stack),
				)
			}
		} else {
			p.metrics.Completed.Inc()
		}
	}()
	task()
}

// Submit enqueues a task for execution. Returns nil if queued successfully,
// ErrPoolClosed if the pool has been stopped or is stopping, ErrQueueFull if the
// queue is at capacity, or an error if the task is nil. It is guaranteed safe
// against concurrent Stop/StopWait/Drain without sending on a closed channel.
func (p *Pool) Submit(task func()) error {
	if task == nil {
		return errors.New("workerpool: task cannot be nil")
	}

	p.mu.RLock()
	defer p.mu.RUnlock()

	if p.isClosed {
		p.metrics.Dropped.Inc()
		return ErrPoolClosed
	}

	select {
	case <-p.closed:
		p.metrics.Dropped.Inc()
		return ErrPoolClosed
	default:
	}

	select {
	case p.tasks <- task:
		p.metrics.Submitted.Inc()
		p.metrics.QueueSize.Set(float64(len(p.tasks)))
		return nil
	default:
		p.metrics.Dropped.Inc()
		return ErrQueueFull
	}
}

// SubmitContext enqueues a task, blocking until the task is accepted, the context expires,
// or the pool is closed. It is guaranteed safe against concurrent Stop/StopWait/Drain
// with zero risk of sending on a closed channel or leaking goroutines.
func (p *Pool) SubmitContext(ctx context.Context, task func()) error {
	if task == nil {
		return errors.New("workerpool: task cannot be nil")
	}
	if err := ctx.Err(); err != nil {
		p.metrics.Dropped.Inc()
		return err
	}

	p.mu.RLock()
	if p.isClosed {
		p.mu.RUnlock()
		p.metrics.Dropped.Inc()
		return ErrPoolClosed
	}

	select {
	case <-p.closed:
		p.mu.RUnlock()
		p.metrics.Dropped.Inc()
		return ErrPoolClosed
	default:
	}

	select {
	case <-p.closed:
		p.mu.RUnlock()
		p.metrics.Dropped.Inc()
		return ErrPoolClosed
	case <-ctx.Done():
		p.mu.RUnlock()
		p.metrics.Dropped.Inc()
		return ctx.Err()
	case p.tasks <- task:
		p.mu.RUnlock()
		p.metrics.Submitted.Inc()
		p.metrics.QueueSize.Set(float64(len(p.tasks)))
		return nil
	}
}

// Stop closes the task queue and stops accepting new tasks. Existing queued tasks will still be processed.
// It synchronizes with concurrent Submit/SubmitContext calls using p.mu to prevent channel panics.
func (p *Pool) Stop() {
	p.closeOnce.Do(func() {
		close(p.closed)
		p.mu.Lock()
		p.isClosed = true
		close(p.tasks)
		p.mu.Unlock()
	})
}

// StopWait stops the pool and waits for all active workers to complete.
func (p *Pool) StopWait() {
	p.Stop()
	p.wg.Wait()
}

// Drain stops accepting new tasks and waits for all active and queued tasks to complete.
// It is equivalent to StopWait.
func (p *Pool) Drain() {
	p.StopWait()
}

// IsClosed returns true if the pool has been stopped or is in the process of stopping.
func (p *Pool) IsClosed() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.isClosed {
		return true
	}
	select {
	case <-p.closed:
		return true
	default:
		return false
	}
}
