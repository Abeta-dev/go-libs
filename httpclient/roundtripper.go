// SPDX-License-Identifier: MIT

package httpclient

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"math/rand"
	"net/http"
	"time"

	"github.com/umesh0492/go-libs/circuitbreaker"
	"github.com/umesh0492/go-libs/retry"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

type resilientTransport struct {
	opts options
}

func (rt *resilientTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.Body != nil && req.Body != http.NoBody && req.GetBody == nil {
		bodyBytes, err := io.ReadAll(req.Body)
		if err != nil {
			return nil, err
		}
		_ = req.Body.Close()
		req.Body = io.NopCloser(bytes.NewReader(bodyBytes))
		req.GetBody = func() (io.ReadCloser, error) {
			return io.NopCloser(bytes.NewReader(bodyBytes)), nil
		}
	}

	ctx := req.Context()
	if rt.opts.totalTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, rt.opts.totalTimeout)
		defer cancel()
	}

	// Layer 1 (Outermost): Rate Limiting
	// Drop excess requests immediately before spending circuit breaker or retry budget.
	if rt.opts.rateLimiterFunc != nil {
		if !rt.opts.rateLimiterFunc(req) {
			return nil, ErrRateLimited
		}
	} else if rt.opts.rateLimiter != nil {
		key := req.URL.Host
		if rt.opts.rateLimiterKeyFunc != nil {
			key = rt.opts.rateLimiterKeyFunc(req)
		}
		if !rt.opts.rateLimiter.Allow(key) {
			return nil, ErrRateLimited
		}
	}

	// Layer 2: Circuit Breaker
	// Fail fast immediately if downstream is known dead, avoiding unnecessary network attempts.
	if rt.opts.circuitBreaker != nil {
		var resp *http.Response
		var execErr error
		cbErr := rt.opts.circuitBreaker.Execute(ctx, func() error {
			//nolint:bodyclose // response body is passed to caller for deferred closure
			resp, execErr = rt.executeRetries(ctx, req)
			if execErr != nil {
				return execErr
			}
			if resp != nil && resp.StatusCode >= 500 {
				return fmt.Errorf("httpclient: downstream returned HTTP %d", resp.StatusCode)
			}
			return nil
		})
		if cbErr != nil {
			if errors.Is(cbErr, circuitbreaker.ErrCircuitOpen) {
				return nil, cbErr
			}
			if resp != nil {
				return resp, nil
			}
			return nil, cbErr
		}
		return resp, nil
	}

	return rt.executeRetries(ctx, req)
}

func (rt *resilientTransport) executeRetries(ctx context.Context, req *http.Request) (*http.Response, error) {
	attempts := rt.opts.retryCfg.Attempts
	if attempts <= 0 {
		attempts = 1
	}

	var lastResp *http.Response
	var lastErr error

	for attempt := 0; attempt < attempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		resp, retryable, err := rt.performAttempt(ctx, req, attempt)
		if !retryable {
			return resp, nil
		}

		lastResp = resp
		lastErr = err

		if rt.opts.retryCfg.ShouldRetry != nil && !rt.opts.retryCfg.ShouldRetry(lastErr) {
			break
		}

		if attempt == attempts-1 {
			break
		}

		if lastResp != nil && lastResp.Body != nil {
			_ = lastResp.Body.Close()
			lastResp = nil
		}

		if err := rt.backoffWait(ctx, attempt); err != nil {
			return nil, err
		}
	}

	if lastResp != nil {
		return lastResp, nil
	}
	return nil, lastErr
}

func (rt *resilientTransport) performAttempt(ctx context.Context, req *http.Request, attempt int) (*http.Response, bool, error) {
	attemptCtx := ctx
	var cancel context.CancelFunc
	if rt.opts.perAttemptTimeout > 0 {
		attemptCtx, cancel = context.WithTimeout(ctx, rt.opts.perAttemptTimeout)
	}
	defer func() {
		if cancel != nil {
			cancel()
		}
	}()

	var span trace.Span
	if rt.opts.tracer != nil {
		var spanCtx context.Context
		spanCtx, span = rt.opts.tracer.Start(attemptCtx, "HTTP "+req.Method)
		attemptCtx = spanCtx
		defer span.End()
	}

	attemptReq := req.Clone(attemptCtx)
	if attempt > 0 && req.GetBody != nil {
		body, err := req.GetBody()
		if err == nil {
			attemptReq.Body = body
		}
	}

	if rt.opts.tracer != nil {
		propagation.TraceContext{}.Inject(attemptCtx, propagation.HeaderCarrier(attemptReq.Header))
	}

	startTime := rt.opts.clock.Now()
	rt.opts.metrics.Requests.Inc()

	resp, err := rt.opts.transport.RoundTrip(attemptReq)
	rt.opts.metrics.Duration.Observe(rt.opts.clock.Since(startTime).Seconds())

	if span != nil && err != nil {
		span.RecordError(err)
	}

	switch {
	case err != nil:
		rt.opts.metrics.Failures.Inc()
		if attemptCtx.Err() != nil && ctx.Err() == nil {
			return nil, true, ErrAttemptTimeout
		}
		return nil, true, err
	case resp.StatusCode >= 500:
		rt.opts.metrics.Failures.Inc()
		return resp, true, fmt.Errorf("httpclient: server error HTTP %d", resp.StatusCode)
	default:
		return resp, false, nil
	}
}

func (rt *resilientTransport) backoffWait(ctx context.Context, attempt int) error {
	wait := computeRetryWait(rt.opts.retryCfg, attempt)
	if wait <= 0 {
		return nil
	}
	timer := rt.opts.clock.NewTimer(wait)
	select {
	case <-ctx.Done():
		timer.Stop()
		return ctx.Err()
	case <-timer.C():
		return nil
	}
}

func computeRetryWait(cfg retry.Config, attempt int) time.Duration {
	if cfg.InitialWait <= 0 {
		return 0
	}
	expAttempt := attempt
	if expAttempt > 30 {
		expAttempt = 30
	}

	var wait time.Duration
	switch cfg.Strategy {
	case retry.Linear:
		wait = cfg.InitialWait * time.Duration(attempt+1)
	case retry.Exponential:
		mult := math.Pow(2, float64(expAttempt))
		wait = time.Duration(float64(cfg.InitialWait) * mult)
	case retry.ExponentialJitter:
		mult := math.Pow(2, float64(expAttempt))
		maxWait := float64(cfg.InitialWait) * mult
		if maxWait <= 0 {
			wait = 0
		} else {
			//nolint:gosec // G404: pseudo-random jitter is appropriate for network retry backoff
			wait = time.Duration(rand.Float64() * maxWait)
		}
	case retry.Constant:
		wait = cfg.InitialWait
	default:
		wait = cfg.InitialWait
	}

	if cfg.MaxWait > 0 && wait > cfg.MaxWait {
		wait = cfg.MaxWait
	}
	return wait
}
