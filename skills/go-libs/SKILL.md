---
name: go-libs
description: Umbrella router and integration guide for github.com/abeta-dev/go-libs (v0.5.0, 36 Go packages, 100.0% statement coverage). Use when selecting packages, wiring HTTP/Gin services with resilience, RBAC, PostgreSQL, or OpenTelemetry, or running go-libs truth gates.
---

# `go-libs` Umbrella Skill (`v0.5.0`)

Module (`go.mod` import path): `github.com/abeta-dev/go-libs` (`@v0.5.0`, `Go 1.26.0`, **100.0% statement coverage**)

## When to Use

- Choosing which of the **36 packages** in `github.com/abeta-dev/go-libs` solves a backend engineering requirement
- Wiring a production HTTP/Gin service with structured logging, OpenTelemetry, resilience, and RBAC
- Maintaining or contributing to `go-libs` and passing `./scripts/check_secrets.sh`, `./scripts/check_version.sh`, `./scripts/check_leaf_imports.sh`, and `./scripts/check_coverage.sh`

## Subskill Routing

| Domain | Subskill | Covered Packages |
| :--- | :--- | :--- |
| **Resilience & Concurrency** | [`go-libs-resilience`](../go-libs-resilience/SKILL.md) | `retry`, `circuitbreaker`, `ratelimit`, `workerpool`, `cache`, `async`, `shutdown`, `fsm`, `clock`, `metrics` |
| **HTTP, Gin & Edge Middleware** | [`go-libs-http-gin`](../go-libs-http-gin/SKILL.md) | `ginmw`, `compress`, `httpclient`, `httputil`, `pagination`, `idempotency`, `health`, `requestid`, `recovery`, `securityheaders`, `bodylimit`, `validation` |
| **Data, Security & Observability** | [`go-libs-data-security`](../go-libs-data-security/SKILL.md) | `db`, `rbac`, `rbaccontext`, `jwks`, `cryptoutil`, `telemetry`, `logger` |
| **Foundation & Type-Safe Utilities** | [`go-libs-foundation`](../go-libs-foundation/SKILL.md) | `apperror`, `env`, `sliceutil`, `maputil`, `stringutil`, `timeutil`, `uuidutil` |

## Complete 36-Package Inventory

1. `apperror` — Canonical error codes and structured domain errors (`Error`, `Code`, `New`, `Wrap`, `BadRequest`, `Unauthorized`, `Forbidden`, `NotFound`, `Conflict`, `Internal`).
2. `async` — Panic-safe goroutines and bounded concurrency orchestration (`Go`, `GoCtx`, `Parallel`, `ParallelLimit`).
3. `bodylimit` — `net/http` request body byte-limit middleware (`New`, `MB`, `KB`).
4. `cache` — Generic singleflight stampede-protected multi-policy cache (`TypedCache[T]`, `NewTypedCache`, `NewMemoryCache`, `WithEvictionPolicy` supporting `PolicySampledLRU`, `PolicyLRU`, `PolicyLFU`, `PolicyFIFO`, `PolicyTTL`).
5. `circuitbreaker` — Outbound 3-state (`Closed`, `Open`, `HalfOpen`) failure machine (`ConsecutiveBreaker`, `RatioBreaker`, `NewConsecutiveBreaker`, `NewRatioBreaker`).
6. `clock` — Deterministic mockable time abstraction (`Clock`, `RealClock`, `FakeClock`, `NewReal`, `NewFake`).
7. `compress` — RFC 9110 Brotli (`br`) & Gzip (`gzip`) HTTP response compression and bounded request decompression middleware (`Middleware`, `Negotiate`, `Config`, `DefaultConfig`, `EncodingBrotli`, `EncodingGzip`, `EncodingIdentity`).
8. `cryptoutil` — Bcrypt password hashing (cost 12) and CSPRNG password generation (`HashPassword`, `ComparePassword`, `IsCorrectPassword`, `GenerateTempPassword`).
9. `db` — PostgreSQL `pgxpool` lifecycle management, `DBTX` / `TxBeginner` abstractions, and transactional helpers (`Connect`, `DBTX`, `TxBeginner`, `WithTx`, `WithQuerier`, `GetQuerier`, `WithMaxConns`).
10. `env` — Zero-dependency typed environment variable parsers (`String`, `Int`, `Bool`, `Duration`, `MustString`, `MustInt`).
11. `fsm` — Declarative in-memory finite state machine with O(1) state transitions (`Machine`, `NewMachine`, `Transition`, `CanTransition`, `ValidateTransition`).
12. `ginmw` — Unified Gin middleware pipeline (`RequestID`, `LimitBodyDefault`, `LimitBodyAuth`, `Compress`, `CORS`, `Logger`, `Telemetry`, `REDMetrics`, `MetricsEndpoint`, `GlobalRateLimit`, `AuthRateLimit`, `AuthMiddleware`, `RBAC`, `RequirePermission`, `Idempotency`, `ErrorHandler`).
13. `health` — Parallel dependency health check and Kubernetes probe handler (`Service`, `New`, `WithChecker`, `LivenessHandler`, `ReadinessHandler`).
14. `httpclient` — Resilient composed HTTP client (`New`, `NewRoundTripper`, `WithRetry`, `WithCircuitBreaker`, `WithRateLimiter`, `WithPerAttemptTimeout`).
15. `httputil` — Standardized JSON response envelopes and domain error mapping (`OK`, `Created`, `NoContent`, `Error`, `ErrorFromDomain`).
16. `idempotency` — Two-phase atomic HTTP request deduplication (`Store`, `MemoryStore`, `PGStore`, `NewMemoryStore`, `NewPGStore`, `Middleware`).
17. `jwks` — Thread-safe cached ECDSA P-256 JWKS public key resolver (`Resolver`, `NewResolver`, `GetECPublicKey`).
18. `logger` — Request-context aware structured `slog` logging with sampling and sensitive field redaction (`Default`, `WithContext`, `FromContext`, `NewRedactingHandler`, `NewSamplingHandler`).
19. `maputil` — Generic type-safe map transformations (`Merge`, `Filter`).
20. `metrics` — Framework-agnostic `Counter`, `Gauge`, and `Histogram` interfaces with atomic/noop implementations.
21. `pagination` — Clamped offset and cursor pagination parsers and response envelopes (`Parse`, `Offset`, `Response`, `ParseCursor`, `EncodeCursor`, `DecodeCursor`).
22. `ratelimit` — IP rate limiters (`TokenBucketLimiter`, `SlidingWindowLimiter`, `NewTokenBucket`, `NewSlidingWindow`, `NewPGRateLimiter`) with CIDR proxy parsing.
23. `rbac` — Multi-segment wildcard Role-Based Access Control evaluator (`Engine`, `NewEngine`, `Match`, `HasPermission`, `HasAnyPermission`, `HasAllPermissions`).
24. `rbaccontext` — Context-bound permission lookup helpers (`WithPermissions`, `Permissions`, `Can`, `CanAny`, `CanAll`).
25. `recovery` — Panic-to-JSON-500 middleware with structured logging (`Middleware`, `WithLogger`).
26. `requestid` — Correlation ID generator and context propagator (`Middleware`, `FromContext`, `GetRequestID`).
27. `retry` — Context-aware backoff and retry execution algorithms (`Do`, `DoWithResult`, `Constant`, `ExponentialJitter`).
28. `securityheaders` — OWASP defensive security headers middleware (`New`, `Default`).
29. `shutdown` — Coordinated graceful OS signal teardown manager (`Manager`, `New`, `Register`, `Wait`).
30. `sliceutil` — Generic slice algorithms (`Reduce`, `GroupBy`, `Chunk`, `Unique`, `Flatten`, `First`).
31. `stringutil` — Sensitive info masking, truncation, and CSPRNG string helpers (`MaskEmail`, `MaskPhone`, `Truncate`, `RandomAlphanumeric`, `RandomSecureString`).
32. `telemetry` — OpenTelemetry distributed tracing provider and span helpers (`InitProvider`, `NewTracerProvider`, `StartSpan`).
33. `timeutil` — UTC normalization, RFC parsing, business days, and day boundaries (`NowIn`, `FormatIn`, `AddBusinessDays`, `BusinessDays`, `StartOfDay`, `EndOfDay`).
34. `uuidutil` — Centralized RFC 4122 / RFC 9562 UUID v4 & v7 operations and validation (`New`, `NewString`, `NewV7`, `NewV7String`, `Parse`, `IsValid`, `Nil`).
35. `validation` — Declarative `validator/v10` error formatter (`FormatErrors`, `FieldError`).
36. `workerpool` — Bounded panic-safe concurrent worker pool (`Pool`, `New`, `Submit`, `SubmitContext`, `Stop`).

## Truth Gates & Verification (`./scripts/check_*.sh`)

- **`./scripts/check_secrets.sh`**: Scans git commit history and the working tree with TruffleHog (`--results=verified,unknown --fail`) to ensure zero leaked credentials or keys.
- **`./scripts/check_version.sh`**: Enforces `EXPECTED_PKG_COUNT=36`, `@v0.5.0` tag parity across docs, `Go 1.26.0` baseline, and AST verification of `CHANGELOG.md` symbols.
- **`./scripts/check_coverage.sh`**: Verifies **100.0% statement coverage** across all 36 packages and zero drift against `README.md` and `.github/badges/coverage.json`.
- **`./scripts/check_leaf_imports.sh`**: Verifies in an isolated sandbox that leaf packages never pull heavy framework dependencies (`gin`, `pgx`, `opentelemetry`).
