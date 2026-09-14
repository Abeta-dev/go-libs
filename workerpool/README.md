# `workerpool` Package

The `workerpool` package provides a bounded, panic-safe worker pool for concurrent background task execution with strictly zero external dependencies (pure Go standard library).

> **Production Rule**: Unbounded goroutine creation (`go func()`) under burst traffic causes memory exhaustion and runtime crashes. A fixed-size worker pool bounds CPU and memory consumption while safely buffering task bursts.

## When to Use
- **Heavy Background Processing**: Processing bulk invoice generation, batch database updates, or data exports concurrently.
- **Resource Protection & Rate Limiting**: Limiting the maximum number of concurrent tasks accessing a downstream bottleneck (e.g. max 10 concurrent calls).
- **Graceful Worker Teardown**: Waiting for in-flight tasks to finish before pod shutdown without dropping jobs.

## Why It Is Written Like That
- **Bounded Concurrency & Queueing**: Uses a buffered channel queue with a fixed number of long-running worker goroutines, preventing runaway thread creation.
- **Worker Panic Isolation**: Each worker wraps task execution in a panic recovery perimeter; panics are caught and logged via `log/slog` without terminating the worker goroutine or crashing the host process.
- **Context-Aware Enqueuing**: `SubmitContext(ctx, task)` honors client timeouts and cancellations, rejecting submissions if the queue remains full past the deadline.
- **Graceful Teardown Modes**: Supports both `Stop()` (halts new admissions while draining) and `StopWait()` (blocks until all in-flight jobs finish).

## Alternatives Evaluated

| Alternative | Pros | Cons | Why We Chose `go-libs/workerpool` |
|---|---|---|---|
| **Unbounded Goroutines (`go func()`)** | Zero setup code | An unhandled panic crashes the entire process; high burst traffic exhausts container memory | Bounded memory footprint and guaranteed panic isolation per worker |
| **`panjf2000/ants`** | High-performance goroutine pool | Complex dependency; recycling goroutines introduces subtle state leaks; heavy API surface | Zero-dependency implementation using idiomatic Go channels and `sync.WaitGroup` |
| **External Distributed Queues (RabbitMQ/Kafka)** | Distributed across multiple pods | Heavy infrastructure overhead; network latency; overkill for in-process async parallelism | Ultra-low latency, in-memory concurrency bounding for intra-service workloads |

---

## Key Features
- **Bounded Concurrency**: Limits concurrent goroutines to a fixed, configurable worker count.
- **Panic Isolation**: Each worker executes inside a panic-recovery perimeter; any panic is caught and logged via `log/slog` without taking down the worker or the application.
- **Graceful Teardown**: `Stop()` prevents new task admission while draining queues, and `StopWait()` blocks until all in-flight jobs complete.
- **Context-Aware Enqueuing**: `SubmitContext(ctx, task)` allows callers to enforce enqueue deadlines or honor HTTP request cancellation.

---

## Usage

### Basic Initialization
```go
import "github.com/umesh0492/go-libs/workerpool"

// Initialize pool with 8 workers and a queue capacity of 200
pool := workerpool.New(8, 200)

// Enqueue tasks
err := pool.Submit(func() {
    processReport()
})
if err != nil {
    // Queue is full (workerpool.ErrQueueFull) or pool has been stopped (workerpool.ErrPoolClosed)
}
```

### Context-Aware Submission
```go
ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
defer cancel()

err := pool.SubmitContext(ctx, func() {
    exportLargeDataset()
})
if err != nil {
    // Context deadline exceeded or pool closed
}
```

### Graceful Shutdown with `shutdown.Manager`
```go
mgr := shutdown.New(10 * time.Second)
mgr.Register("workerpool", func(ctx context.Context) error {
    pool.StopWait()
    return nil
})
```

---

## 🛡️ Concurrency & Correctness Guarantees

- **`-race` Certified**: Stress-tested under 50+ concurrent goroutines submitting while concurrently calling `StopWait()` with 0 data races.
- **Panic Boundary Resilience**: If a task panics, the worker catches the panic, increments metrics, and logs the stack trace. Even if a custom `panicHandler` itself panics, internal recovery keeps the worker goroutine alive.
- **Zero-Allocation Backpressure**: `SubmitContext` fast-path evaluates available slots non-blockingly without any timer allocations, and uses a single reusable timer under contention, avoiding runtime heap thrashing.
- **Safe Teardown Guarantee**: Calling `Stop()` or `StopWait()` synchronizes safely with in-flight `Submit` and `SubmitContext` calls. Zero risk of panicking on "send on closed channel".

