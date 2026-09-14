// SPDX-License-Identifier: MIT

// Package retry provides context-aware retry execution with configurable
// backoff strategies. It has zero external dependencies (stdlib only).
//
// Four backoff strategies are built-in:
//   - [Constant]     — fixed wait between attempts
//   - [Linear]       — wait grows linearly (attempt * InitialWait)
//   - [Exponential]  — wait doubles each attempt, capped at MaxWait
//   - [ExponentialJitter] — exponential with full jitter (recommended for distributed systems)
//
// Usage:
//
//	err := retry.Do(ctx, retry.Config{
//	    Attempts:    5,
//	    InitialWait: 100 * time.Millisecond,
//	    MaxWait:     5 * time.Second,
//	    Strategy:    retry.ExponentialJitter,
//	}, func(ctx context.Context) error {
//	    return callExternalService(ctx)
//	})
package retry

import (
	"context"
	"errors"
	"math"
	"math/rand"
	"time"
)

// Strategy defines how the wait time between attempts is calculated.
type Strategy int

const (
	// Constant waits InitialWait between every attempt.
	Constant Strategy = iota
	// Linear increases the wait by InitialWait each attempt.
	Linear
	// Exponential doubles the wait each attempt, capped at MaxWait.
	Exponential
	// ExponentialJitter applies full jitter to exponential backoff.
	// Recommended for distributed systems to avoid thundering-herd.
	ExponentialJitter
)

// Config controls retry behaviour.
type Config struct {
	// Attempts is the maximum number of times fn will be called. Must be >= 1.
	Attempts int
	// InitialWait is the base sleep duration between attempts.
	// For Constant strategy this is the fixed wait; for others it is the starting value.
	InitialWait time.Duration
	// MaxWait caps the computed wait duration. Zero means no cap.
	MaxWait time.Duration
	// Strategy selects the backoff algorithm. Defaults to Constant.
	Strategy Strategy
	// ShouldRetry is an optional predicate. When non-nil, retry only continues
	// if ShouldRetry(err) returns true. Default: always retry on non-nil error.
	ShouldRetry func(err error) bool
}

// ErrMaxAttemptsReached is wrapped around the last error when all attempts are exhausted.
var ErrMaxAttemptsReached = errors.New("retry: max attempts reached")

// Do calls fn up to cfg.Attempts times, sleeping between failures according
// to the configured backoff strategy. It returns nil on first success.
// If all attempts fail it returns an error wrapping the last fn error
// with ErrMaxAttemptsReached in the chain.
func Do(ctx context.Context, cfg Config, fn func(context.Context) error) error {
	_, err := doInternal(ctx, cfg, func(ctx context.Context) (struct{}, error) {
		return struct{}{}, fn(ctx)
	})
	return err
}

// DoWithResult is like Do but fn returns a value in addition to an error.
// On success it returns the value; on failure it returns the zero value and an error.
func DoWithResult[T any](ctx context.Context, cfg Config, fn func(context.Context) (T, error)) (T, error) {
	return doInternal(ctx, cfg, fn)
}

func doInternal[T any](ctx context.Context, cfg Config, fn func(context.Context) (T, error)) (T, error) {
	if cfg.Attempts <= 0 {
		cfg.Attempts = 1
	}

	shouldRetry := cfg.ShouldRetry
	if shouldRetry == nil {
		shouldRetry = func(error) bool { return true }
	}

	var (
		zero T
		last error
	)

	for attempt := 0; attempt < cfg.Attempts; attempt++ {
		// Respect context cancellation before each attempt.
		if err := ctx.Err(); err != nil {
			return zero, err
		}

		val, err := fn(ctx)
		if err == nil {
			return val, nil
		}
		last = err

		if !shouldRetry(err) {
			return zero, err
		}

		// Do not sleep after the final attempt.
		if attempt == cfg.Attempts-1 {
			break
		}

		wait := computeWait(cfg, attempt)
		if wait > 0 {
			timer := time.NewTimer(wait)
			select {
			case <-ctx.Done():
				timer.Stop()
				return zero, ctx.Err()
			case <-timer.C:
			}
		}
	}

	return zero, errors.Join(ErrMaxAttemptsReached, last)
}

// computeWait returns the sleep duration for the given attempt index (0-based).
func computeWait(cfg Config, attempt int) time.Duration {
	if cfg.InitialWait <= 0 {
		return 0
	}

	// Cap attempt exponent at 30 to prevent float/int64 overflow wrapping into negative duration.
	expAttempt := attempt
	if expAttempt > 30 {
		expAttempt = 30
	}

	var d time.Duration
	switch cfg.Strategy {
	case Linear:
		if attempt > 1000000 {
			attempt = 1000000
		}
		d = cfg.InitialWait * time.Duration(attempt+1)
	case Exponential:
		// 2^attempt * InitialWait, capped at MaxWait
		mult := math.Pow(2, float64(expAttempt))
		val := float64(cfg.InitialWait) * mult
		if val >= float64(math.MaxInt64) {
			d = time.Duration(math.MaxInt64)
		} else {
			d = time.Duration(val)
		}
	case ExponentialJitter:
		// Full jitter: random value in [0, 2^attempt * InitialWait]
		ceiling := float64(cfg.InitialWait) * math.Pow(2, float64(expAttempt))
		if ceiling >= float64(math.MaxInt64) {
			ceiling = float64(math.MaxInt64)
		}
		// #nosec G404 -- crypto randomness not required for backoff jitter
		//nolint:gosec
		d = time.Duration(rand.Float64() * ceiling)
	default: // Constant
		d = cfg.InitialWait
	}

	if d < 0 {
		d = 0
	}

	if cfg.MaxWait > 0 && d > cfg.MaxWait {
		d = cfg.MaxWait
	}
	return d
}
