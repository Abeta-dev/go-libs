---
name: go-libs-foundation
description: Foundational Go utilities and domain error handling using github.com/Abeta-dev/go-libs (apperror, env, sliceutil, maputil, stringutil, timeutil, uuidutil). Use when modeling domain errors, parsing environment config, or manipulating slices, maps, strings, dates, and UUIDv7/v4.
---

# `go-libs-foundation` Subskill (`v0.3.2`)

Packages covered:
- `github.com/Abeta-dev/go-libs/apperror`
- `github.com/Abeta-dev/go-libs/env`
- `github.com/Abeta-dev/go-libs/sliceutil`
- `github.com/Abeta-dev/go-libs/maputil`
- `github.com/Abeta-dev/go-libs/stringutil`
- `github.com/Abeta-dev/go-libs/timeutil`
- `github.com/Abeta-dev/go-libs/uuidutil`

## 1. Domain Error Taxonomy (`apperror`)

Never return raw database or internal errors (`pgx.ErrNoRows`, SQL syntax errors) directly to HTTP clients. Wrap them into typed `apperror.Error` constructors so `httputil.WriteError` and `ginmw` map them deterministically to HTTP status codes:

| Constructor | HTTP Status | Typical Use |
| :--- | :--- | :--- |
| `apperror.BadRequest(code, msg)` | `400 Bad Request` | Malformed input or business rule violation |
| `apperror.Unauthorized(code, msg)` | `401 Unauthorized` | Missing or expired JWT |
| `apperror.Forbidden(code, msg)` | `403 Forbidden` | RBAC permission denied |
| `apperror.NotFound(code, msg)` | `404 Not Found` | Entity lookup miss |
| `apperror.Conflict(code, msg)` | `409 Conflict` | Duplicate idempotency key or state transition conflict |
| `apperror.RateLimited(code, msg)` | `429 Too Many Requests` | Rate limit exceeded |
| `apperror.Internal(code, msg, cause)` | `500 Internal Server Error` | Unexpected infrastructure failure (cause logged, never leaked to client) |

## 2. Typed Environment Configuration (`env`)

Fail fast at process startup (`main.go`) using `env.MustGetString`, `env.MustGetDuration`, or `env.GetInt("PORT", 8080)` instead of scattered `os.Getenv` calls inside business packages.

## 3. Time-Ordered Primary Keys (`uuidutil`)

Prefer **UUIDv7** (`uuidutil.NewV7()`) over random UUIDv4 for PostgreSQL B-tree primary keys (`orders.id`, `outbox_events.id`, `audit_events.id`) to preserve insert locality and eliminate index page fragmentation.

## 4. Generic Collection & String Helpers (`sliceutil`, `maputil`, `stringutil`, `timeutil`)

- `sliceutil.Chunk(items, 500)` for batching bulk SQL inserts or API calls.
- `sliceutil.IndexBy(items, keyFn)` and `sliceutil.GroupBy(items, keyFn)` for `O(n)` in-memory joins without nested loops.
- `stringutil.Mask(val, visiblePrefix, visibleSuffix)` for safe logging of identifiers (for Aadhaar/PAN/GSTIN masking, prefer `go-fintech-india`).
