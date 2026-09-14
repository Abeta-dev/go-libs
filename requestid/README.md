# `requestid` Package

The `requestid` package provides HTTP middleware for request correlation tracing, generating or propagating `X-Request-ID` headers and binding them to standard `context.Context` across microservice topologies.

## When to Use
- **End-to-End Request Tracing**: Correlating log records, APM spans, and database queries belonging to a single client request across multiple microservices.
- **Upstream Gateway Integration**: Preserving correlation IDs injected by ingress controllers, Cloudflare, AWS ALB, or Envoy proxies.
- **Client Debugging**: Returning the `X-Request-ID` in HTTP response headers so frontend applications and API clients can quote incident IDs when reporting bugs.

## Why It Is Written Like That
- **Preserve-or-Generate Semantics**: Checks incoming `X-Request-ID` first; if present, validates and reuses it; if missing or empty, generates a cryptographically secure UUIDv4.
- **Dual Layer Binding**: Attaches the request ID to both the outgoing HTTP response header (`w.Header().Set("X-Request-ID", id)`) and the Go context (`context.Context`), enabling downstream services to log it without importing Gin.
- **Zero Allocations for Fast-Path Extraction**: Context extraction uses dedicated unexported context keys, avoiding map lookups or interface casting overhead.

## Alternatives Evaluated

| Alternative | Pros | Cons | Why We Chose `go-libs/requestid` |
|---|---|---|---|
| **No Request ID Tracing** | Zero setup overhead | Nearly impossible to diagnose production bugs across microservice log streams | Enables instant log grep and distributed request correlation |
| **Gin Contrib Request ID** | Pre-built Gin plugin | External dependency; does not bind to standard library `context.Context` for repository/service logging | Lightweight, zero-dependency, and propagates cleanly into standard `context.Context` |
| **Relying Solely on OpenTelemetry Trace ID** | Part of W3C traceparent | Trace IDs are 32-hex characters; not user-friendly for support tickets or client headers | UUIDv4 format is readable, standard in HTTP APIs, and complements distributed tracing |

---

## Quickstart

```go
import (
    "github.com/gin-gonic/gin"
    "github.com/umesh0492/go-libs/requestid"
)

func main() {
    r := gin.New()

    // 1. Mount middleware at ingress perimeter
    r.Use(requestid.Middleware())

    // 2. Extract in handlers or service layers
    r.GET("/api/v1/orders", func(c *gin.Context) {
        reqID := requestid.FromContext(c.Request.Context())
        // Log with request ID:
        logger.Info("fetching orders", "request_id", reqID)
    })
}
```

---

## 🛡️ Edge Cases Handled
- **Gateway Spanning**: If an upstream API Gateway generates and injects an `X-Request-ID`, the middleware preserves and adopts it rather than replacing it, maintaining end-to-end traceability across ingress boundaries.
- **Context Propagation**: Safely attaches the ID to `c.Request.Context()` so non-Gin service layers can access it without coupling to the HTTP transport layer.
