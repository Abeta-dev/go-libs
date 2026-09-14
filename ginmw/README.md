# `ginmw` Package

The `ginmw` package provides unified, production-grade Gin HTTP middleware combining authentication, RBAC authorization, CORS policies, rate limiting, request ID tracing, body size limits, and security headers under a single import path.

## When to Use
- **Gin Web Server Bootstrapping**: Constructing standard HTTP middleware pipelines for Gin microservices.
- **Enterprise JWT Authentication & RBAC**: Enforcing stateless JWT validation and dynamic role-based access control with fail-closed guarantees.
- **Unified Ingress Protection**: Enforcing CORS, body limits, security headers, and rate limiting in the correct architectural execution order.

## Why It Is Written Like That
- **Thin Adapter Over Agnostic Core**: Wraps underlying framework-agnostic `go-libs` engines (`ratelimit`, `requestid`, `securityheaders`, `recovery`) to avoid vendor lock-in to Gin in domain code while giving Gin routes idiomatic middleware handlers.
- **Standardized Execution Pipeline**: Provides tested, recommended middleware ordering to prevent subtle security bugs (e.g. logging un-recovered panics, or reading bodies before size limits).
- **Fail-Closed Security Model**: If the permission provider encounters a database failure or token verification fails, the request is immediately rejected (`401` or `500`) rather than silently succeeding.

## Alternatives Evaluated

| Alternative | Pros | Cons | Why We Chose `go-libs/ginmw` |
|---|---|---|---|
| **Scattered Third-Party Gin Middlewares** | Individual plugins exist | Inconsistent API conventions; disparate versioning; unpredictable middleware execution order | Single unified, vetted package with standardized error responses and telemetry integration |
| **Gin Default Middlewares (`gin.Default()`)** | Built-in logger and recovery | Basic console output; no structured JSON logs; lacks auth, RBAC, CORS, or rate limiting | Production-ready middleware chain with OWASP headers, APM tracing, and structured logging |
| **Custom Handlers per Route** | Maximum flexibility | Code duplication; easy to forget security headers, rate limits, or auth checks on new endpoints | Declarative middleware guarantees uniform defense across all routes |

## Breaking Changes in v0.5.0

| Symbol | v0.4.4 | v0.5.0 |
|---|---|---|
| `Claims.Role string` | Single role string | **Removed** → `Claims.Roles []string` |
| `Claims.UserType` | Present | **Removed** |
| `Claims` multi-tenant | — | `TenantID uuid.UUID`, `Custom map[string]any` |
| JWT access token TTL | 24h | **15 minutes** (or custom parameter) |
| `PermissionProvider` method | `GetPermissionsForRole(ctx, role)` | **`GetPermissionsForUser(ctx, userID, tenantID)`** |
| `RequireType(...)` | Present | **Removed** → use `RequirePermission` or `AdminOnly` |

---

## Full API Reference

### Auth & JWT

```go
// Set once at startup before any token is minted or validated
ginmw.SetJWTSecret(os.Getenv("JWT_SECRET"))

// Mint a signed access token
token, err := ginmw.GenerateToken(&ginmw.Claims{
    UserID:   userID,
    TenantID: tenantID,
    Email:    email,
    Roles:    []string{"admin", "operator"},
}, 15)

// Validate Bearer token, inject *Claims into gin context
router.Use(ginmw.AuthMiddleware())

// Extract claims inside a handler
claims := ginmw.GetClaims(c)  // returns nil if unauthenticated
```

### RBAC

```go
// Implement in consuming microservice (never in go-libs)
type dbProvider struct { pool *pgxpool.Pool }

func (p *dbProvider) GetPermissionsForUser(
    ctx context.Context,
    userID, tenantID uuid.UUID,
) ([]string, error) {
    // Query union of all permissions across all roles for this user+tenant
    return rbacRepo.GetPermissionsForUser(ctx, userID, tenantID)
}

// Mount after AuthMiddleware
api.Use(ginmw.RBAC(&dbProvider{pool}))

// Route-level guard (aborts 403 if permission missing)
api.POST("/orders", ginmw.RequirePermission("order:create"), orderHandler.Create)

// Multi-permission guard (aborts 403 if ALL are missing)
api.GET("/reports", ginmw.RequireAnyPermission("report:view", "finance:read"), reportHandler.List)

// Platform-admin only group
admin := api.Group("/admin", ginmw.AdminOnly())
```

### Rate Limiting

```go
// Global: 200 requests/min per IP — apply to all routes
router.Use(ginmw.GlobalRateLimit())

// Auth: 10 requests/min per IP — apply only to login/refresh routes
authGroup.Use(ginmw.AuthRateLimit())

// The underlying ratelimit.New(capacity, window) is also available for custom limits
```

### Body Limits

```go
// 2 MB maximum — apply to all API routes
router.Use(ginmw.LimitBodyDefault())

// 4 KB maximum — apply to login/register endpoints  
authGroup.Use(ginmw.LimitBodyAuth())
```

### Request ID, CORS, Security, Logging

```go
// X-Request-ID: injects if missing, propagates from upstream if present
router.Use(ginmw.RequestID())

// CORS: exact origins or wildcard-suffix (*.example.com)
router.Use(ginmw.CORS([]string{
    "https://core-web.example.com",
    "http://localhost:3000",
}))

// OWASP security headers: CSP, HSTS, X-Frame-Options, X-Content-Type
router.Use(ginmw.SecurityHeaders("core-api"))

// Access log: method, path, status, latency
router.Use(ginmw.Logger())
```

### Observability, Tracing & Recovery

```go
// Telemetry: extracts W3C traceparent/tracestate, starts root/child span,
// uses low-cardinality route labels (c.FullPath()), propagates W3C headers to response
router.Use(ginmw.Telemetry("core-api"))

// Recovery: traps panics, marks active OTel span with codes.Error and stack trace,
// logs structured panic details, returns HTTP 500 JSON
router.Use(ginmw.Recovery())

// ErrorHandler: inspects c.Errors, marks active OTel span with codes.Error,
// records exception events, and formats client-safe JSON via httputil.ErrorFromDomain
router.Use(ginmw.ErrorHandler())
```

### Recommended Middleware Chain Order

```go
r := gin.New()
r.Use(ginmw.RequestID())                    // 1
r.Use(ginmw.Telemetry("core-api"))          // 2 — OTel distributed tracing & W3C propagation
r.Use(ginmw.Recovery())                     // 3 — Panic recovery with span status marking
r.Use(ginmw.ErrorHandler())                 // 4 — Error translation & span error recording
r.Use(ginmw.SecurityHeaders("core-api"))    // 5
r.Use(ginmw.CORS(allowedOrigins))           // 6
r.Use(ginmw.Logger())                       // 7
r.Use(ginmw.GlobalRateLimit())              // 8
r.Use(ginmw.LimitBodyDefault())             // 9

authGroup.Use(ginmw.AuthRateLimit(), ginmw.LimitBodyAuth())

api.Use(ginmw.AuthMiddleware())
api.Use(ginmw.RBAC(provider))
```

## Edge Cases Handled

| Scenario | Behaviour |
|---|---|
| Missing `Authorization` header | `AuthMiddleware` → `401 UNAUTHORIZED` |
| Expired token | `AuthMiddleware` → `401 TOKEN_EXPIRED` |
| DB error in `RBAC(provider)` | `RBAC` middleware → `500 RBAC_ERROR` (fails closed) |
| Missing JWT (before RBAC) | `RBAC` injects empty permissions; `Can()` returns false |
| User demoted mid-session | Next request: DB returned empty perm, `RequirePermission` → `403` |
| Rate limit exceeded | `429 Too Many Requests` + `Retry-After: 60` header |
| Body over limit | `413 Request Entity Too Large` |
| Preflight OPTIONS | CORS handler returns `204` immediately without touching DB |
