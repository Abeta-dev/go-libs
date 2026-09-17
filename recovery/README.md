# `recovery` Package

The `recovery` package provides panic-trapping HTTP middleware with structured stack trace logging, preventing unhandled runtime panics from crashing the web server and isolating failures cleanly.

## When to Use
- **HTTP Server Perimeter Defense**: Intercepting unexpected runtime panics (e.g. nil pointer dereferences, slice index out of bounds) inside HTTP request handlers.
- **Structured Error Diagnostics**: Logging complete panic call stacks and request paths via `log/slog` for rapid incident post-mortems.
- **Client Failure Protection**: Returning a clean, uniform HTTP 500 JSON response without leaking internal source file paths or credentials to callers.

## Why It Is Written Like That
- **Structured JSON Logging with `slog`**: Uses standard library `log/slog` to format panic messages, call stacks, and request metadata into machine-readable JSON logs for log shippers (e.g. FluentBit, Datadog, Loki).
- **Configurable Logger Injection**: Supports `WithLogger(*slog.Logger)` to integrate with existing microservice logger configurations.
- **Zero Information Leakage**: Prevents internal stack traces from leaking to public API clients, adhering to OWASP security guidelines by returning generic 500 payloads.

## Alternatives Evaluated

| Alternative | Pros | Cons | Why We Chose `go-libs/recovery` |
|---|---|---|---|
| **Gin Default Recovery (`gin.Recovery()`)** | Built into Gin | Dumps unstructured ASCII text to stderr; cannot route to JSON log forwarders; risks leaking code lines to clients | Emits structured JSON logs via `log/slog` with clean, sanitized client responses |
| **No Panic Handling** | Exposes root cause immediately | Uncaught panics crash the entire container process, terminating all concurrent requests in flight | Traps panics per request, preserving process uptime for all other callers |
| **Manual `recover()` in Every Handler** | Localized recovery | Repetitive boilerplate across dozens of routes; easy to omit | Centralized middleware provides guaranteed panic trapping across the entire router |

---

## Quickstart

```go
import (
    "log/slog"
    "net/http"
    "os"

    "github.com/umesh0492/go-libs/recovery"
)

func main() {
    mux := http.NewServeMux()
    mux.HandleFunc("/api/v1/orders", orderHandler)

    // 1. Basic usage with default JSON logger
    handler := recovery.Middleware()(mux)

    // 2. Or configure with a custom slog instance
    customLogger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
    handler = recovery.Middleware(recovery.WithLogger(customLogger))(mux)

    _ = http.ListenAndServe(":8080", handler)
}
```

---

## 🛡️ Edge Cases Handled
- **Full Stack Preservation**: Captures `debug.Stack()` at the exact point of panic before goroutine unwinding finishes, ensuring the root-cause line number is preserved in logs.
- **Safe JSON Response Fallback**: If a handler panics halfway through writing a response, the middleware aborts cleanly with HTTP 500 and prevents invalid chunked-encoding socket errors.
