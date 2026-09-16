// SPDX-License-Identifier: MIT

package telemetry_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/umesh0492/go-libs/telemetry"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
)

func TestMiddleware_Tracing(t *testing.T) {
	tp := noop.NewTracerProvider()
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.TraceContext{})

	mw := telemetry.Middleware("test-service", telemetry.WithPropagator(propagation.TraceContext{}))

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		span := trace.SpanFromContext(r.Context())
		assert.NotNil(t, span)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("traced"))
	}))

	req := httptest.NewRequest(http.MethodGet, "/users/123", nil)
	req.Header.Set("traceparent", "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "traced", rec.Body.String())
}

func TestMiddleware_5xx_ErrorStatus(t *testing.T) {
	tp := noop.NewTracerProvider()
	otel.SetTracerProvider(tp)

	mw := telemetry.Middleware("test-service")

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))

	req := httptest.NewRequest(http.MethodPost, "/fail", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}
