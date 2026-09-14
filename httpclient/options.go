// SPDX-License-Identifier: MIT

package httpclient

import (
	"net/http"
	"time"

	"github.com/umesh0492/go-libs/circuitbreaker"
	"github.com/umesh0492/go-libs/clock"
	"github.com/umesh0492/go-libs/metrics"
	"github.com/umesh0492/go-libs/retry"
	"go.opentelemetry.io/otel/trace"
)

// RateLimiter defines the minimal rate limiting interface expected by httpclient.
type RateLimiter interface {
	Allow(key string) bool
}

// Metrics provides observability instrumentation hooks for the HTTP client.
type Metrics struct {
	Requests metrics.Counter
	Failures metrics.Counter
	Duration metrics.Histogram
}

type options struct {
	transport          http.RoundTripper
	circuitBreaker     circuitbreaker.Breaker
	retryCfg           retry.Config
	rateLimiter        RateLimiter
	rateLimiterKeyFunc func(*http.Request) string
	rateLimiterFunc    func(*http.Request) bool
	perAttemptTimeout  time.Duration
	totalTimeout       time.Duration
	metrics            Metrics
	tracer             trace.Tracer
	clock              clock.Clock
}

// Option configures the resilient HTTP client and RoundTripper.
type Option func(*options)

// WithTransport sets the underlying innermost transport. Defaults to http.DefaultTransport.
func WithTransport(rt http.RoundTripper) Option {
	return func(o *options) {
		if rt != nil {
			o.transport = rt
		}
	}
}

// WithCircuitBreaker wires a circuit breaker into the request pipeline.
func WithCircuitBreaker(cb circuitbreaker.Breaker) Option {
	return func(o *options) {
		o.circuitBreaker = cb
	}
}

// WithRetry configures automatic retry with backoff and jitter.
func WithRetry(cfg retry.Config) Option {
	return func(o *options) {
		o.retryCfg = cfg
	}
}

// WithRateLimiter wires a RateLimiter using the request host as the default key.
func WithRateLimiter(rl RateLimiter) Option {
	return func(o *options) {
		o.rateLimiter = rl
		o.rateLimiterKeyFunc = func(r *http.Request) string {
			return r.URL.Host
		}
	}
}

// WithRateLimiterKey wires a RateLimiter with a custom key extraction function.
func WithRateLimiterKey(rl RateLimiter, keyFunc func(*http.Request) string) Option {
	return func(o *options) {
		o.rateLimiter = rl
		o.rateLimiterKeyFunc = keyFunc
	}
}

// WithRateLimiterFunc configures a custom rate limiter boolean evaluation predicate.
func WithRateLimiterFunc(fn func(*http.Request) bool) Option {
	return func(o *options) {
		o.rateLimiterFunc = fn
	}
}

// WithPerAttemptTimeout sets a bounded timeout for each individual HTTP attempt.
func WithPerAttemptTimeout(d time.Duration) Option {
	return func(o *options) {
		o.perAttemptTimeout = d
	}
}

// WithTotalTimeout sets a bounded timeout covering all attempts and retry backoffs.
func WithTotalTimeout(d time.Duration) Option {
	return func(o *options) {
		o.totalTimeout = d
	}
}

// WithMetrics attaches RED instrumentation collectors to the client.
func WithMetrics(m Metrics) Option {
	return func(o *options) {
		o.metrics = m
	}
}

// WithTelemetry wires OpenTelemetry tracing to automatically record client spans and inject W3C trace headers.
func WithTelemetry(tracer trace.Tracer) Option {
	return func(o *options) {
		o.tracer = tracer
	}
}

// WithClock injects an abstract time source for deterministic testing.
func WithClock(c clock.Clock) Option {
	return func(o *options) {
		if c != nil {
			o.clock = c
		}
	}
}
