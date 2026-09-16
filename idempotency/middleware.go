// SPDX-License-Identifier: MIT

package idempotency

import (
	"bytes"
	"net/http"
	"strings"
)

// DefaultHeaderName is the default HTTP header checked for idempotency keys.
const DefaultHeaderName = "Idempotency-Key"

// MiddlewareOptions configures standard net/http idempotency middleware.
type MiddlewareOptions struct {
	HeaderName        string
	EnforceHeader     bool
	IgnoredMethods    map[string]struct{}
	StatusCodeMatcher func(code int) bool
}

// MiddlewareOption sets configuration parameters for idempotency middleware.
type MiddlewareOption func(*MiddlewareOptions)

// WithHeaderName overrides the header checked for idempotency keys.
func WithHeaderName(header string) MiddlewareOption {
	return func(o *MiddlewareOptions) {
		if header != "" {
			o.HeaderName = header
		}
	}
}

// WithEnforceHeader mandates the presence of the idempotency key for unsafe HTTP methods.
// When enabled, requests without the header return 400 Bad Request.
func WithEnforceHeader(enforce bool) MiddlewareOption {
	return func(o *MiddlewareOptions) {
		o.EnforceHeader = enforce
	}
}

// WithIgnoredMethods specifies methods that bypass idempotency checking (e.g. GET, HEAD, OPTIONS).
func WithIgnoredMethods(methods ...string) MiddlewareOption {
	return func(o *MiddlewareOptions) {
		o.IgnoredMethods = make(map[string]struct{}, len(methods))
		for _, m := range methods {
			o.IgnoredMethods[strings.ToUpper(m)] = struct{}{}
		}
	}
}

// WithStatusCodeMatcher sets a predicate determining whether a response should be cached.
func WithStatusCodeMatcher(matcher func(code int) bool) MiddlewareOption {
	return func(o *MiddlewareOptions) {
		if matcher != nil {
			o.StatusCodeMatcher = matcher
		}
	}
}

type responseRecorder struct {
	http.ResponseWriter
	statusCode int
	body       bytes.Buffer
}

func (rw *responseRecorder) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseRecorder) Write(b []byte) (int, error) {
	rw.body.Write(b)
	return rw.ResponseWriter.Write(b)
}

// Middleware creates universal net/http middleware using any idempotency Store backend.
func Middleware(store Store, opts ...MiddlewareOption) func(http.Handler) http.Handler {
	cfg := MiddlewareOptions{
		HeaderName: DefaultHeaderName,
		IgnoredMethods: map[string]struct{}{
			http.MethodGet:     {},
			http.MethodHead:    {},
			http.MethodOptions: {},
		},
		StatusCodeMatcher: func(code int) bool {
			// Cache 2xx, 3xx, and 4xx responses; 5xx errors unlock to allow retries
			return code >= 200 && code < 500
		},
	}

	for _, opt := range opts {
		opt(&cfg)
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if _, ignored := cfg.IgnoredMethods[r.Method]; ignored {
				next.ServeHTTP(w, r)
				return
			}

			key := strings.TrimSpace(r.Header.Get(cfg.HeaderName))
			if key == "" {
				if cfg.EnforceHeader {
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusBadRequest)
					_, _ = w.Write([]byte(`{"error":"missing idempotency key","code":"MISSING_IDEMPOTENCY_KEY"}`))
					return
				}
				next.ServeHTTP(w, r)
				return
			}

			ctx := r.Context()
			exists, record, err := store.Lock(ctx, key)
			if err != nil {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte(`{"error":"idempotency store error","code":"IDEMPOTENCY_STORE_ERROR"}`))
				return
			}

			if exists && record != nil {
				if record.Status == StatusInProgress {
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusConflict)
					_, _ = w.Write([]byte(`{"error":"idempotent operation already in progress","code":"IDEMPOTENCY_IN_PROGRESS"}`))
					return
				}

				if record.Status == StatusCompleted && record.Response != nil {
					for hKey, hVals := range record.Response.Headers {
						for _, v := range hVals {
							w.Header().Add(hKey, v)
						}
					}
					w.Header().Set("X-Cache-Lookup", "HIT - Idempotency")
					w.WriteHeader(record.Response.StatusCode)
					_, _ = w.Write(record.Response.Body)
					return
				}
			}

			rec := &responseRecorder{
				ResponseWriter: w,
				statusCode:     http.StatusOK,
			}

			defer func() {
				if p := recover(); p != nil {
					_ = store.Unlock(ctx, key)
					panic(p)
				}
			}()

			next.ServeHTTP(rec, r)

			if cfg.StatusCodeMatcher(rec.statusCode) {
				headers := make(map[string][]string, len(rec.Header()))
				for k, v := range rec.Header() {
					headers[k] = append([]string(nil), v...)
				}

				resp := Response{
					StatusCode: rec.statusCode,
					Headers:    headers,
					Body:       rec.body.Bytes(),
				}
				_ = store.Save(ctx, key, resp)
			} else {
				// Server error occurred: release lock so client can retry
				_ = store.Unlock(ctx, key)
			}
		})
	}
}
