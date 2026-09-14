# `retry` Package

The `retry` package provides context-aware, zero-dependency retry execution with configurable backoff strategies and jitter for distributed resilience.

## When to Use
- **Outbound HTTP / RPC Calls**: Retrying transient network blips, 502/503/504 gateways, or connection timeouts.
- **Database Connection Reconnects**: Retrying operations during transient primary failovers or connection pool exhaustion.
- **Message Consumer Retries**: Re-attempting event processing before forwarding to a dead-letter queue.

## Why It Is Written Like That
- **Zero External Dependencies**: Pure Go standard library (`context`, `time`, `math/rand`, `errors`).
- **Thundering-Herd Prevention**: Includes `ExponentialJitter` (full jitter), preventing thousands of client instances from synchronizing their retry spikes against recovering upstream servers.
- **Strict Context Propagation**: Every retry loop evaluates `ctx.Err()` before executing attempts and honors context cancellation during sleep intervals.
- **Functional Generics**: `DoWithResult[T]` executes operations that return typed results (`(T, error)`), eliminating verbose manual type casting.

## Backoff Strategies

1. **`Constant`**: Fixed sleep interval between every attempt.
2. **`Linear`**: Wait increases linearly (`attempt * InitialWait`).
3. **`Exponential`**: Wait doubles each attempt (`InitialWait * 2^(attempt-1)`), capped at `MaxWait`.
4. **`ExponentialJitter`**: Exponential backoff with random full jitter (`rand(0, computedWait)`), recommended for distributed microservices.

## Alternatives Evaluated

| Alternative | Pros | Cons | Why We Chose `go-libs/retry` |
|---|---|---|---|
| **`cenkalti/backoff`** | Widely used, mature | Adds external dependency; lacks native generic `DoWithResult[T]` | Stdlib-only, zero dependencies, full generic type safety |
| **`avast/retry-go`** | Functional options API | External dependency, heavier allocation profile | Lightweight, explicit `Config` struct with full context awareness |
| **Manual for-loop sleep** | No library required | Frequently omits context cancellation, lacks jitter, prone to thundering-herd | Standardized, robust, race-free implementation |

## Quickstart

```go
package main

import (
    "context"
    "fmt"
    "net/http"
    "time"
    "github.com/umesh0492/go-libs/retry"
)

func FetchUser(ctx context.Context, userID string) (*User, error) {
    return retry.DoWithResult(ctx, retry.Config{
        Attempts:    3,
        InitialWait: 50 * time.Millisecond,
        MaxWait:     500 * time.Millisecond,
        Strategy:    retry.ExponentialJitter,
        ShouldRetry: func(err error) bool {
            // Only retry on temporary network errors or 5xx server errors
            return isTransient(err)
        },
    }, func(attemptCtx context.Context) (*User, error) {
        req, _ := http.NewRequestWithContext(attemptCtx, "GET", "https://api.internal/users/"+userID, nil)
        resp, err := http.DefaultClient.Do(req)
        if err != nil {
            return nil, err
        }
        defer resp.Body.Close()
        if resp.StatusCode >= 500 {
            return nil, fmt.Errorf("upstream error: %d", resp.StatusCode)
        }
        return parseUser(resp.Body)
    })
}
```

---

## 🛡️ Concurrency & Correctness Guarantees

- **`-race` Certified**: Purely stateless execution; concurrent calls across multiple goroutines operate independently with zero shared mutable state and 0 data races.
- **Immediate Context Termination**: Interruptible sleep via `time.NewTimer` ensures that if `ctx` is canceled or times out while sleeping, the function unblocks immediately without waiting for the sleep timer to expire.
- **Overflow & Exponent Clamping**: Attempt counter exponentiation is clamped at 30 to prevent `math.Pow` or `int64` duration arithmetic overflow into negative values.
- **Full Jitter Decorrelation**: `ExponentialJitter` draws from Go's concurrent-safe pseudo-random source, effectively decorrelating retries across distributed instances and preventing thundering-herd retry storms.

