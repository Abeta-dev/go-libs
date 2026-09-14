// SPDX-License-Identifier: MIT

package ratelimit

import (
	"net/http"
	"strconv"
)

// Limiter is the universal interface implemented by all rate limiting algorithms.
type Limiter interface {
	// Allow checks if 1 event is permitted for the given key.
	Allow(key string) bool
	// AllowN checks if n events are permitted for the given key.
	AllowN(key string, n int) bool
	// Remaining returns the estimated number of permitted requests remaining in the current period.
	Remaining(key string) int
	// Reset clears rate limit state for the given key.
	Reset(key string)
}

// KeyFunc extracts a rate limit identifier from an HTTP request.
type KeyFunc func(r *http.Request) string

// DefaultKeyFunc extracts the client IP address from the request.
func DefaultKeyFunc(r *http.Request) string {
	return RealIP(r)
}

// NewWithLimiter creates standard net/http middleware using any Limiter implementation.
func NewWithLimiter(limiter Limiter, keyFunc ...KeyFunc) func(http.Handler) http.Handler {
	kf := DefaultKeyFunc
	if len(keyFunc) > 0 && keyFunc[0] != nil {
		kf = keyFunc[0]
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := kf(r)
			if !limiter.Allow(key) {
				w.Header().Set("RateLimit-Remaining", "0")
				w.Header().Set("X-RateLimit-Remaining", "0")
				w.Header().Set("Retry-After", "1")
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusTooManyRequests)
				_, _ = w.Write([]byte(`{"error":"too many requests","code":"RATE_LIMITED"}`))
				return
			}
			rem := limiter.Remaining(key)
			w.Header().Set("RateLimit-Remaining", strconv.Itoa(rem))
			w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(rem))
			next.ServeHTTP(w, r)
		})
	}
}
