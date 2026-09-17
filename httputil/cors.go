// SPDX-License-Identifier: MIT

package httputil

import (
	"net/http"
	"strings"
)

// CORSConfig configures Cross-Origin Resource Sharing (CORS) behavior.
type CORSConfig struct {
	AllowedOrigins   []string
	AllowedMethods   []string
	AllowedHeaders   []string
	ExposeHeaders    []string
	AllowCredentials bool
}

// DefaultCORSConfig provides standard REST API cross-origin defaults for the provided origins.
func DefaultCORSConfig(allowedOrigins ...string) CORSConfig {
	return CORSConfig{
		AllowedOrigins:   allowedOrigins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Content-Type", "Authorization", "X-Requested-With", "Idempotency-Key"},
		ExposeHeaders:    []string{"Content-Disposition", "Content-Type", "Content-Length", "X-RateLimit-Remaining"},
		AllowCredentials: true,
	}
}

func checkOrigin(origin string, allowedOrigins []string, allowCredentials bool) (allowed, isWildcard bool) {
	for _, o := range allowedOrigins {
		trimmed := strings.TrimSpace(o)
		if trimmed == "*" {
			if !allowCredentials {
				return true, true
			}
			continue
		}
		if trimmed == origin {
			return true, false
		}
		// Wildcard suffix check: "*.example.com"
		if strings.HasPrefix(trimmed, "*.") {
			suffix := trimmed[1:] // ".example.com"
			if strings.HasSuffix(origin, suffix) {
				return true, false
			}
		}
	}
	return false, false
}

// CORS creates universal standard net/http middleware that enforces CORS headers and handles preflight OPTIONS requests.
func CORS(cfg CORSConfig) func(http.Handler) http.Handler {
	methods := strings.Join(cfg.AllowedMethods, ", ")
	headers := strings.Join(cfg.AllowedHeaders, ", ")
	exposed := strings.Join(cfg.ExposeHeaders, ", ")

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")

			if origin != "" {
				allowed, isStar := checkOrigin(origin, cfg.AllowedOrigins, cfg.AllowCredentials)
				if allowed {
					if isStar {
						w.Header().Set("Access-Control-Allow-Origin", "*")
					} else {
						w.Header().Set("Access-Control-Allow-Origin", origin)
						if cfg.AllowCredentials {
							w.Header().Set("Access-Control-Allow-Credentials", "true")
						}
					}
					if methods != "" {
						w.Header().Set("Access-Control-Allow-Methods", methods)
					}
					if headers != "" {
						w.Header().Set("Access-Control-Allow-Headers", headers)
					}
					if exposed != "" {
						w.Header().Set("Access-Control-Expose-Headers", exposed)
					}
				}
			}

			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
