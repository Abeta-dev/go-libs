---
name: go-libs-http-gin
description: HTTP server/client and Gin middleware patterns using github.com/abeta-dev/go-libs@v0.5.0 (ginmw, compress, httpclient, httputil, pagination, idempotency, health, requestid, recovery, securityheaders, bodylimit, validation — 100.0% statement coverage). Use when building REST APIs, middleware chains, or resilient HTTP clients.
---

# `go-libs-http-gin` Subskill (`@v0.5.0` · `100.0%` coverage)

Packages covered:
- `github.com/abeta-dev/go-libs/ginmw`
- `github.com/abeta-dev/go-libs/compress`
- `github.com/abeta-dev/go-libs/httpclient`
- `github.com/abeta-dev/go-libs/httputil`
- `github.com/abeta-dev/go-libs/pagination`
- `github.com/abeta-dev/go-libs/idempotency`
- `github.com/abeta-dev/go-libs/health`
- `github.com/abeta-dev/go-libs/requestid`
- `github.com/abeta-dev/go-libs/recovery`
- `github.com/abeta-dev/go-libs/securityheaders`
- `github.com/abeta-dev/go-libs/bodylimit`
- `github.com/abeta-dev/go-libs/validation`

## 1. Canonical Gin Middleware Topology (`ginmw`)

Order matters in Gin middleware chains. Follow the canonical pipeline from `docs/ARCHITECTURE.md`:

```
requestid ➔ bodylimit ➔ compress ➔ cors ➔ recovery ➔ logger ➔ telemetry ➔ ratelimit ➔ auth ➔ rbac ➔ idempotency
```

```go
r := gin.New()
r.Use(
	ginmw.RequestID(),                                // 1. Inject/propagate X-Request-ID
	ginmw.LimitBodyDefault(),                         // 2. Cap payload size (2MB default, LimitBodyAuth 4KB)
	ginmw.Compress(),                                 // 2b. Brotli (br) & Gzip (gzip) compression + optional decompression
	ginmw.CORS([]string{"https://app.example.com"}),  // 3. Handle OPTIONS preflight early ("*" supported)
	ginmw.Recovery(log),                              // 4. Catch panics and return safe 500 JSON
	ginmw.Logger(),                                   // 5. Structured request completion logging + context enrichment
	ginmw.Telemetry("order-service"),                 // 6. Root OpenTelemetry span & W3C propagation
	ginmw.REDMetrics("order_service"),                // 6b. Prometheus RED metrics (pair with ginmw.MetricsEndpoint())
	ginmw.GlobalRateLimit(),                          // 7. TokenBucket / SlidingWindow IP rate limit (aborts on 429)
	ginmw.AuthMiddleware(),                           // 8. Validate Bearer JWT claims (via SetJWTSecret)
	ginmw.RBAC(permissionProvider),                   // 9. Resolve permissions into rbaccontext
	ginmw.Idempotency(idemStore),                     // 10. Two-phase lock on mutating endpoints
)
```

## 2. Brotli & Gzip HTTP Compression (`compress` + `ginmw.Compress`)

- Use `compress.Middleware(cfg...)` on standard `net/http` routers or `ginmw.Compress(cfg...)` on Gin routers.
- Negotiates RFC 9110 `Accept-Encoding` quality weights (`compress.Negotiate`), preferring Brotli (`br`) over Gzip (`gzip`), buffers up to `MinSize` (default `1024` bytes), and bounds request decompression via `MaxDecompressedBytes` (default `10 MiB`).

## 3. Standardized JSON Envelopes & Validation (`httputil` + `validation`)

- Return consistent JSON envelopes using `httputil.OK(w, data)`, `httputil.Created(w, data)`, `httputil.NoContent(w)`, and `httputil.ErrorFromDomain(w, err)`.
- Format `validator/v10` errors into clean client field diagnostics using `validation.FormatErrors(err)`.

## 4. Cursor & Offset Pagination (`pagination`)

- Use `pagination.Parse(r)` and `pagination.NewTypedResponse(items, total, params)` for bounded offset pagination.
- Use `pagination.ParseCursor(r)`, `pagination.EncodeCursor`, `pagination.DecodeCursor`, and `pagination.NewTypedCursorResponse` for keyset cursor pagination on large tables.

## 5. Kubernetes Health Probes (`health`)

```go
h := health.New(health.WithChecker("db", func(ctx context.Context) error {
	return pool.Ping(ctx)
}))
r.GET("/healthz", gin.WrapF(health.LivenessHandler))
r.GET("/readyz", gin.WrapF(h.Handler))
```
