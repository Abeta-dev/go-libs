// SPDX-License-Identifier: MIT
package clock_test

import (
	"sync"
	"testing"
	"time"

	"github.com/umesh0492/go-libs/clock"
)

func TestRealClock(t *testing.T) {
	c := clock.NewReal()

	t1 := c.Now()
	if t1.IsZero() {
		t.Fatal("expected non-zero time")
	}

	time.Sleep(5 * time.Millisecond)
	if c.Since(t1) <= 0 {
		t.Fatal("expected positive Since")
	}

	future := t1.Add(time.Hour)
	if c.Until(future) <= 0 {
		t.Fatal("expected positive Until")
	}

	c.Sleep(5 * time.Millisecond)

	afterCh := c.After(5 * time.Millisecond)
	select {
	case <-afterCh:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("After timed out")
	}

	timer := c.NewTimer(5 * time.Millisecond)
	select {
	case <-timer.C():
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Timer timed out")
	}

	timer2 := c.NewTimer(time.Hour)
	if !timer2.Stop() {
		t.Fatal("expected timer stop to succeed")
	}
	if timer2.Stop() {
		t.Fatal("expected second timer stop to return false")
	}

	timer3 := c.NewTimer(time.Hour)
	if !timer3.Reset(5 * time.Millisecond) {
		t.Fatal("expected timer reset to return true for active timer")
	}
	select {
	case <-timer3.C():
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Reset timer timed out")
	}

	ticker := c.NewTicker(5 * time.Millisecond)
	select {
	case <-ticker.C():
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Ticker timed out")
	}
	ticker.Reset(10 * time.Millisecond)
	ticker.Stop()
}

func TestFakeClock_Basic(t *testing.T) {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	fc := clock.NewFakeAt(start)

	if !fc.Now().Equal(start) {
		t.Fatalf("expected %v, got %v", start, fc.Now())
	}

	if fc.Since(start) != 0 {
		t.Fatalf("expected 0 duration, got %v", fc.Since(start))
	}

	if fc.Until(start.Add(time.Minute)) != time.Minute {
		t.Fatalf("expected 1m, got %v", fc.Until(start.Add(time.Minute)))
	}

	// Advance
	fc.Add(10 * time.Second)
	if fc.Since(start) != 10*time.Second {
		t.Fatalf("expected 10s elapsed, got %v", fc.Since(start))
	}

	// Set
	next := start.Add(time.Hour)
	fc.Set(next)
	if !fc.Now().Equal(next) {
		t.Fatalf("expected %v, got %v", next, fc.Now())
	}

	// Default NewFake
	defFc := clock.NewFake()
	if defFc.Now().Year() != 2026 {
		t.Fatalf("expected year 2026 default, got %v", defFc.Now())
	}
}

func TestFakeClock_Timer(t *testing.T) {
	fc := clock.NewFake()

	// Zero/negative duration timer
	immTimer := fc.NewTimer(0)
	select {
	case <-immTimer.C():
	default:
		t.Fatal("expected immediate timer to have fired")
	}

	// Future timer
	timer := fc.NewTimer(5 * time.Second)
	select {
	case <-timer.C():
		t.Fatal("timer should not have fired yet")
	default:
	}

	fc.Add(4 * time.Second)
	select {
	case <-timer.C():
		t.Fatal("timer should not have fired after 4s")
	default:
	}

	fc.Add(1 * time.Second)
	select {
	case tm := <-timer.C():
		if tm.Before(fc.Now()) && !tm.Equal(fc.Now()) {
			t.Fatalf("unexpected timer stamp %v", tm)
		}
	default:
		t.Fatal("timer should have fired after 5s total")
	}

	// Timer Stop
	t2 := fc.NewTimer(10 * time.Second)
	if !t2.Stop() {
		t.Fatal("expected Stop() to return true on active timer")
	}
	if t2.Stop() {
		t.Fatal("expected second Stop() to return false")
	}
	fc.Add(20 * time.Second)
	select {
	case <-t2.C():
		t.Fatal("stopped timer should not fire")
	default:
	}

	// Timer Reset
	t3 := fc.NewTimer(10 * time.Second)
	if !t3.Reset(2 * time.Second) {
		t.Fatal("expected Reset() to return true for active timer")
	}
	fc.Add(1 * time.Second)
	select {
	case <-t3.C():
		t.Fatal("reset timer should not have fired after 1s")
	default:
	}
	fc.Add(1 * time.Second)
	select {
	case <-t3.C():
	default:
		t.Fatal("reset timer should have fired after 2s")
	}

	// Reset inactive timer
	if t3.Reset(5 * time.Second) {
		t.Fatal("expected Reset() to return false for inactive timer")
	}
	// Reset to 0
	t4 := fc.NewTimer(10 * time.Second)
	t4.Reset(0)
	select {
	case <-t4.C():
	default:
		t.Fatal("expected zero duration reset to fire immediately")
	}
}

func TestFakeClock_Ticker(t *testing.T) {
	fc := clock.NewFake()

	ticker := fc.NewTicker(2 * time.Second)
	fc.Add(1 * time.Second)
	select {
	case <-ticker.C():
		t.Fatal("ticker should not have ticked yet")
	default:
	}

	fc.Add(1 * time.Second)
	select {
	case <-ticker.C():
	default:
		t.Fatal("ticker should have ticked at 2s")
	}

	fc.Add(4 * time.Second)
	// Should have at least one tick buffered
	select {
	case <-ticker.C():
	default:
		t.Fatal("ticker should have ticked again")
	}

	ticker.Reset(10 * time.Second)
	fc.Add(5 * time.Second)
	select {
	case <-ticker.C():
		t.Fatal("ticker should not tick after 5s on 10s reset")
	default:
	}

	ticker.Stop()
	fc.Add(20 * time.Second)
	select {
	case <-ticker.C():
		t.Fatal("stopped ticker should not tick")
	default:
	}
}

func TestFakeClock_SleepAndBlockUntil(t *testing.T) {
	fc := clock.NewFake()

	// Sleep 0
	fc.Sleep(0)

	var wg sync.WaitGroup
	wg.Add(1)

	slept := false
	go func() {
		defer wg.Done()
		fc.Sleep(5 * time.Second)
		slept = true
	}()

	fc.BlockUntil(1)
	if slept {
		t.Fatal("goroutine should still be sleeping")
	}

	fc.Add(5 * time.Second)
	wg.Wait()

	if !slept {
		t.Fatal("goroutine should have finished sleeping")
	}
}

func TestFakeClock_BlockUntilWaitsForRegistration(t *testing.T) {
	fc := clock.NewFake()
	started := make(chan struct{})
	registered := make(chan struct{})

	go func() {
		close(started)
		fc.BlockUntil(1)
		close(registered)
	}()

	<-started
	select {
	case <-registered:
		t.Fatal("BlockUntil returned before a waiter registered")
	default:
	}

	timer := fc.NewTimer(time.Hour)
	defer timer.Stop()

	select {
	case <-registered:
	case <-time.After(time.Second):
		t.Fatal("BlockUntil did not return after a waiter registered")
	}
}

func TestFakeClock_After(t *testing.T) {
	fc := clock.NewFake()
	ch := fc.After(3 * time.Second)

	fc.Add(2 * time.Second)
	select {
	case <-ch:
		t.Fatal("After channel should not have fired yet")
	default:
	}

	fc.Add(1 * time.Second)
	select {
	case <-ch:
	default:
		t.Fatal("After channel should have fired")
	}
}

func TestFakeClock_Concurrent(t *testing.T) {
	fc := clock.NewFake()
	var wg sync.WaitGroup

	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				_ = fc.Now()
				_ = fc.Since(time.Now())
				t := fc.NewTimer(time.Millisecond)
				t.Stop()
			}
		}()
	}

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				fc.Add(time.Millisecond)
			}
		}()
	}

	wg.Wait()
}
