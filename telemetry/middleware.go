// SPDX-License-Identifier: MIT

package telemetry

import (
	"net/http"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
	"go.opentelemetry.io/otel/trace"
)

// MiddlewareOptions configures HTTP tracing middleware.
type MiddlewareOptions struct {
	Propagator propagation.TextMapPropagator
}

// MiddlewareOption sets configuration parameters for tracing middleware.
type MiddlewareOption func(*MiddlewareOptions)

// WithPropagator overrides the TextMapPropagator used to extract incoming trace context.
func WithPropagator(p propagation.TextMapPropagator) MiddlewareOption {
	return func(o *MiddlewareOptions) {
		if p != nil {
			o.Propagator = p
		}
	}
}

type statusRecorder struct {
	http.ResponseWriter
	statusCode int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.statusCode = code
	r.ResponseWriter.WriteHeader(code)
}

// Middleware creates standard net/http middleware that instruments HTTP requests with OpenTelemetry spans.
// It extracts W3C trace context from request headers, starts a server span, records HTTP status attributes,
// and sets error status on 5xx responses.
func Middleware(tracerName string, opts ...MiddlewareOption) func(http.Handler) http.Handler {
	cfg := MiddlewareOptions{
		Propagator: otel.GetTextMapPropagator(),
	}

	for _, opt := range opts {
		opt(&cfg)
	}

	tracer := otel.Tracer(tracerName)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := cfg.Propagator.Extract(r.Context(), propagation.HeaderCarrier(r.Header))

			spanName := r.Method + " " + r.URL.Path
			ctx, span := tracer.Start(
				ctx,
				spanName,
				trace.WithSpanKind(trace.SpanKindServer),
				trace.WithAttributes(
					semconv.HTTPRequestMethodKey.String(r.Method),
					semconv.URLPath(r.URL.Path),
					attribute.String("http.user_agent", r.UserAgent()),
				),
			)
			defer span.End()

			rec := &statusRecorder{
				ResponseWriter: w,
				statusCode:     http.StatusOK,
			}

			next.ServeHTTP(rec, r.WithContext(ctx))

			span.SetAttributes(semconv.HTTPResponseStatusCode(rec.statusCode))
			if rec.statusCode >= 500 {
				span.SetStatus(codes.Error, http.StatusText(rec.statusCode))
			} else {
				span.SetStatus(codes.Ok, "")
			}
		})
	}
}
