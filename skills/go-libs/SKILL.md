---
name: go-libs
description: Umbrella router and integration guide for github.com/abeta-dev/go-libs (v0.6.0, 41 Go packages, 100.0% statement coverage). Use when selecting packages, wiring HTTP/Gin or stdlib net/http services with SSE, ETag, loopback CSRF guards, resilience, RBAC, PostgreSQL/SQLite, or OpenTelemetry, or running go-libs truth gates.
---

# `go-libs` Umbrella Skill (`v0.6.0`)

Module (`go.mod` import path): `github.com/abeta-dev/go-libs` (`@v0.6.0`, `Go 1.26.0`, **100.0% statement coverage**)

## When to Use

- Choosing which of the **41 packages** in `github.com/abeta-dev/go-libs` solves a backend engineering requirement
- Wiring a production HTTP/Gin or zero-Gin stdlib `net/http` desktop/edge BFF with SSE, ETag/304, loopback CSRF guards, structured logging, OpenTelemetry, resilience, PostgreSQL/SQLite, and RBAC
- Maintaining or contributing to `go-libs` and passing `./scripts/check_secrets.sh`, `./scripts/check_version.sh`, `./scripts/check_leaf_imports.sh`, `./scripts/check_deprecation.sh`, and `./scripts/check_coverage.sh`

## Subskill Routing

| Domain | Subskill | Covered Packages |
| :--- | :--- | :--- |
| **Resilience & Concurrency** | [`go-libs-resilience`](../go-libs-resilience/SKILL.md) | `retry`, `circuitbreaker`, `ratelimit`, `workerpool`, `cache`, `async`, `shutdown`, `fsm`, `clock`, `metrics` |
| **HTTP, Gin & Edge Middleware** | [`go-libs-http-gin`](../go-libs-http-gin/SKILL.md) | `ginmw`, `httpmw`, `compress`, `etag`, `loopbackguard`, `sse`, `httpclient`, `httputil`, `pagination`, `idempotency`, `health`, `requestid`, `recovery`, `securityheaders`, `bodylimit`, `validation` |
| **Data, Security & Observability** | [`go-libs-data-security`](../go-libs-data-security/SKILL.md) | `db`, `sqlite`, `rbac`, `rbaccontext`, `jwks`, `cryptoutil`, `telemetry`, `logger` |
| **Foundation & Type-Safe Utilities** | [`go-libs-foundation`](../go-libs-foundation/SKILL.md) | `apperror`, `env`, `sliceutil`, `maputil`, `stringutil`, `timeutil`, `uuidutil` |

## Complete 41-Package Inventory

1. `apperror` — Canonical error codes and structured domain errors (`Error`, `Code`, `New`, `Wrap`, `BadRequest`, `Unauthorized`, `Forbidden`, `NotFound`, `Conflict`, `Internal`).
2. `async` — Panic-safe goroutines and bounded concurrency orchestration (`Go`, `GoCtx`, `Parallel`, `ParallelLimit`).
3. `bodylimit` — `net/http` request body byte-limit middleware (`New`, `MB`, `KB`).
4. `cache` — Generic singleflight stampede-protected multi-policy cache with prefix/tag invalidation and fetch status (`TypedCache[T]`, `NewTypedCache`, `GetOrFetchWithStatus`, `GetOrFetchWithTags`, `DeletePrefix`, `SetWithTags`, `InvalidateTag`, `FetchStatus`).
5. `circuitbreaker` — Outbound 3-state (`Closed`, `Open`, `HalfOpen`) failure machine (`ConsecutiveBreaker`, `RatioBreaker`, `NewConsecutiveBreaker`, `NewRatioBreaker`).
6. `clock` — Deterministic mockable time abstraction (`Clock`, `RealClock`, `FakeClock`, `NewReal`, `NewFake`).
7. `compress` — SSE-safe RFC 9110 Brotli (`br`) & Gzip (`gzip`) HTTP compression, bounded request decompression, and precompressed `.br`/`.gz` SPA file server (`Middleware`, `Negotiate`, `PrecompressedFileServer`, `PrecompressedFS`, `WithSPAFallback`, `WithCSRFMetaToken`).
8. `cryptoutil` — Bcrypt password hashing (cost 12) and CSPRNG password generation (`HashPassword`, `ComparePassword`, `IsCorrectPassword`, `GenerateTempPassword`).
9. `db` — PostgreSQL `pgxpool` lifecycle management, `DBTX` / `TxBeginner` abstractions, and transactional helpers (`Connect`, `DBTX`, `TxBeginner`, `WithTx`, `WithQuerier`, `GetQuerier`, `WithMaxConns`).
10. `env` — Zero-dependency typed environment variable parsers (`String`, `Int`, `Bool`, `Duration`, `MustString`, `MustInt`).
11. `etag` — RFC 9110 `ETag` / `If-None-Match` (`304 Not Modified`) and version-gated JSON middleware with volatile JSON key stripping and `X-Client-App-Version` auto-bust (`Middleware`, `Compute`, `Match`, `WriteVersionedJSON`, `WithAppVersion`, `WithIgnoreJSONKeys`).
12. `fsm` — Declarative in-memory finite state machine with O(1) state transitions (`Machine`, `NewMachine`, `Transition`, `CanTransition`, `ValidateTransition`).
13. `ginmw` — Unified Gin middleware pipeline (`RequestID`, `LimitBodyDefault`, `LimitBodyAuth`, `Compress`, `PrecompressedFileServer`, `ETag`, `WriteVersionedJSON`, `SecurityHeaders`, `SecurityHeadersWithOptions`, `LoopbackGuard`, `SSE`, `CORS`, `Logger`, `Telemetry`, `REDMetrics`, `MetricsEndpoint`, `GlobalRateLimit`, `AuthRateLimit`, `AuthMiddleware`, `RBAC`, `RequirePermission`, `Idempotency`, `ErrorHandler`).
14. `health` — Parallel dependency health check and Kubernetes probe handler (`Service`, `New`, `WithChecker`, `LivenessHandler`, `ReadinessHandler`).
15. `httpclient` — Resilient composed HTTP client (`New`, `NewRoundTripper`, `WithRetry`, `WithCircuitBreaker`, `WithRateLimiter`, `WithPerAttemptTimeout`).
16. `httpmw` — Zero-Gin, zero-pgx, zero-OTel stdlib `net/http` middleware suite (`Chain`, `Use`, `RequestID`, `LimitBodyDefault`, `Compress`, `PrecompressedFileServer`, `ETag`, `SecurityHeaders`, `LoopbackGuard`, `CORS`, `Recovery`, `Logger`, `REDMetrics`, `MetricsEndpoint`, `GlobalRateLimit`, `RequirePermission`, `RequireTenant`, `SSE`).
17. `httputil` — Standardized JSON response envelopes and domain error mapping (`OK`, `Created`, `NoContent`, `Error`, `ErrorFromDomain`).
18. `idempotency` — Two-phase atomic HTTP request deduplication (`Store`, `MemoryStore`, `PGStore`, `NewMemoryStore`, `NewPGStore`, `Middleware`).
19. `jwks` — Thread-safe cached ECDSA P-256 JWKS public key resolver (`Resolver`, `NewResolver`, `GetECPublicKey`).
20. `logger` — Request-context aware structured `slog` logging with sampling and sensitive field redaction (`Default`, `WithContext`, `FromContext`, `NewRedactingHandler`, `NewSamplingHandler`).
21. `loopbackguard` — Local-first loopback address validator, DNS-rebinding `Host` allowlist, `Sec-Fetch-Site` / `Origin` Same-Origin guard, and constant-time per-boot ephemeral CSRF middleware (`ValidateLoopbackAddr`, `New`, `Guard.Middleware`, `GenerateCSRFToken`).
22. `maputil` — Generic type-safe map transformations (`Merge`, `Filter`).
23. `metrics` — Framework-agnostic `Counter`, `Gauge`, and `Histogram` interfaces with atomic/noop implementations.
24. `pagination` — Clamped offset and cursor pagination parsers and response envelopes (`Parse`, `Offset`, `Response`, `ParseCursor`, `EncodeCursor`, `DecodeCursor`).
25. `ratelimit` — IP rate limiters (`TokenBucketLimiter`, `SlidingWindowLimiter`, `NewTokenBucket`, `NewSlidingWindow`, `NewPGRateLimiter`) with CIDR proxy parsing.
26. `rbac` — Multi-segment wildcard Role-Based Access Control evaluator (`Engine`, `NewEngine`, `Match`, `HasPermission`, `HasAnyPermission`, `HasAllPermissions`).
27. `rbaccontext` — Context-bound permission lookup helpers (`WithPermissions`, `Permissions`, `Can`, `CanAny`, `CanAll`).
28. `recovery` — Panic-to-JSON-500 middleware with structured logging (`Middleware`, `WithLogger`).
29. `requestid` — Correlation ID generator and context propagator (`Middleware`, `FromContext`, `GetRequestID`).
30. `retry` — Context-aware backoff and retry execution algorithms (`Do`, `DoWithResult`, `Constant`, `ExponentialJitter`).
31. `securityheaders` — OWASP defensive security headers middleware (`New`, `Default`, `WithDisableHSTS`, `WithCacheControl`).
32. `shutdown` — Coordinated graceful OS signal teardown manager (`Manager`, `New`, `Register`, `Wait`).
33. `sliceutil` — Generic slice algorithms (`Reduce`, `GroupBy`, `Chunk`, `Unique`, `Flatten`, `First`).
34. `sqlite` — Driver-agnostic embedded SQLite WAL helper (`Open`, `Wrap`, `WithImmediateTx`, `Migrate`, `Backup`, `Checkpoint`, `WithBusyTimeout`).
35. `sse` — First-class Server-Sent Events (`text/event-stream`) streaming helper (`NewStream`, `Handler`, `Event`, `WithHeartbeat`, `WithRetry`).
36. `stringutil` — Sensitive info masking, truncation, and CSPRNG string helpers (`MaskEmail`, `MaskPhone`, `Truncate`, `RandomAlphanumeric`, `RandomSecureString`).
37. `telemetry` — OpenTelemetry distributed tracing provider and span helpers (`InitProvider`, `NewTracerProvider`, `StartSpan`).
38. `timeutil` — UTC normalization, RFC parsing, business days, and day boundaries (`NowIn`, `FormatIn`, `AddBusinessDays`, `BusinessDays`, `StartOfDay`, `EndOfDay`).
39. `uuidutil` — Centralized RFC 4122 / RFC 9562 UUID v4 & v7 operations and validation (`New`, `NewString`, `NewV7`, `NewV7String`, `Parse`, `IsValid`, `Nil`).
40. `validation` — Declarative `validator/v10` error formatter (`FormatErrors`, `FieldError`).
41. `workerpool` — Bounded panic-safe concurrent worker pool (`Pool`, `New`, `Submit`, `SubmitContext`, `Stop`).

## Truth Gates & Verification (`./scripts/check_*.sh`)

- **`./scripts/check_secrets.sh`**: Scans git commit history and the working tree with TruffleHog (`--results=verified,unknown --fail`) to ensure zero leaked credentials or keys.
- **`./scripts/check_version.sh`**: Enforces `EXPECTED_PKG_COUNT=41`, `@v0.6.0` tag parity across docs, `Go 1.26.0` baseline, and AST verification of `CHANGELOG.md` symbols.
- **`./scripts/check_deprecation.sh`**: Enforces strict `N + 2` minor-release deprecation windows on any `// Deprecated:` annotations.
- **`./scripts/check_coverage.sh`**: Verifies **100.0% statement coverage** across all 41 packages and zero drift against `README.md`, `docs/VERSIONING.md`, and `.github/badges/coverage.json`.
- **`./scripts/check_leaf_imports.sh`**: Verifies that all **29 Depth-0 pure internal leaves** have 0 internal imports and all **35 minimal-footprint leaf packages** never pull heavy frameworks (`gin`, `pgx`, `opentelemetry`).
