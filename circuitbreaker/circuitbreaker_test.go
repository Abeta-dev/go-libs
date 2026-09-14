// SPDX-License-Identifier: MIT

package circuitbreaker_test

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/umesh0492/go-libs/circuitbreaker"
)

func TestCircuitBreaker(t *testing.T) {
	cb := circuitbreaker.NewConsecutiveBreaker(2, 50*time.Millisecond)
	ctx := context.Background()
	reqErr := errors.New("failed")

	// 1. Success
	err := cb.Execute(ctx, func() error { return nil })
	assert.NoError(t, err)

	// 2. Failure 1
	err = cb.Execute(ctx, func() error { return reqErr })
	assert.ErrorIs(t, err, reqErr)

	// 3. Failure 2 (Should open circuit)
	err = cb.Execute(ctx, func() error { return reqErr })
	assert.ErrorIs(t, err, reqErr)

	// 4. Circuit Open - should return ErrCircuitOpen without calling func
	called := false
	err = cb.Execute(ctx, func() error { called = true; return nil })
	assert.ErrorIs(t, err, circuitbreaker.ErrCircuitOpen)
	assert.False(t, called)

	// 5. Wait for reset timeout
	time.Sleep(60 * time.Millisecond)

	// 6. Half-Open -> Success
	err = cb.Execute(ctx, func() error { return nil })
	assert.NoError(t, err)

	// 7. Should be fully closed now, next call succeeds normally
	err = cb.Execute(ctx, func() error { return nil })
	assert.NoError(t, err)
}

func TestCircuitBreaker_HalfOpenFailure(t *testing.T) {
	cb := circuitbreaker.NewConsecutiveBreaker(1, 10*time.Millisecond)
	ctx := context.Background()
	reqErr := errors.New("failed")

	// Trip it immediately
	_ = cb.Execute(ctx, func() error { return reqErr })

	// Wait
	time.Sleep(15 * time.Millisecond)

	// Fail the half-open test
	err := cb.Execute(ctx, func() error { return reqErr })
	assert.ErrorIs(t, err, reqErr)

	// Should be open again
	err = cb.Execute(ctx, func() error { return nil })
	assert.ErrorIs(t, err, circuitbreaker.ErrCircuitOpen)
}

func TestCircuitBreaker_HalfOpenRejectionWhileProbeInFlight(t *testing.T) {
	cb := circuitbreaker.NewConsecutiveBreaker(1, 10*time.Millisecond)
	ctx := context.Background()
	reqErr := errors.New("failed")

	_ = cb.Execute(ctx, func() error { return reqErr })
	time.Sleep(15 * time.Millisecond)

	// In half-open, while a probe is executing, subsequent requests must be rejected
	started := make(chan struct{})
	done := make(chan struct{})

	go func() {
		_ = cb.Execute(ctx, func() error {
			close(started)
			<-done
			return nil
		})
	}()

	<-started
	// The circuit is now in StateHalfOpen, and the probe hasn't finished yet.
	// A second Execute call must be rejected to prevent thundering herd.
	err := cb.Execute(ctx, func() error { return nil })
	assert.ErrorIs(t, err, circuitbreaker.ErrCircuitOpen)

	close(done)
	time.Sleep(5 * time.Millisecond)

	// After the probe completes successfully, circuit is CLOSED and requests succeed.
	err = cb.Execute(ctx, func() error { return nil })
	assert.NoError(t, err)
}

type mockCounter struct {
	count float64
}

func (m *mockCounter) Inc()          { m.count++ }
func (m *mockCounter) Add(d float64) { m.count += d }

type mockGauge struct {
	val float64
}

func (m *mockGauge) Set(v float64) { m.val = v }
func (m *mockGauge) Add(d float64) { m.val += d }

func TestCircuitBreaker_OptionsAndMetrics(t *testing.T) {
	reqCounter := &mockCounter{}
	failCounter := &mockCounter{}
	stateGauge := &mockGauge{}

	var transitions []struct{ from, to circuitbreaker.State }
	cb := circuitbreaker.NewConsecutiveBreaker(1, 10*time.Millisecond,
		circuitbreaker.WithMetrics(circuitbreaker.Metrics{
			Requests: reqCounter,
			Failures: failCounter,
			State:    stateGauge,
		}),
		circuitbreaker.WithOnStateChange(func(from, to circuitbreaker.State) {
			transitions = append(transitions, struct{ from, to circuitbreaker.State }{from, to})
		}),
	)

	ctx := context.Background()
	reqErr := errors.New("boom")

	// Initial gauge should be 0 (StateClosed)
	assert.Equal(t, float64(circuitbreaker.StateClosed), stateGauge.val)

	// Trip it
	_ = cb.Execute(ctx, func() error { return reqErr })
	assert.Equal(t, 1.0, reqCounter.count)
	assert.Equal(t, 1.0, failCounter.count)
	assert.Equal(t, float64(circuitbreaker.StateOpen), stateGauge.val)
	require.Len(t, transitions, 1)
	assert.Equal(t, circuitbreaker.StateClosed, transitions[0].from)
	assert.Equal(t, circuitbreaker.StateOpen, transitions[0].to)

	// Wait for reset timeout
	time.Sleep(15 * time.Millisecond)

	// Probe call (transitions to half-open, then success transitions to closed)
	err := cb.Execute(ctx, func() error { return nil })
	assert.NoError(t, err)
	assert.Equal(t, float64(circuitbreaker.StateClosed), stateGauge.val)
}

func TestState_String(t *testing.T) {
	assert.Equal(t, "CLOSED", circuitbreaker.StateClosed.String())
	assert.Equal(t, "OPEN", circuitbreaker.StateOpen.String())
	assert.Equal(t, "HALF_OPEN", circuitbreaker.StateHalfOpen.String())
	assert.Equal(t, "UNKNOWN", circuitbreaker.State(999).String())
}

func TestCircuitBreaker_StateGetter(t *testing.T) {
	cb := circuitbreaker.NewConsecutiveBreaker(1, 20*time.Millisecond)
	assert.Equal(t, circuitbreaker.StateClosed, cb.State())

	ctx := context.Background()
	_ = cb.Execute(ctx, func() error { return errors.New("fail") })
	assert.Equal(t, circuitbreaker.StateOpen, cb.State())

	time.Sleep(30 * time.Millisecond)
	assert.Equal(t, circuitbreaker.StateHalfOpen, cb.State())
}

func TestRatioBreaker(t *testing.T) {
	// Defaults check
	defRB := circuitbreaker.NewRatioBreaker(0, 0, 0, 0)
	assert.Equal(t, circuitbreaker.StateClosed, defRB.State())

	reqCounter := &mockCounter{}
	failCounter := &mockCounter{}
	stateGauge := &mockGauge{}
	var transitions []struct{ from, to circuitbreaker.State }

	// 50% failure ratio, min 4 requests, 100ms window, 20ms resetTimeout
	rb := circuitbreaker.NewRatioBreaker(0.5, 4, 100*time.Millisecond, 20*time.Millisecond,
		circuitbreaker.WithMetrics(circuitbreaker.Metrics{
			Requests: reqCounter,
			Failures: failCounter,
			State:    stateGauge,
		}),
		circuitbreaker.WithOnStateChange(func(from, to circuitbreaker.State) {
			transitions = append(transitions, struct{ from, to circuitbreaker.State }{from, to})
		}),
	)

	ctx := context.Background()
	reqErr := errors.New("api down")

	// 1. Send 3 requests (2 fail, 1 succeeds) -> 66% failure, but total < minRequests (4) -> stays CLOSED
	_ = rb.Execute(ctx, func() error { return reqErr })
	_ = rb.Execute(ctx, func() error { return reqErr })
	_ = rb.Execute(ctx, func() error { return nil })
	assert.Equal(t, circuitbreaker.StateClosed, rb.State())

	// 2. 4th request succeeds (2 fail / 4 total = 50% failure) -> reaches minRequests and ratio >= 0.5 -> TRIPS OPEN
	_ = rb.Execute(ctx, func() error { return reqErr })
	assert.Equal(t, circuitbreaker.StateOpen, rb.State())
	require.NotEmpty(t, transitions)

	// 3. Execution rejected while OPEN
	err := rb.Execute(ctx, func() error { return nil })
	assert.ErrorIs(t, err, circuitbreaker.ErrCircuitOpen)

	// 4. Wait for reset timeout -> transitions to HALF_OPEN
	time.Sleep(30 * time.Millisecond)
	assert.Equal(t, circuitbreaker.StateHalfOpen, rb.State())

	// 5. Half-open failure re-trips back to OPEN
	_ = rb.Execute(ctx, func() error { return reqErr })
	assert.Equal(t, circuitbreaker.StateOpen, rb.State())

	// 6. Wait again for reset timeout -> probe succeeds -> recovers to CLOSED
	time.Sleep(30 * time.Millisecond)
	assert.Equal(t, circuitbreaker.StateHalfOpen, rb.State())

	err = rb.Execute(ctx, func() error { return nil })
	assert.NoError(t, err)
	assert.Equal(t, circuitbreaker.StateClosed, rb.State())
}

func TestRatioBreaker_WindowPruning(t *testing.T) {
	// Window 30ms, 50% failure, min 2 requests
	rb := circuitbreaker.NewRatioBreaker(0.5, 2, 30*time.Millisecond, 20*time.Millisecond)
	ctx := context.Background()

	// 1 failure
	_ = rb.Execute(ctx, func() error { return errors.New("old fail") })
	assert.Equal(t, circuitbreaker.StateClosed, rb.State())

	// Sleep past the window so old failure is pruned
	time.Sleep(50 * time.Millisecond)

	// Now 1 success, then 1 failure (1/2 = 50% failure) -> trips
	_ = rb.Execute(ctx, func() error { return nil })
	_ = rb.Execute(ctx, func() error { return errors.New("new fail") })
	assert.Equal(t, circuitbreaker.StateOpen, rb.State())
}

func TestCircuitBreaker_Adversarial_HalfOpenStampede(t *testing.T) {
	cb := circuitbreaker.NewConsecutiveBreaker(1, 20*time.Millisecond)
	ctx := context.Background()

	// Trip it open
	_ = cb.Execute(ctx, func() error { return errors.New("err") })
	assert.Equal(t, circuitbreaker.StateOpen, cb.State())

	// Wait for reset timeout so it enters half-open
	time.Sleep(25 * time.Millisecond)

	// Launch 100 concurrent requests. Exactly 1 should probe; 99 must be rejected immediately!
	concurrency := 100
	var executedCount int32
	var rejectedCount int32
	var wg sync.WaitGroup

	probeRunning := make(chan struct{})
	probeRelease := make(chan struct{})
	startGate := make(chan struct{})
	var probeStarted sync.Once

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-startGate
			err := cb.Execute(ctx, func() error {
				atomic.AddInt32(&executedCount, 1)
				probeStarted.Do(func() {
					close(probeRunning)
				})
				<-probeRelease
				return nil
			})
			if errors.Is(err, circuitbreaker.ErrCircuitOpen) {
				atomic.AddInt32(&rejectedCount, 1)
			}
		}()
	}

	// Release all goroutines to stampede simultaneously
	close(startGate)
	// Wait until the single probe is in flight
	<-probeRunning
	// Allow all other concurrent goroutines to hit the breaker while probe is in flight
	for atomic.LoadInt32(&rejectedCount) < int32(concurrency-1) {
		time.Sleep(1 * time.Millisecond)
	}
	// Release the in-flight probe to complete successfully
	close(probeRelease)

	wg.Wait()

	// Herd stampede prevented: exactly 1 probe ran during half-open
	assert.Equal(t, int32(1), atomic.LoadInt32(&executedCount))
	assert.Equal(t, int32(99), atomic.LoadInt32(&rejectedCount))
	// And after the single probe succeeded, the breaker is CLOSED
	assert.Equal(t, circuitbreaker.StateClosed, cb.State())
}

func TestCircuitBreaker_Adversarial_ContextCancellation(t *testing.T) {
	cb := circuitbreaker.NewConsecutiveBreaker(1, 50*time.Millisecond)
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // pre-cancel context

	called := false
	err := cb.Execute(ctx, func() error {
		called = true
		return nil
	})
	assert.ErrorIs(t, err, context.Canceled)
	assert.False(t, called)
}

func TestCircuitBreaker_Adversarial_ProbePanicDoesNotFreezeBreaker(t *testing.T) {
	cb := circuitbreaker.NewConsecutiveBreaker(1, 20*time.Millisecond)
	ctx := context.Background()

	// 1. Trip breaker open
	_ = cb.Execute(ctx, func() error { return errors.New("initial failure") })
	assert.Equal(t, circuitbreaker.StateOpen, cb.State())

	// 2. Wait for reset timeout into half-open
	time.Sleep(25 * time.Millisecond)
	assert.Equal(t, circuitbreaker.StateHalfOpen, cb.State())

	// 3. Probe panics
	assert.Panics(t, func() {
		_ = cb.Execute(ctx, func() error {
			panic("probe crash")
		})
	})

	// 4. Breaker must NOT freeze; it must transition back to StateOpen
	assert.Equal(t, circuitbreaker.StateOpen, cb.State())

	// Requests while open should be rejected
	err := cb.Execute(ctx, func() error { return nil })
	assert.ErrorIs(t, err, circuitbreaker.ErrCircuitOpen)

	// 5. Wait for reset timeout again
	time.Sleep(25 * time.Millisecond)
	assert.Equal(t, circuitbreaker.StateHalfOpen, cb.State())

	// Next probe succeeds and recovers the circuit to CLOSED
	err = cb.Execute(ctx, func() error { return nil })
	assert.NoError(t, err)
	assert.Equal(t, circuitbreaker.StateClosed, cb.State())
}

func TestRatioBreaker_Adversarial_ProbePanicDoesNotFreezeBreaker(t *testing.T) {
	rb := circuitbreaker.NewRatioBreaker(0.5, 2, 50*time.Millisecond, 20*time.Millisecond)
	ctx := context.Background()

	// Trip it open
	_ = rb.Execute(ctx, func() error { return errors.New("err 1") })
	_ = rb.Execute(ctx, func() error { return errors.New("err 2") })
	assert.Equal(t, circuitbreaker.StateOpen, rb.State())

	// Wait into half-open
	time.Sleep(25 * time.Millisecond)
	assert.Equal(t, circuitbreaker.StateHalfOpen, rb.State())

	// Probe panics
	assert.Panics(t, func() {
		_ = rb.Execute(ctx, func() error {
			panic("probe panic in ratio")
		})
	})

	// Must be Open again, not frozen
	assert.Equal(t, circuitbreaker.StateOpen, rb.State())

	time.Sleep(25 * time.Millisecond)
	assert.Equal(t, circuitbreaker.StateHalfOpen, rb.State())

	// Successful probe closes it
	err := rb.Execute(ctx, func() error { return nil })
	assert.NoError(t, err)
	assert.Equal(t, circuitbreaker.StateClosed, rb.State())
}

func TestCircuitBreaker_Adversarial_SlowInFlightDoesNotCorruptHalfOpen(t *testing.T) {
	cb := circuitbreaker.NewConsecutiveBreaker(1, 20*time.Millisecond)
	ctx := context.Background()

	slowStarted := make(chan struct{})
	slowDone := make(chan struct{})
	var wg sync.WaitGroup

	// 1. Slow request starts while CLOSED
	var slowErr error
	wg.Add(1)
	go func() {
		defer wg.Done()
		slowErr = cb.Execute(ctx, func() error {
			close(slowStarted)
			<-slowDone
			return nil // slow request eventually succeeds
		})
	}()

	<-slowStarted

	// 2. While slow request is in-flight, another request fails and trips the breaker
	_ = cb.Execute(ctx, func() error { return errors.New("fast error") })
	assert.Equal(t, circuitbreaker.StateOpen, cb.State())

	// 3. Wait for reset timeout so breaker enters HALF_OPEN
	time.Sleep(25 * time.Millisecond)
	assert.Equal(t, circuitbreaker.StateHalfOpen, cb.State())

	// 4. Release the slow request now and wait for it
	close(slowDone)
	wg.Wait()

	// Slow request from previous epoch succeeded
	assert.NoError(t, slowErr)

	// Breaker MUST NOT have been prematurely closed by the old slow request!
	// It must still be HALF_OPEN awaiting its legitimate canary probe!
	assert.Equal(t, circuitbreaker.StateHalfOpen, cb.State())

	// The legitimate canary probe now runs and fails
	err := cb.Execute(ctx, func() error { return errors.New("canary failed") })
	assert.Error(t, err)

	// Must properly trip back to OPEN
	assert.Equal(t, circuitbreaker.StateOpen, cb.State())
}

func TestRatioBreaker_Adversarial_SlowInFlightDoesNotCorruptHalfOpen(t *testing.T) {
	rb := circuitbreaker.NewRatioBreaker(0.5, 2, 50*time.Millisecond, 20*time.Millisecond)
	ctx := context.Background()

	slowStarted := make(chan struct{})
	slowDone := make(chan struct{})
	var wg sync.WaitGroup

	// 1. Slow request starts while CLOSED
	var slowErr error
	wg.Add(1)
	go func() {
		defer wg.Done()
		slowErr = rb.Execute(ctx, func() error {
			close(slowStarted)
			<-slowDone
			return nil
		})
	}()

	<-slowStarted

	// 2. While slow request is in-flight, 2 requests fail and trip the breaker
	_ = rb.Execute(ctx, func() error { return errors.New("err 1") })
	_ = rb.Execute(ctx, func() error { return errors.New("err 2") })
	assert.Equal(t, circuitbreaker.StateOpen, rb.State())

	// 3. Wait for reset timeout so breaker enters HALF_OPEN
	time.Sleep(25 * time.Millisecond)
	assert.Equal(t, circuitbreaker.StateHalfOpen, rb.State())

	// 4. Release slow request and wait for it
	close(slowDone)
	wg.Wait()

	assert.NoError(t, slowErr)

	// Breaker MUST NOT have been prematurely closed by the old slow request
	assert.Equal(t, circuitbreaker.StateHalfOpen, rb.State())

	// Legitimate canary probe fails
	err := rb.Execute(ctx, func() error { return errors.New("canary failed") })
	assert.Error(t, err)

	// Trips back to OPEN
	assert.Equal(t, circuitbreaker.StateOpen, rb.State())
}

func TestCircuitBreaker_HalfOpenProbeThrottling(t *testing.T) {
	cb := circuitbreaker.NewConsecutiveBreaker(1, 20*time.Millisecond)
	ctx := context.Background()

	// Trip it open
	_ = cb.Execute(ctx, func() error { return errors.New("err") })
	assert.Equal(t, circuitbreaker.StateOpen, cb.State())

	time.Sleep(25 * time.Millisecond)
	assert.Equal(t, circuitbreaker.StateHalfOpen, cb.State())

	concurrency := 50
	var executedCount int32
	var rejectedCount int32
	var wg sync.WaitGroup

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := cb.Execute(ctx, func() error {
				atomic.AddInt32(&executedCount, 1)
				time.Sleep(10 * time.Millisecond)
				return nil
			})
			if errors.Is(err, circuitbreaker.ErrCircuitOpen) {
				atomic.AddInt32(&rejectedCount, 1)
			}
		}()
	}

	wg.Wait()

	assert.Equal(t, int32(1), atomic.LoadInt32(&executedCount), "Strict probe quota: only 1 probe in flight")
	assert.Equal(t, int32(49), atomic.LoadInt32(&rejectedCount), "All concurrent requests rejected")
	assert.Equal(t, circuitbreaker.StateClosed, cb.State())
}

func TestCircuitBreaker_ContextCancellation_DuringProbe(t *testing.T) {
	cb := circuitbreaker.NewConsecutiveBreaker(1, 20*time.Millisecond)

	// Trip it open
	_ = cb.Execute(context.Background(), func() error { return errors.New("err") })
	time.Sleep(25 * time.Millisecond)
	assert.Equal(t, circuitbreaker.StateHalfOpen, cb.State())

	// Cancelled context should not consume probe
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := cb.Execute(ctx, func() error { return nil })
	assert.ErrorIs(t, err, context.Canceled)

	// Legitimate probe can now run
	probeRan := false
	err = cb.Execute(context.Background(), func() error {
		probeRan = true
		return nil
	})
	assert.NoError(t, err)
	assert.True(t, probeRan)
	assert.Equal(t, circuitbreaker.StateClosed, cb.State())
}

func TestConsecutiveBreaker_ContextCancellation_NoFailureIncrement(t *testing.T) {
	failCounter := &mockCounter{}
	cb := circuitbreaker.NewConsecutiveBreaker(2, 50*time.Millisecond,
		circuitbreaker.WithMetrics(circuitbreaker.Metrics{
			Failures: failCounter,
		}),
	)

	// Pre-cancel context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	called := false
	err := cb.Execute(ctx, func() error {
		called = true
		return nil
	})

	assert.ErrorIs(t, err, context.Canceled)
	assert.False(t, called, "Function should not have been executed on canceled context")
	assert.Equal(t, 0, cb.Failures(), "Failure counter on breaker must not increment")
	assert.Equal(t, 0.0, failCounter.count, "Metrics failure counter must not increment")
	assert.Equal(t, circuitbreaker.StateClosed, cb.State())

	// Threshold is 2 failures; a single real failure must not trip the breaker
	err = cb.Execute(context.Background(), func() error { return errors.New("actual downstream error") })
	assert.Error(t, err)
	assert.Equal(t, 1, cb.Failures(), "Only real failure should increment counter")
	assert.Equal(t, 1.0, failCounter.count)
	assert.Equal(t, circuitbreaker.StateClosed, cb.State(), "Breaker should still be closed after 1 failure")
}

func TestConsecutiveBreaker_HalfOpen_OnlyOneProbeConcurrentRejected(t *testing.T) {
	cb := circuitbreaker.NewConsecutiveBreaker(1, 20*time.Millisecond)
	ctx := context.Background()

	// 1. Trip breaker open
	err := cb.Execute(ctx, func() error { return errors.New("trip breaker") })
	assert.Error(t, err)
	assert.Equal(t, circuitbreaker.StateOpen, cb.State())

	// 2. Wait for reset timeout
	time.Sleep(25 * time.Millisecond)
	assert.Equal(t, circuitbreaker.StateHalfOpen, cb.State())

	// 3. Launch probe goroutine that blocks while in-flight
	probeStarted := make(chan struct{})
	probeHold := make(chan struct{})
	var probeCount int32
	var rejectedCount int32
	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		probeErr := cb.Execute(ctx, func() error {
			atomic.AddInt32(&probeCount, 1)
			close(probeStarted)
			<-probeHold
			return nil
		})
		assert.NoError(t, probeErr)
	}()

	<-probeStarted

	// Breaker is currently half-open with probe in flight. Launch 20 concurrent requests.
	concurrentCallers := 20
	for i := 0; i < concurrentCallers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			callErr := cb.Execute(ctx, func() error {
				atomic.AddInt32(&probeCount, 1)
				return nil
			})
			if errors.Is(callErr, circuitbreaker.ErrCircuitOpen) {
				atomic.AddInt32(&rejectedCount, 1)
			}
		}()
	}

	// Wait briefly for all concurrent callers to attempt execution
	time.Sleep(10 * time.Millisecond)
	assert.Equal(t, int32(1), atomic.LoadInt32(&probeCount), "Strictly only 1 probe admitted")
	assert.Equal(t, int32(concurrentCallers), atomic.LoadInt32(&rejectedCount), "All other concurrent callers must receive ErrCircuitOpen")

	// Release probe
	close(probeHold)
	wg.Wait()

	// After successful probe, breaker must be CLOSED
	assert.Equal(t, circuitbreaker.StateClosed, cb.State())
}

func TestCircuitBreaker_State_StrictThreadSafetyUnderReadLocks(t *testing.T) {
	cb := circuitbreaker.NewConsecutiveBreaker(1, 20*time.Millisecond)
	ctx := context.Background()

	_ = cb.Execute(ctx, func() error { return errors.New("trip") })
	time.Sleep(25 * time.Millisecond)

	var wg sync.WaitGroup
	// 50 concurrent goroutines calling State()
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s := cb.State()
			assert.Equal(t, circuitbreaker.StateHalfOpen, s)
		}()
	}
	wg.Wait()
}

func TestCircuitBreaker_ConfigurableHalfOpenProbes(t *testing.T) {
	// Configure breaker to admit up to 2 concurrent probes in half-open state
	cb := circuitbreaker.NewConsecutiveBreaker(1, 20*time.Millisecond,
		circuitbreaker.WithHalfOpenProbes(2),
	)
	ctx := context.Background()

	_ = cb.Execute(ctx, func() error { return errors.New("trip") })
	time.Sleep(25 * time.Millisecond)
	assert.Equal(t, circuitbreaker.StateHalfOpen, cb.State())

	probeStarted := make(chan struct{})
	probeHold := make(chan struct{})
	var probeCount int32
	var rejectedCount int32
	var wg sync.WaitGroup

	// Start 2 probes concurrently
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = cb.Execute(ctx, func() error {
				if atomic.AddInt32(&probeCount, 1) == 2 {
					close(probeStarted)
				}
				<-probeHold
				return nil
			})
		}()
	}

	<-probeStarted

	// 3rd concurrent caller while 2 probes are in-flight should be rejected
	err := cb.Execute(ctx, func() error {
		atomic.AddInt32(&probeCount, 1)
		return nil
	})
	if errors.Is(err, circuitbreaker.ErrCircuitOpen) {
		atomic.AddInt32(&rejectedCount, 1)
	}

	assert.Equal(t, int32(2), atomic.LoadInt32(&probeCount))
	assert.Equal(t, int32(1), atomic.LoadInt32(&rejectedCount))

	close(probeHold)
	wg.Wait()
}

func TestCircuitBreaker_OnStateChange_NoDeadlockOnStateQuery(t *testing.T) {
	var observedState circuitbreaker.State
	var cb *circuitbreaker.ConsecutiveBreaker

	cb = circuitbreaker.NewConsecutiveBreaker(1, 20*time.Millisecond,
		circuitbreaker.WithOnStateChange(func(from, to circuitbreaker.State) {
			// Querying breaker state directly from the callback MUST NOT deadlock
			observedState = cb.State()
			_ = cb.Failures()
		}),
	)

	ctx := context.Background()
	_ = cb.Execute(ctx, func() error {
		return errors.New("trip breaker")
	})

	assert.Equal(t, circuitbreaker.StateOpen, observedState)
	assert.Equal(t, circuitbreaker.StateOpen, cb.State())

	// Test RatioBreaker as well
	var rbObservedState circuitbreaker.State
	var rb *circuitbreaker.RatioBreaker
	rb = circuitbreaker.NewRatioBreaker(0.5, 1, 10*time.Second, 20*time.Millisecond,
		circuitbreaker.WithOnStateChange(func(from, to circuitbreaker.State) {
			rbObservedState = rb.State()
			_ = rb.Failures()
		}),
	)

	_ = rb.Execute(ctx, func() error {
		return errors.New("trip ratio breaker")
	})

	assert.Equal(t, circuitbreaker.StateOpen, rbObservedState)
	assert.Equal(t, circuitbreaker.StateOpen, rb.State())
}

func TestConsecutiveBreaker_ZeroValue(t *testing.T) {
	var cb circuitbreaker.ConsecutiveBreaker
	assert.Equal(t, circuitbreaker.StateClosed, cb.State())
	assert.Equal(t, 0, cb.Failures())

	ctx := context.Background()
	err := cb.Execute(ctx, func() error {
		return nil
	})
	assert.NoError(t, err)

	for i := 0; i < 5; i++ {
		_ = cb.Execute(ctx, func() error {
			return errors.New("err")
		})
	}
	assert.Equal(t, circuitbreaker.StateOpen, cb.State())
	err = cb.Execute(ctx, func() error { return nil })
	assert.ErrorIs(t, err, circuitbreaker.ErrCircuitOpen)
}

func TestRatioBreaker_ZeroValue(t *testing.T) {
	var rb circuitbreaker.RatioBreaker
	assert.Equal(t, circuitbreaker.StateClosed, rb.State())
	assert.Equal(t, 0, rb.Failures())

	ctx := context.Background()
	err := rb.Execute(ctx, func() error {
		return nil
	})
	assert.NoError(t, err)

	for i := 0; i < 10; i++ {
		_ = rb.Execute(ctx, func() error {
			return errors.New("err")
		})
	}
	assert.Equal(t, circuitbreaker.StateOpen, rb.State())
	err = rb.Execute(ctx, func() error { return nil })
	assert.ErrorIs(t, err, circuitbreaker.ErrCircuitOpen)
}
