---
name: go-libs-resilience
description: Resilience, concurrency, caching, and lifecycle patterns using github.com/Abeta-dev/go-libs (retry, circuitbreaker, ratelimit, workerpool, cache, async, shutdown, fsm, clock, metrics). Use when hardening external calls, background workers, caches, or state machines.
---

# `go-libs-resilience` Subskill (`v0.3.2`)

Packages covered:
- `github.com/Abeta-dev/go-libs/retry`
- `github.com/Abeta-dev/go-libs/circuitbreaker`
- `github.com/Abeta-dev/go-libs/ratelimit`
- `github.com/Abeta-dev/go-libs/workerpool`
- `github.com/Abeta-dev/go-libs/cache`
- `github.com/Abeta-dev/go-libs/async`
- `github.com/Abeta-dev/go-libs/shutdown`
- `github.com/Abeta-dev/go-libs/fsm`
- `github.com/Abeta-dev/go-libs/clock`
- `github.com/Abeta-dev/go-libs/metrics`

## 1. Layered Outbound Call Hardening (`circuitbreaker` + `retry`)

Always wrap **retry outside** or **circuit breaker inside** each attempt so a tripped breaker fails fast without burning retry budgets, or use `retry.Permanent(err)` when `circuitbreaker.ErrCircuitOpen` is returned:

```go
cb := circuitbreaker.New(circuitbreaker.Config{
	Name:             "bank-payout-api",
	FailureThreshold: 5,
	ResetTimeout:     30 * time.Second,
	HalfOpenMaxCalls: 2,
})

resp, err := retry.DoWithData(ctx, retry.Config{
	MaxAttempts: 3,
	BaseDelay:   100 * time.Millisecond,
	MaxDelay:    2 * time.Second,
}, func(ctx context.Context) (*PayoutResponse, error) {
	var out *PayoutResponse
	err := cb.Execute(func() error {
		var callErr error
		out, callErr = client.SendPayout(ctx, req)
		return callErr
	})
	if errors.Is(err, circuitbreaker.ErrCircuitOpen) {
		return nil, retry.Permanent(err) // Fail fast when breaker is open
	}
	return out, err
})
```

## 2. Stampede-Safe Caching (`cache`)

Use `GetOrLoad` to coalesce concurrent cache misses via singleflight:

```go
memCache := cache.NewMemory[string, TenantConfig](cache.MemoryConfig{
	MaxEntries: 10_000,
	DefaultTTL: 5 * time.Minute,
})

cfg, err := memCache.GetOrLoad(ctx, tenantID, func(ctx context.Context) (TenantConfig, error) {
	return repo.FetchTenantConfig(ctx, tenantID)
})
```

## 3. Bounded Concurrency (`workerpool` & `async`)

- Use `workerpool.New[T, R]` when processing large slices or streams with fixed goroutine concurrency and panic safety.
- Use `async.Parallel(ctx, fn1, fn2, ...)` when fanning out a small fixed set of heterogeneous calls in a single request handler.

## 4.Deterministic Time & Graceful Shutdown (`clock` & `shutdown`)

- Inject `clock.Clock` (`clock.RealClock{}` in production, `clock.NewMockClock(t0)` in unit tests) into services that compute TTLs, SLA deadlines, or backoff windows.
- Register cleanup hooks (`http.Server.Shutdown`, worker pool drain, OTel flush, DB pool close) with `shutdown.Manager` so resources close in deterministic order on `SIGTERM`.

## Guardrails

- Never retry non-idempotent mutations unless guarded by an idempotency key (`go-libs/idempotency`).
- Always respect `ctx.Done()` inside `retry` and `workerpool` task functions.
