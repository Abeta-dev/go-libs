// SPDX-License-Identifier: MIT

// Package circuitbreaker provides an outbound failure state machine for microservice resilience.
package circuitbreaker

import (
	"context"
	"errors"

	"github.com/umesh0492/go-libs/clock"
	"github.com/umesh0492/go-libs/metrics"
)

// ErrCircuitOpen is returned when an execution request is rejected because the circuit is open.
var ErrCircuitOpen = errors.New("circuit breaker is open")

// Breaker is the common interface implemented by all circuit breaker algorithms.
type Breaker interface {
	Execute(ctx context.Context, req func() error) error
	State() State
}

// State represents the current operational status of the circuit breaker.
type State int

// Circuit breaker lifecycle states.
const (
	StateClosed State = iota
	StateOpen
	StateHalfOpen
)

// String returns the uppercase name of the circuit state.
func (s State) String() string {
	switch s {
	case StateClosed:
		return "CLOSED"
	case StateOpen:
		return "OPEN"
	case StateHalfOpen:
		return "HALF_OPEN"
	default:
		return "UNKNOWN"
	}
}

// Metrics holds optional metrics instrumentation collectors for the circuit breaker.
type Metrics struct {
	Requests metrics.Counter
	Failures metrics.Counter
	State    metrics.Gauge
}

type options struct {
	onStateChange func(from, to State)
	metrics       Metrics
	maxHalfOpen   int
	clock         clock.Clock
}

// Option configures a circuit breaker.
type Option func(*options)

// WithClock injects an abstract time source into the circuit breaker.
// If nil or not provided, clock.NewReal() is used.
func WithClock(c clock.Clock) Option {
	return func(o *options) {
		if c != nil {
			o.clock = c
		}
	}
}

// WithOnStateChange registers a callback invoked whenever the circuit breaker transitions between states.
func WithOnStateChange(fn func(from, to State)) Option {
	return func(o *options) {
		o.onStateChange = fn
	}
}

// WithMetrics attaches metrics collectors to the circuit breaker.
func WithMetrics(m Metrics) Option {
	return func(o *options) {
		o.metrics = m
	}
}

// WithHalfOpenProbes configures the maximum number of concurrent probe requests admitted in StateHalfOpen (default 1).
func WithHalfOpenProbes(n int) Option {
	return func(o *options) {
		if n > 0 {
			o.maxHalfOpen = n
		}
	}
}
