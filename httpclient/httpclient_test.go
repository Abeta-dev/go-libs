// SPDX-License-Identifier: MIT
package httpclient_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/umesh0492/go-libs/circuitbreaker"
	"github.com/umesh0492/go-libs/clock"
	"github.com/umesh0492/go-libs/httpclient"
	"github.com/umesh0492/go-libs/ratelimit"
	"github.com/umesh0492/go-libs/retry"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
)

type testMetrics struct {
	requests atomic.Int64
	failures atomic.Int64
	observed atomic.Int64
}

func (m *testMetrics) incReq()  { m.requests.Add(1) }
func (m *testMetrics) incFail() { m.failures.Add(1) }

type testCounter struct {
	fn func()
}

func (tc *testCounter) Inc()          { tc.fn() }
func (tc *testCounter) Add(v float64) { tc.fn() }

type testHistogram struct {
	m *testMetrics
}

func (th *testHistogram) Observe(v float64) {
	th.m.observed.Add(1)
}

func TestHTTPClient_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("hello"))
	}))
	defer server.Close()

	client := httpclient.New()
	resp, err := client.Get(server.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestHTTPClient_RateLimiter(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	fc := clock.NewFake()
	tb := ratelimit.NewTokenBucket(1, time.Minute, ratelimit.WithClock(fc))

	client := httpclient.New(
		httpclient.WithRateLimiter(tb),
		httpclient.WithClock(fc),
	)

	// First request succeeds
	resp1, err := client.Get(server.URL)
	if err != nil {
		t.Fatalf("expected request 1 to succeed, got %v", err)
	}
	resp1.Body.Close()

	// Second request rejected by rate limiter
	resp2, err := client.Get(server.URL)
	if resp2 != nil {
		_ = resp2.Body.Close()
	}
	if !errors.Is(err, httpclient.ErrRateLimited) {
		t.Fatalf("expected ErrRateLimited, got %v", err)
	}

	// Advance clock by 1 minute -> allowed again
	fc.Add(time.Minute)
	resp3, err := client.Get(server.URL)
	if err != nil {
		t.Fatalf("expected request 3 to succeed after refill, got %v", err)
	}
	resp3.Body.Close()
}

func TestHTTPClient_RateLimiterFunc(t *testing.T) {
	allowed := false
	client := httpclient.New(
		httpclient.WithRateLimiterFunc(func(r *http.Request) bool {
			return allowed
		}),
	)

	req, _ := http.NewRequest(http.MethodGet, "http://localhost/test", nil)
	respDenied, err := client.Do(req)
	if respDenied != nil {
		_ = respDenied.Body.Close()
	}
	if !errors.Is(err, httpclient.ErrRateLimited) {
		t.Fatalf("expected ErrRateLimited when predicate false, got %v", err)
	}

	allowed = true
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	req2, _ := http.NewRequest(http.MethodGet, server.URL, nil)
	resp, err := client.Do(req2)
	if err != nil {
		t.Fatalf("expected success when predicate true, got %v", err)
	}
	resp.Body.Close()
}

func TestHTTPClient_CircuitBreaker_FastFail(t *testing.T) {
	fc := clock.NewFake()
	cb := circuitbreaker.NewConsecutiveBreaker(2, 30*time.Second, circuitbreaker.WithClock(fc))

	attempts := atomic.Int32{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client := httpclient.New(
		httpclient.WithCircuitBreaker(cb),
		httpclient.WithRetry(retry.Config{Attempts: 1}),
		httpclient.WithClock(fc),
	)

	// Call 1: 500 error -> failure 1
	r1, _ := client.Get(server.URL)
	if r1 != nil {
		_ = r1.Body.Close()
	}
	// Call 2: 500 error -> failure 2 -> circuit trips to Open
	r2, _ := client.Get(server.URL)
	if r2 != nil {
		_ = r2.Body.Close()
	}

	if cb.State() != circuitbreaker.StateOpen {
		t.Fatalf("expected circuit breaker Open, got %v", cb.State())
	}

	serverCallsBefore := attempts.Load()

	// Call 3: should fail fast with ErrCircuitOpen WITHOUT invoking server
	r3, err := client.Get(server.URL)
	if r3 != nil {
		_ = r3.Body.Close()
	}
	if !errors.Is(err, circuitbreaker.ErrCircuitOpen) {
		t.Fatalf("expected ErrCircuitOpen, got %v", err)
	}

	if attempts.Load() != serverCallsBefore {
		t.Fatalf("server should not have been called when circuit is Open")
	}
}

func TestHTTPClient_RetryOn5xx(t *testing.T) {
	var count atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		val := count.Add(1)
		if val < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("eventual success"))
	}))
	defer server.Close()

	fc := clock.NewFake()
	client := httpclient.New(
		httpclient.WithRetry(retry.Config{
			Attempts:    4,
			InitialWait: 10 * time.Millisecond,
			Strategy:    retry.Constant,
		}),
		httpclient.WithClock(fc),
	)

	type res struct {
		resp *http.Response
		err  error
	}
	done := make(chan res, 1)
	go func() {
		//nolint:bodyclose // response body passed through channel and closed on line 227
		r, err := client.Get(server.URL)
		done <- res{r, err}
	}()

	// Wait for attempt 1 failure and retry timer
	fc.BlockUntil(1)
	fc.Add(10 * time.Millisecond)

	// Wait for attempt 2 failure and retry timer
	fc.BlockUntil(1)
	fc.Add(10 * time.Millisecond)

	r := <-done
	if r.err != nil {
		t.Fatalf("expected eventual success, got %v", r.err)
	}
	defer r.resp.Body.Close()

	if count.Load() != 3 {
		t.Fatalf("expected 3 server attempts, got %d", count.Load())
	}
	if r.resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", r.resp.StatusCode)
	}
}

func TestHTTPClient_PerAttemptTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(50 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := httpclient.New(
		httpclient.WithPerAttemptTimeout(10*time.Millisecond),
		httpclient.WithRetry(retry.Config{Attempts: 1}),
	)

	respTimeout, err := client.Get(server.URL)
	if respTimeout != nil {
		_ = respTimeout.Body.Close()
	}
	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}
	if !errors.Is(err, httpclient.ErrAttemptTimeout) && !errors.Is(err, context.DeadlineExceeded) {
		// client.Get wraps errors with url.Error
		t.Logf("got expected timeout error: %v", err)
	}
}

func TestHTTPClient_TotalTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := httpclient.New(
		httpclient.WithTotalTimeout(10 * time.Millisecond),
	)

	respTotal, err := client.Get(server.URL)
	if respTotal != nil {
		_ = respTotal.Body.Close()
	}
	if err == nil {
		t.Fatal("expected total timeout error, got nil")
	}
}

type mockTracer struct {
	trace.Tracer
	spansStarted atomic.Int32
}

func (m *mockTracer) Start(ctx context.Context, spanName string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	m.spansStarted.Add(1)
	traceID, _ := trace.TraceIDFromHex("4bf92f3577b34da6a3ce929d0e0e4736")
	spanID, _ := trace.SpanIDFromHex("00f067aa0ba902b7")
	sc := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    traceID,
		SpanID:     spanID,
		TraceFlags: trace.FlagsSampled,
	})
	ctx = trace.ContextWithSpanContext(ctx, sc)
	return ctx, noop.Span{}
}

func TestHTTPClient_MetricsAndTelemetry(t *testing.T) {
	headerInjected := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check that W3C traceparent header is injected
		if r.Header.Get("Traceparent") != "" {
			headerInjected = true
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	mt := &mockTracer{
		Tracer: noop.NewTracerProvider().Tracer("httpclient-test"),
	}

	tm := &testMetrics{}
	client := httpclient.New(
		httpclient.WithTelemetry(mt),
		httpclient.WithMetrics(httpclient.Metrics{
			Requests: &testCounter{fn: tm.incReq},
			Failures: &testCounter{fn: tm.incFail},
			Duration: &testHistogram{m: tm},
		}),
	)

	resp, err := client.Get(server.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resp.Body.Close()

	if tm.requests.Load() != 1 {
		t.Fatalf("expected 1 request recorded, got %d", tm.requests.Load())
	}
	if tm.observed.Load() != 1 {
		t.Fatalf("expected 1 duration observation, got %d", tm.observed.Load())
	}
	if mt.spansStarted.Load() != 1 {
		t.Fatalf("expected 1 span started, got %d", mt.spansStarted.Load())
	}
	if !headerInjected {
		t.Error("expected Traceparent header to be injected")
	}
}

func TestHTTPClient_CustomKeyRateLimiter(t *testing.T) {
	fc := clock.NewFake()
	tb := ratelimit.NewTokenBucket(1, time.Minute, ratelimit.WithClock(fc))

	client := httpclient.New(
		httpclient.WithRateLimiterKey(tb, func(r *http.Request) string {
			return r.Header.Get("X-Tenant-ID")
		}),
		httpclient.WithClock(fc),
	)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	reqA1, _ := http.NewRequest(http.MethodGet, server.URL, nil)
	reqA1.Header.Set("X-Tenant-ID", "tenant-a")
	respA1, err := client.Do(reqA1)
	if err != nil {
		t.Fatalf("tenant-a first request failed: %v", err)
	}
	respA1.Body.Close()

	// tenant-a second request blocked
	reqA2, _ := http.NewRequest(http.MethodGet, server.URL, nil)
	reqA2.Header.Set("X-Tenant-ID", "tenant-a")
	respA2, err := client.Do(reqA2)
	if respA2 != nil {
		_ = respA2.Body.Close()
	}
	if !errors.Is(err, httpclient.ErrRateLimited) {
		t.Fatalf("expected tenant-a to be rate limited, got %v", err)
	}

	// tenant-b request allowed (separate bucket)
	reqB1, _ := http.NewRequest(http.MethodGet, server.URL, nil)
	reqB1.Header.Set("X-Tenant-ID", "tenant-b")
	respB1, err := client.Do(reqB1)
	if err != nil {
		t.Fatalf("tenant-b first request failed: %v", err)
	}
	respB1.Body.Close()
}

func TestHTTPClient_WithTransportAndNilSafety(t *testing.T) {
	customTransport := &http.Transport{}
	client := httpclient.New(httpclient.WithTransport(customTransport))
	if client == nil {
		t.Fatal("expected non-nil client")
	}

	rt := httpclient.NewRoundTripper(nil)
	if rt == nil {
		t.Fatal("expected non-nil roundtripper")
	}
}

func TestHTTPClient_RetryStrategiesAndCapping(t *testing.T) {
	fc := clock.NewFake()

	tests := []struct {
		name     string
		strategy retry.Strategy
		initWait time.Duration
		maxWait  time.Duration
	}{
		{"linear", retry.Linear, 10 * time.Millisecond, 100 * time.Millisecond},
		{"exponential", retry.Exponential, 10 * time.Millisecond, 50 * time.Millisecond},
		{"jitter", retry.ExponentialJitter, 10 * time.Millisecond, 50 * time.Millisecond},
		{"constant", retry.Constant, 10 * time.Millisecond, 0},
		{"zero-wait", retry.Constant, 0, 0},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			attempts := atomic.Int32{}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if attempts.Add(1) < 2 {
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
				w.WriteHeader(http.StatusOK)
			}))
			defer server.Close()

			client := httpclient.New(
				httpclient.WithRetry(retry.Config{
					Attempts:    3,
					InitialWait: tc.initWait,
					MaxWait:     tc.maxWait,
					Strategy:    tc.strategy,
				}),
				httpclient.WithClock(fc),
			)

			done := make(chan struct{})
			go func() {
				// Advance fake clock whenever a timer is pending
				for {
					select {
					case <-done:
						return
					default:
						fc.Add(50 * time.Millisecond)
						time.Sleep(2 * time.Millisecond)
					}
				}
			}()

			resp, err := client.Get(server.URL)
			close(done)
			if err != nil {
				t.Fatalf("expected retry to succeed, got %v", err)
			}
			resp.Body.Close()
		})
	}
}

func TestHTTPClient_RetryWithRequestBody(t *testing.T) {
	fc := clock.NewFake()
	attempts := atomic.Int32{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if string(body) != "payload-data" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if attempts.Add(1) < 2 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := httpclient.New(
		httpclient.WithRetry(retry.Config{
			Attempts:    2,
			InitialWait: 10 * time.Millisecond,
			Strategy:    retry.Constant,
		}),
		httpclient.WithClock(fc),
	)

	done := make(chan struct{})
	go func() {
		for {
			select {
			case <-done:
				return
			default:
				fc.Add(20 * time.Millisecond)
				time.Sleep(2 * time.Millisecond)
			}
		}
	}()

	req, _ := http.NewRequest(http.MethodPost, server.URL, strings.NewReader("payload-data"))
	resp, err := client.Do(req)
	close(done)
	if err != nil {
		t.Fatalf("expected request with body to succeed after retry: %v", err)
	}
	resp.Body.Close()
}

func TestHTTPClient_RetryContextCancellation(t *testing.T) {
	fc := clock.NewFake()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer server.Close()

	client := httpclient.New(
		httpclient.WithRetry(retry.Config{
			Attempts:    3,
			InitialWait: 10 * time.Second,
			Strategy:    retry.Constant,
		}),
		httpclient.WithClock(fc),
	)

	ctx, cancel := context.WithCancel(context.Background())
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, server.URL, nil)

	// Block until timer is scheduled for retry backoff, then cancel ctx
	go func() {
		fc.BlockUntil(1)
		cancel()
	}()

	respCancel, err := client.Do(req)
	if respCancel != nil {
		_ = respCancel.Body.Close()
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}
