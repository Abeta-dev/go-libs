# AGENTS.md — `go-libs` (`github.com/abeta-dev/go-libs`)

This repository provides **36 focused, production-hardened Go packages** (`v0.5.0`, `Go 1.26.0`, **100.0% statement coverage**) covering HTTP & Gin middleware, resilience & concurrency, data/caching/observability, security & RBAC, and foundational type-safe utilities.

## Skill Routing Table

Load the smallest subskill that matches your task:

| Skill | Path | When to Load |
| :--- | :--- | :--- |
| **`go-libs` (Umbrella)** | [`skills/go-libs/SKILL.md`](skills/go-libs/SKILL.md) | Choosing among the 36 packages, cross-package wiring, or running repository truth gates. |
| **`go-libs-resilience`** | [`skills/go-libs-resilience/SKILL.md`](skills/go-libs-resilience/SKILL.md) | `retry`, `circuitbreaker`, `ratelimit`, `workerpool`, `cache`, `async`, `shutdown`, `fsm`, `clock`, `metrics`. |
| **`go-libs-http-gin`** | [`skills/go-libs-http-gin/SKILL.md`](skills/go-libs-http-gin/SKILL.md) | `ginmw` (23 middleware helpers), `compress`, `httpclient`, `httputil`, `pagination`, `idempotency`, `health`, `requestid`, `recovery`, `securityheaders`, `bodylimit`, `validation`. |
| **`go-libs-data-security`** | [`skills/go-libs-data-security/SKILL.md`](skills/go-libs-data-security/SKILL.md) | `db` (`pgxpool` + `WithTx`, `WithQuerier`, `TxBeginner`), `rbac`, `rbaccontext`, `jwks`, `cryptoutil`, `telemetry`, `logger`. |
| **`go-libs-foundation`** | [`skills/go-libs-foundation/SKILL.md`](skills/go-libs-foundation/SKILL.md) | `apperror`, `env`, `sliceutil`, `maputil`, `stringutil`, `timeutil`, `uuidutil` (`NewV7`, `NewV7String`). |

## Module Code-Owner Agents & Automated PR Review

Every file and package in `go-libs` is governed by one of **5 Module Code-Owner Agents** defined in [`.github/CODEOWNERS`](.github/CODEOWNERS) and [`.github/module-agents.json`](.github/module-agents.json), and enforced automatically on every PR via [`.github/workflows/module-agent-review.yml`](.github/workflows/module-agent-review.yml) and [`./scripts/module_agent_review.sh`](scripts/module_agent_review.sh):

| Module Code-Owner Agent | Companion Skill | Owned Modules / Scope |
| :--- | :--- | :--- |
| **`@go-libs-foundation-agent`** | [`skills/go-libs-foundation/SKILL.md`](skills/go-libs-foundation/SKILL.md) | `/apperror/`, `/env/`, `/sliceutil/`, `/maputil/`, `/stringutil/`, `/timeutil/`, `/uuidutil/` |
| **`@go-libs-resilience-agent`** | [`skills/go-libs-resilience/SKILL.md`](skills/go-libs-resilience/SKILL.md) | `/retry/`, `/circuitbreaker/`, `/ratelimit/`, `/workerpool/`, `/cache/`, `/async/`, `/shutdown/`, `/fsm/`, `/clock/`, `/metrics/` |
| **`@go-libs-http-gin-agent`** | [`skills/go-libs-http-gin/SKILL.md`](skills/go-libs-http-gin/SKILL.md) | `/ginmw/`, `/compress/`, `/httpclient/`, `/httputil/`, `/pagination/`, `/idempotency/`, `/health/`, `/requestid/`, `/recovery/`, `/securityheaders/`, `/bodylimit/`, `/validation/` |
| **`@go-libs-data-security-agent`** | [`skills/go-libs-data-security/SKILL.md`](skills/go-libs-data-security/SKILL.md) | `/db/`, `/rbac/`, `/rbaccontext/`, `/jwks/`, `/cryptoutil/`, `/telemetry/`, `/logger/` |
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
./scripts/check_coverage.sh
go test -race ./...
```

- **Secret Scanning Gate**: `scripts/check_secrets.sh` runs TruffleHog secret scanning across both git commit history and the working tree (also enforced in CI via the `secret-scan` job).
- **Package Count Gate**: `scripts/check_version.sh` enforces `EXPECTED_PKG_COUNT=36`. Adding or removing a top-level package requires updating `EXPECTED_PKG_COUNT` in `scripts/check_version.sh` and documenting the package in `README.md`.
- **Coverage Gate**: `scripts/check_coverage.sh` enforces `100.0%` statement coverage across all 36 packages and zero drift against `README.md` and `.github/badges/coverage.json`.
- **Version Parity Gate**: All `@v` version references in `README.md`, `CONTRIBUTING.md`, and `docs/**/*.md` must match the latest release tag (`@v0.5.0`).
- **Toolchain Gate**: Requires `Go 1.26.0` (`go.mod` directive `go 1.26.0`).
