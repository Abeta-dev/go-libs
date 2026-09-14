// SPDX-License-Identifier: MIT

// Package bodylimit provides an HTTP middleware that enforces a maximum request
// body size. It wraps net/http.MaxBytesReader so that reading beyond the limit
// immediately closes the connection — preventing OOM from malicious payloads.
//
// Usage:
//
//	router.Use(bodylimit.New(2 << 20))           // 2 MB on all routes
//	router.With(bodylimit.KB(4)).Post("/login", h) // 4 KB on login only
package bodylimit

import (
	"fmt"
	"net/http"
)

// New returns a middleware that limits every request body to maxBytes.
// After the limit is hit, further reads return an error and the connection closes.
func New(maxBytes int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Body != nil {
				r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
			}
			next.ServeHTTP(w, r)
		})
	}
}

// MB returns a middleware that limits the body to n megabytes.
func MB(n int64) func(http.Handler) http.Handler {
	return New(n << 20)
}

// KB returns a middleware that limits the body to n kilobytes.
func KB(n int64) func(http.Handler) http.Handler {
	return New(n << 10)
}

// String returns a human-readable description of a byte count.
// Useful for logging middleware configuration at startup.
func String(bytes int64) string {
	switch {
	case bytes >= 1<<20:
		return fmt.Sprintf("%d MB", bytes>>20)
	case bytes >= 1<<10:
		return fmt.Sprintf("%d KB", bytes>>10)
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}
