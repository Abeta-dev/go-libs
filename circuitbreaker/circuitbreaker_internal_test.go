// SPDX-License-Identifier: MIT

package circuitbreaker

import (
	"context"
	"testing"
	"time"
)

func TestCircuitBreaker_InternalBranches(t *testing.T) {
	// 1. ConsecutiveBreaker: req() cancels ctx and returns nil
	cb := NewConsecutiveBreaker(1, time.Minute, WithOnStateChange(func(from, to State) {}))
	ctx, cancel := context.WithCancel(context.Background())
	err := cb.Execute(ctx, func() error {
		cancel()
		return nil
	})
	if err == nil {
		t.Fatal("expected context canceled error when ctx is canceled inside req returning nil")
	}

	// 2. ConsecutiveBreaker: panic while StateClosed trips breaker and invokes onStateChange
	notified := false
	cbPanic := NewConsecutiveBreaker(1, time.Minute, WithOnStateChange(func(from, to State) {
		notified = true
	}))
	func() {
		defer func() { _ = recover() }()
		_ = cbPanic.Execute(context.Background(), func() error {
			panic("closed panic")
		})
	}()
	if !notified || cbPanic.State() != StateOpen {
		t.Fatal("expected panic in StateClosed to trip ConsecutiveBreaker and notify")
	}

	// 3. ConsecutiveBreaker: decrementHalfOpen below 0 and unknown state in canExecute
	cbPanic.halfOpenInFlight = 0
	cbPanic.decrementHalfOpen()
	if cbPanic.halfOpenInFlight != 0 {
		t.Fatalf("expected halfOpenInFlight clamped to 0, got %d", cbPanic.halfOpenInFlight)
	}
	cbPanic.state = State(99)
	allowed, isProbe := cbPanic.canExecute()
	if allowed || isProbe {
		t.Fatal("expected unknown state to reject execution")
	}

	// 4. RatioBreaker: transition to same state, req() cancels ctx returning nil, panic in StateClosed
	rbNotified := false
	rb := NewRatioBreaker(0.5, 1, time.Minute, time.Minute, WithOnStateChange(func(from, to State) {
		rbNotified = true
	}))
	if rb.transition(StateClosed) != nil {
		t.Fatal("expected nil notify when transitioning to current state")
	}

	ctx2, cancel2 := context.WithCancel(context.Background())
	err2 := rb.Execute(ctx2, func() error {
		cancel2()
		return nil
	})
	if err2 == nil {
		t.Fatal("expected context canceled error when ctx2 is canceled inside req returning nil")
	}

	func() {
		defer func() { _ = recover() }()
		_ = rb.Execute(context.Background(), func() error {
			panic("ratio closed panic")
		})
	}()
	if !rbNotified || rb.State() != StateOpen {
		t.Fatal("expected panic in StateClosed to trip RatioBreaker and notify")
	}

	// 5. RatioBreaker: decrementHalfOpen below 0 and unknown state in canExecute
	rb.halfOpenInFlight = 0
	rb.decrementHalfOpen()
	if rb.halfOpenInFlight != 0 {
		t.Fatalf("expected halfOpenInFlight clamped to 0, got %d", rb.halfOpenInFlight)
	}
	rb.state = State(99)
	rAllowed, rProbe := rb.canExecute()
	if rAllowed || rProbe {
		t.Fatal("expected unknown state to reject execution")
	}
}
