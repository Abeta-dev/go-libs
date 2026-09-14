// SPDX-License-Identifier: MIT

package ginmw

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

// RouteLabel returns the registered route template from c.FullPath(), or "unmatched"
// if the route is unhandled or not found. This prevents high-cardinality Prometheus metric
// and trace route label explosions from raw parameterized URL paths.
func RouteLabel(c *gin.Context) string {
	if c == nil {
		return "unmatched"
	}
	if p := c.FullPath(); p != "" {
		return p
	}
	return "unmatched"
}

// Telemetry extracts tracing information from the incoming request headers,
// starts a new root or child span using low-cardinality route naming, injects
// W3C Trace Context (traceparent, tracestate) into outgoing response headers,
// and records errors and panics with codes.Error.
func Telemetry(serviceName string) gin.HandlerFunc {
	tracer := otel.Tracer(serviceName)
	propagator := otel.GetTextMapPropagator()

	return func(c *gin.Context) {
		ctx := propagator.Extract(c.Request.Context(), propagation.HeaderCarrier(c.Request.Header))
		route := RouteLabel(c)
		ctx, span := tracer.Start(ctx, route)
		defer span.End()

		// Inject W3C Trace Context into outgoing response headers so clients and downstream services correlate traces
		propagator.Inject(ctx, propagation.HeaderCarrier(c.Writer.Header()))
		if sc := span.SpanContext(); sc.IsValid() {
			propagation.TraceContext{}.Inject(ctx, propagation.HeaderCarrier(c.Writer.Header()))
		}

		span.SetAttributes(
			attribute.String("http.route", route),
			attribute.String("http.method", c.Request.Method),
		)

		c.Request = c.Request.WithContext(ctx)

		defer func() {
			if r := recover(); r != nil {
				span.SetStatus(codes.Error, fmt.Sprintf("panic: %v", r))
				var err error
				if e, ok := r.(error); ok {
					err = e
				} else {
					err = fmt.Errorf("panic: %v", r)
				}
				span.RecordError(err, trace.WithStackTrace(true))
				panic(r)
			}
		}()

		c.Next()

		status := c.Writer.Status()
		span.SetAttributes(attribute.Int("http.status_code", status))

		if len(c.Errors) > 0 {
			span.SetStatus(codes.Error, c.Errors.String())
			for _, ginErr := range c.Errors {
				span.RecordError(ginErr.Err)
			}
		} else if status >= http.StatusInternalServerError {
			span.SetStatus(codes.Error, fmt.Sprintf("HTTP %d", status))
			span.RecordError(fmt.Errorf("HTTP %d: %s", status, http.StatusText(status)))
		}
	}
}
