// SPDX-License-Identifier: MIT

// Package requestid provides HTTP middleware that injects a unique request ID
// into every request context and response header. This enables end-to-end
// request tracing across services, logs, and error reports.
//
// If the client supplies an X-Request-ID header, that value is preserved so
// the same ID propagates through the entire distributed call chain.
//
// Usage:
//
//	router.Use(requestid.Middleware)
//
//	// In any handler or service:
//	id := requestid.FromContext(r.Context())
//	logger.With("request_id", id).Info("processing request")
package requestid

import (
	"context"
	"net/http"

	"github.com/google/uuid"
)

// Header is the canonical request-ID HTTP header name.
const Header = "X-Request-ID"

type contextKey struct{}

// Middleware injects a request ID into the request context and response headers.
// It is safe to use with any net/http compatible router.
func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get(Header)
		if id == "" {
			id = uuid.New().String()
		}

		// Echo to client so they can correlate frontend logs with server logs
		w.Header().Set(Header, id)

		// Store in context for downstream handlers, services, and logging
		ctx := context.WithValue(r.Context(), contextKey{}, id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// FromContext retrieves the request ID from the context.
// Returns an empty string if no ID is present.
func FromContext(ctx context.Context) string {
	id, _ := ctx.Value(contextKey{}).(string)
	return id
}
