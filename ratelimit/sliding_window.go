// SPDX-License-Identifier: MIT

package ratelimit

import (
	"sync"
	"time"

	"github.com/umesh0492/go-libs/clock"
)

// SlidingWindowLimiter implements the Sliding Window Counter algorithm (Cloudflare approximation).
// It weights the previous window's counter by the remaining fraction of the window,
// eliminating the boundary burst vulnerability of fixed windows while maintaining O(1) memory.
type SlidingWindowLimiter struct {
	mu          sync.RWMutex
	windows     map[string]*swEntry
	limit       int
	window      time.Duration
	lastGC      time.Time
	gcInterval  time.Duration
	idleTimeout time.Duration
	clock       clock.Clock
	preLockHook func()
}

type swEntry struct {
	mu              sync.Mutex
	currWindowStart time.Time
	currCount       int
	prevCount       int
}

// NewSlidingWindow creates a rate limiter using the Sliding Window Counter algorithm.
// - limit: maximum number of requests allowed within any sliding window
// - window: duration of the sliding window
func NewSlidingWindow(limit int, window time.Duration, opts ...Option) *SlidingWindowLimiter {
	if limit <= 0 {
		limit = 1
	}
	if window <= 0 {
		window = time.Minute
	}

	var cfg options
	for _, opt := range opts {
		opt(&cfg)
	}

	clk := cfg.clock
	if clk == nil {
		clk = clock.NewReal()
	}

	return &SlidingWindowLimiter{
		windows:     make(map[string]*swEntry),
		limit:       limit,
		window:      window,
		clock:       clk,
		lastGC:      clk.Now(),
		gcInterval:  5 * time.Minute,
		idleTimeout: 5 * time.Minute,
	}
}

func (l *SlidingWindowLimiter) initDefaults() {
	if l.windows == nil {
		l.windows = make(map[string]*swEntry)
	}
	if l.limit <= 0 {
		l.limit = 1
	}
	if l.window <= 0 {
		l.window = time.Minute
	}
	if l.clock == nil {
		l.clock = clock.NewReal()
	}
	if l.gcInterval <= 0 {
		l.gcInterval = 5 * time.Minute
	}
	if l.idleTimeout <= 0 {
		l.idleTimeout = 5 * time.Minute
	}
	if l.lastGC.IsZero() {
		l.lastGC = l.clock.Now()
	}
}

func (l *SlidingWindowLimiter) getEntry(key string) *swEntry {
	l.mu.RLock()
	if l.windows != nil {
		e, ok := l.windows[key]
		l.mu.RUnlock()
		if ok {
			return e
		}
	} else {
		l.mu.RUnlock()
	}

	if l.preLockHook != nil {
		l.preLockHook()
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	l.initDefaults()
	l.maybeGC()

	if e, ok := l.windows[key]; ok {
		return e
	}

	e := &swEntry{
		currWindowStart: l.clock.Now().Truncate(l.window),
		currCount:       0,
		prevCount:       0,
	}
	l.windows[key] = e
	return e
}

func (l *SlidingWindowLimiter) maybeGC() {
	if l.clock.Since(l.lastGC) < l.gcInterval {
		return
	}
	cutoff := l.clock.Now().Add(-l.idleTimeout)
	for k, e := range l.windows {
		e.mu.Lock()
		idle := e.currWindowStart.Before(cutoff)
		e.mu.Unlock()
		if idle {
			delete(l.windows, k)
		}
	}
	l.lastGC = l.clock.Now()
}

func (e *swEntry) advance(now time.Time, window time.Duration) {
	currentWindow := now.Truncate(window)
	if currentWindow.Equal(e.currWindowStart) {
		return
	}
	if currentWindow.Equal(e.currWindowStart.Add(window)) {
		e.prevCount = e.currCount
		e.currCount = 0
		e.currWindowStart = currentWindow
	} else if currentWindow.After(e.currWindowStart.Add(window)) {
		e.prevCount = 0
		e.currCount = 0
		e.currWindowStart = currentWindow
	}
}

func (e *swEntry) estimatedCount(now time.Time, window time.Duration) float64 {
	e.advance(now, window)
	timeInCurrentWindow := max(0, min(window, now.Sub(e.currWindowStart)))
	prevWeight := float64(window-timeInCurrentWindow) / float64(window)
	return float64(e.prevCount)*prevWeight + float64(e.currCount)
}

// Allow reports whether 1 event may occur now.
func (l *SlidingWindowLimiter) Allow(key string) bool {
	return l.AllowN(key, 1)
}

// AllowN reports whether n events may occur now.
func (l *SlidingWindowLimiter) AllowN(key string, n int) bool {
	if n <= 0 {
		return true
	}
	e := l.getEntry(key)
	e.mu.Lock()
	defer e.mu.Unlock()

	now := l.clock.Now()
	est := e.estimatedCount(now, l.window)

	if int(est)+n > l.limit {
		return false
	}

	e.currCount += n
	return true
}

// Remaining returns the estimated number of requests remaining in the current sliding window.
func (l *SlidingWindowLimiter) Remaining(key string) int {
	e := l.getEntry(key)
	e.mu.Lock()
	defer e.mu.Unlock()

	est := e.estimatedCount(l.clock.Now(), l.window)
	return max(0, l.limit-int(est))
}

// Reset clears the rate limit state for the specified key.
func (l *SlidingWindowLimiter) Reset(key string) {
	l.mu.Lock()
	if l.windows != nil {
		delete(l.windows, key)
	}
	l.mu.Unlock()
}
