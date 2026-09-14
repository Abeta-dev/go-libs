# Good First Issues for External Contributors

Welcome to `go-libs`! If you are looking to make your first contribution, the issues below are self-contained, well-specified tasks that do not require architectural refactoring.

Before starting, please read [CONTRIBUTING.md](../CONTRIBUTING.md) to understand our pull request workflow, code conventions, and automated truth gates.

---

## Issue Template 1: Add Benchmark Suite for `sliceutil.Chunk`

- **Labels**: `good first issue`, `performance`, `benchmarks`
- **Difficulty**: Easy
- **Package**: `sliceutil`

### Problem Statement
`sliceutil.Chunk` partitions any slice into fixed-size chunks using generics. While unit tests and edge cases (such as `size <= 0`) are thoroughly covered, we do not currently have micro-benchmarks measuring allocation behavior across small and large slice sizes.

### Scope & Acceptance Criteria
1. Add `BenchmarkChunk` to `sliceutil/sliceutil_test.go`.
2. Measure chunking performance across 3 distinct sub-benchmarks using `b.Run`:
   - Small slice: 100 elements chunked into sizes of 10.
   - Medium slice: 10,000 elements chunked into sizes of 100.
   - Large slice: 100,000 elements chunked into sizes of 1,000.
3. Use `b.ReportAllocs()` to track memory allocations per operation.
4. Ensure `go test -bench=BenchmarkChunk ./sliceutil/...` runs cleanly and all existing unit tests continue to pass.

### Files to Touch
- `sliceutil/sliceutil_test.go`

---

## Issue Template 2: Add `env.Float64` Typed Environment Variable Helper

- **Labels**: `good first issue`, `enhancement`, `api`
- **Difficulty**: Easy
- **Package**: `env`

### Problem Statement
The `env` package currently provides typed parsers for `String`, `Int`, `Bool`, and `Duration`. Microservices configuring floating-point thresholds (such as machine learning inference cutoffs, sampling probabilities, or circuit breaker failure ratios) currently need to call `strconv.ParseFloat` manually.

### Scope & Acceptance Criteria
1. Add `Float64(key string, fallback float64) float64` and `MustFloat64(key string) float64` in `env/env.go`.
2. Behavior:
   - If the environment variable is unset or empty, return `fallback` (for `Float64`) or panic with descriptive message (for `MustFloat64`).
   - If parsing fails via `strconv.ParseFloat(val, 64)`, return `fallback` (for `Float64`) or panic (for `MustFloat64`).
3. Add unit tests in `env/env_test.go` covering:
   - Value set to valid float (e.g. `"3.14"`, `"0.05"`).
   - Value unset (returns fallback).
   - Value set to non-numeric string (returns fallback or panics for `MustFloat64`).
4. Add runnable `ExampleFloat64` in `env/example_test.go` with `// Output:` verification.
5. Update `CHANGELOG.md` under `## [Unreleased]` -> `### Added`.

### Files to Touch
- `env/env.go`
- `env/env_test.go`
- `env/example_test.go`
- `CHANGELOG.md`

---

## Issue Template 3: Add `stringutil.TruncateWithEllipsis` Helper

- **Labels**: `good first issue`, `enhancement`, `utilities`
- **Difficulty**: Easy
- **Package**: `stringutil`

### Problem Statement
`stringutil.Truncate(s string, maxLen int)` currently truncates strings cleanly at byte length. In user interfaces, log previews, and notification snippets, it is common to append an ellipsis (`...`) when a string is truncated without exceeding the specified maximum length constraint.

### Scope & Acceptance Criteria
1. Add `TruncateWithEllipsis(s string, maxLen int) string` to `stringutil/stringutil.go`.
2. Behavior:
   - If `len(s) <= maxLen`, return `s` unchanged.
   - If `maxLen <= 3`, truncate to `maxLen` characters without ellipsis (or return first `maxLen` bytes).
   - If `len(s) > maxLen`, truncate to `maxLen - 3` and append `"..."` so total length strictly equals `maxLen`.
   - Handle empty strings and unicode/rune boundaries gracefully.
3. Add unit tests in `stringutil/stringutil_test.go` covering:
   - String shorter than `maxLen`.
   - String exactly equal to `maxLen`.
   - String longer than `maxLen`.
   - Edge cases (`maxLen <= 3`, `maxLen <= 0`, multi-byte characters).
4. Add runnable `ExampleTruncateWithEllipsis` in `stringutil/example_test.go` with `// Output:`.
5. Update `CHANGELOG.md` under `## [Unreleased]` -> `### Added`.

### Files to Touch
- `stringutil/stringutil.go`
- `stringutil/stringutil_test.go`
- `stringutil/example_test.go`
- `CHANGELOG.md`
