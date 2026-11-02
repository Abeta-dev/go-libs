---
name: go-libs-http-gin
description: HTTP server/client and Gin middleware patterns using github.com/Abeta-dev/go-libs (ginmw, httpclient, httputil, pagination, idempotency, health, requestid, recovery, securityheaders, bodylimit, validation). Use when building REST APIs, middleware chains, or resilient HTTP clients.
---

# `go-libs-http-gin` Subskill (`v0.3.2`)

Packages covered:
- `github.com/Abeta-dev/go-libs/ginmw`
- `github.com/Abeta-dev/go-libs/httpclient`
- `github.com/Abeta-dev/go-libs/httputil`
- `github.com/Abeta-dev/go-libs/pagination`
- `github.com/Abeta-dev/go-libs/idempotency`
- `github.com/Abeta-dev/go-libs/health`
- `github.com/Abeta-dev/go-libs/requestid`
- `github.com/Abeta-dev/go-libs/recovery`
- `github.com/Abeta-dev/go-libs/securityheaders`
- `github.com/Abeta-dev/go-libs/bodylimit`
- `github.com/Abeta-dev/go-libs/validation`

## 1. Recommended Gin Middleware Ordering (`ginmw`)

Order matters in Gin middleware chains. Apply middlewares in this canonical order:

```go
r := gin.New()
r.Use(
	ginmw.RequestID(),                      // 1. Ensure X-Request-ID exists first
	ginmw.CanonicalLogger(slogLogger),      // 2. Emit 1 structured canonical log line per request
	ginmw.Recovery(slogLogger),             // 3. Catch panics and return structured 500 JSON
	ginmw.SecurityHeaders(ginmw.DefaultSecurityHeadersConfig()), // 4. OWASP headers
	ginmw.CORS(corsCfg),                    // 5. Preflight & origin checks
	ginmw.BodyLimit(2 << 20),               // 6. Cap request payloads (e.g. 2 MiB)
	ginmw.RateLimit(limiter),               // 7. Reject abusive traffic early
	ginmw.JWTAuth(jwksValidator),           // 8. Authenticate principal
	ginmw.RBAC(policyEngine),               // 9. Authorize route permission
	ginmw.Idempotency(idemStore),           // 10. Deduplicate mutating POST/PUT/PATCH requests
)
```

## 2. Standardized JSON Envelopes & Validation (`httputil` + `validation`)

- Use `httputil.DecodeJSON(r, &dst)` to enforce strict JSON decoding (`DisallowUnknownFields`) and size limits.
- Accumulate field-level errors with `validation.New()` so callers receive all invalid fields in a single `400 Bad Request` response rather than one field at a time:

```go
v := validation.New()
v.Required("vendor_id", req.VendorID)
v.Positive("amount_paise", req.AmountPaise)
if err := v.Err(); err != nil {
	httputil.WriteError(w, r, err)
	return
}
```

## 3. Cursor & Offset Pagination (`pagination`)

- Prefer **keyset cursor pagination** (`pagination.EncodeCursor` / `pagination.DecodeCursor`) for high-volume tables (`invoices`, `audit_events`) to avoid `OFFSET` scan degradation.
- Always bound `limit` via `pagination.NormalizeLimit(rawLimit, defaultLimit, maxLimit)`.

## 4. Kubernetes Health Probes (`health`)

- Mount `health.LivenessHandler()` on `/healthz` (process alive, zero external I/O).
- Mount `health.ReadinessHandler(probes...)` on `/readyz` (verifies `db.Pool.Ping` and Redis connectivity with bounded timeout).
