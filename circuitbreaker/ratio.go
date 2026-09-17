// SPDX-License-Identifier: MIT

package circuitbreaker

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/umesh0492/go-libs/clock"
	"github.com/umesh0492/go-libs/metrics"
)

type reqRecord struct {
	ts     time.Time
	failed bool
}

// RatioBreaker implements an error-rate based circuit breaker following
// Hystrix and Resilience4j enterprise standards. It trips when the percentage
// of failed requests exceeds failureRatio within a rolling time window.
type RatioBreaker struct {
	mu               sync.RWMutex
	state            State
	failureRatio     float64
	minRequests      int
	window           time.Duration
	resetTimeout     time.Duration
	openedAt         time.Time
	halfOpenInFlight int
	maxHalfOpen      int
	records          []reqRecord
	failureCount     int
	onStateChange    func(from, to State)
	metrics          Metrics
	clock            clock.Clock
}

// NewRatioBreaker creates a circuit breaker that trips based on failure percentage.
//   - failureRatio: threshold fraction (e.g. 0.5 for 50% failure rate)
//   - minRequests: minimum number of requests in window before ratio evaluation begins
//   - window: rolling time window to calculate failure percentage
//   - resetTimeout: duration to remain in StateOpen before probing in StateHalfOpen
func NewRatioBreaker(failureRatio float64, minRequests int, window, resetTimeout time.Duration, opts ...Option) *RatioBreaker {
	if failureRatio <= 0 || failureRatio > 1.0 {
		failureRatio = 0.5
	}
	if minRequests <= 0 {
		minRequests = 10
	}
	if window <= 0 {
		window = 10 * time.Second
	}
	if resetTimeout <= 0 {
		resetTimeout = 30 * time.Second
	}

	var o options
	for _, opt := range opts {
		opt(&o)
	}

	maxHalfOpen := o.maxHalfOpen
	if maxHalfOpen <= 0 {
		maxHalfOpen = 1
	}

	clk := o.clock
	if clk == nil {
		clk = clock.NewReal()
	}

	rb := &RatioBreaker{
		state:         StateClosed,
		failureRatio:  failureRatio,
		minRequests:   minRequests,
		window:        window,
		resetTimeout:  resetTimeout,
		maxHalfOpen:   maxHalfOpen,
		clock:         clk,
		records:       make([]reqRecord, 0, minRequests*2),
		onStateChange: o.onStateChange,
		metrics: Metrics{
			Requests: metrics.NoopCounter{},
			Failures: metrics.NoopCounter{},
			State:    metrics.NoopGauge{},
		},
	}

	if o.metrics.Requests != nil {
		rb.metrics.Requests = o.metrics.Requests
	}
	if o.metrics.Failures != nil {
		rb.metrics.Failures = o.metrics.Failures
	}
	if o.metrics.State != nil {
		rb.metrics.State = o.metrics.State
	}

	rb.metrics.State.Set(float64(StateClosed))
	return rb
}

func (rb *RatioBreaker) initDefaults() {
	if rb.clock == nil {
		rb.clock = clock.NewReal()
	}
	if rb.failureRatio <= 0 || rb.failureRatio > 1.0 {
		rb.failureRatio = 0.5
	}
	if rb.minRequests <= 0 {
		rb.minRequests = 10
	}
	if rb.window <= 0 {
		rb.window = 10 * time.Second
	}
	if rb.resetTimeout <= 0 {
		rb.resetTimeout = 30 * time.Second
	}
	if rb.maxHalfOpen <= 0 {
		rb.maxHalfOpen = 1
	}
	if rb.metrics.Requests == nil {
		rb.metrics.Requests = metrics.NoopCounter{}
	}
	if rb.metrics.Failures == nil {
		rb.metrics.Failures = metrics.NoopCounter{}
	}
	if rb.metrics.State == nil {
		rb.metrics.State = metrics.NoopGauge{}
	}
}

func (rb *RatioBreaker) transition(to State) func() {
	if rb.state == to {
		return nil
	}
	from := rb.state
	rb.state = to
	rb.metrics.State.Set(float64(to))
	if rb.onStateChange != nil {
		fn := rb.onStateChange
		return func() { fn(from, to) }
	}
	return nil
}

// Execute runs the function if permitted by the current circuit breaker state.
// It checks ctx.Err() prior to admission and propagates context errors during execution
// without incrementing the failure counter on pre-admission cancellation.
func (rb *RatioBreaker) Execute(ctx context.Context, req func() error) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	allowed, isProbe := rb.canExecute()
	if !allowed {
		return ErrCircuitOpen
	}

	rb.metrics.Requests.Inc()

	// Guard against panics inside req() so half-open probes or rolling metrics
	// are never left in a poisoned or frozen state.
	defer func() {
		if r := recover(); r != nil {
			rb.onPanic(isProbe)
			panic(r)
		}
	}()

	err := req()
	if err == nil && ctx.Err() != nil {
		err = ctx.Err()
	}

	rb.mu.Lock()
	notify := rb.recordResult(isProbe, err, rb.clock.Now())
	rb.mu.Unlock()

	if notify != nil {
		notify()
	}

	return err
}

func (rb *RatioBreaker) onPanic(isProbe bool) {
	var notify func()
	rb.mu.Lock()
	now := rb.clock.Now()
	if isProbe {
		rb.decrementHalfOpen()
		rb.openedAt = now
		rb.metrics.Failures.Inc()
		notify = rb.transition(StateOpen)
	} else if rb.state == StateClosed {
		rb.records = append(rb.records, reqRecord{
			ts:     now,
			failed: true,
		})
		rb.failureCount++
		rb.metrics.Failures.Inc()
		notify = rb.evaluateRatio(now)
	}
	rb.mu.Unlock()

	if notify != nil {
		notify()
	}
}

func (rb *RatioBreaker) decrementHalfOpen() {
	rb.halfOpenInFlight--
	if rb.halfOpenInFlight < 0 {
		rb.halfOpenInFlight = 0
	}
}

func (rb *RatioBreaker) recordResult(isProbe bool, err error, now time.Time) func() {
	if isProbe {
		rb.decrementHalfOpen()
		if err != nil {
			if errors.Is(err, context.Canceled) {
				return nil
			}
			rb.openedAt = now
			rb.metrics.Failures.Inc()
			return rb.transition(StateOpen)
		}
		rb.records = rb.records[:0]
		rb.failureCount = 0
		return rb.transition(StateClosed)
	}

	if rb.state == StateClosed {
		if errors.Is(err, context.Canceled) {
			return nil
		}
		failed := err != nil
		rb.records = append(rb.records, reqRecord{
			ts:     now,
			failed: failed,
		})
		if failed {
			rb.failureCount++
			rb.metrics.Failures.Inc()
		}
		return rb.evaluateRatio(now)
	}
	return nil
}

func (rb *RatioBreaker) evaluateRatio(now time.Time) func() {
	// Prune and compact records older than window
	cutoff := now.Add(-rb.window)
	validIdx := 0
	for validIdx < len(rb.records) && rb.records[validIdx].ts.Before(cutoff) {
		if rb.records[validIdx].failed {
			rb.failureCount--
		}
		validIdx++
	}
	if validIdx > 0 {
		copy(rb.records, rb.records[validIdx:])
		rb.records = rb.records[:len(rb.records)-validIdx]
	}

	total := len(rb.records)
	if total >= rb.minRequests {
		ratio := float64(rb.failureCount) / float64(total)
		if ratio >= rb.failureRatio {
			rb.openedAt = now
			return rb.transition(StateOpen)
		}
	}
	return nil
}

func (rb *RatioBreaker) canExecute() (allowed, isProbe bool) {
	rb.mu.Lock()
	rb.initDefaults()
	var notify func()
	now := rb.clock.Now()

	switch rb.state {
	case StateClosed:
		rb.mu.Unlock()
		return true, false

	case StateOpen:
		if now.Sub(rb.openedAt) > rb.resetTimeout {
			notify = rb.transition(StateHalfOpen)
			rb.halfOpenInFlight = 1
			rb.mu.Unlock()
			if notify != nil {
				notify()
			}
			return true, true
		}
		rb.mu.Unlock()
		return false, false

	case StateHalfOpen:
		// Herd stampede prevention: allow bounded canary probes (default 1)
		if rb.halfOpenInFlight < rb.maxHalfOpen {
			rb.halfOpenInFlight++
			rb.mu.Unlock()
			return true, true
		}
		rb.mu.Unlock()
		return false, false

	default:
		rb.mu.Unlock()
		return false, false
	}
}

// State returns the current operational status of the RatioBreaker under read lock.
// It reports StateHalfOpen if the reset timeout has elapsed, without mutating
// internal state or firing callbacks as a side effect of a read.
func (rb *RatioBreaker) State() State {
	rb.mu.RLock()
	if rb.clock == nil {
		rb.mu.RUnlock()
		rb.mu.Lock()
		rb.initDefaults()
		rb.mu.Unlock()
		rb.mu.RLock()
	}
	defer rb.mu.RUnlock()
	if rb.state == StateOpen && rb.clock.Since(rb.openedAt) > rb.resetTimeout {
		return StateHalfOpen
	}
	return rb.state
}

// Failures returns the current number of recorded failures in the active window under read lock.
func (rb *RatioBreaker) Failures() int {
	rb.mu.RLock()
	defer rb.mu.RUnlock()
	return rb.failureCount
}
