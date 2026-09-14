// SPDX-License-Identifier: MIT

// Package recovery provides panic-trapping middleware with structured stack trace logging.
package recovery

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"runtime/debug"

	"github.com/gin-gonic/gin"
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

// Middleware returns a Gin middleware that recovers from any panics and writes a generic 500 JSON API contract.
// It structurally logs the stack trace and marks active OpenTelemetry spans with codes.Error and exception events.
func Middleware(opts ...Option) gin.HandlerFunc {
	cfg := &config{
		logger: slog.New(slog.NewJSONHandler(os.Stderr, nil)),
	}
	for _, opt := range opts {
		opt(cfg)
	}

	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				route := c.FullPath()
				if route == "" {
					route = "unmatched"
				}

				var recErr error
				if e, ok := err.(error); ok {
					recErr = e
				} else {
					recErr = fmt.Errorf("panic: %v", err)
				}

				span := trace.SpanFromContext(c.Request.Context())
				if span != nil {
					span.SetStatus(codes.Error, fmt.Sprintf("panic: %v", err))
					span.RecordError(recErr, trace.WithStackTrace(true))
					span.SetAttributes(
						attribute.String("http.route", route),
						attribute.Int("http.status_code", http.StatusInternalServerError),
					)
				}

				cfg.logger.Error("PANIC RECOVERED",
					slog.Any("error", err),
					slog.String("stack", string(debug.Stack())),
					slog.String("route", route),
					slog.String("path", c.Request.URL.Path),
				)

				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
					"error":   "Internal Server Error",
					"message": "The server encountered an unexpected condition that prevented it from fulfilling the request.",
				})
			}
		}()
		c.Next()
	}
}
