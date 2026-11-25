// SPDX-License-Identifier: MIT

package httpmw

import (
	"context"
	"io/fs"
	"net/http"
	"time"

	"github.com/abeta-dev/go-libs/bodylimit"
	"github.com/abeta-dev/go-libs/compress"
	"github.com/abeta-dev/go-libs/etag"
	"github.com/abeta-dev/go-libs/httputil"
	"github.com/abeta-dev/go-libs/loopbackguard"
	"github.com/abeta-dev/go-libs/ratelimit"
	"github.com/abeta-dev/go-libs/rbaccontext"
	"github.com/abeta-dev/go-libs/requestid"
	"github.com/abeta-dev/go-libs/securityheaders"
	"github.com/abeta-dev/go-libs/sse"
)

// Middleware is the standard net/http middleware function signature.
type Middleware = func(http.Handler) http.Handler

// Chain wraps h with mws in left-to-right execution order:
// Chain(h, m1, m2) executes m1 first, then m2, then h.
// Nil middleware entries are safely skipped.
func Chain(h http.Handler, mws ...Middleware) http.Handler {
	wrapped := h
	for i := len(mws) - 1; i >= 0; i-- {
		if mws[i] != nil {
			wrapped = mws[i](wrapped)
		}
	}
	return wrapped
}

// Use composes multiple middlewares into a single reusable Middleware that
// executes mws in left-to-right order.
func Use(mws ...Middleware) Middleware {
	return func(next http.Handler) http.Handler {
		return Chain(next, mws...)
	}
}

// RequestID injects or propagates X-Request-ID in headers and request context.
func RequestID() Middleware {
	return requestid.Middleware
}

// LimitBody limits incoming request bodies to maxBytes bytes.
func LimitBody(maxBytes int64) Middleware {
	return bodylimit.New(maxBytes)
}

// LimitBodyDefault limits incoming request bodies to 2 MB.
func LimitBodyDefault() Middleware {
	return bodylimit.MB(2)
}

// LimitBodyAuth limits incoming authentication request bodies to 4 KB.
func LimitBodyAuth() Middleware {
	return bodylimit.KB(4)
}

// Compress negotiates Brotli ("br") or Gzip ("gzip") response compression and
// optionally decompresses incoming request bodies (excluding SSE streams).
func Compress(cfg ...compress.Config) Middleware {
	return compress.Middleware(cfg...)
}

// PrecompressedFileServer serves static and SPA assets from rootDir using
// build-time .br / .gz sidecars with optional SPA fallback and CSRF meta injection.
func PrecompressedFileServer(rootDir string, opts ...compress.FileServerOption) http.Handler {
	return compress.PrecompressedFileServer(rootDir, opts...)
}

// PrecompressedFS serves static and SPA assets from fsys (such as embed.FS)
// using build-time .br / .gz sidecars with optional SPA fallback and CSRF meta injection.
func PrecompressedFS(fsys fs.FS, opts ...compress.FileServerOption) http.Handler {
	return compress.PrecompressedFS(fsys, opts...)
}

// ETag computes deterministic ETags, stamps X-App-Version / X-Page-Version headers,
// auto-busts on X-Client-App-Version mismatch, and serves 304 Not Modified.
func ETag(opts ...etag.Option) Middleware {
	return etag.Middleware(opts...)
}

// SecurityHeaders applies OWASP defensive HTTP security headers.
func SecurityHeaders(opts ...securityheaders.Option) Middleware {
	return securityheaders.New(opts...)
}

// LoopbackGuard enforces DNS-rebinding Host allowlisting, Same-Origin checks,
// and constant-time per-boot ephemeral CSRF token validation.
func LoopbackGuard(g *loopbackguard.Guard) Middleware {
	if g == nil {
		return func(next http.Handler) http.Handler { return next }
	}
	return g.Middleware
}

// CORS enforces Cross-Origin Resource Sharing using DefaultCORSConfig for allowedOrigins.
func CORS(allowedOrigins ...string) Middleware {
	return httputil.CORS(httputil.DefaultCORSConfig(allowedOrigins...))
}

// CORSWithConfig enforces Cross-Origin Resource Sharing using an explicit CORSConfig.
func CORSWithConfig(cfg httputil.CORSConfig) Middleware {
	return httputil.CORS(cfg)
}

// RateLimit enforces a custom IP token-bucket rate limit with RFC RateLimit-* headers.
func RateLimit(capacity int, window time.Duration, opts ...ratelimit.Option) Middleware {
	return ratelimit.New(capacity, window, opts...)
}

// GlobalRateLimit enforces the standard global API limit (200 req/min per IP).
func GlobalRateLimit(opts ...ratelimit.Option) Middleware {
	return ratelimit.NewGlobal(opts...)
}

// AuthRateLimit enforces the strict authentication limit (10 req/min per IP).
func AuthRateLimit(opts ...ratelimit.Option) Middleware {
	return ratelimit.NewAuth(opts...)
}

// SSE adapts fn into a standard net/http Server-Sent Events HandlerFunc.
func SSE(fn func(ctx context.Context, stream *sse.Stream) error, opts ...sse.Option) http.HandlerFunc {
	return sse.Handler(fn, opts...)
}

// RequirePermission aborts with HTTP 403 JSON if the request context lacks perm.
func RequirePermission(perm string) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !rbaccontext.Can(r.Context(), perm) {
				writeJSONStatus(w, http.StatusForbidden, `{"error":"Forbidden","code":"FORBIDDEN"}`)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// RequireAnyPermission aborts with HTTP 403 JSON unless the request context holds
// at least one of perms.
func RequireAnyPermission(perms ...string) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !rbaccontext.CanAny(r.Context(), perms...) {
				writeJSONStatus(w, http.StatusForbidden, `{"error":"Forbidden","code":"FORBIDDEN"}`)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func writeJSONStatus(w http.ResponseWriter, status int, body string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_, _ = w.Write([]byte(body))
}
