// SPDX-License-Identifier: MIT

package ginmw_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/umesh0492/go-libs/ginmw"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
)

type otelMockSpan struct {
	noop.Span
	name       string
	status     codes.Code
	statusDesc string
	errors     []error
	attrs      map[attribute.Key]attribute.Value
	sc         trace.SpanContext
}

func (s *otelMockSpan) IsRecording() bool              { return true }
func (s *otelMockSpan) SpanContext() trace.SpanContext { return s.sc }
func (s *otelMockSpan) SetStatus(c codes.Code, desc string) {
	s.status = c
	s.statusDesc = desc
}
func (s *otelMockSpan) RecordError(err error, opts ...trace.EventOption) {
	s.errors = append(s.errors, err)
}
func (s *otelMockSpan) SetName(name string) { s.name = name }
func (s *otelMockSpan) SetAttributes(kvs ...attribute.KeyValue) {
	if s.attrs == nil {
		s.attrs = make(map[attribute.Key]attribute.Value)
	}
	for _, kv := range kvs {
		s.attrs[kv.Key] = kv.Value
	}
}

type otelMockTracer struct {
	noop.Tracer
	lastSpan *otelMockSpan
	validSC  bool
}

func (m *otelMockTracer) Start(ctx context.Context, spanName string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	var sc trace.SpanContext
	if m.validSC {
		sc = trace.NewSpanContext(trace.SpanContextConfig{
			TraceID:    trace.TraceID{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16},
			SpanID:     trace.SpanID{1, 2, 3, 4, 5, 6, 7, 8},
			TraceFlags: trace.FlagsSampled,
		})
	}
	s := &otelMockSpan{
		name:  spanName,
		attrs: make(map[attribute.Key]attribute.Value),
		sc:    sc,
	}
	m.lastSpan = s
	return trace.ContextWithSpan(ctx, s), s
}

type otelMockTracerProvider struct {
	noop.TracerProvider
	tracer *otelMockTracer
}

func (p *otelMockTracerProvider) Tracer(name string, opts ...trace.TracerOption) trace.Tracer {
	return p.tracer
}

func TestRouteLabel(t *testing.T) {
	// 1. nil context fallback
	assert.Equal(t, "unmatched", ginmw.RouteLabel(nil))

	// 2. empty FullPath fallback
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	assert.Equal(t, "unmatched", ginmw.RouteLabel(c))

	// 3. registered route FullPath
	r := gin.New()
	var route string
	r.GET("/api/v1/orders/:id", func(ctx *gin.Context) {
		route = ginmw.RouteLabel(ctx)
		ctx.Status(http.StatusOK)
	})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/orders/123", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, "/api/v1/orders/:id", route)
}

func TestTelemetry_SuccessWithW3CPropagation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mockTracer := &otelMockTracer{validSC: true}
	tp := &otelMockTracerProvider{tracer: mockTracer}
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
	))

	r := gin.New()
	r.Use(ginmw.Telemetry("test-service"))
	r.GET("/items/:id", func(c *gin.Context) {
		c.String(http.StatusOK, "item "+c.Param("id"))
	})

	req := httptest.NewRequest(http.MethodGet, "/items/99", nil)
	req.Header.Set("traceparent", "00-0102030405060708090a0b0c0d0e0f10-0102030405060708-01")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "item 99", w.Body.String())

	// Assert W3C Trace Context propagated into response header
	respTraceparent := w.Header().Get("traceparent")
	assert.NotEmpty(t, respTraceparent)
	assert.Contains(t, respTraceparent, "0102030405060708090a0b0c0d0e0f10")

	// Assert low-cardinality route template used as span name
	assert.Equal(t, "/items/:id", mockTracer.lastSpan.name)
	assert.Equal(t, "/items/:id", mockTracer.lastSpan.attrs["http.route"].AsString())
	assert.Equal(t, "GET", mockTracer.lastSpan.attrs["http.method"].AsString())
	assert.Equal(t, int64(200), mockTracer.lastSpan.attrs["http.status_code"].AsInt64())
	assert.Equal(t, codes.Unset, mockTracer.lastSpan.status)
}

func TestTelemetry_Unhandled404_LowCardinality(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mockTracer := &otelMockTracer{validSC: true}
	tp := &otelMockTracerProvider{tracer: mockTracer}
	otel.SetTracerProvider(tp)

	r := gin.New()
	r.Use(ginmw.Telemetry("test-service"))
	// No routes registered

	req := httptest.NewRequest(http.MethodGet, "/unregistered/path/12345", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
	// Must fall back to unmatched rather than raw /unregistered/path/12345
	assert.Equal(t, "unmatched", mockTracer.lastSpan.name)
	assert.Equal(t, "unmatched", mockTracer.lastSpan.attrs["http.route"].AsString())
	assert.Equal(t, int64(404), mockTracer.lastSpan.attrs["http.status_code"].AsInt64())
}

func TestTelemetry_InvalidSpanContext(t *testing.T) {
	gin.SetMode(gin.TestMode)
	// validSC is false, so sc.IsValid() is false
	mockTracer := &otelMockTracer{validSC: false}
	tp := &otelMockTracerProvider{tracer: mockTracer}
	otel.SetTracerProvider(tp)

	r := gin.New()
	r.Use(ginmw.Telemetry("test-service"))
	r.GET("/ping", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.False(t, mockTracer.lastSpan.sc.IsValid())
}

func TestTelemetry_PanicError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mockTracer := &otelMockTracer{validSC: true}
	tp := &otelMockTracerProvider{tracer: mockTracer}
	otel.SetTracerProvider(tp)

	r := gin.New()
	// Outer recovery to catch re-panic from Telemetry
	r.Use(gin.Recovery())
	r.Use(ginmw.Telemetry("test-service"))
	r.GET("/panic-err", func(c *gin.Context) {
		panic(errors.New("database connection refused"))
	})

	req := httptest.NewRequest(http.MethodGet, "/panic-err", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Equal(t, codes.Error, mockTracer.lastSpan.status)
	assert.Contains(t, mockTracer.lastSpan.statusDesc, "database connection refused")
	assert.Len(t, mockTracer.lastSpan.errors, 1)
	assert.Equal(t, "database connection refused", mockTracer.lastSpan.errors[0].Error())
}

func TestTelemetry_PanicString(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mockTracer := &otelMockTracer{validSC: true}
	tp := &otelMockTracerProvider{tracer: mockTracer}
	otel.SetTracerProvider(tp)

	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(ginmw.Telemetry("test-service"))
	r.GET("/panic-str", func(c *gin.Context) {
		panic("raw string panic")
	})

	req := httptest.NewRequest(http.MethodGet, "/panic-str", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Equal(t, codes.Error, mockTracer.lastSpan.status)
	assert.Contains(t, mockTracer.lastSpan.statusDesc, "panic: raw string panic")
	assert.Len(t, mockTracer.lastSpan.errors, 1)
	assert.Equal(t, "panic: raw string panic", mockTracer.lastSpan.errors[0].Error())
}

func TestTelemetry_GinErrors(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mockTracer := &otelMockTracer{validSC: true}
	tp := &otelMockTracerProvider{tracer: mockTracer}
	otel.SetTracerProvider(tp)

	r := gin.New()
	r.Use(ginmw.Telemetry("test-service"))
	r.GET("/gin-err", func(c *gin.Context) {
		_ = c.Error(errors.New("validation failed"))
		_ = c.Error(errors.New("unauthorized field access"))
		c.Status(http.StatusBadRequest)
	})

	req := httptest.NewRequest(http.MethodGet, "/gin-err", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Equal(t, codes.Error, mockTracer.lastSpan.status)
	assert.Len(t, mockTracer.lastSpan.errors, 2)
	assert.Equal(t, "validation failed", mockTracer.lastSpan.errors[0].Error())
	assert.Equal(t, "unauthorized field access", mockTracer.lastSpan.errors[1].Error())
}

func TestTelemetry_Status500WithoutGinError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mockTracer := &otelMockTracer{validSC: true}
	tp := &otelMockTracerProvider{tracer: mockTracer}
	otel.SetTracerProvider(tp)

	r := gin.New()
	r.Use(ginmw.Telemetry("test-service"))
	r.GET("/server-err", func(c *gin.Context) {
		c.Status(http.StatusInternalServerError)
	})

	req := httptest.NewRequest(http.MethodGet, "/server-err", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Equal(t, codes.Error, mockTracer.lastSpan.status)
	assert.Equal(t, "HTTP 500", mockTracer.lastSpan.statusDesc)
	assert.Len(t, mockTracer.lastSpan.errors, 1)
	assert.Contains(t, mockTracer.lastSpan.errors[0].Error(), "500")
}
