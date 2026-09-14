# `telemetry` Package & `ginmw.Telemetry`

The `telemetry` package provides OpenTelemetry (OTel) instrumentation for distributed tracing across microservices.

## When to Use
- **Always** in microservice HTTP routers to propagate distributed trace contexts across service boundaries.
- When tracing downstream database queries, external API calls, or long-running async background operations.

## Why It Is Written Like That
- **Open W3C Standard**: Adheres strictly to `go.opentelemetry.io/otel` and standard W3C TraceContext propagation. You can route spans to Jaeger, Tempo, or any OpenTelemetry-compatible collector without altering application code.
- **Header Extraction**: `ginmw.Telemetry` automatically extracts incoming `traceparent` and `baggage` headers from HTTP requests, establishing seamless parent-child span relationships across distributed architectures.
- **Zero Overhead in Tests**: Integrates effortlessly with `go.opentelemetry.io/otel/trace/noop` so unit tests execute at full speed without mock OTel collectors.

## Alternatives Evaluated

| Alternative | Pros | Cons | Why We Chose `go-libs/telemetry` |
|---|---|---|---|
| **Proprietary Monitoring SDKs** | Deep provider-specific features | Proprietary lock-in; code changes required if monitoring provider changes | OTel standard gives complete platform independence |
| **Log-based Correlation ID alone (`requestid`)** | Simple, lightweight | Cannot measure span timings, parent-child latency breakdowns, or distributed spans | `telemetry` works in tandem with `requestid`: RequestID for log queries, Telemetry for span graphs |
| **Manual Span Creation in every handler** | Fine control | Repetitive boilerplate, easy to forget context extraction | `ginmw.Telemetry` automatically creates root request spans for all routes |

## Quickstart

### 1. Initialize Global Tracer Provider (`main.go`)
```go
import (
    "github.com/umesh0492/go-libs/telemetry"
    "go.opentelemetry.io/otel/trace/noop"
)

func main() {
    // In production, pass your OTLP exporter TracerProvider
    tp := noop.NewTracerProvider() 
    telemetry.InitProvider(tp)
}
```

### 2. Attach Middleware to Router (`router.go`)
```go
import (
    "github.com/umesh0492/go-libs/ginmw"
)

r := gin.New()
r.Use(ginmw.Telemetry("order-service"))
```

### 3. Trace Downstream Custom Spans
```go
import "github.com/umesh0492/go-libs/telemetry"

func processOrder(ctx context.Context, orderID string) {
    ctx, span := telemetry.StartSpan(ctx, "order-service", "calculate_tax")
    defer span.End()

    // Business logic...
}
```

### 4. Instance-Scoped Tracer Provider (No Global State Mutation)
```go
tp, cleanup, err := telemetry.NewTracerProvider(
    telemetry.WithServiceName("order-service"),
    telemetry.WithEnvironment("production"),
    telemetry.WithSampleRate(1.0),
)
if err != nil {
    log.Fatal(err)
}
defer cleanup(context.Background())

tracer := tp.Tracer("order-service")
_ = tracer
```

