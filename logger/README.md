# `logger` Package

The `logger` package provides context-aware structured logging wrappers around the standard library's `log/slog` package (`Go 1.21+`).

## When to Use
- **HTTP Handlers**: Propagating request IDs, tenant IDs, and user IDs through handler context trees.
- **Service Layers**: Extracting context-scoped loggers without passing explicit logger parameters to every business method.
- **Microservice Logging**: Standardizing JSON logging output and log level filtering via `LOG_LEVEL` environment configuration.

## Why It Is Written Like That
- **Native Standard Library Foundation**: Built directly on `log/slog`, eliminating heavyweight third-party logger dependencies (`uber-go/zap`, `rs/zerolog`).
- **Context Injection**:
  - `WithContext(ctx, logger)` binds an active logger to a request context.
  - `FromContext(ctx)` retrieves the context-scoped logger, falling back cleanly to `Default()` if unset.
  - `WithField(ctx, key, val)` and `WithFields(ctx, ...)` attach contextual key-value pairs and return an updated context.
- **Environment Driven**: `LOG_LEVEL` (`DEBUG`, `INFO`, `WARN`, `ERROR`) configures output verbosity automatically upon initialization.

## Alternatives Evaluated

| Alternative | Pros | Cons | Why We Chose `go-libs/logger` |
|---|---|---|---|
| **`uber-go/zap`** | Blazing fast allocation benchmarks | External dependency, complex custom interface contracts | Stdlib `log/slog` delivers competitive speed with native Go runtime support |
| **`rs/zerolog`** | Zero-allocation JSON logger | Non-standard API idioms; couples code to 3rd party formatters | `slog` is the official Go standard for modern structured logging |
| **Standard `log` package** | Built into Go | Unstructured plaintext; impossible to query in modern APM tools (Datadog/Grafana) | JSON-structured output compatible with cloud log ingestion pipelines |

## Quickstart

```go
package main

import (
    "context"
    "net/http"
    "github.com/gin-gonic/gin"
    "github.com/umesh0492/go-libs/logger"
)

func OrderHandler(c *gin.Context) {
    ctx := c.Request.Context()

    // Attach request metadata to context logger
    ctx = logger.WithField(ctx, "order_id", "ord-12345")
    ctx = logger.WithField(ctx, "user_id", "usr-889")

    processOrder(ctx)

    c.Status(http.StatusOK)
}

func processOrder(ctx context.Context) {
    // Retrieve logger containing all previously attached fields
    log := logger.FromContext(ctx)
    log.Info("processing order payment") // Emits JSON with order_id and user_id fields
}
```
