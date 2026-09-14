// SPDX-License-Identifier: MIT

package ginmw

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

// RecoveryOption defines functional configuration for the Recovery middleware.
type RecoveryOption func(*recoveryConfig)

type recoveryConfig struct {
	logger *slog.Logger
}

// WithRecoveryLogger sets a custom structured logger for panic logging.
func WithRecoveryLogger(l *slog.Logger) RecoveryOption {
	return func(c *recoveryConfig) {
		c.logger = l
	}
}

// Recovery returns a Gin middleware that traps panics, marks the active OpenTelemetry
// span with codes.Error and an exception event, logs the stack trace with structured attributes,
// and returns an HTTP 500 Internal Server Error JSON response.
func Recovery(opts ...RecoveryOption) gin.HandlerFunc {
	cfg := &recoveryConfig{
		logger: slog.New(slog.NewJSONHandler(os.Stderr, nil)),
	}
	for _, opt := range opts {
		if opt != nil {
			opt(cfg)
		}
	}

	return func(c *gin.Context) {
		defer func() {
			if r := recover(); r != nil {
				route := RouteLabel(c)

				var err error
				if e, ok := r.(error); ok {
					err = e
				} else {
					err = fmt.Errorf("panic: %v", r)
				}

				stack := string(debug.Stack())

				span := trace.SpanFromContext(c.Request.Context())
				if span != nil {
					span.SetStatus(codes.Error, fmt.Sprintf("panic: %v", r))
					span.RecordError(err, trace.WithStackTrace(true))
					span.SetAttributes(
						attribute.String("http.route", route),
						attribute.Int("http.status_code", http.StatusInternalServerError),
					)
				}

				cfg.logger.Error("PANIC RECOVERED",
					slog.Any("error", err),
					slog.String("stack", stack),
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
