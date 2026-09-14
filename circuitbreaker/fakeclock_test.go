// SPDX-License-Identifier: MIT
package circuitbreaker_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/umesh0492/go-libs/circuitbreaker"
	"github.com/umesh0492/go-libs/clock"
)

func TestConsecutiveBreaker_WithFakeClock_DeterministicReset(t *testing.T) {
	fc := clock.NewFake()
	resetTimeout := 30 * time.Second

	cb := circuitbreaker.NewConsecutiveBreaker(2, resetTimeout, circuitbreaker.WithClock(fc))

	if cb.State() != circuitbreaker.StateClosed {
		t.Fatalf("expected initial state Closed, got %v", cb.State())
	}

	testErr := errors.New("upstream failed")

	// Fail 1
	_ = cb.Execute(context.Background(), func() error { return testErr })
	if cb.State() != circuitbreaker.StateClosed {
		t.Fatalf("expected state Closed after 1 failure, got %v", cb.State())
	}

	// Fail 2 -> Trip to Open
	_ = cb.Execute(context.Background(), func() error { return testErr })
	if cb.State() != circuitbreaker.StateOpen {
		t.Fatalf("expected state Open after 2 failures, got %v", cb.State())
	}

	// Advance time by 29s -> still open
	fc.Add(29 * time.Second)
	err := cb.Execute(context.Background(), func() error { return nil })
	if !errors.Is(err, circuitbreaker.ErrCircuitOpen) {
		t.Fatalf("expected ErrCircuitOpen at 29s, got %v", err)
	}
	if cb.State() != circuitbreaker.StateOpen {
		t.Fatalf("expected state Open at 29s, got %v", cb.State())
	}

	// Advance time by 2s (total 31s > 30s resetTimeout) -> HalfOpen probe succeeds -> Closed
	fc.Add(2 * time.Second)
	err = cb.Execute(context.Background(), func() error { return nil })
	if err != nil {
		t.Fatalf("expected successful probe, got %v", err)
	}
	if cb.State() != circuitbreaker.StateClosed {
		t.Fatalf("expected state Closed after successful probe, got %v", cb.State())
	}
}

func TestRatioBreaker_WithFakeClock_DeterministicReset(t *testing.T) {
	fc := clock.NewFake()
	window := 10 * time.Second
	resetTimeout := 30 * time.Second

	rb := circuitbreaker.NewRatioBreaker(0.5, 4, window, resetTimeout, circuitbreaker.WithClock(fc))

	testErr := errors.New("upstream error")

	// 2 successes, 2 failures -> 50% failure rate -> Open
	_ = rb.Execute(context.Background(), func() error { return nil })
	_ = rb.Execute(context.Background(), func() error { return nil })
	_ = rb.Execute(context.Background(), func() error { return testErr })
	_ = rb.Execute(context.Background(), func() error { return testErr })

	if rb.State() != circuitbreaker.StateOpen {
		t.Fatalf("expected state Open after 50%% failures, got %v", rb.State())
	}

	// 15 seconds later -> still open
	fc.Add(15 * time.Second)
	err := rb.Execute(context.Background(), func() error { return nil })
	if !errors.Is(err, circuitbreaker.ErrCircuitOpen) {
		t.Fatalf("expected ErrCircuitOpen, got %v", err)
	}

	// Advance past 30s reset timeout -> probe succeeds -> Closed
	fc.Add(16 * time.Second)
	err = rb.Execute(context.Background(), func() error { return nil })
	if err != nil {
		t.Fatalf("expected successful probe, got %v", err)
	}
	if rb.State() != circuitbreaker.StateClosed {
		t.Fatalf("expected state Closed after successful probe, got %v", rb.State())
	}
}
