---
name: go-libs
description: Umbrella router and integration guide for github.com/abeta-dev/go-libs (v0.4.0, 35 Go packages). Use when selecting packages, wiring HTTP/Gin services with resilience, RBAC, PostgreSQL, or OpenTelemetry, or running go-libs truth gates.
---

# `go-libs` Umbrella Skill (`v0.4.0`)

Module (`go.mod` import path): `github.com/abeta-dev/go-libs` (`v0.4.0`, `Go 1.26.0`)

## When to Use

- Choosing which of the **35 packages** in `github.com/abeta-dev/go-libs` solves a backend engineering requirement
- Wiring a production HTTP/Gin service with structured logging, OpenTelemetry, resilience, and RBAC
- Maintaining or contributing to `go-libs` and passing `./scripts/check_version.sh`

## Subskill Routing

| Domain | Subskill | Covered Packages |
| :--- | :--- | :--- |
| **Resilience & Concurrency** | [`go-libs-resilience`](../go-libs-resilience/SKILL.md) | `retry`, `circuitbreaker`, `ratelimit`, `workerpool`, `cache`, `async`, `shutdown`, `fsm`, `clock`, `metrics` |
| **HTTP, Gin & Edge Middleware** | [`go-libs-http-gin`](../go-libs-http-gin/SKILL.md) | `ginmw`, `httpclient`, `httputil`, `pagination`, `idempotency`, `health`, `requestid`, `recovery`, `securityheaders`, `bodylimit`, `validation` |
| **Data, Security & Observability** | [`go-libs-data-security`](../go-libs-data-security/SKILL.md) | `db`, `rbac`, `rbaccontext`, `jwks`, `cryptoutil`, `telemetry`, `logger` |
| **Foundation & Type-Safe Utilities** | [`go-libs-foundation`](../go-libs-foundation/SKILL.md) | `apperror`, `env`, `sliceutil`, `maputil`, `stringutil`, `timeutil`, `uuidutil` |

## Complete 35-Package Inventory

1. `apperror` — Canonical error codes and structured domain errors (`Error`, `Code`, `New`, `Wrap`, `BadRequest`, `Unauthorized`, `Forbidden`, `NotFound`, `Conflict`, `Internal`).
2. `async` — Panic-safe goroutines and bounded concurrency orchestration (`Go`, `GoCtx`, `Parallel`, `ParallelLimit`).
3. `bodylimit` — `net/http` request body byte-limit middleware (`New`, `MB`, `KB`).
4. `cache` — Generic singleflight stampede-protected multi-policy cache (`TypedCache[T]`, `NewTypedCache`, `NewMemoryCache`, `WithEvictionPolicy` supporting `PolicySampledLRU`, `PolicyLRU`, `PolicyLFU`, `PolicyFIFO`, `PolicyTTL`).
5. `circuitbreaker` — Outbound 3-state (`Closed`, `Open`, `HalfOpen`) failure machine (`ConsecutiveBreaker`, `RatioBreaker`, `NewConsecutiveBreaker`, `NewRatioBreaker`).
6. `clock` — Deterministic mockable time abstraction (`Clock`, `RealClock`, `FakeClock`, `NewReal`, `NewFake`).
7. `cryptoutil` — Bcrypt password hashing (cost 12) and CSPRNG password generation (`HashPassword`, `ComparePassword`, `IsCorrectPassword`, `GenerateTempPassword`).
8. `db` — PostgreSQL `pgxpool` lifecycle management, `DBTX` querier abstraction, and transactional helper (`Connect`, `DBTX`, `WithTx`, `WithQuerier`, `GetQuerier`, `WithMaxConns`).
9. `env` — Zero-dependency typed environment variable parsers (`String`, `Int`, `Bool`, `Duration`, `MustString`, `MustInt`).
10. `fsm` — Declarative in-memory finite state machine with O(1) state transitions (`Machine`, `NewMachine`, `Transition`, `CanTransition`, `ValidateTransition`).
11. `ginmw` — Unified Gin middleware pipeline (`RequestID`, `LimitBodyDefault`, `LimitBodyAuth`, `CORS`, `Logger`, `Telemetry`, `REDMetrics`, `MetricsEndpoint`, `GlobalRateLimit`, `AuthRateLimit`, `AuthMiddleware`, `RBAC`, `RequirePermission`, `Idempotency`, `ErrorHandler`).
12. `health` — Parallel dependency health check and Kubernetes probe handler (`Service`, `New`, `WithChecker`, `LivenessHandler`, `ReadinessHandler`).
13. `httpclient` — Resilient composed HTTP client (`New`, `NewRoundTripper`, `WithRetry`, `WithCircuitBreaker`, `WithRateLimiter`, `WithPerAttemptTimeout`).
14. `httputil` — Standardized JSON response envelopes and domain error mapping (`OK`, `Created`, `NoContent`, `Error`, `ErrorFromDomain`).
15. `idempotency` — Two-phase atomic HTTP request deduplication (`Store`, `MemoryStore`, `PGStore`, `NewMemoryStore`, `NewPGStore`, `Middleware`).
16. `jwks` — Thread-safe cached ECDSA P-256 JWKS public key resolver (`Resolver`, `NewResolver`, `GetECPublicKey`).
17. `logger` — Request-context aware structured `slog` logging with sampling and sensitive field redaction (`Default`, `WithContext`, `FromContext`, `NewRedactingHandler`, `NewSamplingHandler`).
18. `maputil` — Generic type-safe map transformations (`Merge`, `Filter`).
19. `metrics` — Framework-agnostic `Counter`, `Gauge`, and `Histogram` interfaces with atomic/noop implementations.
20. `pagination` — Clamped offset and cursor pagination parsers and response envelopes (`Parse`, `Offset`, `Response`, `ParseCursor`, `EncodeCursor`, `DecodeCursor`).
21. `ratelimit` — IP rate limiters (`TokenBucketLimiter`, `SlidingWindowLimiter`, `NewTokenBucket`, `NewSlidingWindow`, `NewPGRateLimiter`) with CIDR proxy parsing.
22. `rbac` — Multi-segment wildcard Role-Based Access Control evaluator (`Engine`, `NewEngine`, `Match`, `HasPermission`, `HasAnyPermission`, `HasAllPermissions`).
23. `rbaccontext` — Context-bound permission lookup helpers (`WithPermissions`, `Permissions`, `Can`, `CanAny`, `CanAll`).
24. `recovery` — Panic-to-JSON-500 middleware with structured logging (`Middleware`, `WithLogger`).
25. `requestid` — Correlation ID generator and context propagator (`Middleware`, `FromContext`, `GetRequestID`).
26. `retry` — Context-aware backoff and retry execution algorithms (`Do`, `DoWithResult`, `Constant`, `ExponentialJitter`).
27. `securityheaders` — OWASP defensive security headers middleware (`New`, `Default`).
28. `shutdown` — Coordinated graceful OS signal teardown manager (`Manager`, `New`, `Register`, `Wait`).
29. `sliceutil` — Generic slice algorithms (`Reduce`, `GroupBy`, `Chunk`, `Unique`, `Flatten`, `First`).
30. `stringutil` — Sensitive info masking, truncation, and CSPRNG string helpers (`MaskEmail`, `MaskPhone`, `Truncate`, `RandomAlphanumeric`, `RandomSecureString`).
31. `telemetry` — OpenTelemetry distributed tracing provider and span helpers (`InitProvider`, `NewTracerProvider`, `StartSpan`).
32. `timeutil` — UTC normalization, RFC parsing, business days, and day boundaries (`NowIn`, `FormatIn`, `AddBusinessDays`, `BusinessDays`, `StartOfDay`, `EndOfDay`).
33. `uuidutil` — Centralized RFC 4122 / RFC 9562 UUID v4 & v7 operations and validation (`New`, `NewString`, `NewV7`, `NewV7String`, `Parse`, `IsValid`, `Nil`).
34. `validation` — Declarative `validator/v10` error formatter (`FormatErrors`, `FieldError`).
35. `workerpool` — Bounded panic-safe concurrent worker pool (`Pool`, `New`, `Submit`, `SubmitContext`, `Stop`).

## Truth Gates (`./scripts/check_version.sh`)

- `EXPECTED_PKG_COUNT=35` must match the number of top-level Go package directories.
- All versioned install commands and references in docs must match `@v0.4.0`.
