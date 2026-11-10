---
name: go-libs-foundation
description: Foundational Go utilities and domain error handling using github.com/abeta-dev/go-libs@v0.4.0 (apperror, env, sliceutil, maputil, stringutil, timeutil, uuidutil — 100.0% statement coverage). Use when modeling domain errors, parsing environment config, or manipulating slices, maps, strings, business days, and RFC 9562 UUIDv7 / UUIDv4 identifiers.
---

# `go-libs-foundation` Subskill (`@v0.4.0` · `100.0%` coverage)

Packages covered:
- `github.com/abeta-dev/go-libs/apperror`
- `github.com/abeta-dev/go-libs/env`
- `github.com/abeta-dev/go-libs/sliceutil`
- `github.com/abeta-dev/go-libs/maputil`
- `github.com/abeta-dev/go-libs/stringutil`
- `github.com/abeta-dev/go-libs/timeutil`
- `github.com/abeta-dev/go-libs/uuidutil`

## 1. Domain Error Taxonomy (`apperror`)

Never return raw database or internal errors (`pgx.ErrNoRows`, SQL syntax errors) directly to HTTP clients. Construct typed `*apperror.Error` instances so `httputil.ErrorFromDomain` and `ginmw.ErrorHandler` map them deterministically to HTTP status codes:

| Constructor | Error Code | Typical Use |
| :--- | :--- | :--- |
| `apperror.BadRequest(msg)` | `CodeBadRequest` (`400`) | Malformed input or business rule violation |
| `apperror.Unauthorized(msg)` | `CodeUnauthorized` (`401`) | Missing or expired JWT |
| `apperror.Forbidden(msg)` | `CodeForbidden` (`403`) | RBAC permission denied |
| `apperror.NotFound(msg)` | `CodeNotFound` (`404`) | Entity lookup miss |
| `apperror.Conflict(msg)` | `CodeConflict` (`409`) | Duplicate idempotency key or state conflict |
| `apperror.Internal(cause)` | `CodeInternal` (`500`) | Unexpected infrastructure failure (cause wrapped, never leaked) |

Inspect error codes using `apperror.Is(err, apperror.CodeNotFound)` and `apperror.CodeOf(err)`.

## 2. Typed Environment Configuration (`env`)

Fail fast at process startup (`main.go`) using `env.MustString("DATABASE_URL")`, `env.MustInt("WORKERS")`, `env.String("PORT", "8080")`, `env.Int("MAX_CONNS", 25)`, `env.Bool("DEBUG", false)`, and `env.Duration("TIMEOUT", 5*time.Second)`.

## 3. Generic Slice & Map Operations (`sliceutil`, `maputil`)

- **`sliceutil`**: `sliceutil.Chunk(items, 500)`, `sliceutil.GroupBy(items, keyFn)`, `sliceutil.Unique(items)`, `sliceutil.Flatten(nested)`, `sliceutil.Reduce(items, init, fn)`, and `sliceutil.First(items, pred)`.
  - *(Note: For simple slice mapping/filtering or map keys/values, use standard library `slices`/`maps` packages; do not reference removed `sliceutil` or `maputil` helpers).*
- **`maputil`**: `maputil.Merge(m1, m2)` and `maputil.Filter(m, pred)`.

## 4. String Masking, Time & UUIDv4/UUIDv7 Helpers (`stringutil`, `timeutil`, `uuidutil`)

- **`stringutil`**: `stringutil.MaskEmail(email)`, `stringutil.MaskPhone(phone)`, `stringutil.Truncate(s, n)`, `stringutil.RandomAlphanumeric(n)`, `stringutil.RandomSecureString(n)`.
- **`timeutil`**: `timeutil.AddBusinessDays(t, days)`, `timeutil.BusinessDays(start, end)`, `timeutil.StartOfDay(t)`, `timeutil.EndOfDay(t)`, `timeutil.NowIn(tz)`.
- **`uuidutil`**:
  - Time-ordered RFC 9562 UUIDv7 (recommended for B-Tree database primary keys): `uuidutil.NewV7()` (`uuid.UUID`) and `uuidutil.NewV7String()` (`string`).
  - Random RFC 4122 UUIDv4 & parsing/validation: `uuidutil.New()`, `uuidutil.NewString()`, `uuidutil.Parse(s)`, `uuidutil.ParsePtr(s)`, `uuidutil.MustParse(s)`, `uuidutil.IsValid(s)`, `uuidutil.IsNil(id)`, `uuidutil.Nil`.
