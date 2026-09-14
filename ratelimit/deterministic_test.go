// SPDX-License-Identifier: MIT

package ratelimit

import (
	"testing"
	"time"

	"github.com/umesh0492/go-libs/clock"
)

func TestDeterministic_TokenBucket_AllBranches(t *testing.T) {
	fc := clock.NewFake()
	tb := NewTokenBucket(5, time.Second, WithClock(fc))

	// 1. AllowN <= 0 returns true
	if !tb.AllowN("k1", 0) {
		t.Fatal("AllowN(0) must return true")
	}

	// 2. Consume all tokens
	if !tb.AllowN("k1", 5) {
		t.Fatal("expected 5 tokens allowed")
	}

	// 3. Excess request fails
	if tb.AllowN("k1", 1) {
		t.Fatal("expected request rejected when out of tokens")
	}

	// 4. Advance 1s -> refill 1 token (< capacity 5) -> triggers 'else' branch
	fc.Add(1 * time.Second)
	if !tb.AllowN("k1", 1) {
		t.Fatal("expected 1 token allowed after 1s")
	}

	// 5. Advance 10s -> refill 10 tokens (>= capacity 5) -> triggers 'if tokens >= capacity'
	fc.Add(10 * time.Second)
	if !tb.AllowN("k1", 1) {
		t.Fatal("expected token allowed after capping to capacity")
	}

	// 6. Remaining() with tokens + refill > capacity
	fc.Add(10 * time.Second)
	rem := tb.Remaining("k1")
	if rem != 5 {
		t.Fatalf("expected remaining capped at 5, got %d", rem)
	}

	// 7. Test maybeGC with both idle and non-idle buckets
	bOld := tb.getBucket("old-key")
	bOld.mu.Lock()
	bOld.last = fc.Now().Add(-10 * time.Hour)
	bOld.mu.Unlock()

	fc.Add(6 * time.Minute)
	// Refresh fresh-key so its last is current
	tb.Allow("fresh-key")

	tb.maybeGC()

	tb.mu.RLock()
	_, oldExists := tb.buckets["old-key"]
	_, freshExists := tb.buckets["fresh-key"]
	tb.mu.RUnlock()

	if oldExists {
		t.Fatal("expected old-key deleted by GC")
	}
	if !freshExists {
		t.Fatal("expected fresh-key retained by GC")
	}

	// 8. Deterministic double-checked locking in getBucket
	tb.preLockHook = func() {
		tb.mu.Lock()
		tb.buckets["det-double-check"] = &tbBucket{tokens: 5, last: fc.Now()}
		tb.mu.Unlock()
		tb.preLockHook = nil
	}
	bDet := tb.getBucket("det-double-check")
	if bDet == nil {
		t.Fatal("expected non-nil bucket from double-checked lock")
	}
}

func TestDeterministic_SlidingWindow_AllBranches(t *testing.T) {
	fc := clock.NewFake()
	sw := NewSlidingWindow(5, 10*time.Second, WithClock(fc))

	// 1. AllowN <= 0 returns true
	if !sw.AllowN("k2", 0) {
		t.Fatal("AllowN(0) must return true")
	}

	// 2. Consume all limit
	if !sw.AllowN("k2", 5) {
		t.Fatal("expected 5 allowed")
	}

	// 3. Excess request fails
	if sw.AllowN("k2", 1) {
		t.Fatal("expected rejected when limit reached")
	}

	// 4. Same window advance (Equal currWindowStart)
	e := sw.getEntry("k2")
	e.advance(fc.Now(), 10*time.Second)

	// 5. Next window rollover: currentWindow.Equal(currWindowStart.Add(window))
	fc.Add(12 * time.Second)
	if !sw.Allow("k2") {
		t.Fatal("expected allowed in next window")
	}

	// 6. Multiple windows gap: currentWindow.After(currWindowStart.Add(window))
	fc.Add(35 * time.Second)
	if !sw.Allow("k2") {
		t.Fatal("expected allowed after multiple windows")
	}

	// 7. Remaining() in sliding window
	rem := sw.Remaining("k2")
	if rem < 0 || rem > 5 {
		t.Fatalf("invalid remaining: %d", rem)
	}

	// 8. Test maybeGC with both idle and non-idle entries
	eOld := sw.getEntry("old-sw")
	eOld.mu.Lock()
	eOld.currWindowStart = fc.Now().Add(-10 * time.Hour)
	eOld.mu.Unlock()

	fc.Add(6 * time.Minute)
	// Refresh fresh-sw so its window is current
	sw.Allow("fresh-sw")

	sw.maybeGC()

	sw.mu.RLock()
	_, oldExists := sw.windows["old-sw"]
	_, freshExists := sw.windows["fresh-sw"]
	sw.mu.RUnlock()

	if oldExists {
		t.Fatal("expected old-sw deleted by GC")
	}
	if !freshExists {
		t.Fatal("expected fresh-sw retained by GC")
	}

	// 9. Deterministic double-checked locking in getEntry
	sw.preLockHook = func() {
		sw.mu.Lock()
		sw.windows["det-sw-double-check"] = &swEntry{currWindowStart: fc.Now(), currCount: 0, prevCount: 0}
		sw.mu.Unlock()
		sw.preLockHook = nil
	}
	eDet := sw.getEntry("det-sw-double-check")
	if eDet == nil {
		t.Fatal("expected non-nil entry from double-checked lock")
	}
}
