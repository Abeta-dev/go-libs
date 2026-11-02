---
name: go-libs
description: Umbrella router and integration guide for github.com/Abeta-dev/go-libs (v0.3.2, 35 Go packages). Use when selecting packages, wiring HTTP/Gin services with resilience, RBAC, PostgreSQL, or OpenTelemetry, or running go-libs truth gates.
---

# `go-libs` Umbrella Skill (`v0.3.2`)

Module: `github.com/Abeta-dev/go-libs` (`v0.3.2`, `Go 1.26.0`)

## When to Use

- Choosing which of the **35 packages** in `github.com/Abeta-dev/go-libs` solves a backend engineering requirement
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

1. `apperror` — Structured domain errors with HTTP/gRPC status mapping (`Code`, `Message`, `Details`, `Unwrap`).
2. `async` — Generic `Future[T]`, `Parallel`, and `Race` helpers with panic recovery and context cancellation.
3. `bodylimit` — `net/http` request body byte-limit middleware (`http.MaxBytesReader`).
4. `cache` — Generic `Cache[K, V]` interface with sharded in-memory LRU/TTL and Redis (`go-redis/v9`) backends, plus `GetOrLoad` stampede protection (`singleflight`).
5. `circuitbreaker` — Three-state (`Closed`, `Open`, `HalfOpen`) circuit breaker with configurable failure thresholds and state-change callbacks.
6. `clock` — `Clock` interface (`RealClock`, `MockClock`) for deterministic time testing.
7. `cryptoutil` — AES-256-GCM authenticated encryption, HMAC-SHA256 signing/verification, and CSPRNG token generation.
8. `db` — PostgreSQL `pgxpool.Pool` lifecycle management, health check, and `WithTx` transactional helper.
9. `env` — Typed environment variable loaders (`GetString`, `GetInt`, `GetBool`, `GetDuration`, `MustGet*`).
10. `fsm` — Generic table-driven finite state machine (`FSM[S, E]`) with guard predicates and transition hooks.
11. `ginmw` — 22 composable Gin middleware helpers (request ID, structured logging, recovery, CORS, security headers, body limit, timeout, rate limit, idempotency, RBAC, JWT/JWKS, compression, canonical log line, deprecation, circuit breaker, etc.).
12. `health` — Kubernetes liveness (`/healthz`) and readiness (`/readyz`) HTTP handlers with concurrent dependency probes.
13. `httpclient` — Resilient `http.Client` wrapper with automatic retry, circuit breaker, per-attempt timeout, and OpenTelemetry trace propagation.
14. `httputil` — Standardized JSON response envelopes (`WriteJSON`, `WriteError`, `DecodeJSON` with strict unknown-field rejection).
15. `idempotency` — HTTP idempotency-key middleware and store (`InMemoryStore`, Redis) preventing duplicate mutations.
16. `jwks` — Caching JWKS keyfetcher and RS256/ES256 JWT validator with background key rotation refresh.
17. `logger` — Structured `slog` wrapper with context-aware trace/span/request-ID injection and PII redaction.
18. `maputil` — Generic map transformations (`Keys`, `Values`, `Merge`, `Filter`, `MapKeys`, `MapValues`, `Invert`).
19. `metrics` — Prometheus/OpenTelemetry counter, histogram, and gauge registration helpers.
20. `pagination` — Cursor-based (opaque base64 keyset) and offset/limit pagination parsers and response envelopes.
21. `ratelimit` — Token-bucket and sliding-window rate limiters (in-memory and Redis Lua script) with `net/http` middleware.
22. `rbac` — Role-Based Access Control policy evaluator with wildcard permissions (`resource:action`, `*`).
23. `rbaccontext` — Context propagation helpers for authenticated principal roles and permissions.
24. `recovery` — Panic-recovery `net/http` middleware with stack-trace logging and structured `500` response.
25. `requestid` — `X-Request-ID` generation (UUIDv4/ULID) and context propagation middleware.
26. `retry` — Context-aware exponential backoff with full jitter (`Do`, `DoWithData[T]`, `RetryableError`).
27. `securityheaders` — OWASP recommended HTTP response headers (`Strict-Transport-Security`, `X-Content-Type-Options`, `X-Frame-Options`, `Content-Security-Policy`).
28. `shutdown` — Graceful shutdown coordinator executing registered hooks in priority/LIFO order on `SIGINT`/`SIGTERM`.
29. `sliceutil` — Generic slice operations (`Map`, `Filter`, `Reduce`, `Chunk`, `Unique`, `GroupBy`, `IndexBy`, `Compact`).
30. `stringutil` — Safe string truncation, case conversion (`ToSnakeCase`, `ToCamelCase`, `ToKebabCase`), masking, and slugification.
31. `telemetry` — OpenTelemetry SDK bootstrap (`TracerProvider`, `MeterProvider`, OTLP gRPC/HTTP exporters, W3C propagator).
32. `timeutil` — Business-day arithmetic, start/end of day/month/quarter helpers, and ISO-8601 duration parsing.
33. `uuidutil` — UUIDv4 and time-ordered UUIDv7 generation, validation, and nil-check utilities.
34. `validation` — Composable struct/field validation accumulator returning `apperror.ValidationError` details.
35. `workerpool` — Bounded concurrency worker pool (`Pool[T, R]`) with backpressure, panic isolation, and graceful drain.

## Truth Gates (`./scripts/check_version.sh`)

- `EXPECTED_PKG_COUNT=35` must match the number of top-level Go package directories.
- All versioned install commands and references in docs must match `@v0.3.2`.
