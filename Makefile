.PHONY: all build test test-race test-scale coverage bench fuzz stress lint tidy clean help fmt-check

GOLANGCI_LINT := $(shell which golangci-lint 2>/dev/null || echo $(HOME)/go/bin/golangci-lint)

# Default target
all: fmt-check lint test-race coverage

# Verify that all Go source files are formatted with gofmt
fmt-check:
	@test -z "$$(gofmt -l .)" || (echo "❌ Unformatted files detected. Run 'gofmt -w .':" && gofmt -l . && exit 1)
	@echo "✅ All Go files are formatted with gofmt."

# Run golangci-lint across all packages
lint:
	$(GOLANGCI_LINT) run ./...

# Run standard unit tests (fast, excludes scale tests)
test:
	go test -v ./...

# Run tests with the Go race detector enabled
test-race:
	go test -race ./...

# Run scale and 1-minute load tests
test-scale:
	go test -v -tags=scale -timeout=15m ./examples/microservice/...

# Run comprehensive statement coverage gate script (global >=90%, pkg >=85%, ginmw ==100%)
coverage:
	./scripts/check_coverage.sh

# Run benchmarks and compare against baseline
bench:
	./scripts/check_benchmarks.sh

# Run fuzz tests for 10s each
fuzz:
	go test -fuzz=FuzzRealIP -fuzztime=10s ./ratelimit
	go test -fuzz=FuzzJWTExtract -fuzztime=10s ./ginmw
	go test -fuzz=FuzzErrorChain -fuzztime=10s ./apperror
	go test -fuzz=FuzzDuration -fuzztime=10s ./env
	go test -fuzz=FuzzMaskEmail -fuzztime=10s ./stringutil

# Run resilience concurrency stress matrix under race detector
stress:
	go test -race -count=10 -v ./circuitbreaker/... ./ratelimit/... ./workerpool/... ./cache/... ./idempotency/... ./shutdown/... ./retry/...

# Tidy go module dependencies
tidy:
	go mod tidy

# Clean build and test artifacts
clean:
	rm -f coverage.out coverage_ginmw.out
