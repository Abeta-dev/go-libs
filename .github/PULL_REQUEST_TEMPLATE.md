## Summary
<!-- Provide a concise summary of the architectural problem solved and the changes introduced by this PR. -->

## Related Issue
<!-- Fixes #(issue) or Relates to #(issue) -->

## Type of Change
- [ ] `feat`: New package primitive or backward-compatible capability
- [ ] `fix`: Bug fix, concurrency/race fix, or security hardening
- [ ] `perf`: Zero-allocation or throughput optimization with benchmark evidence
- [ ] `docs` / `skills`: Documentation, `llms.txt`, `docs/index.html`, or `skills/*/SKILL.md` sync
- [ ] `ci` / `chore`: Build, dependency, or verification gate update

## Subsystem / Package(s) Affected
- [ ] **HTTP & Gin Middleware**: `ginmw` · `bodylimit` · `compress` · `health` · `httpclient` · `httputil` · `idempotency` · `pagination` · `recovery` · `requestid` · `securityheaders` · `validation`
- [ ] **Resilience & Concurrency**: `async` · `cache` · `circuitbreaker` · `clock` · `fsm` · `metrics` · `ratelimit` · `retry` · `shutdown` · `workerpool`
- [ ] **Data, Security & Observability**: `cryptoutil` · `db` · `jwks` · `logger` · `rbac` · `rbaccontext` · `telemetry`
- [ ] **Foundational Utilities**: `apperror` · `env` · `maputil` · `sliceutil` · `stringutil` · `timeutil` · `uuidutil`
- [ ] **Reference Service & Docs**: `examples/microservice` · `docs/` · `skills/` · `.github/workflows/`

## Mandatory Quality & Security Gates
- [ ] `./scripts/check_secrets.sh` passed with **0 leaked credentials or verified TruffleHog findings**.
- [ ] `./scripts/check_version.sh` passed (`vMAJOR.MINOR.PATCH` parity, 36-package BOM sync, `gofmt`, and AST-verified `CHANGELOG.md` symbols).
- [ ] `./scripts/check_leaf_imports.sh` passed (all 21 leaf packages remain free of `gin`, `pgx`, or `otel` dependencies).
- [ ] `./scripts/check_coverage.sh` passed with **100.0% statement coverage** across all 36 packages and zero badge drift.
- [ ] `go test -race ./...` and `(cd examples/microservice && go test -race -cover ./...)` passed with **0 data races** and **100.0% coverage**.
- [ ] `golangci-lint run ./...` passed with **0 issues**.
- [ ] **Documentation & AI Context Sync**: Updated `README.md`, `CHANGELOG.md`, `docs/index.html`, `docs/llms.txt` (`llms.txt`), `AGENTS.md`, and `skills/*/SKILL.md` where applicable.
- [ ] **Public API Contract**: Added Godoc comments and testable `Example*()` functions for all newly exported symbols.
