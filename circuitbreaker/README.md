# `circuitbreaker` Package

The `circuitbreaker` package provides an outbound client resiliency state machine with **two industry-standard algorithms** to prevent cascading failures across microservices, with strictly zero external dependencies (pure Go standard library).

## When to Use
- **Outbound HTTP / RPC Calls**: Protecting microservices from hanging or exhausting connection pools when third-party or downstream microservices suffer degraded latency or outages.
- **Cascading Failure Protection**: Halting calls immediately with zero network I/O (`ErrCircuitOpen`) once downstream error thresholds are reached.
- **Fail-Fast Fallback Execution**: Instantly returning cached data, degraded defaults, or offline queued responses when dependent subsystems are down.
- **Controlled Canary Probing**: Permitting single canary probe requests during `StateHalfOpen` to safely test downstream recovery before reopening full traffic.

## Why It Is Written Like That
- **Dual Industry-Standard Algorithms**: Supports both **Consecutive Failures** (critical for hard single-point dependencies) and **Rolling Window Failure Ratio** (standard for high-throughput distributed microservices), unified under a single `Breaker` interface.
- **Strictly Zero External Dependencies**: Built purely on Go standard library primitives (`sync.Mutex`, `time.Time`, `context.Context`) with sub-150ns steady-state execution overhead and zero heap allocations for closed-state checks.
- **Pluggable Observability Hooks**: Exposes state change callbacks (`WithOnStateChange`) and Prometheus/OpenTelemetry metric hooks (`WithMetrics`) to feed Golden Signals dashboards directly without coupling to any specific telemetry provider.
- **Context Awareness**: `Execute(ctx, fn)` respects caller deadlines and cancellations, ensuring blocked half-open probes release instantly if upstream clients disconnect.

## Alternatives Evaluated

| Alternative | Pros | Cons | Why We Chose `go-libs/circuitbreaker` |
|---|---|---|---|
| **`sony/gobreaker`** | Popular Go circuit breaker library | Single failure count model; lacks failure-ratio sliding window; no zero-allocation fast path | Provides unified `Breaker` interface with both Consecutive and Ratio algorithms, zero external dependencies, and APM metric hooks |
| **Envoy / Istio Service Mesh Circuit Breaking** | Infrastructure-level; language-agnostic | Heavy operational footprint; lacks application-level fallback control; cannot intercept database or non-HTTP drivers | In-process execution enables sub-microsecond fail-fast fallbacks directly in Go business logic |
| **Raw Timeouts & Retries Only** | Standard `http.Client` timeout | Retries against struggling downstreams cause retry storms and exacerbate downstream outages | Circuit breaker halts traffic completely, allowing downstreams time to recover without retry storms |

---

## 🎯 2 Industry-Standard Algorithms

Both algorithms implement the unified `Breaker` interface:
```go
type Breaker interface {
    Execute(ctx context.Context, req func() error) error
    State() State
}
```

| Algorithm | Constructor | Best For | Tripping Condition | Steady Latency |
|:---|:---|:---|:---|:---|
| **Consecutive Failures** | `New(maxFailures, resetTimeout)` | Hard dependencies, payment gateways, critical single-point external APIs | $N$ consecutive errors without any intervening success | **48.7 ns/op** (0 allocs) |
| **Failure Ratio / Error Rate** | `NewRatioBreaker(failureRatio, minRequests, window, resetTimeout)` | High-throughput distributed services (Hystrix / Resilience4j / Envoy standard) | $\frac{\text{failures}}{\text{total}} \ge \text{ratio}$ within rolling time window (min requests met) | **131.6 ns/op** |

---

## 🚀 Lifecycle States

- **`StateClosed` (Normal)**: Outbound requests proceed normally. Failures are tracked.
- **`StateOpen` (Tripped)**: All calls fail immediately with `ErrCircuitOpen` with **0 network I/O** and **0 latency delay**, protecting both your service and the struggling downstream dependency.
- **`StateHalfOpen` (Canary Probe)**: After `resetTimeout`, a single test probe request is permitted through:
  - If the probe succeeds: resets to `StateClosed`.
  - If the probe fails: re-trips back to `StateOpen`.

---

## 💻 Quickstart

### 1. Failure Ratio Breaker (Recommended for Microservices)
```go
import (
    "context"
    "errors"
    "time"
    "github.com/umesh0492/go-libs/circuitbreaker"
)

// Trip if error rate >= 50% over a 10s window (min 10 requests), cooling down for 30s
var breaker = circuitbreaker.NewRatioBreaker(0.50, 10, 10*time.Second, 30*time.Second)

func CallInventoryService(ctx context.Context) (*Inventory, error) {
    var inv *Inventory
    err := breaker.Execute(ctx, func() error {
        var callErr error
        inv, callErr = httpGetInventory(ctx)
        return callErr
    })

    if errors.Is(err, circuitbreaker.ErrCircuitOpen) {
        // Circuit open fallback: return cached inventory or degraded response
        return getCachedInventory(), nil
    }
    return inv, err
}
```

### 2. Consecutive Failures Breaker
```go
// Allow max 3 consecutive failures before opening circuit for 30 seconds
var cb = circuitbreaker.NewConsecutiveBreaker(3, 30*time.Second)

func CallPaymentGateway(ctx context.Context, payload PaymentReq) (*PaymentResp, error) {
    var resp *PaymentResp
    err := cb.Execute(ctx, func() error {
        var callErr error
        resp, callErr = httpPostPayment(ctx, payload)
        return callErr
    })
    return resp, err
}
```

### 3. Observability & State Change Callbacks
```go
cb := circuitbreaker.NewRatioBreaker(0.5, 20, time.Minute, 15*time.Second,
    circuitbreaker.WithOnStateChange(func(from, to circuitbreaker.State) {
        log.Printf("Circuit transitioned from %s to %s", from, to)
    }),
    circuitbreaker.WithMetrics(circuitbreaker.Metrics{
        Requests: reqCounter,
        Failures: failCounter,
        State:    stateGauge,
    }),
)
```

---

## 🛡️ Concurrency & Correctness Guarantees

- **`-race` Certified**: Stress-tested under 100+ concurrent goroutines with 0 data races.
- **Herd Stampede Prevention**: When entering `StateHalfOpen`, exactly **1 canary probe** is permitted in-flight; all concurrent requests are immediately rejected with `ErrCircuitOpen` until the probe completes.
- **Panic Safety**: If downstream business logic in `req()` panics, internal defer logic catches the panic, unblocks the half-open probe lock, transitions the breaker back to `StateOpen`, and re-panics. The circuit breaker is never poisoned or frozen in `StateHalfOpen`.
- **Epoch & Stale Request Isolation**: Slow in-flight requests that began when the circuit was `StateClosed` cannot corrupt, reset, or prematurely close `StateHalfOpen` probes when they finally complete.
- **O(1) Rolling Ratio Evaluation**: Ratio evaluation runs in sub-microsecond $O(1)$ time via running failure count tracking rather than linear slice iteration.

