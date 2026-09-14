// SPDX-License-Identifier: MIT

package retry_test

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/umesh0492/go-libs/retry"
)

var errFake = errors.New("fake error")

func TestDo_SuccessOnFirstAttempt(t *testing.T) {
	calls := 0
	err := retry.Do(context.Background(), retry.Config{Attempts: 3}, func(_ context.Context) error {
		calls++
		return nil
	})
	assert.NoError(t, err)
	assert.Equal(t, 1, calls)
}

func TestDo_SuccessOnThirdAttempt(t *testing.T) {
	var calls int32
	err := retry.Do(context.Background(), retry.Config{
		Attempts:    5,
		InitialWait: time.Millisecond,
	}, func(_ context.Context) error {
		n := atomic.AddInt32(&calls, 1)
		if n < 3 {
			return errFake
		}
		return nil
	})
	assert.NoError(t, err)
	assert.Equal(t, int32(3), atomic.LoadInt32(&calls))
}

func TestDo_AllAttemptsFail(t *testing.T) {
	err := retry.Do(context.Background(), retry.Config{
		Attempts:    3,
		InitialWait: time.Millisecond,
	}, func(_ context.Context) error {
		return errFake
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, retry.ErrMaxAttemptsReached)
	assert.ErrorIs(t, err, errFake)
}

func TestDo_ZeroAttempts_DefaultsToOne(t *testing.T) {
	calls := 0
	err := retry.Do(context.Background(), retry.Config{Attempts: 0}, func(_ context.Context) error {
		calls++
		return errFake
	})
	require.Error(t, err)
	assert.Equal(t, 1, calls)
}

func TestDo_ContextCancelledBeforeAttempt(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	calls := 0
	err := retry.Do(ctx, retry.Config{Attempts: 3}, func(_ context.Context) error {
		calls++
		return nil
	})
	assert.ErrorIs(t, err, context.Canceled)
	assert.Equal(t, 0, calls)
}

func TestDo_ContextCancelledDuringSleep(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()

	calls := 0
	err := retry.Do(ctx, retry.Config{
		Attempts:    10,
		InitialWait: 200 * time.Millisecond, // longer than timeout
	}, func(_ context.Context) error {
		calls++
		return errFake
	})
	require.Error(t, err)
	// First call fails, then sleeps, context expires during sleep
	assert.Equal(t, 1, calls)
}

func TestDo_ShouldRetry_False_StopsEarly(t *testing.T) {
	calls := 0
	errPermanent := errors.New("permanent")
	err := retry.Do(context.Background(), retry.Config{
		Attempts:    5,
		InitialWait: time.Millisecond,
		ShouldRetry: func(e error) bool { return !errors.Is(e, errPermanent) },
	}, func(_ context.Context) error {
		calls++
		return errPermanent
	})
	require.Error(t, err)
	assert.Equal(t, 1, calls)
	assert.ErrorIs(t, err, errPermanent)
}

func TestDoWithResult_Success(t *testing.T) {
	calls := 0
	val, err := retry.DoWithResult(context.Background(), retry.Config{
		Attempts:    3,
		InitialWait: time.Millisecond,
	}, func(_ context.Context) (int, error) {
		calls++
		if calls < 2 {
			return 0, errFake
		}
		return 42, nil
	})
	assert.NoError(t, err)
	assert.Equal(t, 42, val)
}

func TestDoWithResult_AllFail(t *testing.T) {
	val, err := retry.DoWithResult(context.Background(), retry.Config{
		Attempts: 2,
	}, func(_ context.Context) (string, error) {
		return "", errFake
	})
	assert.Error(t, err)
	assert.ErrorIs(t, err, retry.ErrMaxAttemptsReached)
	assert.Empty(t, val)
}

func TestStrategies_NoZeroWait(t *testing.T) {
	strategies := []retry.Strategy{
		retry.Constant,
		retry.Linear,
		retry.Exponential,
		retry.ExponentialJitter,
	}
	for _, s := range strategies {
		var calls int32
		err := retry.Do(context.Background(), retry.Config{
			Attempts:    3,
			InitialWait: time.Millisecond,
			MaxWait:     10 * time.Millisecond,
			Strategy:    s,
		}, func(_ context.Context) error {
			atomic.AddInt32(&calls, 1)
			return errFake
		})
		assert.Error(t, err)
		assert.Equal(t, int32(3), atomic.LoadInt32(&calls))
	}
}

func TestDo_NoInitialWait_NoSleep(t *testing.T) {
	// Should complete immediately since InitialWait = 0
	start := time.Now()
	err := retry.Do(context.Background(), retry.Config{
		Attempts:    3,
		InitialWait: 0,
	}, func(_ context.Context) error {
		return errFake
	})
	assert.Error(t, err)
	assert.Less(t, time.Since(start), 50*time.Millisecond)
}

func TestRetry_Adversarial_HighAttemptsNoOverflow(t *testing.T) {
	// 100 attempts on Exponential and ExponentialJitter must never wrap to negative duration
	for _, strategy := range []retry.Strategy{retry.Exponential, retry.ExponentialJitter} {
		var calls int32
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
		defer cancel()

		err := retry.Do(ctx, retry.Config{
			Attempts:    100,
			InitialWait: time.Millisecond,
			MaxWait:     5 * time.Millisecond,
			Strategy:    strategy,
		}, func(_ context.Context) error {
			atomic.AddInt32(&calls, 1)
			return errFake
		})
		assert.Error(t, err)
		// Should execute several attempts bounded by MaxWait without spinning through 100 instantly
		assert.Greater(t, atomic.LoadInt32(&calls), int32(1))
		assert.Less(t, atomic.LoadInt32(&calls), int32(100))
	}
}

func TestRetry_Adversarial_ConcurrentContextCancel(t *testing.T) {
	var calls int32
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Millisecond)
	defer cancel()

	err := retry.Do(ctx, retry.Config{
		Attempts:    10,
		InitialWait: 50 * time.Millisecond, // long sleep
		Strategy:    retry.Exponential,
	}, func(_ context.Context) error {
		atomic.AddInt32(&calls, 1)
		return errFake
	})
	assert.ErrorIs(t, err, context.DeadlineExceeded)
	assert.Equal(t, int32(1), atomic.LoadInt32(&calls))
}
