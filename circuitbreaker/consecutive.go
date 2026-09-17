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

// ConsecutiveBreaker implements a circuit breaker that trips after a configured number
// of consecutive failures.
type ConsecutiveBreaker struct {
	mu               sync.RWMutex
	state            State
	failures         int
	maxFailures      int
	resetTimeout     time.Duration
	openedAt         time.Time
	halfOpenInFlight int
	maxHalfOpen      int
	onStateChange    func(from, to State)
	metrics          Metrics
	clock            clock.Clock
}

// NewConsecutiveBreaker constructs a new ConsecutiveBreaker with the given failure threshold,
// reset timeout, and optional configuration.
func NewConsecutiveBreaker(maxFailures int, resetTimeout time.Duration, opts ...Option) *ConsecutiveBreaker {
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

	cb := &ConsecutiveBreaker{
		state:         StateClosed,
		maxFailures:   maxFailures,
		resetTimeout:  resetTimeout,
		maxHalfOpen:   maxHalfOpen,
		onStateChange: o.onStateChange,
		clock:         clk,
		metrics: Metrics{
			Requests: metrics.NoopCounter{},
			Failures: metrics.NoopCounter{},
			State:    metrics.NoopGauge{},
		},
	}
	if o.metrics.Requests != nil {
		cb.metrics.Requests = o.metrics.Requests
	}
	if o.metrics.Failures != nil {
		cb.metrics.Failures = o.metrics.Failures
	}
	if o.metrics.State != nil {
		cb.metrics.State = o.metrics.State
	}
	cb.metrics.State.Set(float64(StateClosed))
	return cb
}

func (cb *ConsecutiveBreaker) initDefaults() {
	if cb.clock == nil {
		cb.clock = clock.NewReal()
	}
	if cb.maxFailures <= 0 {
		cb.maxFailures = 5
	}
	if cb.resetTimeout <= 0 {
		cb.resetTimeout = 60 * time.Second
	}
	if cb.maxHalfOpen <= 0 {
		cb.maxHalfOpen = 1
	}
	if cb.metrics.Requests == nil {
		cb.metrics.Requests = metrics.NoopCounter{}
	}
	if cb.metrics.Failures == nil {
		cb.metrics.Failures = metrics.NoopCounter{}
	}
	if cb.metrics.State == nil {
		cb.metrics.State = metrics.NoopGauge{}
	}
}

func (cb *ConsecutiveBreaker) transition(to State) func() {
	if cb.state == to {
		return nil
	}
	from := cb.state
	cb.state = to
	cb.metrics.State.Set(float64(to))
	if cb.onStateChange != nil {
		fn := cb.onStateChange
		return func() { fn(from, to) }
	}
	return nil
}

// Execute runs the function if the circuit is closed or half-open.
// It checks ctx.Err() prior to admission and propagates context errors during execution
// without incrementing the failure counter on pre-admission cancellation.
func (cb *ConsecutiveBreaker) Execute(ctx context.Context, req func() error) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	allowed, isProbe := cb.canExecute()
	if !allowed {
		return ErrCircuitOpen
	}

	cb.metrics.Requests.Inc()

	// Guard against panics inside req() so half-open probes or closed counters
	// are never left in a poisoned or frozen state.
	defer func() {
		if r := recover(); r != nil {
			cb.onPanic(isProbe)
			panic(r)
		}
	}()

	err := req()
	if err == nil && ctx.Err() != nil {
		err = ctx.Err()
	}

	cb.mu.Lock()
	notify, recErr := cb.recordResult(isProbe, err)
	cb.mu.Unlock()

	if notify != nil {
		notify()
	}

	return recErr
}

func (cb *ConsecutiveBreaker) onPanic(isProbe bool) {
	cb.mu.Lock()
	var notify func()
	now := cb.clock.Now()
	if isProbe {
		cb.decrementHalfOpen()
		cb.openedAt = now
		cb.metrics.Failures.Inc()
		notify = cb.transition(StateOpen)
	} else if cb.state == StateClosed {
		cb.failures++
		cb.metrics.Failures.Inc()
		if cb.failures >= cb.maxFailures {
			cb.openedAt = now
			notify = cb.transition(StateOpen)
		}
	}
	cb.mu.Unlock()

	if notify != nil {
		notify()
	}
}

func (cb *ConsecutiveBreaker) recordResult(isProbe bool, err error) (func(), error) {
	now := cb.clock.Now()
	if isProbe {
		cb.decrementHalfOpen()
		if err != nil {
			if errors.Is(err, context.Canceled) {
				return nil, err
			}
			cb.openedAt = now
			cb.metrics.Failures.Inc()
			notify := cb.transition(StateOpen)
			return notify, err
		}
		cb.failures = 0
		notify := cb.transition(StateClosed)
		return notify, nil
	}

	if cb.state == StateClosed {
		if err != nil {
			if errors.Is(err, context.Canceled) {
				return nil, err
			}
			cb.failures++
			cb.metrics.Failures.Inc()
			if cb.failures >= cb.maxFailures {
				cb.openedAt = now
				notify := cb.transition(StateOpen)
				return notify, err
			}
			return nil, err
		}
		cb.failures = 0
	}
	return nil, err
}

func (cb *ConsecutiveBreaker) decrementHalfOpen() {
	cb.halfOpenInFlight--
	if cb.halfOpenInFlight < 0 {
		cb.halfOpenInFlight = 0
	}
}

func (cb *ConsecutiveBreaker) canExecute() (allowed, isProbe bool) {
	cb.mu.Lock()
	cb.initDefaults()
	var notify func()
	now := cb.clock.Now()

	switch cb.state {
	case StateClosed:
		cb.mu.Unlock()
		return true, false

	case StateOpen:
		if now.Sub(cb.openedAt) > cb.resetTimeout {
			notify = cb.transition(StateHalfOpen)
			cb.halfOpenInFlight = 1
			cb.mu.Unlock()
			if notify != nil {
				notify()
			}
			return true, true
		}
		cb.mu.Unlock()
		return false, false

	case StateHalfOpen:
		// Herd stampede prevention: allow bounded probes in flight (default 1)
		if cb.halfOpenInFlight < cb.maxHalfOpen {
			cb.halfOpenInFlight++
			cb.mu.Unlock()
			return true, true
		}
		cb.mu.Unlock()
		return false, false

	default:
		cb.mu.Unlock()
		return false, false
	}
}

// State returns the current operational status of the circuit breaker under read lock.
// It reports StateHalfOpen if the reset timeout has elapsed, without mutating
// internal state or firing callbacks as a side effect of a read.
func (cb *ConsecutiveBreaker) State() State {
	cb.mu.RLock()
	if cb.clock == nil {
		cb.mu.RUnlock()
		cb.mu.Lock()
		cb.initDefaults()
		cb.mu.Unlock()
		cb.mu.RLock()
	}
	defer cb.mu.RUnlock()
	if cb.state == StateOpen && cb.clock.Since(cb.openedAt) > cb.resetTimeout {
		return StateHalfOpen
	}
	return cb.state
}

// Failures returns the current number of recorded consecutive failures under read lock.
func (cb *ConsecutiveBreaker) Failures() int {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.failures
}
