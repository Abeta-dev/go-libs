# AGENTS.md — `go-libs` (`github.com/abeta-dev/go-libs`)

This repository provides **41 focused, production-hardened Go packages** (`v0.6.0`, `Go 1.26.0`, **100.0% statement coverage**) covering HTTP & Gin middleware, stdlib `net/http` parity, SSE streaming, ETag/version-gated caching, loopback CSRF guards, resilience & concurrency, PostgreSQL & embedded SQLite WAL helpers, observability, security & RBAC, and foundational type-safe utilities.

## Skill Routing Table

Load the smallest subskill that matches your task:

| Skill | Path | When to Load |
| :--- | :--- | :--- |
| **`go-libs` (Umbrella)** | [`skills/go-libs/SKILL.md`](skills/go-libs/SKILL.md) | Choosing among the 41 packages, cross-package wiring, or running repository truth gates. |
| **`go-libs-resilience`** | [`skills/go-libs-resilience/SKILL.md`](skills/go-libs-resilience/SKILL.md) | `retry`, `circuitbreaker`, `ratelimit`, `workerpool`, `cache` (prefix/tag invalidation + `FetchStatus`), `async`, `shutdown`, `fsm`, `clock`, `metrics`. |
| **`go-libs-http-gin`** | [`skills/go-libs-http-gin/SKILL.md`](skills/go-libs-http-gin/SKILL.md) | `ginmw` (31 helpers), `httpmw` (stdlib `net/http` parity), `compress` (`PrecompressedFileServer` + SSE-safe compression), `etag`, `loopbackguard`, `sse`, `httpclient`, `httputil`, `pagination`, `idempotency`, `health`, `requestid`, `recovery`, `securityheaders` (`WithDisableHSTS`, `WithCacheControl`), `bodylimit`, `validation`. |
| **`go-libs-data-security`** | [`skills/go-libs-data-security/SKILL.md`](skills/go-libs-data-security/SKILL.md) | `db` (`pgxpool` + `WithTx`, `WithQuerier`, `TxBeginner`), `sqlite` (WAL, `BEGIN IMMEDIATE`, `user_version` migrations, `VACUUM INTO` backup), `rbac`, `rbaccontext`, `jwks`, `cryptoutil`, `telemetry`, `logger`. |
| **`go-libs-foundation`** | [`skills/go-libs-foundation/SKILL.md`](skills/go-libs-foundation/SKILL.md) | `apperror`, `env`, `sliceutil`, `maputil`, `stringutil`, `timeutil`, `uuidutil` (`NewV7`, `NewV7String`). |

## Module Code-Owner Agents & Automated PR Review

Every file and package in `go-libs` is governed by one of **5 Module Code-Owner Agents** defined in [`.github/CODEOWNERS`](.github/CODEOWNERS) and [`.github/module-agents.json`](.github/module-agents.json), and enforced automatically on every PR via [`.github/workflows/module-agent-review.yml`](.github/workflows/module-agent-review.yml) and [`./scripts/module_agent_review.sh`](scripts/module_agent_review.sh):

| Module Code-Owner Agent | Companion Skill | Owned Modules / Scope |
| :--- | :--- | :--- |
| **`@go-libs-foundation-agent`** | [`skills/go-libs-foundation/SKILL.md`](skills/go-libs-foundation/SKILL.md) | `/apperror/`, `/env/`, `/sliceutil/`, `/maputil/`, `/stringutil/`, `/timeutil/`, `/uuidutil/` |
| **`@go-libs-resilience-agent`** | [`skills/go-libs-resilience/SKILL.md`](skills/go-libs-resilience/SKILL.md) | `/retry/`, `/circuitbreaker/`, `/ratelimit/`, `/workerpool/`, `/cache/`, `/async/`, `/shutdown/`, `/fsm/`, `/clock/`, `/metrics/` |
| **`@go-libs-http-gin-agent`** | [`skills/go-libs-http-gin/SKILL.md`](skills/go-libs-http-gin/SKILL.md) | `/ginmw/`, `/httpmw/`, `/compress/`, `/etag/`, `/loopbackguard/`, `/sse/`, `/httpclient/`, `/httputil/`, `/pagination/`, `/idempotency/`, `/health/`, `/requestid/`, `/recovery/`, `/securityheaders/`, `/bodylimit/`, `/validation/` |
| **`@go-libs-data-security-agent`** | [`skills/go-libs-data-security/SKILL.md`](skills/go-libs-data-security/SKILL.md) | `/db/`, `/sqlite/`, `/rbac/`, `/rbaccontext/`, `/jwks/`, `/cryptoutil/`, `/telemetry/`, `/logger/` |
| **`@go-libs-platform-governance-agent`** | [`skills/go-libs/SKILL.md`](skills/go-libs/SKILL.md) | `/.github/`, `/scripts/`, `/docs/`, `/skills/`, `/examples/`, `/deploy/`, `/benchmarks/`, `/AGENTS.md`, `/README.md`, `/CHANGELOG.md`, `/llms.txt`, `/go.mod`, `/go.sum` |

Run the Module Code-Owner Agent review locally against any branch or PR diff:

```bash
./scripts/module_agent_review.sh --base origin/main --head HEAD
```

## Mandatory Verification Gates

Before committing or opening a PR in this repository, always run:

```bash
./scripts/check_secrets.sh
./scripts/check_version.sh
./scripts/check_leaf_imports.sh
./scripts/check_deprecation.sh
./scripts/check_coverage.sh
go test -race ./...
```

- **Secret Scanning Gate**: `scripts/check_secrets.sh` runs TruffleHog secret scanning across both git commit history and the working tree (also enforced in CI via the `secret-scan` job).
- **Package Count Gate**: `scripts/check_version.sh` enforces `EXPECTED_PKG_COUNT=41`. Adding or removing a top-level package requires updating `EXPECTED_PKG_COUNT` in `scripts/check_version.sh` and documenting the package in `README.md`.
- **Leaf Import & DAG Isolation Gate**: `scripts/check_leaf_imports.sh` verifies that all **29 Depth-0 pure internal leaf packages** have zero internal `github.com/abeta-dev/go-libs/*` production imports and all **35 minimal-footprint packages** remain free of `gin`, `pgx`, and `otel` dependencies in an isolated `go.mod` sandbox.
- **Strict `N + 2` Deprecation Window Gate**: `scripts/check_deprecation.sh` enforces `// Deprecated: Since vX.Y.Z (remove in >= vA.B.C). Use <Replacement> instead.` with at least a 2-minor-release (`N + 2`) deprecation window and blocks premature removal against previous release tags.
- **Coverage Gate**: `scripts/check_coverage.sh` enforces `100.0%` statement coverage across all 41 packages and zero drift against `README.md` and `.github/badges/coverage.json`.
- **Version Parity Gate**: All `@v` version references in `README.md`, `CONTRIBUTING.md`, and `docs/**/*.md` must match the latest release tag (`@v0.6.0`).
- **Toolchain Gate**: Requires `Go 1.26.0` (`go.mod` directive `go 1.26.0`).
