# `metrics` Package

The `metrics` package defines framework-agnostic instrumentation primitives (`Counter`, `Gauge`, `Histogram`) with zero-allocation default implementations, allowing all packages across `go-libs` to be thoroughly instrumented without coupling to any specific telemetry backend.

## When to Use
- **Instrumenting Reusable Packages**: Exposing queue depth, error rates, cache hit ratios, and state transitions from shared libraries.
- **Microservice Telemetry Integration**: Bridging standard metrics into Prometheus, StatsD, OpenTelemetry, or custom metrics backends.
- **Unit & Performance Testing**: Verifying operational counters and gauges in memory without starting metrics scrapers.

## Why It Is Written Like That
- **Dependency Inversion**: Shared libraries must not dictate telemetry vendor choices. Hardcoding Prometheus or Datadog packages forces heavyweight transitive dependencies onto all consumers.
- **Zero-Allocation No-Ops**: Provides `NoopCounter`, `NoopGauge`, and `NoopHistogram`. In the default unconfigured state, calls compile down to zero runtime heap allocations.
- **Standard Observability Interfaces**:
  - `Counter`: Monotonically increasing metric (`Inc()`, `Add(float64)`).
  - `Gauge`: Variable metric that goes up or down (`Set(float64)`, `Add(float64)`).
  - `Histogram`: Distribution observation (`Observe(float64)`).

## Alternatives Evaluated

| Alternative | Pros | Cons | Why We Chose `go-libs/metrics` |
|---|---|---|---|
| **Direct Prometheus Client (`client_golang`)** | Industry standard, rich collector features | Pulls heavy third-party dependency tree into every microservice | Decoupled interfaces allow optional Prometheus binding without vendor lock-in |
| **OpenTelemetry Metrics SDK** | CNCF standard, unified logs/traces/metrics | Verbose API, steep learning curve, large binary size | Simple 3-interface contract (`Counter`, `Gauge`, `Histogram`) satisfies 99% of microservice needs |
| **No Metrics in Shared Libraries** | Zero boilerplate | Black-box components; debugging connection pool or circuit breaker trips requires log scraping | Built-in instrumentation hooks provide full runtime visibility |

## Quickstart

```go
package main

import (
    "time"
    "github.com/umesh0492/go-libs/circuitbreaker"
    "github.com/umesh0492/go-libs/metrics"
)

// Custom Prometheus counter adapter
type promCounter struct{ /* prometheus.Counter */ }
func (p *promCounter) Inc()          { /* p.c.Inc() */ }
func (p *promCounter) Add(v float64) { /* p.c.Add(v) */ }

func main() {
    // Attach metrics collectors via WithMetrics option
    cb := circuitbreaker.NewConsecutiveBreaker(5, 30*time.Second,
        circuitbreaker.WithMetrics(circuitbreaker.Metrics{
            Requests: &promCounter{},
            Failures: &promCounter{},
            State:    metrics.NoopGauge{}, // discard state or wire to gauge
        }),
    )
    _ = cb
}
```
