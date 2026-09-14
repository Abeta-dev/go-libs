// SPDX-License-Identifier: MIT

package workerpool_test

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/umesh0492/go-libs/workerpool"
)

func TestPool_DefaultOptions(t *testing.T) {
	p := workerpool.New(0, 0)
	defer p.StopWait()

	var executed int32
	err := p.Submit(func() {
		atomic.StoreInt32(&executed, 1)
	})
	assert.NoError(t, err)

	time.Sleep(50 * time.Millisecond)
	assert.Equal(t, int32(1), atomic.LoadInt32(&executed))
}

func TestPool_Submit_Nil(t *testing.T) {
	p := workerpool.New(2, 2)
	defer p.StopWait()

	assert.Error(t, p.Submit(nil))
}

func TestPool_SubmitContext_Nil(t *testing.T) {
	p := workerpool.New(2, 2)
	defer p.StopWait()

	err := p.SubmitContext(context.Background(), nil)
	assert.Error(t, err)
}

func TestPool_SubmitContext_PreCanceled(t *testing.T) {
	p := workerpool.New(2, 2)
	defer p.StopWait()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := p.SubmitContext(ctx, func() {})
	assert.ErrorIs(t, err, context.Canceled)
}

func TestPool_ConcurrentExecution(t *testing.T) {
	p := workerpool.New(4, 50)
	var count int32
	var wg sync.WaitGroup

	for i := 0; i < 20; i++ {
		wg.Add(1)
		err := p.Submit(func() {
			defer wg.Done()
			atomic.AddInt32(&count, 1)
		})
		assert.NoError(t, err)
	}

	wg.Wait()
	p.StopWait()
	assert.Equal(t, int32(20), atomic.LoadInt32(&count))
}

func TestPool_SubmitContext_SuccessAndCancel(t *testing.T) {
	p := workerpool.New(1, 1)
	defer p.StopWait()

	// Fill worker and queue
	blocker := make(chan struct{})
	defer close(blocker)

	err := p.SubmitContext(context.Background(), func() {
		<-blocker
	})
	assert.NoError(t, err)

	err = p.SubmitContext(context.Background(), func() {})
	assert.NoError(t, err)

	// Now queue is full; test timeout
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	err = p.SubmitContext(ctx, func() {})
	assert.ErrorIs(t, err, context.DeadlineExceeded)
}

func TestPool_Closed_Errors(t *testing.T) {
	p := workerpool.New(2, 5)
	p.StopWait()

	// Submitting after closed
	assert.ErrorIs(t, p.Submit(func() {}), workerpool.ErrPoolClosed)

	err := p.SubmitContext(context.Background(), func() {})
	assert.ErrorIs(t, err, workerpool.ErrPoolClosed)
}

func TestPool_ErrQueueFull(t *testing.T) {
	p := workerpool.New(1, 1)
	defer p.StopWait()

	blocker := make(chan struct{})
	defer close(blocker)

	_ = p.Submit(func() { <-blocker })
	_ = p.Submit(func() {})

	err := p.Submit(func() {})
	assert.ErrorIs(t, err, workerpool.ErrQueueFull)
}

func TestPool_PanicRecovery(t *testing.T) {
	p := workerpool.New(1, 2)
	defer p.StopWait()

	var secondExecuted int32
	_ = p.Submit(func() {
		panic("simulated worker failure")
	})

	time.Sleep(20 * time.Millisecond)

	_ = p.Submit(func() {
		atomic.StoreInt32(&secondExecuted, 1)
	})

	time.Sleep(20 * time.Millisecond)
	assert.Equal(t, int32(1), atomic.LoadInt32(&secondExecuted))
}

type safeCounter struct {
	mu sync.Mutex
	n  float64
}

func (c *safeCounter) Inc() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.n++
}

func (c *safeCounter) Add(d float64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.n += d
}

func (c *safeCounter) Get() float64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.n
}

type safeGauge struct {
	mu sync.Mutex
	n  float64
}

func (g *safeGauge) Set(v float64) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.n = v
}

func (g *safeGauge) Add(d float64) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.n += d
}

func TestPool_OptionsAndMetrics(t *testing.T) {
	subCounter := &safeCounter{}
	compCounter := &safeCounter{}
	dropCounter := &safeCounter{}
	panicCounter := &safeCounter{}
	queueGauge := &safeGauge{}

	var panicMu sync.Mutex
	var customPanicRecovered any

	p := workerpool.New(1, 10,
		workerpool.WithMetrics(workerpool.Metrics{
			Submitted: subCounter,
			Completed: compCounter,
			Dropped:   dropCounter,
			Panics:    panicCounter,
			QueueSize: queueGauge,
		}),
		workerpool.WithPanicHandler(func(r any, stack []byte) {
			panicMu.Lock()
			customPanicRecovered = r
			panicMu.Unlock()
		}),
	)
	defer p.StopWait()

	// 1. Submit normal task
	var wg sync.WaitGroup
	wg.Add(1)
	err := p.Submit(func() {
		defer wg.Done()
	})
	assert.NoError(t, err)
	wg.Wait()

	time.Sleep(20 * time.Millisecond)
	assert.Equal(t, 1.0, subCounter.Get())
	assert.Equal(t, 1.0, compCounter.Get())

	// 2. Panic task triggers custom panic handler and panics counter
	_ = p.Submit(func() {
		panic("custom pool panic")
	})
	time.Sleep(30 * time.Millisecond)
	assert.Equal(t, 1.0, panicCounter.Get())

	panicMu.Lock()
	recovered := customPanicRecovered
	panicMu.Unlock()
	assert.Equal(t, "custom pool panic", recovered)
}

func TestPool_Drain(t *testing.T) {
	p := workerpool.New(2, 10)
	var count atomic.Int32

	for i := 0; i < 5; i++ {
		err := p.Submit(func() {
			time.Sleep(10 * time.Millisecond)
			count.Add(1)
		})
		assert.NoError(t, err)
	}

	p.Drain()
	assert.Equal(t, int32(5), count.Load())
	assert.True(t, p.IsClosed())

	err := p.Submit(func() {})
	assert.ErrorIs(t, err, workerpool.ErrPoolClosed)
}

func TestPool_IsClosed(t *testing.T) {
	p := workerpool.New(2, 4)
	assert.False(t, p.IsClosed())

	p.Stop()
	assert.True(t, p.IsClosed())
	p.StopWait()
	assert.True(t, p.IsClosed())
}

func TestPool_Adversarial_ConcurrentSubmitAndStop(t *testing.T) {
	for iteration := 0; iteration < 20; iteration++ {
		p := workerpool.New(4, 10)
		var wg sync.WaitGroup
		concurrency := 50

		// Launch 50 concurrent submitters
		for i := 0; i < concurrency; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for j := 0; j < 50; j++ {
					err := p.Submit(func() {
						time.Sleep(10 * time.Microsecond)
					})
					if err != nil {
						assert.True(t, errors.Is(err, workerpool.ErrPoolClosed) || errors.Is(err, workerpool.ErrQueueFull))
					}
				}
			}()
		}

		// Concurrently stop the pool
		time.Sleep(200 * time.Microsecond)
		p.StopWait()
		wg.Wait()
	}
}

func TestPool_Adversarial_ConcurrentSubmitContextAndStop(t *testing.T) {
	for iteration := 0; iteration < 10; iteration++ {
		p := workerpool.New(2, 5)
		var wg sync.WaitGroup
		concurrency := 30

		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
		defer cancel()

		for i := 0; i < concurrency; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for j := 0; j < 20; j++ {
					err := p.SubmitContext(ctx, func() {
						time.Sleep(50 * time.Microsecond)
					})
					if err != nil {
						assert.True(t, errors.Is(err, workerpool.ErrPoolClosed) || errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled))
					}
				}
			}()
		}

		time.Sleep(500 * time.Microsecond)
		p.StopWait()
		wg.Wait()
	}
}

func TestPool_Adversarial_PanickingPanicHandlerKeepsWorkersAlive(t *testing.T) {
	// Create pool with exactly 1 worker
	p := workerpool.New(1, 10, workerpool.WithPanicHandler(func(r any, stack []byte) {
		// Panic handler itself maliciously panics!
		panic("panicHandler explosion")
	}))
	defer p.StopWait()

	// Submit task that panics
	err := p.Submit(func() {
		panic("task explosion")
	})
	assert.NoError(t, err)

	// Sleep briefly for worker to process and recover
	time.Sleep(20 * time.Millisecond)

	// Now submit normal tasks: the worker must still be alive!
	var completedCount atomic.Int32
	for i := 0; i < 5; i++ {
		err := p.Submit(func() {
			completedCount.Add(1)
		})
		assert.NoError(t, err)
	}

	time.Sleep(20 * time.Millisecond)
	assert.Equal(t, int32(5), completedCount.Load(), "Worker goroutine must not have died when panicHandler panicked")
}

func TestPool_Adversarial_SubmitContextBurstUnderStop(t *testing.T) {
	p := workerpool.New(2, 4)
	var wg sync.WaitGroup
	ctx := context.Background()

	// Launch 50 concurrent submissions trying to block on full queue
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = p.SubmitContext(ctx, func() {
				time.Sleep(200 * time.Microsecond)
			})
		}()
	}

	time.Sleep(100 * time.Microsecond)
	p.StopWait()
	wg.Wait()
}

func TestPool_Adversarial_ConcurrentStopAnd10kSubmit(t *testing.T) {
	p := workerpool.New(8, 20000)
	var wg sync.WaitGroup
	ctx := context.Background()
	total := 10000

	for i := 0; i < total; i++ {
		wg.Add(1)
		idx := i
		go func() {
			defer wg.Done()
			var err error
			if idx%2 == 0 {
				err = p.Submit(func() {
					time.Sleep(10 * time.Microsecond)
				})
			} else {
				err = p.SubmitContext(ctx, func() {
					time.Sleep(10 * time.Microsecond)
				})
			}
			if err != nil {
				assert.ErrorIs(t, err, workerpool.ErrPoolClosed)
			}
		}()
	}

	// Concurrently stop the pool while submissions are pouring in
	time.Sleep(500 * time.Microsecond)
	p.StopWait()
	wg.Wait()
}

// TestPool_HighConcurrencyRace_100GoroutinesSubmitAndStop fulfills Requirement 4:
// 100 goroutines concurrently call Submit/SubmitContext while another goroutine calls Stop(),
// asserting ZERO panics occur under 'go test -race ./workerpool'.
func TestPool_HighConcurrencyRace_100GoroutinesSubmitAndStop(t *testing.T) {
	for iter := 0; iter < 10; iter++ {
		p := workerpool.New(8, 20)
		var wg sync.WaitGroup
		var panicCount atomic.Int64
		numGoroutines := 100

		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()

		startSignal := make(chan struct{})

		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			idx := i
			go func() {
				defer wg.Done()
				defer func() {
					if r := recover(); r != nil {
						panicCount.Add(1)
						t.Errorf("goroutine %d panicked: %v", idx, r)
					}
				}()

				<-startSignal

				for j := 0; j < 50; j++ {
					var err error
					if (idx+j)%2 == 0 {
						err = p.Submit(func() {
							time.Sleep(10 * time.Microsecond)
						})
						if err != nil {
							assert.True(t, errors.Is(err, workerpool.ErrPoolClosed) || errors.Is(err, workerpool.ErrQueueFull))
						}
					} else {
						err = p.SubmitContext(ctx, func() {
							time.Sleep(10 * time.Microsecond)
						})
						if err != nil {
							assert.True(t, errors.Is(err, workerpool.ErrPoolClosed) || errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled))
						}
					}
				}
			}()
		}

		// Dedicated goroutine calling Stop() concurrently
		var stopWg sync.WaitGroup
		stopWg.Add(1)
		go func() {
			defer stopWg.Done()
			defer func() {
				if r := recover(); r != nil {
					panicCount.Add(1)
					t.Errorf("stop goroutine panicked: %v", r)
				}
			}()

			<-startSignal
			time.Sleep(500 * time.Microsecond)
			p.Stop()
		}()

		close(startSignal)
		wg.Wait()
		stopWg.Wait()
		p.StopWait()

		assert.Equal(t, int64(0), panicCount.Load(), "asserting ZERO panics occur during concurrent Submit/SubmitContext and Stop")
	}
}

func TestPool_HighConcurrencyRace_100GoroutinesSubmitAndDrain(t *testing.T) {
	for iter := 0; iter < 5; iter++ {
		p := workerpool.New(4, 15)
		var wg sync.WaitGroup
		var panicCount atomic.Int64
		numGoroutines := 100

		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()

		startSignal := make(chan struct{})

		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			idx := i
			go func() {
				defer wg.Done()
				defer func() {
					if r := recover(); r != nil {
						panicCount.Add(1)
						t.Errorf("goroutine %d panicked: %v", idx, r)
					}
				}()

				<-startSignal

				for j := 0; j < 30; j++ {
					var err error
					if (idx+j)%2 == 0 {
						err = p.Submit(func() {
							time.Sleep(10 * time.Microsecond)
						})
						if err != nil {
							assert.True(t, errors.Is(err, workerpool.ErrPoolClosed) || errors.Is(err, workerpool.ErrQueueFull))
						}
					} else {
						err = p.SubmitContext(ctx, func() {
							time.Sleep(10 * time.Microsecond)
						})
						if err != nil {
							assert.True(t, errors.Is(err, workerpool.ErrPoolClosed) || errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled))
						}
					}
				}
			}()
		}

		var drainWg sync.WaitGroup
		drainWg.Add(1)
		go func() {
			defer drainWg.Done()
			defer func() {
				if r := recover(); r != nil {
					panicCount.Add(1)
					t.Errorf("drain goroutine panicked: %v", r)
				}
			}()

			<-startSignal
			time.Sleep(300 * time.Microsecond)
			p.Drain()
		}()

		close(startSignal)
		wg.Wait()
		drainWg.Wait()

		assert.Equal(t, int64(0), panicCount.Load(), "asserting ZERO panics occur during concurrent Submit/SubmitContext and Drain")
	}
}
