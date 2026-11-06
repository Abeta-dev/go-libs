// SPDX-License-Identifier: MIT

package httpclient

import (
	"context"
	"errors"
	"math"
	"net/http"
	"testing"
	"time"

	"github.com/abeta-dev/go-libs/circuitbreaker"
	"github.com/abeta-dev/go-libs/retry"
	"go.opentelemetry.io/otel/trace/noop"
)

type errRoundTripper struct {
	err error
}

func (e errRoundTripper) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, e.err
}

func TestHTTPClient_InternalBranches(t *testing.T) {
	// 1. NewRoundTripper nil metrics & nil transport fallback
	rt := NewRoundTripper(
		WithMetrics(Metrics{}),
		func(o *options) { o.transport = nil },
	)
	if rt == nil {
		t.Fatal("expected non-nil RoundTripper")
	}

	// 2. CircuitBreaker transport error (resp == nil, execErr != nil) + Telemetry error recording + ShouldRetry false
	netErr := errors.New("network unreachable")
	cb := circuitbreaker.NewConsecutiveBreaker(5, time.Minute)
	tracer := noop.NewTracerProvider().Tracer("test")

	rtErr := NewRoundTripper(
		WithTransport(errRoundTripper{err: netErr}),
		WithCircuitBreaker(cb),
		WithTelemetry(tracer),
		WithRetry(retry.Config{
			Attempts:    3,
			ShouldRetry: func(err error) bool { return false },
		}),
	)

	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://example.com", nil)
	resp, err := rtErr.RoundTrip(req)
	if resp != nil {
		_ = resp.Body.Close()
	}
	if !errors.Is(err, netErr) {
		t.Fatalf("expected netErr, got %v", err)
	}

	// 3. Pre-cancelled context entering executeRetries
	ctxCancel, cancel := context.WithCancel(context.Background())
	cancel()
	reqCancel, _ := http.NewRequestWithContext(ctxCancel, http.MethodGet, "http://example.com", nil)
	rtPlain := NewRoundTripper(WithTransport(errRoundTripper{err: netErr}))
	respCancel, err := rtPlain.RoundTrip(reqCancel)
	if respCancel != nil {
		_ = respCancel.Body.Close()
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}

	// 4. computeRetryWait branches: >30 attempts, MaxWait cap, unknown strategy, and negative maxWait on ExponentialJitter
	w1 := computeRetryWait(retry.Config{
		InitialWait: 10 * time.Millisecond,
		MaxWait:     15 * time.Millisecond,
		Strategy:    retry.Exponential,
	}, 35)
	if w1 != 15*time.Millisecond {
		t.Fatalf("expected MaxWait cap 15ms, got %v", w1)
	}

	w2 := computeRetryWait(retry.Config{
		InitialWait: 10 * time.Millisecond,
		Strategy:    retry.Strategy(999),
	}, 1)
	if w2 != 10*time.Millisecond {
		t.Fatalf("expected default strategy wait 10ms, got %v", w2)
	}

	// ExponentialJitter where float64(InitialWait)*mult overflows to +Inf -> <=0 check or NaN
	w3 := computeRetryWait(retry.Config{
		InitialWait: time.Duration(math.MaxInt64),
		Strategy:    retry.ExponentialJitter,
	}, 2000)
	_ = w3
}
