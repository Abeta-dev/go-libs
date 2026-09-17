# System Architecture & Technical Blueprint

This document defines the architectural topology, layering boundaries, middleware pipeline ordering, and systemic engineering contracts for `github.com/umesh0492/go-libs`.

---

## 1. Core Architectural Philosophy

`go-libs` is engineered as a unified, zero-business-logic foundation monorepo for distributed backend microservices.

### The Strict Boundary Rule
> **Golden Rule**: If a package has *any* awareness of specific business domain concepts (such as customer entities, order schemas, or payment workflow logic), it **DOES NOT** belong in `go-libs`.

- **`go-libs` provides mechanisms**: How services authenticate, rate limit, recover from panics, trace requests, connect to databases, and enforce HTTP security.
- **Microservices provide policies**: What business logic, entity schemas, and user workflows exist.

### Key Engineering Tenets
1. **Ports & Adapters (Hexagonal Independence)**: Interfaces define behavioral contracts (`metrics.Counter`, `db.DBTX`, `idempotency.Store`). In-memory implementations operate out-of-the-box with zero external dependencies, while cloud drivers (Redis, AWS, Prometheus) adapt to these contracts in application space.
2. **Zero Unbounded Goroutines**: Naked `go func()` invocations are prohibited. High-throughput concurrency must execute through bounded worker pools (`workerpool.Pool`).
3. **Reflection-Free Lifecycle**: Startup and teardown operate without runtime reflection, ensuring fast binary startup and compile-time type safety.
4. **Defense-in-Depth Pipeline**: Inbound HTTP requests traverse mandatory security barriers (body size caps, IP rate limits, OWASP headers, dynamic RBAC) before hitting application route handlers.

---

## 2. Four-Tier Dependency Hierarchy

Circular dependencies between packages are strictly forbidden. Imports must strictly flow downward.

```
Layer 4: Transport Adapters
         ├── ginmw (Gin-specific middleware pipeline & token utilities)
         └── examples/microservice (reference microservice composition)
              │
              ▼
Layer 3: Cross-Cutting Middleware & HTTP Utilities
         ├── idempotency, pagination, rbac, rbaccontext
         ├── health, recovery, securityheaders, ratelimit, requestid, httputil
         ├── httpclient (resilient client composing rate limit, circuit breaker, retry)
         └── db, validation, telemetry
              │
              ▼
Layer 2: Core Infrastructure Primitives
         ├── cache (generic singleflight stampede protection & eviction policies)
         ├── circuitbreaker (outbound 3-state failure machine)
         ├── retry (context-aware backoff algorithms)
         ├── workerpool (bounded concurrency execution)
         └── shutdown (graceful teardown manager)
              │
              ▼
Layer 1: Low-Level Foundation & Primitives (Zero internal/external dependencies)
         ├── clock (Clock, RealClock, FakeClock mockable time abstraction)
         ├── metrics (Counter, Gauge, Histogram interfaces & noop fallbacks)
         ├── apperror (canonical application error codes)
         ├── sliceutil, maputil, stringutil, timeutil, cryptoutil
         ├── env (typed OS environment parsing)
         └── logger (contextual slog wrapper with sampling and redaction)
```

### Layer Rules
1. **Layer 1** packages must not import any packages from Layer 2, 3, or 4.
2. **Layer 2** packages may only import Layer 1 packages.
3. **Layer 3** packages may import Layer 1 and Layer 2 packages.
4. **Layer 4** packages (`ginmw`) may compose Layer 1, 2, and 3 packages.
5. **Zero Domain Imports**: No package may import domain code from calling services.

---

## 3. Middleware Execution Topology (Pipeline Order)

When assembling an HTTP router, middleware registration order is critical for security, resource bounding, and observability:

```
requestid ➔ bodylimit ➔ cors ➔ recovery ➔ logger ➔ telemetry ➔ ratelimit ➔ auth ➔ rbac ➔ idempotency
```

### Stage Responsibilities
1. **`requestid` (`ginmw.RequestID()` / `requestid.Middleware()` )**:
   Injects or extracts `X-Request-ID`. Guaranteed to exist for all downstream logs and spans.
2. **`bodylimit` (`ginmw.LimitBodyDefault()` / `bodylimit.New()` )**:
   Wraps `http.MaxBytesReader` to terminate oversized requests (2MB default, 4KB auth) before buffering.
3. **`cors` (`ginmw.CORS(...)` )**:
   Handles HTTP `OPTIONS` preflight immediately without expending database or auth cycles.
4. **`recovery` (`recovery.Middleware(...)` )**:
   Catches panics, logs formatted stack traces via `logger`, and returns safe JSON 500 envelopes.
5. **`logger` (`ginmw.Logger()` )**:
   Records inbound request start, completion latency, status codes, and context attributes.
6. **`telemetry` (`ginmw.Telemetry(...)` )**:
   Initiates root OpenTelemetry span and propagates W3C trace contexts.
7. **`ratelimit` (`ginmw.GlobalRateLimit()` / `ginmw.AuthRateLimit()` )**:
   Evaluates IP-based TokenBucket or SlidingWindow thresholds to drop volumetric bursts.
8. **`auth` (`ginmw.AuthMiddleware()` )**:
   Validates Bearer JWTs and binds authenticated `*Claims` to the request context.
9. **`rbac` (`ginmw.RBAC(provider)` )**:
   Fetches dynamic live permissions and binds them for handler-level `rbaccontext.Can` checks.
10. **`idempotency` (`ginmw.Idempotency(store)` )**:
    Enforces atomic two-phase request locking on mutating endpoints to eliminate duplicate side effects.

---

## 4. Master Module Directory (31 Packages)

| Package | Layer | Primary Responsibility | Key Types / Constructors |
|---|:---:|---|---|
| `apperror` | 1 | Canonical error codes and structured error wrapping | `Error`, `Code`, `New`, `Wrap` |
| `bodylimit` | 3 | HTTP request body size bounding | `New`, `MB`, `KB` |
| `cache` | 2 | Typed cache with singleflight and eviction policies | `TypedCache`, `NewTypedCache`, `WithEvictionPolicy` |
| `circuitbreaker` | 2 | 3-state failure machine for outbound dependencies | `ConsecutiveBreaker`, `RatioBreaker`, `NewConsecutiveBreaker` |
| `clock` | 1 | Mockable time abstraction with RealClock and advanceable FakeClock | `Clock`, `RealClock`, `FakeClock`, `NewReal`, `NewFake` |
| `cryptoutil` | 1 | Bcrypt hashing (cost 12) and CSPRNG string generation | `HashPassword`, `GenerateTempPassword` |
| `db` | 3 | PostgreSQL pgxpool lifecycle and DBTX querier abstraction | `Connect`, `DBTX`, `WithMaxConns` |
| `env` | 1 | Zero-dependency typed environment variable retrieval | `String`, `Int`, `Bool`, `MustString` |
| `ginmw` | 4 | Complete Gin-native middleware pipeline and token utilities | `RequestID`, `Telemetry`, `AuthMiddleware`, `RBAC` |
| `health` | 3 | Kubernetes-compliant liveness and readiness probes | `Handler`, `New`, `WithChecker` |
| `httpclient` | 3 | Resilient composed HTTP client pipeline with circuit breaker, rate limit, and retry | `New`, `NewRoundTripper`, `WithRetry`, `WithCircuitBreaker` |
| `httputil` | 3 | Standard JSON response envelopes and domain error mapping | `OK`, `Created`, `ErrorFromDomain` |
| `idempotency` | 3 | Two-phase atomic request locking and response replay | `Store`, `MemoryStore`, `Middleware` |
| `logger` | 1 | Context-aware structured JSON logging via `log/slog` | `Default`, `WithContext`, `FromContext` |
| `maputil` | 1 | Generic type-safe map transformations | `Keys`, `Values`, `Merge`, `Filter` |
| `metrics` | 1 | Framework-agnostic instrumentation interfaces | `Counter`, `Gauge`, `Histogram` |
| `pagination` | 3 | Clamped offset pagination and SQL calculations | `Parse`, `Offset`, `Response` |
| `ratelimit` | 3 | In-memory IP rate limiting (TokenBucket, SlidingWindow) | `NewTokenBucket`, `NewSlidingWindow` |
| `rbac` | 3 | Multi-segment wildcard permission evaluator | `Match`, `HasPermission` |
| `rbaccontext` | 3 | Context-bound permission attachment and verification | `WithPermissions`, `Can`, `CanAll` |
| `recovery` | 3 | Panic recovery middleware with structured logging | `Middleware`, `WithLogger` |
| `requestid` | 3 | Correlation ID extraction and context propagation | `Middleware`, `FromContext` |
| `retry` | 2 | Context-aware retry execution with backoff algorithms | `Do`, `Constant`, `ExponentialJitter` |
| `securityheaders` | 3 | OWASP defensive security headers middleware | `New`, `Default` |
| `shutdown` | 2 | Coordinated graceful OS signal teardown manager | `Manager`, `New`, `Register`, `Wait` |
| `sliceutil` | 1 | Generic functional slice transformations | `Map`, `Filter`, `Reduce`, `GroupBy` |
| `stringutil` | 1 | Sensitive data masking and secure random generators | `MaskEmail`, `MaskPhone`, `RandomAlphanumeric`, `RandomSecureString` |
| `telemetry` | 3 | OpenTelemetry provider initialization and span helpers | `InitProvider`, `NewTracerProvider`, `StartSpan` |
| `timeutil` | 1 | UTC normalization and business day calculations | `NowIn`, `FormatIn`, `AddBusinessDays`, `StartOfDay`, `EndOfDay` |
| `validation` | 3 | Format `validator/v10` errors into clean client JSON | `FormatErrors`, `FieldError` |
| `workerpool` | 2 | Bounded, panic-safe background worker pool | `Pool`, `New`, `Submit`, `SubmitContext` |

---

## 5. Core Architectural Contracts

### Server-Driven Security
Client tokens (JWTs) carry identity claims (`UserID`, `Role`), but permissions are dynamically resolved from the database on every request via `ginmw.RBAC(PermissionProvider)`. This guarantees instant revocation upon role changes without waiting for token expiry.

### Resilient Outbound Calls
Outbound network interactions should be wrapped with `circuitbreaker.ConsecutiveBreaker` and `retry.Do` using `retry.ExponentialJitter`. The circuit trips after repeated failures, immediately failing fast and preserving system thread pools.

### Database Connection Management
Persistence operations must utilize thread-pooled instances via `db.Connect` (backed by `pgxpool`). Standard library `database/sql` globals are prohibited to prevent connection leakage. Repositories should accept `db.DBTX` to operate transparently across standalone connections and transactions.

### Graceful Teardown
Microservice `main.go` processes must trap OS signals (`SIGINT`, `SIGTERM`) through `shutdown.New(timeout)`, executing cleanups (HTTP server shutdown, workerpool draining, database pool closing) concurrently within a bounded timeout budget.

---

## 6. Reference Microservice Composition Blueprint

Below is an end-to-end blueprint demonstrating how `go-libs` components assemble in a production service:

```go
package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/umesh0492/go-libs/cache"
	"github.com/umesh0492/go-libs/circuitbreaker"
	"github.com/umesh0492/go-libs/db"
	"github.com/umesh0492/go-libs/env"
	"github.com/umesh0492/go-libs/ginmw"
	"github.com/umesh0492/go-libs/health"
	"github.com/umesh0492/go-libs/httputil"
	"github.com/umesh0492/go-libs/idempotency"
	"github.com/umesh0492/go-libs/logger"
	"github.com/umesh0492/go-libs/recovery"
	"github.com/umesh0492/go-libs/shutdown"
	"github.com/umesh0492/go-libs/telemetry"
	"github.com/umesh0492/go-libs/workerpool"
)

func main() {
	log := logger.Default()
	port := env.String("PORT", "8080")

	// 1. Telemetry Provider (Instance-scoped or Global)
	tp, tpCleanup, _ := telemetry.NewTracerProvider(
		telemetry.WithServiceName("order-service"),
	)
	defer tpCleanup(context.Background())

	// 2. Database Connection Pool
	ctx := context.Background()
	pool, err := db.Connect(ctx, env.String("DATABASE_URL", "postgres://localhost:5432/orders"), db.WithMaxConns(25))
	if err != nil {
		log.Error("database connection failed", slog.Any("error", err))
		os.Exit(1)
	}

	// 3. Resiliency Components
	memCache := cache.NewTypedCache[string](cache.WithCapacity(5000), cache.WithEvictionPolicy(cache.EvictionLRU))
	idemStore := idempotency.NewMemoryStore()
	breaker := circuitbreaker.NewConsecutiveBreaker(5, 30*time.Second)
	poolWorker := workerpool.New(8, 200)

	// 4. HTTP Router Setup
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()

	// Pipeline adhering to execution topology
	r.Use(ginmw.RequestID())
	r.Use(ginmw.LimitBodyDefault())
	r.Use(ginmw.CORS([]string{"*"}))
	r.Use(recovery.Middleware(recovery.WithLogger(log)))
	r.Use(ginmw.Logger())
	r.Use(ginmw.Telemetry("order-service"))
	r.Use(ginmw.GlobalRateLimit())

	// Health Probes
	h := health.New(health.WithChecker("db", func(c context.Context) error { return pool.Ping(c) }))
	r.GET("/healthz", gin.WrapH(h.LivenessHandler()))
	r.GET("/readyz", gin.WrapH(h.ReadinessHandler()))

	// Mutating route protected by idempotency
	api := r.Group("/api/v1")
	api.Use(ginmw.Idempotency(idemStore))
	api.POST("/orders", func(c *gin.Context) {
		err := breaker.Execute(c.Request.Context(), func() error {
			// External payment gateway call
			return nil
		})
		if err != nil {
			httputil.Error(c.Writer, "Payment gateway unavailable", http.StatusBadGateway)
			return
		}
		_ = poolWorker.Submit(func() {
			// Asynchronous fulfillment processing
		})
		httputil.Created(c.Writer, map[string]string{"status": "created"})
	})

	// 5. Server Lifecycle and Teardown
	srv := &http.Server{Addr: ":" + port, Handler: r}
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("server fatal error", slog.Any("error", err))
		}
	}()

	mgr := shutdown.New(15 * time.Second)
	mgr.Register("http", func(c context.Context) error { return srv.Shutdown(c) })
	mgr.Register("workers", func(c context.Context) error { poolWorker.Stop(); return nil })
	mgr.Register("db", func(c context.Context) error { pool.Close(); return nil })
	mgr.Register("cache", func(c context.Context) error { return memCache.Close() })

	if err := mgr.Wait(); err != nil {
		log.Error("shutdown failed", slog.Any("error", err))
	}
}
```

---

## 7. Go 1.26 Baseline Architecture Justification

`go-libs` pins its minimum supported language toolchain and runtime baseline to **Go 1.26.0**. This decision is rooted in systemic architectural requirements rather than arbitrary version chasing:

1. **Standard Library `slog` & Context Improvements**: Go 1.26 provides optimized allocation profiles and enhanced attribute handling in standard library `log/slog`, which `logger`, `ginmw.Logger`, and `recovery` rely on for zero-alloc structured telemetry.
2. **Runtime Map & Generics Enhancements**: `cache.TypedCache[T]`, `sliceutil`, and `maputil` rely on the current Go compiler's generic function inlining and Swiss-table runtime map behavior, delivering sub-100ns execution without interface boxing.
3. **Deterministic Testing Infrastructure**: The mockable `clock.Clock` and fuzzing harnesses take advantage of Go 1.26's expanded testing primitives and fuzz corpus scheduler.
4. **Upstream CVE Mitigation**: Go 1.26 carries critical security patches in standard `crypto/tls`, `net/http`, and `net/url` implementations, establishing a hardened baseline across all consuming microservices.

