// SPDX-License-Identifier: MIT

package circuitbreaker_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/umesh0492/go-libs/circuitbreaker"
)

func TestConsecutiveBreaker_ClientCancellationDoesNotTripBreaker(t *testing.T) {
	// Breaker configured with threshold of 2 failures
	cb := circuitbreaker.NewConsecutiveBreaker(2, 50*time.Millisecond)

	// Simulate repeated client cancellations (e.g. client aborts/disconnects mid-request)
	for i := 0; i < 5; i++ {
		ctx, cancel := context.WithCancel(context.Background())
		err := cb.Execute(ctx, func() error {
			cancel() // cancel context during execution
			return ctx.Err()
		})
		assert.ErrorIs(t, err, context.Canceled)
		assert.Equal(t, 0, cb.Failures(), "client cancellation must not increment failure counter")
		assert.Equal(t, circuitbreaker.StateClosed, cb.State(), "breaker must stay CLOSED despite cancellations")
	}

	// Breaker should still accept requests and succeed
	executed := false
	err := cb.Execute(context.Background(), func() error {
		executed = true
		return nil
	})
	assert.NoError(t, err)
	assert.True(t, executed)
	assert.Equal(t, circuitbreaker.StateClosed, cb.State())

	// Also verify returned context.Canceled error directly from req()
	for i := 0; i < 5; i++ {
		err := cb.Execute(context.Background(), func() error {
			return context.Canceled
		})
		assert.ErrorIs(t, err, context.Canceled)
		assert.Equal(t, 0, cb.Failures())
		assert.Equal(t, circuitbreaker.StateClosed, cb.State())
	}

	// Verify that real failures DO increment and eventually trip the breaker
	realErr := errors.New("server downstream 500")
	_ = cb.Execute(context.Background(), func() error { return realErr })
	assert.Equal(t, 1, cb.Failures())
	assert.Equal(t, circuitbreaker.StateClosed, cb.State())

	_ = cb.Execute(context.Background(), func() error { return realErr })
	assert.Equal(t, 2, cb.Failures())
	assert.Equal(t, circuitbreaker.StateOpen, cb.State(), "real failures must trip the breaker")
}

func TestConsecutiveBreaker_HalfOpenProbeCancellation(t *testing.T) {
	cb := circuitbreaker.NewConsecutiveBreaker(1, 10*time.Millisecond)
	_ = cb.Execute(context.Background(), func() error { return errors.New("trip") })
	assert.Equal(t, circuitbreaker.StateOpen, cb.State())

	time.Sleep(15 * time.Millisecond)
	assert.Equal(t, circuitbreaker.StateHalfOpen, cb.State())

	// Canary probe cancelled mid-execution
	ctx, cancel := context.WithCancel(context.Background())
	err := cb.Execute(ctx, func() error {
		cancel()
		return ctx.Err()
	})
	assert.ErrorIs(t, err, context.Canceled)

	// Breaker should not have transitioned to Open; canary slot freed
	err = cb.Execute(context.Background(), func() error {
		return nil
	})
	assert.NoError(t, err)
	assert.Equal(t, circuitbreaker.StateClosed, cb.State())
}

func TestRatioBreaker_ClientCancellation(t *testing.T) {
	rb := circuitbreaker.NewRatioBreaker(0.5, 2, 50*time.Millisecond, 10*time.Millisecond)

	// Repeated cancellations should not count as failures
	for i := 0; i < 5; i++ {
		err := rb.Execute(context.Background(), func() error {
			return context.Canceled
		})
		assert.ErrorIs(t, err, context.Canceled)
		assert.Equal(t, 0, rb.Failures())
		assert.Equal(t, circuitbreaker.StateClosed, rb.State())
	}

	// Trip it open with real failures
	_ = rb.Execute(context.Background(), func() error { return errors.New("err1") })
	_ = rb.Execute(context.Background(), func() error { return errors.New("err2") })
	assert.Equal(t, circuitbreaker.StateOpen, rb.State())

	time.Sleep(15 * time.Millisecond)
	assert.Equal(t, circuitbreaker.StateHalfOpen, rb.State())

	// Probe cancelled
	err := rb.Execute(context.Background(), func() error {
		return context.Canceled
	})
	assert.ErrorIs(t, err, context.Canceled)

	// Legitimate probe succeeds
	err = rb.Execute(context.Background(), func() error {
		return nil
	})
	assert.NoError(t, err)
	assert.Equal(t, circuitbreaker.StateClosed, rb.State())
}

