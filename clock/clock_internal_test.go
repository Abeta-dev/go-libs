// SPDX-License-Identifier: MIT

package clock

import (
	"testing"
	"time"
)

func TestFakeClock_InternalEdgeCases(t *testing.T) {
	fc := NewFake()

	// 1. Ticker created with period <= 0 and then Reset to > 0
	tick := fc.NewTicker(0)
	tick.Reset(5 * time.Second)
	if fc.activeWaitersLocked() != 1 {
		t.Fatalf("expected 1 active ticker waiter, got %d", fc.activeWaitersLocked())
	}

	// Reset ticker period to 0 while already in tickers slice, then advance
	tick.Reset(0)
	fc.Add(1 * time.Second)

	// 2. Sleep waiter that has not yet expired when advancing partially,
	// plus an already-done sleep waiter in the slice
	fc.sleepWaiters = append(fc.sleepWaiters,
		&sleepWaiter{deadline: fc.Now().Add(10 * time.Second), ch: make(chan struct{}), done: true},
		&sleepWaiter{deadline: fc.Now().Add(10 * time.Second), ch: make(chan struct{}), done: false},
	)
	fc.Add(2 * time.Second)
	if len(fc.sleepWaiters) != 1 {
		t.Fatalf("expected 1 remaining sleep waiter, got %d", len(fc.sleepWaiters))
	}
	fc.Add(10 * time.Second)
	if len(fc.sleepWaiters) != 0 {
		t.Fatalf("expected 0 remaining sleep waiters, got %d", len(fc.sleepWaiters))
	}
}
