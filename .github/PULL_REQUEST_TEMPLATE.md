## Description
<!-- Provide a concise summary of the changes introduced by this PR. -->

## Related Issue
<!-- Fixes #(issue) or Relates to #(issue) -->

## Package(s) Affected
- [ ] `apperror`
- [ ] `bodylimit`
- [ ] `cache`
- [ ] `circuitbreaker`
- [ ] `clock`
- [ ] `cryptoutil`
- [ ] `db`
- [ ] `env`
- [ ] `ginmw`
- [ ] `health`
- [ ] `httpclient`
- [ ] `httputil`
- [ ] `idempotency`
- [ ] `logger`
- [ ] `maputil`
- [ ] `metrics`
- [ ] `pagination`
- [ ] `ratelimit`
- [ ] `rbac` / `rbaccontext`
- [ ] `recovery`
- [ ] `requestid`
- [ ] `retry`
- [ ] `securityheaders`
- [ ] `shutdown`
- [ ] `sliceutil`
- [ ] `stringutil`
- [ ] `telemetry`
- [ ] `timeutil`
- [ ] `validation`
- [ ] `workerpool`

## Quality Checklist
- [ ] `make lint` passed with 0 warnings or errors (`golangci-lint run ./...`).
- [ ] `make test-race` passed with 0 data races (`go test -race ./...`).
- [ ] `./scripts/check_version.sh` passed with exit code 0.
- [ ] `./scripts/check_coverage.sh` passed with exit code 0.
- [ ] **Documented Numbers**: Does this PR touch any documented number (coverage %, package count, benchmark)? If yes, was it regenerated via toolchain scripts, NOT hand-typed?
- [ ] If `ginmw` was touched, statement coverage strictly remains at **100.0%**.
- [ ] Unit tests added or updated to cover all new branches.
- [ ] Godoc comments added for all newly exported identifiers.
- [ ] `Example*()` testable examples added if introducing a new public API.
- [ ] In `examples/microservice`: `go build -v .` and `go test -v ./...` pass cleanly.

