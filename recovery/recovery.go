// SPDX-License-Identifier: MIT

// Package recovery provides panic-trapping middleware with structured stack trace logging.
package recovery

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"runtime/debug"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// Option defines a functional configuration modifier for the recovery telemetry.
type Option func(*config)

type config struct {
	logger *slog.Logger
}

// WithLogger overrides the default stderr structured logger natively.
func WithLogger(l *slog.Logger) Option {
	return func(c *config) {
		c.logger = l
	}
}

// Middleware returns a standard net/http middleware that recovers from any panics and writes a generic 500 JSON API contract.
// It structurally logs the stack trace and marks active OpenTelemetry spans with codes.Error and exception events.
func Middleware(opts ...Option) func(http.Handler) http.Handler {
	cfg := &config{
		logger: slog.New(slog.NewJSONHandler(os.Stderr, nil)),
	}
	for _, opt := range opts {
		if opt != nil {
			opt(cfg)
		}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rec := recover(); rec != nil {
					route := r.Pattern
					if route == "" {
						route = r.URL.Path
					}
					if route == "" {
						route = "unmatched"
					}

					var recErr error
					if e, ok := rec.(error); ok {
						recErr = e
					} else {
						recErr = fmt.Errorf("panic: %v", rec)
					}

					span := trace.SpanFromContext(r.Context())
					if span != nil {
						span.SetStatus(codes.Error, fmt.Sprintf("panic: %v", rec))
						span.RecordError(recErr, trace.WithStackTrace(true))
						span.SetAttributes(
							attribute.String("http.route", route),
							attribute.Int("http.status_code", http.StatusInternalServerError),
						)
					}

					cfg.logger.Error("PANIC RECOVERED",
						slog.Any("error", rec),
						slog.String("stack", string(debug.Stack())),
						slog.String("route", route),
						slog.String("path", r.URL.Path),
					)

					w.Header().Set("Content-Type", "application/json; charset=utf-8")
					w.WriteHeader(http.StatusInternalServerError)
					_ = json.NewEncoder(w).Encode(map[string]string{
						"error":   "Internal Server Error",
						"message": "The server encountered an unexpected condition that prevented it from fulfilling the request.",
					})
				}
			}()

			next.ServeHTTP(w, r)
		})
	}
}
