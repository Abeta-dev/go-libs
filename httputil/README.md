# `httputil` Package

The `httputil` package provides standardized JSON serialization, status code responses, and domain-to-HTTP error mapping helpers across microservices with zero external dependencies.

## When to Use
- **REST API Response Writing**: Writing JSON responses (`httputil.OK`, `httputil.Created`, `httputil.NoContent`) with correct MIME types and security headers.
- **Domain Error Translation**: Mapping rich domain errors (`apperror.Error` or any `httputil.APIError` implementation) directly to client-safe HTTP status codes and JSON envelopes via `httputil.ErrorFromDomain`.
- **Validation Error Handling**: Returning standardized 400 Bad Request error payloads with `httputil.ValidationError`.

## Why It Is Written Like That
- **Decoupled `APIError` Protocol**: Defines a minimal interface (`GetStatus() int`, `GetCode() string`, `Error() string`) so domain error packages do not need to depend on `net/http`.
- **Consistent Response Envelopes**: Guarantees all error responses follow a uniform `{"error": "...", "code": "..."}` shape across all microservices.
- **Defensive Security Headers**: Automatically applies `Content-Type: application/json; charset=utf-8` and `X-Content-Type-Options: nosniff` to prevent MIME-sniffing vulnerabilities.

## Alternatives Evaluated

| Alternative | Pros | Cons | Why We Chose `go-libs/httputil` |
|---|---|---|---|
| **Raw `json.Marshal` & `w.Write`** | Built into standard library | Verbose boilerplate; error-prone (forgetting status codes or headers); inconsistent JSON shapes | Clean one-liner helpers (`OK`, `Created`, `Error`) with built-in security headers |
| **Framework-Specific Context (`c.JSON`)** | Native to Gin/Echo | Couples transport handlers to specific web frameworks; cannot reuse in pure `net/http` handlers | Framework-agnostic: operates directly on standard `http.ResponseWriter` |
| **Ad-hoc Error Serialization** | Tailored per endpoint | Inconsistent error schemas break frontend API clients; risks leaking internal stack traces | Uniform `ErrResponse` schema with automatic safe fallback to generic 500 on unexpected errors |

---

## Quickstart

### 1. Success Responses
```go
import (
    "net/http"
    "github.com/umesh0492/go-libs/httputil"
)

func GetUserHandler(w http.ResponseWriter, r *http.Request) {
    user, err := userService.Find(r.Context(), "123")
    if err != nil {
        httputil.ErrorFromDomain(w, err)
        return
    }

    httputil.OK(w, user) // 200 OK with application/json
}

func CreateUserHandler(w http.ResponseWriter, r *http.Request) {
    user, err := userService.Create(r.Context(), req)
    if err != nil {
        httputil.ErrorFromDomain(w, err)
        return
    }

    httputil.Created(w, user) // 201 Created
}
```

### 2. Domain Error Mapping
```go
// Automatically inspects apperror.Error code:
// CodeNotFound    -> 404 Not Found {"error": "user not found", "code": "NOT_FOUND"}
// CodeConflict    -> 409 Conflict {"error": "email exists", "code": "CONFLICT"}
// Unknown/Generic -> 500 Internal Error {"error": "internal server error", "code": "INTERNAL_ERROR"}
httputil.ErrorFromDomain(w, err)
```

---

## 🛡️ Edge Cases Handled
- **HTML Injection Defense**: Disables HTML escaping (`SetEscapeHTML(false)`) where raw strings are intended, while enforcing `X-Content-Type-Options: nosniff` to prevent browser content sniffing.
- **Unrecognized Error Protection**: If an unexpected raw error (e.g. database connection failure) reaches `ErrorFromDomain`, it automatically obfuscates internal details and returns a safe HTTP 500 response without leaking internal database topology.
