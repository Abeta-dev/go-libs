# Versioning Policy & Package Maturity Matrix

This document outlines the versioning guarantees, breaking change communication policy, package maturity lifecycle, and path to `v1.0.0` for `github.com/umesh0492/go-libs`.

---

## 1. Pre-1.0 Versioning Contract (`v0.x.y`)

`go-libs` follows [Semantic Versioning 2.0.0](https://semver.org/). Because the library is currently pre-1.0 (`v0.1.x`), consumers should be aware of the following expectations:

- **Minor Releases (`v0.X.0`)**: May introduce new packages, enhancements, or breaking API refactorings as APIs are consolidated for long-term maintainability.
- **Patch Releases (`v0.x.Y`)**: Strictly reserved for backward-compatible bug fixes, performance improvements, and security patches. Zero breaking API changes occur in patch releases.
- **Breaking Change Communication**:
  - Every breaking change is explicitly documented in [CHANGELOG.md](../CHANGELOG.md) under `### Removed (BREAKING)` and `### Changed`.
  - When feasible, non-urgent symbol changes will include a `// Deprecated: use <NewSymbol> instead` godoc annotation for at least one release before removal.
  - Compile-time breaking changes are never introduced in patch releases.

---

## 2. Package Maturity Matrix

Not all packages in `go-libs` have the same degree of API finality. To help engineering teams evaluate production risk without guessing, packages are classified into three tiers:

- **Stable**: The API is considered frozen against arbitrary changes. High test coverage (typically >= 90%), proven in production workflows, and zero unaddressed structural issues.
- **Early**: Production-ready implementation that has undergone recent surface consolidation or is subject to ergonomic refinements based on community feedback.
- **Experimental / Deprecated**: Transitional APIs that may be superseded by standard CNCF/Go patterns or scheduled for deprecation.

### Complete Inventory (31 Packages: 23 Stable, 8 Early, 0 Deprecated)

| Package | Maturity Tier | Measured Coverage | API Status & Production Risk |
| :--- | :--- | :--- | :--- |
| `apperror` | **Stable** | 100.0% | Canonical domain error types and JSON encoders. API frozen. |
| `bodylimit` | **Stable** | 100.0% | Request body cap Gin middleware. Complete and stable. |
| `circuitbreaker` | **Stable** | 92.2% | Consecutive-failure and failure-ratio state machines. Low risk. |
| `cryptoutil` | **Stable** | 100.0% | Bcrypt password hashing and CSPRNG token generators. API frozen. |
| `env` | **Stable** | 100.0% | Zero-dependency typed environment variable parser. API frozen. |
| `ginmw` | **Stable** | 100.0% | Unified Gin middleware chain; guarded by strict 100% CI gate. |
| `health` | **Stable** | 100.0% | Parallel dependency checks and Kubernetes HTTP probes. API frozen. |
| `httputil` | **Stable** | 100.0% | Standardized JSON envelopes and error responders. API frozen. |
| `maputil` | **Stable** | 100.0% | Generic map helpers (Merge, Filter). API frozen. |
| `metrics` | **Stable** | *Interfaces* | Framework-agnostic Counter, Gauge, Histogram interfaces. API frozen. |
| `pagination` | **Stable** | 100.0% | Offset and page query parsing and envelopes. API frozen. |
| `rbac` | **Stable** | 100.0% | Role-based access control engine with wildcard matching. Low risk. |
| `rbaccontext` | **Stable** | 100.0% | Context permission attachment and evaluation helpers. Low risk. |
| `recovery` | **Stable** | 100.0% | Panic recovery middleware with structured logging. Low risk. |
| `requestid` | **Stable** | 100.0% | W3C correlation ID middleware and context propagator. API frozen. |
| `retry` | **Stable** | 90.7% | Context-aware exponential backoff with full jitter. Low risk. |
| `securityheaders` | **Stable** | 100.0% | OWASP recommended defensive HTTP security headers. Low risk. |
| `shutdown` | **Stable** | 100.0% | Graceful OS signal interception and teardown coordinator. Low risk. |
| `sliceutil` | **Stable** | 100.0% | Generic algorithmic slice primitives (Reduce, GroupBy, Chunk, Unique, Flatten, First). API frozen. |
| `stringutil` | **Stable** | 100.0% | Masking and cryptographically secure random strings. API frozen. |
| `timeutil` | **Stable** | 100.0% | UTC/location time arithmetic, business day calculation, and heuristic parsing. |
| `validation` | **Stable** | 100.0% | Formatter translating validator/v10 errors into client JSON. |
| `workerpool` | **Stable** | 92.1% | Bounded concurrent worker pool with task queue. Low risk. |
| `cache` | **Early** | 87.5% | Singleflight cache unified into `TypedCache` with `EvictionPolicy`. API undergoing stabilization. |
| `clock` | **Early** | 93.9% | Mockable time abstraction with RealClock and advanceable FakeClock. API undergoing stabilization. |
| `db` | **Early** | 100.0% | pgx connection pool wrapper and transaction runner. Hardened against connection leaks. |
| `httpclient` | **Early** | 92.0% | Resilient composed HTTP client with rate limiting, circuit breaking, and retries. API undergoing stabilization. |
| `idempotency` | **Early** | 98.8% | In-memory and PostgreSQL distributed idempotency key store with bounded TTL response eviction. |
| `logger` | **Early** | 95.0% | Context-aware structured logging with slog, sampling, and sensitive field redaction. |
| `ratelimit` | **Early** | 100.0% | In-memory rate limiting with Token Bucket and Sliding Window algorithms. |
| `telemetry` | **Early** | 100.0% | OpenTelemetry distributed tracing wrapper with global and instance-scoped provider support. |

---

## 3. Path to `v1.0.0`

We will not cut `v1.0.0` arbitrarily or against a calendar deadline. `v1.0.0` marks a permanent backward-compatibility commitment under the Go module compatibility promise.

Before releasing `v1.0.0`, all of the following conditions must be met:

1. **Package Stabilization**: All packages currently marked **Early** (`cache`, `clock`, `db`, `httpclient`, `idempotency`, `logger`, `ratelimit`, `telemetry`) must reach **Stable** status, with zero breaking API modifications across two consecutive minor releases.
2. **Production Validation**: Demonstration of stable production deployment across multiple independent backend services without memory leaks, goroutine leaks, or lock contention regressions.
3. **Complete Godoc & Examples**: 100% of exported packages must feature runnable, testable `Example*()` functions validated by `go test`.
4. **Truth-Gate Pass**: Zero drift in documentation, coverage, and symbol verification scripts across all quality gates.
