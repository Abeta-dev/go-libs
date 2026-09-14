// SPDX-License-Identifier: MIT

// Package metrics defines framework-agnostic instrumentation interfaces.
// It provides Counter, Gauge, and Histogram interfaces along with no-op
// implementations so every package in go-libs can be instrumented without
// requiring any concrete metrics backend.
//
// Usage — plug in your own Prometheus backend:
//
//	type promCounter struct{ c prometheus.Counter }
//	func (p *promCounter) Inc()          { p.c.Inc() }
//	func (p *promCounter) Add(v float64) { p.c.Add(v) }
//
//	cb := circuitbreaker.NewConsecutiveBreaker(5, 30*time.Second,
//	    circuitbreaker.WithMetrics(&myBreakerMetrics{...}),
//	)
package metrics

// Counter records a value that only increases (e.g. total requests).
type Counter interface {
	// Inc increments the counter by 1.
	Inc()
	// Add increments the counter by the given positive delta.
	Add(delta float64)
}

// Gauge records a current value that can go up or down (e.g. queue depth).
type Gauge interface {
	// Set replaces the current value.
	Set(value float64)
	// Add adjusts the current value by delta (may be negative).
	Add(delta float64)
}

// Histogram records a distribution of observed values (e.g. request duration).
type Histogram interface {
	// Observe records one observation of the given value.
	Observe(value float64)
}

// ── No-op implementations ──────────────────────────────────────────────────────

// NoopCounter is a Counter that discards all observations.
// It is the zero-value default for all packages.
type NoopCounter struct{}

// Inc is a no-op.
func (NoopCounter) Inc() {}

// Add is a no-op.
func (NoopCounter) Add(float64) {}

// NoopGauge is a Gauge that discards all observations.
type NoopGauge struct{}

// Set is a no-op.
func (NoopGauge) Set(float64) {}

// Add is a no-op.
func (NoopGauge) Add(float64) {}

// NoopHistogram is a Histogram that discards all observations.
type NoopHistogram struct{}

// Observe is a no-op.
func (NoopHistogram) Observe(float64) {}
