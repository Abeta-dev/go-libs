// SPDX-License-Identifier: MIT

package ginmw

import (
	"log/slog"

	"github.com/gin-gonic/gin"
	"github.com/umesh0492/go-libs/httputil"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// ErrorHandlerOption defines functional configuration for the ErrorHandler middleware.
type ErrorHandlerOption func(*errorHandlerConfig)

type errorHandlerConfig struct {
	logger *slog.Logger
}

// WithErrorHandlerLogger sets a custom structured logger for ErrorHandler.
func WithErrorHandlerLogger(l *slog.Logger) ErrorHandlerOption {
	return func(c *errorHandlerConfig) {
		c.logger = l
	}
}

// ErrorHandler returns a Gin middleware that inspects c.Errors after request processing.
// When errors are present, it marks active OpenTelemetry spans with codes.Error, records
// error events on the span, optionally logs the error, and formats a JSON response using
// httputil.ErrorFromDomain if the response has not already been written.
func ErrorHandler(opts ...ErrorHandlerOption) gin.HandlerFunc {
	cfg := &errorHandlerConfig{}
	for _, opt := range opts {
		if opt != nil {
			opt(cfg)
		}
	}

	return func(c *gin.Context) {
		c.Next()

		if len(c.Errors) == 0 {
			return
		}

		route := RouteLabel(c)
		lastErr := c.Errors.Last().Err

		span := trace.SpanFromContext(c.Request.Context())
		if span != nil {
			span.SetStatus(codes.Error, lastErr.Error())
			for _, ginErr := range c.Errors {
				span.RecordError(ginErr.Err)
			}
			span.SetAttributes(attribute.String("http.route", route))
		}

		if cfg.logger != nil {
			cfg.logger.Error("HTTP error handled",
				slog.Any("error", lastErr),
				slog.String("route", route),
				slog.Int("error_count", len(c.Errors)),
			)
		}

		if !c.Writer.Written() {
			httputil.ErrorFromDomain(c.Writer, lastErr)
			c.Abort()
		}

		if span != nil {
			span.SetAttributes(attribute.Int("http.status_code", c.Writer.Status()))
		}
	}
}
