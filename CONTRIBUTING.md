# Contributing to go-libs

Thank you for your interest in contributing to `go-libs`!

`go-libs` is an open-source, domain-agnostic foundation of Go 1.25+ libraries for building resilient microservices (resilience, concurrency, security, and transport middleware). We welcome contributions ranging from bug fixes and documentation clarifications to performance optimizations and test coverage improvements.

This guide provides everything you need to know to get started, run the test suite, understand our automated quality gates, and submit a pull request that can be merged smoothly.

---

## 1. Ground Rules & Maintainer Expectations

`go-libs` follows a strict **Truth-Gate Protocol**: every claim in documentation, every coverage number, and every version reference is programmatically asserted against the actual codebase in CI.

- **Zero Breaking Changes**: We preserve backward compatibility across minor releases.
- **Hermetic Tests**: Unit tests must remain fully self-contained and runnable locally without requiring external Docker daemons or cloud services.
- **Maintainer SLA**: We are a small engineering team. We aim to review and respond to first-time contributor pull requests within **5 business days**. Issues are triaged **weekly**.

---

## 2. Development Setup & Prerequisites

### Prerequisites
- **Go**: Version `1.25.0` or higher (`go version`)
- **golangci-lint**: Version `v1.64.0` or higher (`golangci-lint version`)
- **make**: Standard GNU Make (`make --version`)

### Quickstart
1. Fork the repository on GitHub and clone your fork locally:
   ```bash
   git clone https://github.com/YOUR_USERNAME/go-libs.git
   cd go-libs
   ```

2. Run the full verification suite to confirm your local baseline:
   ```bash
   make all
   ```
   This command executes:
   - `make lint`: runs all linters defined in `.golangci.yml` (zero issues allowed).
   - `make test-race`: runs all unit tests with the Go race detector enabled (`-race`).
   - `make coverage`: verifies global statement coverage (>=90%), per-package floor (>=85%), and strict `ginmw` 100.0% coverage.

---

## 3. The Truth-Gate Scripts: What They Check and Why

In addition to standard `go test` and `golangci-lint`, `go-libs` includes two deterministic truth-gate scripts that run in CI:

### A. `./scripts/check_version.sh`
- **What it checks**:
  - Ensures the version in `README.md` matches `CHANGELOG.md` and release tags.
  - Asserts that all 30 packages documented in `README.md` match `go list ./...`.
  - Verifies that every exported symbol listed under `### Added` in `CHANGELOG.md` actually exists in the Go source code.
  - Confirms zero stale references to deprecated Go versions or unreleased major tags.
- **Why it exists**: To ensure that documentation never claims features or APIs that do not exist or have drifted.

### B. `./scripts/check_coverage.sh`
- **What it checks**:
  - Measures statement coverage across all packages.
  - Asserts global repository statement coverage is `>= 90.0%` (currently **95.0%**).
  - Asserts that every individual package meets a minimum floor of `>= 85.0%`.
  - Asserts that the core authentication/middleware package (`ginmw`) maintains **100.0%** statement coverage.
  - **Zero-Drift README Sync**: Parses the per-package coverage table in `README.md` line-by-line and fails if any number deviates from live toolchain measurements.
- **Why it exists**: To prevent coverage numbers in documentation from becoming stale or aspirational.

---

## 4. Submitting a Pull Request

1. **Branch Naming**: Create a topic branch from `main`:
   ```bash
   git checkout -b fix/db-connection-leak
   # or
   git checkout -b docs/clarify-retry-jitter
   ```

2. **Commit Message Discipline**: We follow Conventional Commits:
   - `feat(pkg)`: New exported function, type, or capability
   - `fix(pkg)`: Bug fix with regression test
   - `test(pkg)`: Additional test cases or benchmarks
   - `docs(pkg)`: Documentation corrections or examples
   - `refactor(pkg)`: Code refactoring without public API changes
   - `ci(...)`: Workflow or script updates
   *Rule: Every commit subject must accurately describe its diff.*

3. **Checklist Before Opening a PR**:
   - [ ] `go vet ./...` exits with code 0.
   - [ ] `golangci-lint run ./...` exits with code 0 (zero issues).
   - [ ] `gofmt -l $(git ls-files '*.go')` reports no unformatted files.
   - [ ] `go test -race ./...` passes with zero race warnings.
   - [ ] `./scripts/check_version.sh` exits with code 0.
   - [ ] `./scripts/check_coverage.sh` exits with code 0.
   - [ ] If changing any documented metric or table row, numbers were regenerated via `./scripts/check_coverage.sh`, not hand-typed.
   - [ ] In `examples/microservice`: `go build -v . && go test -v ./...` passes.

4. **Pull Request Template**: Fill in the PR template completely. PRs with failing automated checks will be blocked from merging until resolved.

---

## 5. Need Help?

- **Questions & Discussions**: Open a GitHub Discussion or issue for questions on library design or proposed new packages.
- **Security Inquiries**: Please consult [SECURITY.md](SECURITY.md) for our private vulnerability disclosure process. Do NOT file public issues for security vulnerabilities.

