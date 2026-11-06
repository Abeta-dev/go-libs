---
name: go-libs-resilience
description: Resilience, concurrency, caching, and lifecycle patterns using github.com/abeta-dev/go-libs (retry, circuitbreaker, ratelimit, workerpool, cache, async, shutdown, fsm, clock, metrics). Use when hardening external calls, background workers, caches, or state machines.
---

# `go-libs-resilience` Subskill (`v0.4.0`)

Packages covered:
- `github.com/abeta-dev/go-libs/retry`
- `github.com/abeta-dev/go-libs/circuitbreaker`
- `github.com/abeta-dev/go-libs/ratelimit`
- `github.com/abeta-dev/go-libs/workerpool`
- `github.com/abeta-dev/go-libs/cache`
- `github.com/abeta-dev/go-libs/async`
- `github.com/abeta-dev/go-libs/shutdown`
- `github.com/abeta-dev/go-libs/fsm`
- `github.com/abeta-dev/go-libs/clock`
- `github.com/abeta-dev/go-libs/metrics`

## 1. Layered Outbound Call Hardening (`circuitbreaker` + `retry`)

Wrap outbound calls with `circuitbreaker.NewConsecutiveBreaker` (or `NewRatioBreaker`) and `retry.Do` / `retry.DoWithResult` with `retry.Config`:

```go
breaker := circuitbreaker.NewConsecutiveBreaker(5, 30*time.Second)

err := retry.Do(ctx, retry.Config{
	MaxAttempts: 3,
	Backoff:     retry.ExponentialJitter(100*time.Millisecond, 2*time.Second),
}, func() error {
	return breaker.Execute(ctx, func() error {
		return client.SendPayout(ctx, req)
	})
})
```

## 2. Stampede-Safe Caching (`cache`)

Use `cache.NewTypedCache[T]` with `GetOrFetch` to coalesce concurrent cache misses via singleflight across `PolicySampledLRU`, `PolicyLRU`, `PolicyLFU`, `PolicyFIFO`, or `PolicyTTL` eviction policies:

```go
memCache := cache.NewTypedCache[TenantConfig](
	5000,
	cache.WithEvictionPolicy(cache.PolicyLRU),
)

cfg, err := memCache.GetOrFetch(ctx, tenantID, 5*time.Minute, func(ctx context.Context) (TenantConfig, error) {
	return repo.FetchTenantConfig(ctx, tenantID)
})
```

## 3. Bounded Concurrency & State Machines (`workerpool`, `async`, `fsm`)

- Prohibit naked `go func()` in production paths. Use `workerpool.New(workers, queueSize)` with `Submit` / `SubmitContext` and `Stop()` for panic-isolated background execution.
- Use `async.Go`, `async.GoCtx`, `async.Parallel`, and `async.ParallelLimit` when fanning out bounded concurrent tasks inside a request.
- Use `fsm.NewMachine[S](fsm.Transition[S]{From: "PENDING", To: "CONFIRMED"})` and `m.CanTransition(from, to)` / `m.ValidateTransition(from, to)` for O(1) declarative state validation.

## 4. Deterministic Time & Graceful Shutdown (`clock` & `shutdown`)

- Inject `clock.Clock` (`clock.NewReal()` in production, `clock.NewFake(t0)` in unit tests) into caches, circuit breakers, and rate limiters for deterministic testing without `time.Sleep`.
- Trap `SIGINT`/`SIGTERM` with `shutdown.New(shutdown.WithTimeout(15 * time.Second))` and `Register` cleanup hooks (`srv.Shutdown`, `poolWorker.Stop`, `dbPool.Close`, `memCache.Stop`) followed by `mgr.Wait()`.
