# async

Package `async` provides panic-safe concurrency primitives, asynchronous goroutine runners, and bounded parallel task executors with context cancellation.

## Features

- **`Go` and `GoCtx`**: Fire-and-forget goroutines with automatic panic recovery and structured stack trace logging.
- **`Parallel`**: Unbounded fan-out concurrency with context cancellation, first-error capture, and panic isolation.
- **`ParallelLimit`**: Bounded semaphore concurrency control to safeguard database connection pools and external API rate limits during bulk execution.

## Usage

```go
import "github.com/umesh0492/go-libs/async"

// Fan-out concurrent queries
err := async.Parallel(ctx,
    func(c context.Context) error {
        return fetchUsers(c)
    },
    func(c context.Context) error {
        return fetchSettings(c)
    },
)

// Bounded worker batch execution
err := async.ParallelLimit(ctx, 5, tasks...)
```
