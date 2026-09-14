// SPDX-License-Identifier: MIT

package ratelimit

import (
	"sync"
	"time"

	"github.com/umesh0492/go-libs/clock"
)

// TokenBucketLimiter implements the Token Bucket algorithm.
// It permits bursts up to capacity and refills at a steady rate.
type TokenBucketLimiter struct {
	mu          sync.RWMutex
	buckets     map[string]*tbBucket
	capacity    int
	refillRate  time.Duration
	lastGC      time.Time
	gcInterval  time.Duration
	idleTimeout time.Duration
	clock       clock.Clock
	preLockHook func()
}

type tbBucket struct {
	mu     sync.Mutex
	tokens int
	last   time.Time
}

// NewTokenBucket creates a rate limiter using the Token Bucket algorithm.
// - capacity: the maximum burst size (max number of tokens in the bucket)
// - refillRate: duration to add 1 token to the bucket (e.g. 100ms for 10 req/s)
func NewTokenBucket(capacity int, refillRate time.Duration, opts ...Option) *TokenBucketLimiter {
	if capacity <= 0 {
		capacity = 1
	}
	if refillRate <= 0 {
		refillRate = time.Second
	}

	var cfg options
	for _, opt := range opts {
		opt(&cfg)
	}

	clk := cfg.clock
	if clk == nil {
		clk = clock.NewReal()
	}

	return &TokenBucketLimiter{
		buckets:     make(map[string]*tbBucket),
		capacity:    capacity,
		refillRate:  refillRate,
		clock:       clk,
		lastGC:      clk.Now(),
		gcInterval:  5 * time.Minute,
		idleTimeout: 5 * time.Minute,
	}
}

func (l *TokenBucketLimiter) initDefaults() {
	if l.buckets == nil {
		l.buckets = make(map[string]*tbBucket)
	}
	if l.capacity <= 0 {
		l.capacity = 1
	}
	if l.refillRate <= 0 {
		l.refillRate = time.Second
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

func (l *TokenBucketLimiter) getBucket(key string) *tbBucket {
	l.mu.RLock()
	if l.buckets != nil {
		b, ok := l.buckets[key]
		l.mu.RUnlock()
		if ok {
			return b
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

	if b, ok := l.buckets[key]; ok {
		return b
	}

	b := &tbBucket{
		tokens: l.capacity,
		last:   l.clock.Now(),
	}
	l.buckets[key] = b
	return b
}

func (l *TokenBucketLimiter) maybeGC() {
	if l.clock.Since(l.lastGC) < l.gcInterval {
		return
	}
	cutoff := l.clock.Now().Add(-l.idleTimeout)
	for k, b := range l.buckets {
		b.mu.Lock()
		idle := b.last.Before(cutoff)
		b.mu.Unlock()
		if idle {
			delete(l.buckets, k)
		}
	}
	l.lastGC = l.clock.Now()
}

// Allow reports whether 1 event may occur now.
func (l *TokenBucketLimiter) Allow(key string) bool {
	return l.AllowN(key, 1)
}

// AllowN reports whether n events may occur now.
func (l *TokenBucketLimiter) AllowN(key string, n int) bool {
	if n <= 0 {
		return true
	}
	b := l.getBucket(key)
	b.mu.Lock()
	defer b.mu.Unlock()

	now := l.clock.Now()
	elapsed := now.Sub(b.last)
	refill := int(elapsed / l.refillRate)
	if refill > 0 {
		b.tokens += refill
		if b.tokens >= l.capacity {
			b.tokens = l.capacity
			b.last = now
		} else {
			b.last = b.last.Add(time.Duration(refill) * l.refillRate)
		}
	}

	if b.tokens < n {
		return false
	}
	b.tokens -= n
	return true
}

// Remaining returns the estimated number of permitted requests remaining.
func (l *TokenBucketLimiter) Remaining(key string) int {
	b := l.getBucket(key)
	b.mu.Lock()
	defer b.mu.Unlock()

	now := l.clock.Now()
	elapsed := now.Sub(b.last)
	refill := int(elapsed / l.refillRate)
	tokens := b.tokens + refill
	if tokens > l.capacity {
		tokens = l.capacity
	}
	return tokens
}

// Reset clears the rate limit state for the specified key.
func (l *TokenBucketLimiter) Reset(key string) {
	l.mu.Lock()
	if l.buckets != nil {
		delete(l.buckets, key)
	}
	l.mu.Unlock()
}
