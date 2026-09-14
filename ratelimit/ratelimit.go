// SPDX-License-Identifier: MIT

// Package ratelimit provides HTTP middleware for IP-based rate limiting using
// a token-bucket algorithm. It has zero external dependencies (stdlib only).
//
// Two pre-built constructors are provided for the most common web service patterns:
//   - NewGlobal — for API-wide protection (generous, flood prevention)
//   - NewAuth   — for login/sensitive endpoints (strict, brute-force prevention)
//
// Usage:
//
//	// Any chi/http router:
//	router.Use(ratelimit.NewGlobal())                      // 200/min per IP
//	router.With(ratelimit.NewAuth()).Post("/login", h)      // 10/min per IP
//
//	// Custom limits:
//	router.Use(ratelimit.New(500, time.Minute))
package ratelimit

import (
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/umesh0492/go-libs/clock"
	"github.com/umesh0492/go-libs/metrics"
)

// ── Metrics & Options ─────────────────────────────────────────────────────────

// Metrics holds optional metrics collectors for rate limiter observability.
type Metrics struct {
	Allowed metrics.Counter
	Blocked metrics.Counter
}

type options struct {
	metrics        Metrics
	trustedProxies []*net.IPNet
	clock          clock.Clock
}

// Option configures rate limiting middleware.
type Option func(*options)

// WithClock sets an abstract time source for the rate limiter.
// If nil or not provided, clock.NewReal() is used.
func WithClock(c clock.Clock) Option {
	return func(o *options) {
		if c != nil {
			o.clock = c
		}
	}
}

// WithMetrics attaches metrics instrumentation to the rate limiter.
func WithMetrics(m Metrics) Option {
	return func(o *options) {
		o.metrics = m
	}
}

// WithTrustedProxies configures trusted reverse proxy CIDRs/IPs for real IP extraction.
func WithTrustedProxies(cidrs ...string) Option {
	proxies := ParseTrustedProxies(cidrs...)
	return func(o *options) {
		o.trustedProxies = append(o.trustedProxies, proxies...)
	}
}

// WithTrustedIPNets configures trusted reverse proxy *net.IPNet ranges directly.
func WithTrustedIPNets(nets ...*net.IPNet) Option {
	return func(o *options) {
		o.trustedProxies = append(o.trustedProxies, nets...)
	}
}

// WithDefaultTrustedProxies configures standard private and loopback networks as trusted proxies.
func WithDefaultTrustedProxies() Option {
	return WithTrustedProxies(DefaultTrustedCIDRs...)
}

// ── Middleware constructor ─────────────────────────────────────────────────────

// Middleware is an http.Handler middleware function.
type Middleware = func(http.Handler) http.Handler

// New returns an IP-based rate-limiting middleware.
//   - capacity: maximum requests allowed in window
//   - window: rolling time window for the capacity
//
// Example: New(200, time.Minute) → 200 requests per minute per IP
func New(capacity int, window time.Duration, opts ...Option) Middleware {
	var cfg options
	for _, opt := range opts {
		opt(&cfg)
	}
	if cfg.metrics.Allowed == nil {
		cfg.metrics.Allowed = metrics.NoopCounter{}
	}
	if cfg.metrics.Blocked == nil {
		cfg.metrics.Blocked = metrics.NoopCounter{}
	}

	if cfg.clock == nil {
		cfg.clock = clock.NewReal()
	}

	if capacity <= 0 {
		capacity = 1
	}
	if window <= 0 {
		window = time.Minute
	}
	refillRate := window / time.Duration(capacity)
	if refillRate <= 0 {
		refillRate = time.Nanosecond
	}
	limiter := NewTokenBucket(capacity, refillRate, WithClock(cfg.clock))

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := RealIP(r, cfg.trustedProxies...)
			if !limiter.Allow(ip) {
				cfg.metrics.Blocked.Inc()
				w.Header().Set("RateLimit-Limit", strconv.Itoa(capacity))
				w.Header().Set("RateLimit-Remaining", "0")
				w.Header().Set("RateLimit-Reset", strconv.FormatInt(cfg.clock.Now().Add(window).Unix(), 10))
				w.Header().Set("Retry-After", "60")
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusTooManyRequests)
				_, _ = w.Write([]byte(`{"error":"too many requests","code":"RATE_LIMITED"}`))
				return
			}
			cfg.metrics.Allowed.Inc()
			w.Header().Set("RateLimit-Limit", strconv.Itoa(capacity))
			w.Header().Set("RateLimit-Remaining", strconv.Itoa(limiter.Remaining(ip)))
			next.ServeHTTP(w, r)
		})
	}
}

// NewGlobal returns a middleware with the standard global API limit: 200 req/min per IP.
// Apply as the second or third middleware on every route (after security headers).
func NewGlobal(opts ...Option) Middleware {
	return New(200, time.Minute, opts...)
}

// NewAuth returns a middleware with a strict login limit: 10 req/min per IP.
// Apply only on authentication endpoints to prevent brute-force attacks.
func NewAuth(opts ...Option) Middleware {
	return New(10, time.Minute, opts...)
}
