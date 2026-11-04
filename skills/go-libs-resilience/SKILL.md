---
name: go-libs-resilience
description: Resilience, concurrency, caching, and lifecycle patterns using github.com/Abeta-dev/go-libs (retry, circuitbreaker, ratelimit, workerpool, cache, async, shutdown, fsm, clock, metrics). Use when hardening external calls, background workers, caches, or state machines.
---

# `go-libs-resilience` Subskill (`v0.3.3`)

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

Wrap outbound calls with `circuitbreaker.NewConsecutiveBreaker` (or `NewRatioBreaker`) and `retry.Do` / `retry.DoWithResult` with `retry.ExponentialJitter`:

```go
breaker := circuitbreaker.NewConsecutiveBreaker(5, 30*time.Second)

err := retry.Do(ctx, func(ctx context.Context) error {
	return breaker.Execute(ctx, func() error {
		return client.SendPayout(ctx, req)
	})
}, retry.WithBackoff(retry.ExponentialJitter(100*time.Millisecond, 2*time.Second)), retry.WithMaxAttempts(3))
```

## 2. Stampede-Safe Caching (`cache`)

Use `cache.NewTypedCache[T]` with `GetOrFetch` to coalesce concurrent cache misses via singleflight across SampledLRU, LRU, LFU, FIFO, or TTL eviction policies:

```go
memCache := cache.NewTypedCache[TenantConfig](
	cache.WithCapacity(5000),
	cache.WithEvictionPolicy(cache.EvictionLRU),
	cache.WithTTL(5 * time.Minute),
)

cfg, err := memCache.GetOrFetch(ctx, tenantID, func(ctx context.Context) (TenantConfig, error) {
	return repo.FetchTenantConfig(ctx, tenantID)
})
```

## 3. Bounded Concurrency (`workerpool` & `async`)

- Prohibit naked `go func()` in production paths. Use `workerpool.New(workers, queueSize)` with `Submit` / `SubmitContext` and `Stop()` for panic-isolated background execution.
- Use `async.Go`, `async.WaitAll`, and `async.WithTimeout` when fanning out bounded concurrent tasks inside a request.

## 4. Deterministic Time & Graceful Shutdown (`clock` & `shutdown`)

- Inject `clock.Clock` (`clock.NewReal()` in production, `clock.NewFake(t0)` in unit tests) into caches, circuit breakers, and rate limiters for deterministic testing without `time.Sleep`.
- Trap `SIGINT`/`SIGTERM` with `shutdown.New(15 * time.Second)` and `Register` cleanup hooks (`srv.Shutdown`, `poolWorker.Stop`, `dbPool.Close`, `memCache.Close`) followed by `mgr.Wait()`.
