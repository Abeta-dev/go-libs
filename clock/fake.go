// SPDX-License-Identifier: MIT

package clock

import (
	"sync"
	"time"
)

// FakeClock implements Clock with deterministic, user-controlled time.
// It allows tests to simulate time passage instantaneously without sleeping.
type FakeClock struct {
	mu           sync.Mutex
	cond         *sync.Cond
	now          time.Time
	timers       []*fakeTimer
	tickers      []*fakeTicker
	sleepWaiters []*sleepWaiter
}

// NewFake returns a FakeClock initialized to the UTC zero time.
func NewFake() *FakeClock {
	return NewFakeAt(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
}

// NewFakeAt returns a FakeClock initialized to the given start time.
func NewFakeAt(start time.Time) *FakeClock {
	fc := &FakeClock{
		now: start,
	}
	fc.cond = sync.NewCond(&fc.mu)
	return fc
}

// Now returns the simulated current time.
func (f *FakeClock) Now() time.Time {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.now
}

// Since returns the simulated time elapsed since t.
func (f *FakeClock) Since(t time.Time) time.Duration {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.now.Sub(t)
}

// Until returns the simulated duration until t.
func (f *FakeClock) Until(t time.Time) time.Duration {
	f.mu.Lock()
	defer f.mu.Unlock()
	return t.Sub(f.now)
}

// Set sets the simulated time to t and fires any expired timers, tickers, or sleeps.
func (f *FakeClock) Set(t time.Time) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.advanceLocked(t)
}

// Add advances simulated time by d and triggers all expired timers, tickers, and sleeps.
func (f *FakeClock) Add(d time.Duration) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.advanceLocked(f.now.Add(d))
}

func (f *FakeClock) advanceLocked(target time.Time) {
	f.now = target

	// Fire expired timers
	remainingTimers := f.timers[:0]
	for _, t := range f.timers {
		if t.stopped {
			continue
		}
		if !f.now.Before(t.deadline) {
			t.stopped = true
			select {
			case t.c <- t.deadline:
			default:
			}
		} else {
			remainingTimers = append(remainingTimers, t)
		}
	}
	f.timers = remainingTimers

	// Fire expired tickers
	for _, tick := range f.tickers {
		if tick.stopped {
			continue
		}
		for !tick.stopped && !f.now.Before(tick.deadline) {
			select {
			case tick.c <- tick.deadline:
			default:
			}
			if tick.period <= 0 {
				tick.stopped = true
				break
			}
			tick.deadline = tick.deadline.Add(tick.period)
		}
	}

	// Wake expired sleep waiters
	remainingSleepers := f.sleepWaiters[:0]
	for _, sw := range f.sleepWaiters {
		if sw.done {
			continue
		}
		if !f.now.Before(sw.deadline) {
			sw.done = true
			close(sw.ch)
		} else {
			remainingSleepers = append(remainingSleepers, sw)
		}
	}
	f.sleepWaiters = remainingSleepers

	f.cond.Broadcast()
}

type sleepWaiter struct {
	deadline time.Time
	ch       chan struct{}
	done     bool
}

// Sleep pauses the calling goroutine until simulated time advances by at least d.
func (f *FakeClock) Sleep(d time.Duration) {
	if d <= 0 {
		return
	}
	f.mu.Lock()
	sw := &sleepWaiter{
		deadline: f.now.Add(d),
		ch:       make(chan struct{}),
	}
	f.sleepWaiters = append(f.sleepWaiters, sw)
	f.cond.Broadcast()
	f.mu.Unlock()

	<-sw.ch
}

// After returns a channel that will receive the simulated time after duration d.
func (f *FakeClock) After(d time.Duration) <-chan time.Time {
	return f.NewTimer(d).C()
}

// NewTimer creates a new simulated Timer expiring after d.
func (f *FakeClock) NewTimer(d time.Duration) Timer {
	f.mu.Lock()
	defer f.mu.Unlock()

	ft := &fakeTimer{
		clk:      f,
		c:        make(chan time.Time, 1),
		deadline: f.now.Add(d),
	}

	if d <= 0 {
		ft.stopped = true
		ft.c <- f.now
		return ft
	}

	f.timers = append(f.timers, ft)
	f.cond.Broadcast()
	return ft
}

// NewTicker returns a new simulated Ticker with period d.
func (f *FakeClock) NewTicker(d time.Duration) Ticker {
	f.mu.Lock()
	defer f.mu.Unlock()

	tick := &fakeTicker{
		clk:      f,
		c:        make(chan time.Time, 1),
		period:   d,
		deadline: f.now.Add(d),
	}

	if d > 0 {
		f.tickers = append(f.tickers, tick)
		f.cond.Broadcast()
	}
	return tick
}

// BlockUntil waits until at least n waiters (timers, tickers, or sleep calls) are registered.
func (f *FakeClock) BlockUntil(n int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for f.activeWaitersLocked() < n {
		f.cond.Wait()
	}
}

func (f *FakeClock) activeWaitersLocked() int {
	count := 0
	for _, t := range f.timers {
		if !t.stopped {
			count++
		}
	}
	for _, tick := range f.tickers {
		if !tick.stopped {
			count++
		}
	}
	for _, sw := range f.sleepWaiters {
		if !sw.done {
			count++
		}
	}
	return count
}

type fakeTimer struct {
	clk      *FakeClock
	c        chan time.Time
	deadline time.Time
	stopped  bool
}

func (t *fakeTimer) C() <-chan time.Time {
	return t.c
}

func (t *fakeTimer) Reset(d time.Duration) bool {
	t.clk.mu.Lock()
	defer t.clk.mu.Unlock()

	active := !t.stopped
	t.stopped = false
	t.deadline = t.clk.now.Add(d)

	if d <= 0 {
		t.stopped = true
		select {
		case t.c <- t.clk.now:
		default:
		}
		return active
	}

	found := false
	for _, item := range t.clk.timers {
		if item == t {
			found = true
			break
		}
	}
	if !found {
		t.clk.timers = append(t.clk.timers, t)
	}
	t.clk.cond.Broadcast()
	return active
}

func (t *fakeTimer) Stop() bool {
	t.clk.mu.Lock()
	defer t.clk.mu.Unlock()

	if t.stopped {
		return false
	}
	t.stopped = true
	t.clk.cond.Broadcast()
	return true
}

type fakeTicker struct {
	clk      *FakeClock
	c        chan time.Time
	period   time.Duration
	deadline time.Time
	stopped  bool
}

func (t *fakeTicker) C() <-chan time.Time {
	return t.c
}

func (t *fakeTicker) Reset(d time.Duration) {
	t.clk.mu.Lock()
	defer t.clk.mu.Unlock()

	t.period = d
	t.deadline = t.clk.now.Add(d)
	t.stopped = false

	found := false
	for _, item := range t.clk.tickers {
		if item == t {
			found = true
			break
		}
	}
	if !found && d > 0 {
		t.clk.tickers = append(t.clk.tickers, t)
	}
	t.clk.cond.Broadcast()
}

func (t *fakeTicker) Stop() {
	t.clk.mu.Lock()
	defer t.clk.mu.Unlock()

	t.stopped = true
	t.clk.cond.Broadcast()
}
