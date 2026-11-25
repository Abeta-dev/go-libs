# Versioning Policy & Package Maturity Matrix

This document outlines the versioning guarantees, breaking change communication policy, package maturity lifecycle, and path to `v1.0.0` for `github.com/abeta-dev/go-libs`.

---

## 1. Pre-1.0 Versioning Contract (`v0.x.y`)

`go-libs` follows [Semantic Versioning 2.0.0](https://semver.org/). Because the library is currently pre-1.0 (`v0.6.x`), consumers should be aware of the following expectations:

- **Minor Releases (`v0.X.0`)**: May introduce new packages, enhancements, or breaking API refactorings as APIs are consolidated for long-term maintainability.
- **Patch Releases (`v0.x.Y`)**: Strictly reserved for backward-compatible bug fixes, performance improvements, and security patches. Zero breaking API changes occur in patch releases.
- **Breaking Change & Strict `N + 2` Minor-Release Deprecation Window Policy**:
  - Every breaking change is explicitly documented in [CHANGELOG.md](../CHANGELOG.md) under `### Removed (BREAKING)` and `### Changed`.
  - **Strict `N + 2` Deprecation Window (`./scripts/check_deprecation.sh`)**: Any symbol slated for deprecation must carry a machine-verifiable Godoc annotation matching the exact format:
    `// Deprecated: Since vX.Y.Z (remove in >= vA.B.C). Use <NewSymbol> instead.`
    where `(A > X) || (A == X && B >= Y + 2)` (at least **2 minor releases** between initial deprecation and removal). CI (`./scripts/check_deprecation.sh`) rejects malformed deprecation comments, windows shorter than `N + 2` minor releases, and premature symbol removals against the previous release tag.
  - **Leaf Import & Depth-0 DAG Isolation (`./scripts/check_leaf_imports.sh`)**: CI enforces that all **29 Depth-0 pure internal leaf packages** maintain zero internal `github.com/abeta-dev/go-libs/*` production imports and all **35 minimal-footprint leaf packages** resolve in an isolated `go.mod` sandbox with zero `gin`, `pgx`, or `otel` dependencies.
  - Compile-time breaking changes are never introduced in patch releases.

---

## 2. Package Maturity Matrix

Not all packages in `go-libs` have the same degree of API finality. To help engineering teams evaluate production risk without guessing, packages are classified into three tiers:

- **Stable**: The API is considered frozen against arbitrary changes. High test coverage (typically >= 90%), proven in production workflows, and zero unaddressed structural issues.
- **Early**: Production-ready implementation that has undergone recent surface consolidation or is subject to ergonomic refinements based on community feedback.
- **Experimental / Deprecated**: Transitional APIs that may be superseded by standard CNCF/Go patterns or scheduled for deprecation.

### Complete Inventory (41 Packages: 28 Stable, 13 Early, 0 Deprecated)

| Package | Maturity Tier | Measured Coverage | API Status & Production Risk |
| :--- | :--- | :--- | :--- |
| `apperror` | **Stable** | 100.0% | Canonical domain error types and JSON encoders. API frozen. |
| `async` | **Stable** | 100.0% | Panic-safe goroutines and bounded concurrency orchestration. API frozen. |
| `bodylimit` | **Stable** | 100.0% | Request body cap Gin middleware. Complete and stable. |
| `circuitbreaker` | **Stable** | 100.0% | Consecutive-failure and failure-ratio state machines. Low risk. |
| `compress` | **Stable** | 100.0% | Brotli (`br`) & Gzip (`gzip`) HTTP compression and request decompression middleware. API frozen. |
| `cryptoutil` | **Stable** | 100.0% | Bcrypt password hashing and CSPRNG token generators. API frozen. |
| `env` | **Stable** | 100.0% | Zero-dependency typed environment variable parser. API frozen. |
| `fsm` | **Stable** | 100.0% | Declarative in-memory finite state machine with O(1) state transitions. API frozen. |
| `ginmw` | **Stable** | 100.0% | Unified Gin middleware chain; guarded by strict 100% CI gate. |
| `health` | **Stable** | 100.0% | Parallel dependency checks and Kubernetes HTTP probes. API frozen. |
| `httputil` | **Stable** | 100.0% | Standardized JSON envelopes and error responders. API frozen. |
| `jwks` | **Stable** | 100.0% | Thread-safe cached ECDSA P-256 JWKS public key resolver. API frozen. |
| `maputil` | **Stable** | 100.0% | Generic map helpers (Merge, Filter). API frozen. |
| `metrics` | **Stable** | *Interfaces* | Framework-agnostic Counter, Gauge, Histogram interfaces. API frozen. |
| `pagination` | **Stable** | 100.0% | Offset-based request query parser and generic slice pagination. API frozen. |
| `rbac` | **Stable** | 100.0% | Role-based access control engine with wildcard matching. Low risk. |
| `rbaccontext` | **Stable** | 100.0% | Context permission attachment and evaluation helpers. Low risk. |
| `recovery` | **Stable** | 100.0% | Panic recovery middleware with structured logging. Low risk. |
| `requestid` | **Stable** | 100.0% | W3C correlation ID middleware and context propagator. API frozen. |
| `retry` | **Stable** | 100.0% | Context-aware exponential backoff with full jitter. Low risk. |
| `securityheaders` | **Stable** | 100.0% | OWASP recommended defensive HTTP security headers. Low risk. |
| `shutdown` | **Stable** | 100.0% | Graceful OS signal interception and teardown coordinator. Low risk. |
| `sliceutil` | **Stable** | 100.0% | Generic slice algorithms (Reduce, GroupBy, Chunk, Unique, Flatten, First). API frozen. |
| `stringutil` | **Stable** | 100.0% | Masking and cryptographically secure random strings. API frozen. |
| `timeutil` | **Stable** | 100.0% | UTC/location time arithmetic, business day calculation, and heuristic parsing. |
| `uuidutil` | **Stable** | 100.0% | Centralized RFC 4122 UUID operations and validation. API frozen. |
| `validation` | **Stable** | 100.0% | Formatter translating validator/v10 errors into client JSON. |
| `workerpool` | **Stable** | 100.0% | Bounded concurrent worker pool with task queue. Low risk. |
| `cache` | **Early** | 100.0% | Singleflight cache unified into `TypedCache` with `EvictionPolicy`. API undergoing stabilization. |
| `clock` | **Early** | 100.0% | Mockable time abstraction with RealClock and advanceable FakeClock. API undergoing stabilization. |
| `db` | **Early** | 100.0% | pgx connection pool wrapper and transaction runner. Hardened against connection leaks. |
| `httpclient` | **Early** | 100.0% | Resilient composed HTTP client with rate limiting, circuit breaking, and retries. API undergoing stabilization. |
| `idempotency` | **Early** | 100.0% | In-memory and PostgreSQL idempotency stores with bounded TTL response retention. |
| `logger` | **Early** | 100.0% | Context-aware structured logging with slog, sampling, and sensitive field redaction. |
| `ratelimit` | **Early** | 100.0% | In-memory rate limiting with Token Bucket and Sliding Window algorithms. |
| `telemetry` | **Early** | 100.0% | OpenTelemetry distributed tracing wrapper with global and instance-scoped provider support. |
| `etag` | **Early** | 100.0% | RFC 9110 ETag / If-None-Match (304) and version-gated JSON middleware with client auto-bust. |
| `httpmw` | **Early** | 100.0% | Zero-Gin, zero-pgx, zero-OTel stdlib `net/http` middleware suite with full `ginmw` parity. |
| `loopbackguard` | **Early** | 100.0% | Local-first loopback address guard, DNS-rebinding Host allowlist, Same-Origin & CSRF verification. |
| `sqlite` | **Early** | 100.0% | Driver-agnostic embedded SQLite WAL helper (`busy_timeout`, `BEGIN IMMEDIATE`, `user_version` migrations, `VACUUM INTO` backup). |
| `sse` | **Early** | 100.0% | First-class Server-Sent Events (`text/event-stream`) streaming helper with heartbeats and `Last-Event-ID`. |

---

## 3. Path to `v1.0.0`

We will not cut `v1.0.0` arbitrarily or against a calendar deadline. `v1.0.0` marks a permanent backward-compatibility commitment under the Go module compatibility promise.

Before releasing `v1.0.0`, all of the following conditions must be met:

1. **Package Stabilization**: All packages currently marked **Early** (`cache`, `clock`, `db`, `httpclient`, `idempotency`, `logger`, `ratelimit`, `telemetry`) must reach **Stable** status, with zero breaking API modifications across two consecutive minor releases.
2. **Production Validation**: Demonstration of stable production deployment across multiple independent backend services without memory leaks, goroutine leaks, or lock contention regressions.
3. **Complete Godoc & Examples**: 100% of exported packages must feature runnable, testable `Example*()` functions validated by `go test`.
4. **Truth-Gate Pass**: Zero drift in documentation, coverage, and symbol verification scripts across all quality gates.
