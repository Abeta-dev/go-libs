# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.2.1] - 2026-09-17

### Highlights
- **Decoupled Logger Sampler (`logger/sampler.go`)**: Eliminated architectural layering breach by removing dependency on `ratelimit`. Defined standalone `Sampler` interface and an internal lock-synchronized token-bucket sampler with zero third-party dependencies.
- **Bounded HTTP Client Retry Buffering (`httpclient/roundtripper.go`)**: Bounded memory ingestion on retries with `DefaultMaxRetryBodySize` (10MB) via `io.LimitReader`. Oversized streaming request bodies bypass buffering and execute as a single attempt without rewind to prevent process OOM.
- **Interface Segregation on `db.DBTX` (`db/context.go`)**: Segregated core querier interface `DBTX` (`Exec`, `Query`, `QueryRow`) from bulk copy operations, introducing `CopyDBTX` for `CopyFrom`.
- **Raw Error Transparency (`httputil/response.go`)**: Preserved transparent `err.Error()` propagation in `ErrorFromDomain` without unrequested sanitization.

### Added
- logger: Decoupled `Sampler` interface and `WithSampler` functional option for custom log sampling algorithms.
- httpclient: `DefaultMaxRetryBodySize` constant defining 10MB memory threshold for retry body buffering.
- db: `CopyDBTX` interface extending `DBTX` with `CopyFrom` bulk ingestion.

### Changed
- `logger`: Decoupled `SamplingHandler` from `ratelimit`; instantiates an internal token-bucket sampler when no custom sampler is supplied.
- `httpclient`: Bounded retry buffering using `io.LimitReader`; streams exceeding `DefaultMaxRetryBodySize` are executed as a single attempt without replay buffering.
- `db`: Streamlined `DBTX` interface to `Exec`, `Query`, and `QueryRow`; batch `CopyFrom` moved to segregated `CopyDBTX` interface.

### Fixed
- Source Formatting: Applied canonical `gofmt` to `circuitbreaker/consecutive_test.go` and `httpclient/roundtripper_test.go`.
- `httputil`: Verified raw error transparency in `ErrorFromDomain` returning `err.Error()` verbatim.

## [0.2.0] - 2026-09-17

### Highlights
- **PostgreSQL Distributed Idempotency Store (`idempotency/postgres.go` with `db.DBTX`)**: High-performance, production-ready distributed idempotency store using two-phase atomic locking (`Lock` and `Save`) over any `db.DBTX` connection pool or transaction. Features automatic lock expiration recovery, status conflict detection, SHA-256 payload fingerprint validation, and configurable lock/response TTLs.
- **PostgreSQL Atomic Distributed Rate Limiter (`db/ratelimit.go`)**: Clustered, atomic sliding-window rate limiter powered by PostgreSQL CTE queries over `db.DBTX`. Synchronizes request counters across multi-pod deployments without Redis or external infrastructure.
- **Decoupled Universal `net/http` Standard Middleware**: Universal middleware implementations adhering strictly to standard Go `func(http.Handler) http.Handler` signatures:
  - `httputil.CORS`: Flexible CORS handling with credential support, exposed headers, configurable max age, and wildcard/origin validation.
  - `telemetry.Middleware`: W3C distributed tracing extraction and automatic OpenTelemetry span instrumentation.
  - `logger.Middleware`: Structured access logging using standard library `log/slog` with latency tracking, status capture, and request ID correlation.
  - `idempotency.Middleware`: End-to-end HTTP idempotency filter with in-memory or PostgreSQL stores, automatic payload caching, concurrent in-progress 409 Conflict handling, and replay headers.
- **Pruning Standard Library Duplication from `sliceutil` & `maputil`**: Pruned redundant wrapper functions (`sliceutil.Map`, `sliceutil.Filter`, `maputil.Keys`, `maputil.Values`) in favor of idiomatic Go loops and standard library packages (`slices`, `maps`). Stabilized and expanded high-value algorithmic extensions: `sliceutil.Chunk`, `sliceutil.GroupBy`, `sliceutil.Unique`, `sliceutil.Flatten`, `sliceutil.Reduce`, and `maputil.Merge`.
- **Reference Microservice (`examples/microservice`) Updates**: Enhanced reference implementation demonstrating production best practices, item batching via `sliceutil.Chunk`, clean loops, and composition of operational primitives.

### Compatibility Notice
Clean-slate architecture: No backward compatibility preserved. Legacy deprecated interfaces and backwards-compatibility shims have been dropped, as no external developers or downstream production systems are actively consuming pre-release revisions.

### Added
- idempotency: PostgreSQL Distributed Idempotency Store (`PGStore`, `NewPGStore`, `WithPGLockTTL`, `WithPGResponseTTL`, `WithPGTableName`) backed by `db.DBTX` for distributed two-phase atomic locking, conflict mitigation, and crash recovery.
- idempotency: Universal `net/http` standard idempotency middleware (`Middleware`, `MiddlewareOptions`, `MiddlewareOption`, `WithHeaderName`, `WithEnforceHeader`, `WithIgnoredMethods`, `WithStatusCodeMatcher`).
- db: PostgreSQL Atomic Distributed Rate Limiter (`PGRateLimiter`, `NewPGRateLimiter`, `WithPGRateLimiterTableName`, `WithPGRateLimiterTimeout`) implementing sliding window rate limiting via `DBTX`.
- httputil: Decoupled universal `net/http` standard CORS middleware (`CORS`, `CORSConfig`, `DefaultCORSConfig`).
- telemetry: Decoupled universal `net/http` standard OpenTelemetry tracing middleware (`Middleware`, `MiddlewareOption`, `WithPropagator`).
- logger: Decoupled universal `net/http` standard structured logging middleware (`Middleware`, `MiddlewareOption`, `WithLogger`, `WithRequestIDHeader`, `WithExtraAttributes`).

### Changed
- `sliceutil`: Pruned redundant standard library duplication (`Map`, `Filter`) in favor of idiomatic Go loops and standard `slices`; expanded and stabilized algorithmic utilities: `Chunk`, `GroupBy`, `Unique`, `Flatten`, `Reduce`, `First`.
- `maputil`: Pruned redundant standard library duplication (`Keys`, `Values`) in favor of standard `maps`; retained zero-dependency extensions: `Merge`, `Filter`.
- `examples/microservice`: Updated reference microservice showcasing `sliceutil.Chunk` for item batch processing, clean standard filtering loops, and universal middleware composition.
- `securityheaders`: Transitioned `securityheaders.New(cfg Config)` to functional options `securityheaders.New(opts ...Option)` with `WithServerName`, `WithHSTSMaxAge`, `WithCSP`, `WithPermissionsPolicy`.
- `telemetry`: Transitioned `telemetry.NewTracerProvider(cfg Config)` to functional options `telemetry.NewTracerProvider(opts ...Option)` with `WithServiceName`, `WithServiceVersion`, `WithEnvironment`, `WithSampleRate`.
- `cryptoutil`: `ComparePassword` wraps malformed password hashes with sentinel `ErrInvalidHash`.
- `ginmw`: `Idempotency` logs store errors during `Lock`, `Unlock`, and `Save` operations.

### Removed (BREAKING)
- `sliceutil`: Removed redundant wrappers `Map` and `Filter`.
- `maputil`: Removed redundant wrappers `Keys` and `Values`.
- `circuitbreaker`: Removed redundant type alias `CircuitBreaker` and deprecated `New(...)` constructor; use `ConsecutiveBreaker` and `NewConsecutiveBreaker(...)` instead.
- `cache`: Removed redundant `NewTTL[T]` constructor; use `NewTypedCache[T]` instead.
- `httpclient`: Removed redundant `WithTimeout` alias; use `WithTotalTimeout` instead.
- `ratelimit`: Removed unexported pass-through wrappers `realIP`, `isTrusted`, `extractIP`, `parseTrustedProxies`.

### Fixed
- `cache`, `circuitbreaker`, `ratelimit`: Added lazy initialization guards to `TypedCache`, `ConsecutiveBreaker`, `RatioBreaker`, `TokenBucketLimiter`, and `SlidingWindowLimiter` so `var x T{}` zero-values execute safely without panicking.
- Dependencies: Bumped `golang.org/x/crypto` from `v0.55.0` to `v0.56.0` to eliminate CVEs `GO-2026-6354` and `GO-2026-6355`.

### Security
- Workflows: Pinned all GitHub Actions to immutable 40-character commit SHAs.
- Containers: Pinned base container images in `examples/microservice/Dockerfile` to immutable `@sha256:` digests.
- Least Privilege: Set top-level `permissions: read-all` across all GitHub Actions workflows.
- SAST: Added automated GitHub CodeQL analysis workflow for continuous static application security testing.

## [0.1.0] - 2026-09-14

### Compatibility Notice
Clean-slate architecture: No backward compatibility preserved. Legacy deprecated interfaces and backwards-compatibility shims have been dropped, as no external developers or downstream production systems are actively consuming pre-release revisions.

### Added
- apperror: Structured application errors with canonical machine-readable error codes (`Code`), human-readable messages, structured details, and cause wrapping.
- bodylimit: HTTP request body size limiting middleware wrapping `http.MaxBytesReader` (`New`, `MB`, `KB`).
- cache: In-memory TTL cache with 16-way striped mutex locking, singleflight stampede protection, and configurable eviction policies (`Cache`, `TypedCache`, `NewTypedCache`, `EvictionPolicy`, `WithEvictionPolicy`, `WithCapacity`).
- circuitbreaker: Consecutive-failure and failure-ratio circuit breakers with 3-state machine, single-probe half-open state, and context cancellation safety (`Breaker`, `ConsecutiveBreaker`, `RatioBreaker`, `NewConsecutiveBreaker`, `NewRatioBreaker`, `State`).
- clock: Mockable time interfaces (`Clock`, `Timer`, `Ticker`) with production `RealClock` and deterministic advanceable `FakeClock` (`NewReal`, `NewFake`).
- cryptoutil: Bcrypt password hashing (`HashPassword`, `ComparePassword`, `IsCorrectPassword`) and cryptographically secure password generation (`GenerateTempPassword`).
- db: PostgreSQL connection pool lifecycle management, context-aware querier retrieval, and distributed rate limiting (`Connect`, `GetQuerier`, `DBTX`, `WithMaxConns`, `WithMinConns`, `NewPGRateLimiter`, `PGRateLimiter`).
- env: Type-safe environment variable parsing with defaults (`String`, `Int`, `Bool`, `Duration`, `MustString`, `MustInt`).
- ginmw: HTTP middleware suite for Gin covering CORS (`CORS`), structured access logging (`Logger`), panic recovery (`Recovery`), correlation IDs (`RequestID`), security headers (`SecurityHeaders`), distributed tracing (`Telemetry`), and idempotency (`Idempotency`).
- health: HTTP health, readiness, and liveness probe handlers with pluggable dependency checkers (`New`, `Checker`, `Service`, `LivenessHandler`, `ReadinessHandler`, `Response`).
- httpclient: Resilient HTTP client pipeline composing rate limiting (`WithRateLimiter`), circuit breaking (`WithCircuitBreaker`), retry with backoff (`WithRetry`), timeouts (`WithTotalTimeout`, `WithPerAttemptTimeout`), and telemetry (`WithMetrics`, `WithTelemetry`, `New`, `NewRoundTripper`).
- httputil: Standardized JSON envelope responses, domain error translation, and universal CORS middleware (`OK`, `Created`, `NoContent`, `Error`, `ErrorFromDomain`, `WriteJSON`, `APIError`, `CORS`, `CORSConfig`).
- idempotency: Two-phase atomic idempotency locking, in-memory store, PostgreSQL distributed store, and universal HTTP middleware (`Store`, `MemoryStore`, `NewMemoryStore`, `PGStore`, `NewPGStore`, `Middleware`, `Record`, `Response`).
- logger: Context-aware structured logging with `slog` context propagation, log rate sampling, sensitive field redaction, and HTTP middleware (`Default`, `WithContext`, `FromContext`, `NewSamplingHandler`, `NewRedactingHandler`, `Middleware`).
- maputil: Generic, zero-dependency map operations (`Merge`, `Filter`).
- metrics: Framework-agnostic instrumentation interfaces (`Counter`, `Gauge`, `Histogram`) with zero-allocation no-op fallbacks (`NoopCounter`, `NoopGauge`, `NoopHistogram`).
- pagination: Offset and cursor pagination parameter parsing, SQL calculations, and response envelopers (`Parse`, `Params`, `TotalPages`, `Response`, `ParseCursor`, `CursorResponse`).
- ratelimit: Multi-algorithm rate limiting (`Limiter`, `TokenBucketLimiter`, `SlidingWindowLimiter`, `NewTokenBucket`, `NewSlidingWindow`, `NewWithLimiter`) with trusted proxy CIDR parsing.
- rbac: Role-based access control evaluator supporting multi-segment wildcard pattern matching (`Engine`, `NewEngine`, `Match`, `HasPermission`, `HasAnyPermission`, `HasAllPermissions`).
- rbaccontext: Standard context permission attachment and evaluation helpers (`WithPermissions`, `Permissions`, `Can`, `CanAny`, `CanAll`).
- recovery: Panic recovery middleware with structured stack trace logging and OpenTelemetry error recording (`Middleware`, `Option`, `WithLogger`).
- requestid: HTTP middleware injecting and propagating canonical correlation identifiers (`Header`, `Middleware`, `FromContext`).
- retry: Context-aware retry execution supporting constant, linear, exponential, and full jitter backoff strategies (`Do`, `DoWithResult`, `Config`, `Strategy`, `Constant`, `Linear`, `Exponential`, `ExponentialJitter`).
- securityheaders: OWASP recommended defensive HTTP security headers middleware (`New`, `Default`, `Config`, `DefaultConfig`).
- shutdown: Graceful OS signal interception and coordinated concurrent resource teardown manager (`Manager`, `New`, `Hook`).
- sliceutil: Generic algorithmic slice operations (`Reduce`, `GroupBy`, `Chunk`, `Unique`, `Flatten`, `First`).
- stringutil: Sensitive data masking (`MaskEmail`, `MaskPhone`), string truncation (`Truncate`), and cryptographically secure random string generation (`RandomSecureString`, `RandomAlphanumeric`).
- telemetry: OpenTelemetry trace provider initialization, composite text map propagation, span lifecycle helpers, and universal HTTP middleware (`InitProvider`, `NewTracerProvider`, `TracerProvider`, `Config`, `StartSpan`, `Middleware`).
- timeutil: UTC and timezone conversions, heuristic date parsing (`ParseTime`), and business day calculations (`AddBusinessDays`, `BusinessDays`, `BusinessDaysBetween`).
- validation: Request validation error formatting translating validator errors into structured client responses (`FieldError`, `FormatErrors`).
- workerpool: Bounded concurrent worker pool with task queue and saturation handling (`Pool`, `New`, `Submit`, `SubmitContext`).

### Known limitations
- In-memory cache: local per-process only, no distributed clustering.
- Idempotency store: in-memory store provided for single-node testing; production multi-pod clusters should use `PGStore` for distributed atomic locking.
- Rate limiting: in-memory `TokenBucket` and `SlidingWindow` provided for single-node deployments; production multi-pod clusters should use `PGRateLimiter` for distributed PostgreSQL synchronization.
- Packages in Early maturity tier: `cache`, `db`, `idempotency`, `logger`, `ratelimit`, `telemetry`, `clock`, `httpclient`.
