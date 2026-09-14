# `stringutil` Package

The `stringutil` package provides domain-agnostic string manipulation helpers, UTF-8 rune-safe truncation, and privacy-compliant PII masking for email and phone numbers in structured logs.

## When to Use
- **PII Masking for Telemetry & Logging**: Masking sensitive customer email addresses and phone numbers before passing them to `slog` or APM spans.
- **Input Sanitization**: Trimming whitespace and normalizing case for emails and usernames prior to database queries.
- **UTF-8 Safe Truncation**: Truncating descriptions or message previews by runes rather than raw bytes to avoid corrupting multi-byte unicode characters.

## Why It Is Written Like That
- **Unicode / UTF-8 Boundary Safety**: `Truncate` operates on rune boundaries rather than raw byte slices, preventing corrupted half-characters or invalid UTF-8 sequences.
- **GDPR & Privacy Compliance**: Provides standardized `MaskEmail` and `MaskPhone` routines so all microservices mask PII identically without accidental data leakage.
- **Zero Allocations for Identity Cases**: When input strings do not require truncation or modification, functions return the existing string slice without allocating new heap memory.

## Alternatives Evaluated

| Alternative | Pros | Cons | Why We Chose `go-libs/stringutil` |
|---|---|---|---|
| **Standard Library `strings` alone** | Built-in Go package | Lacks PII masking; `s[:n]` byte-slicing panics on multi-byte UTF-8; no `OrDefault` helper | Fills missing utility gaps while remaining 100% compliant with standard library idioms |
| **Ad-hoc Regex Masking in Handlers** | Flexible | Regex evaluation is slow (thousands of ns); prone to incomplete masking or developer bugs | Fast, zero-regex string scanning with audited masking rules |
| **Heavy String Manipulation Libraries** | Hundreds of text helpers | Huge dependency bloat; unnecessary formatting overhead | Focused set of essential utilities with 100% statement coverage |

---

## API

```go
import "github.com/umesh0492/go-libs/stringutil"

// Fallback: returns s if non-empty, otherwise defaultVal
name := stringutil.OrDefault(user.DisplayName, "Unknown User")

// Truncate to n runes (safe for UTF-8)
short := stringutil.Truncate(longDescription, 160)

// PII masking for structured logs
log.Info("login attempt", "email", stringutil.MaskEmail(req.Email))
// "john.doe@example.com" → "jo**@example.com"

log.Info("OTP sent", "phone", stringutil.MaskPhone(req.Phone))
// "+12025550199" → "+*******0199"
// "9876543210"   → "******3210"

// Normalise before DB lookup (always lowercase + trim)
email := stringutil.TrimAndLower(req.Email)

// Blank check (empty or whitespace-only)
if stringutil.IsBlank(req.Remarks) {
    req.Remarks = "No remarks"
}

// Match any of a set of substrings
if stringutil.ContainsAny(filename, ".exe", ".sh", ".bat") {
    return ErrDangerousFileType
}
```

---

## Why not use `strings` directly?

- `MaskEmail` / `MaskPhone` are non-trivial to get right — one shared implementation prevents inconsistent masking across 30+ handler files.
- `TrimAndLower` is a one-liner, but forgetting it causes login failures for `"John@Example.COM"` vs `"john@example.com"`.
- `OrDefault` reads more clearly than ternary idioms.

## 📊 Test Coverage Status
- **Coverage**: `100.0% of statements`
- **Tests**: `stringutil_test.go`, `export_test.go`
