// SPDX-License-Identifier: MIT

package circuitbreaker_test

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/umesh0492/go-libs/circuitbreaker"
)

func TestRatioBreaker_HalfOpenProbeThrottling(t *testing.T) {
	// 50% failure ratio, min 2 requests, 50ms window, 20ms resetTimeout
	rb := circuitbreaker.NewRatioBreaker(0.5, 2, 50*time.Millisecond, 20*time.Millisecond)
	ctx := context.Background()

	// 1. Trip breaker to Open
	_ = rb.Execute(ctx, func() error { return errors.New("err 1") })
	_ = rb.Execute(ctx, func() error { return errors.New("err 2") })
	assert.Equal(t, circuitbreaker.StateOpen, rb.State())

	// 2. Wait for reset timeout and verify State() transitions to StateHalfOpen under write lock
	time.Sleep(25 * time.Millisecond)
	assert.Equal(t, circuitbreaker.StateHalfOpen, rb.State())

	// 3. Launch 50 concurrent requests: exactly 1 must probe, 49 must be rejected immediately
	concurrency := 50
	var executedCount int32
	var rejectedCount int32
	var wg sync.WaitGroup

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := rb.Execute(ctx, func() error {
				atomic.AddInt32(&executedCount, 1)
				time.Sleep(10 * time.Millisecond)
				return nil
			})
			if errors.Is(err, circuitbreaker.ErrCircuitOpen) {
				atomic.AddInt32(&rejectedCount, 1)
			}
		}()
	}

	wg.Wait()

	assert.Equal(t, int32(1), atomic.LoadInt32(&executedCount), "Strict probe quota: only 1 probe in flight during half-open")
	assert.Equal(t, int32(49), atomic.LoadInt32(&rejectedCount), "All concurrent non-probe requests must be rejected")
	assert.Equal(t, circuitbreaker.StateClosed, rb.State(), "Successful probe must recover breaker to Closed")
}

func TestRatioBreaker_ContextCancellation(t *testing.T) {
	rb := circuitbreaker.NewRatioBreaker(0.5, 2, 50*time.Millisecond, 20*time.Millisecond)

	// 1. Pre-cancelled context must immediately return context.Canceled without executing func
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	called := false
	err := rb.Execute(ctx, func() error {
		called = true
		return nil
	})
	assert.ErrorIs(t, err, context.Canceled)
	assert.False(t, called)
	assert.Equal(t, circuitbreaker.StateClosed, rb.State())

	// 2. Context cancelled during probe must return context error and re-trip to StateOpen
	_ = rb.Execute(context.Background(), func() error { return errors.New("err 1") })
	_ = rb.Execute(context.Background(), func() error { return errors.New("err 2") })
	assert.Equal(t, circuitbreaker.StateOpen, rb.State())

	time.Sleep(25 * time.Millisecond)
	assert.Equal(t, circuitbreaker.StateHalfOpen, rb.State())

	probeCtx, probeCancel := context.WithCancel(context.Background())
	probeCancel()

	// Calling Execute with cancelled context during HalfOpen must not consume the probe
	err = rb.Execute(probeCtx, func() error {
		return nil
	})
	assert.ErrorIs(t, err, context.Canceled)

	// Breaker should still allow a subsequent legitimate probe
	legitProbeRan := false
	err = rb.Execute(context.Background(), func() error {
		legitProbeRan = true
		return nil
	})
	assert.NoError(t, err)
	assert.True(t, legitProbeRan)
	assert.Equal(t, circuitbreaker.StateClosed, rb.State())
}
