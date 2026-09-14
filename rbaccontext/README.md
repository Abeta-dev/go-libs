# `rbaccontext` Package

The `rbaccontext` package provides context-key helpers that store and evaluate permission sets loaded by authentication/authorization middleware, allowing handlers, services, and repositories to interrogate permissions via standard `context.Context` without importing Gin or the database.

## When to Use
- **Fine-Grained Authorization**: Checking whether an authenticated caller possesses specific permissions (`orders:create`, `invoices:approve`) inside business service layers.
- **Transport-Agnostic Auth**: Interrogating user permissions in domain services or gRPC/CLI handlers that do not have access to Gin HTTP contexts.
- **Multi-Permission Policy Verification**: Evaluating whether a user satisfies single (`Can`) or multiple alternative (`CanAny`) permission rules.

## Why It Is Written Like That
- **$O(1)$ Hash Set Lookup**: Converts raw permission slices into `map[string]struct{}` upon injection, ensuring subsequent permission evaluations are instant hash lookups rather than $O(N)$ linear scans.
- **Pure `context.Context` Integration**: Decouples business logic completely from HTTP frameworks, allowing domain services to remain pure Go standard library.
- **Fail-Closed Default**: Returns `false` immediately if the context is `nil` or has no permissions attached, preventing accidental permission grants.

## Alternatives Evaluated

| Alternative | Pros | Cons | Why We Chose `go-libs/rbaccontext` |
|---|---|---|---|
| **Direct Gin Context Querying (`c.Get`)** | Readily accessible in handlers | Couples domain logic to Gin framework; cannot evaluate permissions in asynchronous jobs or gRPC | Uses standard library `context.Context` for universal compatibility |
| **Checking Roles Instead of Permissions** | Simpler role strings (`admin`, `editor`) | Coarse-grained; role bloat occurs as features expand; breaking changes when roles are redefined | Fine-grained permission strings (`resource:action`) allow dynamic role reconfiguration |
| **Direct Database Query per Permission Check** | Always 100% fresh from DB | Massive database read amplification (10-20 queries per HTTP request) | Permissions loaded once at middleware ingress and cached in request context |

---

## API

```go
import "github.com/umesh0492/go-libs/rbaccontext"

// Injected by ginmw.RBAC — you never call this directly in service code
ctx = rbaccontext.WithPermissions(ctx, []string{"orders:create", "orders:read", "documents:view"})

// Single permission check — use in handlers and services
if !rbaccontext.Can(ctx, "orders:create") {
    return httputil.Forbidden(w, "Missing orders:create permission")
}

// Multi-permission check — passes if the user has ANY one of the listed perms
if !rbaccontext.CanAny(ctx, "report:view", "finance:read") {
    return httputil.Forbidden(w, "Insufficient permissions for reports")
}
```

---

## Usage Patterns

### In an HTTP Handler (most common)

```go
func (h *OrderHandler) Create(c *gin.Context) {
    // RBAC middleware has already loaded permissions into c.Request.Context()
    if !rbaccontext.Can(c.Request.Context(), "orders:create") {
        c.AbortWithStatusJSON(403, gin.H{"error": "Missing orders:create permission"})
        return
    }
    // ... proceed
}
```

### In a Service (for complex multi-step operations)

```go
func (s *OrderService) Approve(ctx context.Context, orderID, actorID uuid.UUID) error {
    // The context is passed from the handler; permissions are already in it
    if !rbaccontext.Can(ctx, "orders:approve") {
        return domain.ErrForbidden
    }
    // ... proceed
}
```

### In router.go (route-level — prefer RequirePermission instead)

```go
// For simple single-permission guards, use ginmw.RequirePermission (cleaner)
api.POST("/orders", ginmw.RequirePermission("orders:create"), orderHandler.Create)

// Only use rbaccontext in handlers/services when the check is conditional on data
```

---

## Edge Cases

| Condition | `Can` result | Reason |
|---|---|---|
| No JWT (unauthenticated request) | `false` | RBAC injects empty set; any `Can` returns false |
| DB error during permission load | Request aborted 500 | RBAC middleware fails closed — never assumes empty |
| User demoted between requests | `false` from next request onward | Permissions loaded fresh every request from DB |
| Case mismatch (`"Orders:Create"` vs `"orders:create"`) | `false` | Exact string match — always use constants from `domain/permission.go` |
| `CanAny` with empty list | `false` | No permissions to satisfy |

---

## Permission String Convention

All permission strings follow `"resource:action"` lowercase:

```go
// domain/permission.go (downstream services — never in go-libs)
const (
    PermOrderCreate    = "orders:create"
    PermOrderRead      = "orders:read"
    PermOrderApprove   = "orders:approve"
    PermDocumentView   = "documents:view"
    // ... one constant per permission row in DB
)
```

Do **not** hard-code permission strings in handlers. Always import from `domain/permission.go`.
