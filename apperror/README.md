# `apperror` Package

The `apperror` package defines canonical application-level error codes and a structured `Error` type for microservice architectures. Domain layers return `*apperror.Error` instances, which the transport layer automatically maps to client-safe HTTP status codes.

## When to Use
- **Domain & Service Layers**: Returning structured errors from repositories, services, and domain entities without referencing HTTP packages (`net/http`).
- **Error Classification**: Distinguishing between client mistakes (400, 404, 409) and operational/upstream faults (500, 503).
- **Transport Layer Mapping**: Automatically translating domain errors to consistent REST responses via `httputil.ErrorFromDomain`.

## Why It Is Written Like That
- **Domain Purity**: Domain business logic should never import `net/http` or HTTP status constants. Machine-readable `Code` classifications (`CodeNotFound`, `CodeConflict`) keep domain code transport-agnostic.
- **`httputil.APIError` Protocol**: Implements `GetStatus() int` and `GetCode() string`, providing a seamless bridge to HTTP response handlers.
- **Full Cause Preservation**: Implements `Unwrap() error`, ensuring `errors.Is` and `errors.As` inspect the entire root-cause chain across database drivers or external services.

## Canonical Error Codes

| Code | HTTP Status | Meaning |
|---|---|---|
| `CodeBadRequest` | 400 Bad Request | Malformed syntax or invalid request semantics |
| `CodeUnauthorized` | 401 Unauthorized | Missing or expired authentication credentials |
| `CodeForbidden` | 403 Forbidden | Authenticated identity lacks permission for resource |
| `CodeNotFound` | 404 Not Found | Requested entity does not exist |
| `CodeConflict` | 409 Conflict | Resource state conflict (e.g. unique key constraint) |
| `CodeUnprocessable` | 422 Unprocessable | Valid syntax but semantically rejected business rule |
| `CodeInternal` | 500 Internal Error | Unexpected runtime or database failure |
| `CodeUnavailable` | 503 Unavailable | Upstream outage, circuit breaker tripped, queue full |

## Alternatives Evaluated

| Alternative | Pros | Cons | Why We Chose `go-libs/apperror` |
|---|---|---|---|
| **Raw Standard Errors (`fmt.Errorf`)** | Built into Go | Lacks machine-readable codes; forces transport layer to do brittle string matching | Structured `Code` enum enables programmatic handling across API layers |
| **`pkg/errors`** | Popular historical package | Unmaintained and archived; Go 1.13+ native error wrapping superseded it | Modern `Unwrap()` compatibility with standard library error trees |
| **gRPC Status Codes directly** | Universal RPC standard | Couples REST/HTTP microservices to heavy gRPC protobuf runtime | Lightweight, zero-dependency canonical domain codes |

## Quickstart

```go
package service

import (
    "context"
    "fmt"
    "github.com/umesh0492/go-libs/apperror"
)

func (s *OrderService) CancelOrder(ctx context.Context, orderID string) error {
    order, err := s.repo.FindByID(ctx, orderID)
    if err != nil {
        return apperror.Wrap(err, apperror.CodeInternal, "failed to query order")
    }
    if order == nil {
        return apperror.NotFound(fmt.Sprintf("order %s not found", orderID))
    }
    if order.Status == "SHIPPED" {
        return apperror.Conflict("cannot cancel order that has already shipped")
    }
    return s.repo.UpdateStatus(ctx, orderID, "CANCELLED")
}
```

In the HTTP handler layer:
```go
func (h *Handler) CancelOrder(c *gin.Context) {
    err := h.svc.CancelOrder(c.Request.Context(), c.Param("id"))
    if err != nil {
        // Automatically maps CodeNotFound -> 404, CodeConflict -> 409, CodeInternal -> 500
        httputil.ErrorFromDomain(c.Writer, err)
        return
    }
    httputil.OK(c.Writer, map[string]string{"status": "cancelled"})
}
```
