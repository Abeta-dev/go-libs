# `securityheaders` Package

The `securityheaders` package provides HTTP middleware that injects hardened, OWASP-compliant defensive security headers into every HTTP response, mitigating common browser-based vulnerabilities such as Clickjacking, MIME sniffing, and Cross-Site Scripting (XSS).

## When to Use
- **Inbound Edge Protection**: Applying defensive HTTP headers to all public and internal REST API responses.
- **OWASP Compliance**: Meeting SOC2, ISO 27001, and PCI-DSS requirements for HTTP transport security headers.
- **Server Version Obfuscation**: Suppressing default Go runtime server tokens to prevent automated security reconnaissance.

## Why It Is Written Like That
- **Strict Default Security Posture**: Out-of-the-box `DefaultConfig` enforces a strict deny-all Content Security Policy (`default-src 'none'; frame-ancestors 'none'`) tailored for pure JSON REST APIs.
- **Framework-Agnostic Core**: Implemented as standard `func(http.Handler) http.Handler` middleware, compatible with standard `net/http` and wrapped by `ginmw.SecurityHeaders`.
- **Zero Allocations on Write**: Header strings are pre-computed at middleware initialization, eliminating string formatting allocations on the critical HTTP request path.

## Alternatives Evaluated

| Alternative | Pros | Cons | Why We Chose `go-libs/securityheaders` |
|---|---|---|---|
| **`unrolled/secure`** | Feature-rich security package | Large external dependency; heavy configuration struct; unnecessary SSL redirection features for containerized backends | Lightweight, zero-dependency, and tailored specifically for containerized Go microservices |
| **Nginx / Ingress Header Rules** | Centralized proxy configuration | Header injection bypassed in local development, unit tests, and internal service-to-service calls | Defense-in-depth: guarantees security headers apply in every environment and test harness |
| **Manual Header Setting in Handlers** | No libraries needed | High risk of developer omission on new routes; inconsistent header values across services | Global middleware guarantees 100% route coverage |

---

## Quickstart

### Standard `net/http`
```go
import (
    "net/http"
    "github.com/umesh0492/go-libs/securityheaders"
)

// Default: strict CSP, no framing, HSTS 2 years
router := http.NewServeMux()
handler := securityheaders.Default(router)

// Or with functional options:
customHandler := securityheaders.New(
    securityheaders.WithServerName("custom-api"),
    securityheaders.WithCSP("default-src 'self'"),
)(router)

http.ListenAndServe(":8080", handler)
```

### Gin Framework Integration
```go
import (
    "github.com/gin-gonic/gin"
    "github.com/umesh0492/go-libs/ginmw"
)

r := gin.New()
r.Use(ginmw.SecurityHeaders("api-service"))
```

---

## Defensive Headers Applied

| Header | Value | Vulnerability Mitigated |
|---|---|---|
| `X-Frame-Options` | `DENY` | Clickjacking attacks |
| `X-Content-Type-Options` | `nosniff` | MIME type sniffing attacks |
| `X-XSS-Protection` | `1; mode=block` | Legacy browser reflected XSS |
| `Referrer-Policy` | `strict-origin-when-cross-origin` | Sensitive URL path leakage |
| `Strict-Transport-Security` | `max-age=63072000; includeSubDomains; preload` | Man-in-the-Middle HTTPS downgrade |
| `Content-Security-Policy` | `default-src 'none'; frame-ancestors 'none';` | Script injection & iframe embedding |
| `Permissions-Policy` | `geolocation=(), microphone=(), camera=()` | Unauthorized hardware feature access |
| `Cache-Control` | `no-store` | Caching of sensitive API payloads |
| `Server` | Custom string (e.g. `api`) | Go version reconnaissance |

---

## 🛡️ Edge Cases Handled
- **Custom Header Overrides**: Configurable via functional options (`securityheaders.WithCSP`, `securityheaders.WithServerName`, `securityheaders.WithHSTSMaxAge`, `securityheaders.WithPermissionsPolicy`) to allow custom CSP directives if a service serves HTML reports or Swagger UI.
- **HTTPS Enforcement in Proxied Clouds**: Applies HSTS headers appropriately when operating behind TLS-terminating cloud balancers.
