// SPDX-License-Identifier: MIT

package clock

import "time"

// RealClock implements Clock by directly wrapping standard library time functions.
type RealClock struct{}

// NewReal returns a Clock implementation that uses the standard library time package.
func NewReal() Clock {
	return &RealClock{}
}

// Now returns the current local time via time.Now().
func (r *RealClock) Now() time.Time {
	return time.Now()
}

// Since returns the time elapsed since t via time.Since(t).
func (r *RealClock) Since(t time.Time) time.Duration {
	return time.Since(t)
}

// Until returns the duration until t via time.Until(t).
func (r *RealClock) Until(t time.Time) time.Duration {
	return time.Until(t)
}

// Sleep pauses the current goroutine for at least duration d via time.Sleep(d).
func (r *RealClock) Sleep(d time.Duration) {
	time.Sleep(d)
}

// After waits for duration d to elapse and sends the time on the returned channel via time.After(d).
func (r *RealClock) After(d time.Duration) <-chan time.Time {
	return time.After(d)
}

// NewTimer creates a new Timer wrapping time.NewTimer(d).
func (r *RealClock) NewTimer(d time.Duration) Timer {
	return &realTimer{t: time.NewTimer(d)}
}

// NewTicker returns a new Ticker wrapping time.NewTicker(d).
func (r *RealClock) NewTicker(d time.Duration) Ticker {
	return &realTicker{t: time.NewTicker(d)}
}

type realTimer struct {
	t *time.Timer
}

func (rt *realTimer) C() <-chan time.Time {
	return rt.t.C
}

func (rt *realTimer) Reset(d time.Duration) bool {
	return rt.t.Reset(d)
}

func (rt *realTimer) Stop() bool {
	return rt.t.Stop()
}

type realTicker struct {
	t *time.Ticker
}

func (rt *realTicker) C() <-chan time.Time {
	return rt.t.C
}

func (rt *realTicker) Reset(d time.Duration) {
	rt.t.Reset(d)
}

func (rt *realTicker) Stop() {
	rt.t.Stop()
}
