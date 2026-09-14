// SPDX-License-Identifier: MIT

package recovery_test

import (
	"errors"
	"log/slog"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/umesh0492/go-libs/recovery"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
)

type mockSpan struct {
	noop.Span
	status codes.Code
	errors []error
	attrs  map[attribute.Key]attribute.Value
}

func (s *mockSpan) IsRecording() bool                   { return true }
func (s *mockSpan) SetStatus(c codes.Code, desc string) { s.status = c }
func (s *mockSpan) RecordError(err error, opts ...trace.EventOption) {
	s.errors = append(s.errors, err)
}
func (s *mockSpan) SetAttributes(kvs ...attribute.KeyValue) {
	if s.attrs == nil {
		s.attrs = make(map[attribute.Key]attribute.Value)
	}
	for _, kv := range kvs {
		s.attrs[kv.Key] = kv.Value
	}
}

func TestMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(recovery.Middleware())
	r.GET("/", func(c *gin.Context) {
		panic("test panic")
	})

	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, 500, rec.Code)
}

func TestWithLogger(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	logger := slog.New(slog.NewJSONHandler(os.Stderr, nil))
	r.Use(recovery.Middleware(recovery.WithLogger(logger)))
	r.GET("/", func(c *gin.Context) {
		panic("test panic logger")
	})

	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, 500, rec.Code)
}

func TestMiddlewareSafe(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(recovery.Middleware())
	r.GET("/", func(c *gin.Context) {
		c.Status(200)
	})

	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, 200, rec.Code)
	time.Sleep(1 * time.Millisecond)
}

func TestMiddleware_PanicErrorAndSpanRecording(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(recovery.Middleware())

	span := &mockSpan{}
	r.GET("/crash", func(c *gin.Context) {
		c.Request = c.Request.WithContext(trace.ContextWithSpan(c.Request.Context(), span))
		panic(errors.New("fatal database crash"))
	})

	req := httptest.NewRequest("GET", "/crash", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, 500, rec.Code)
	assert.Equal(t, codes.Error, span.status)
	assert.Len(t, span.errors, 1)
	assert.Equal(t, "fatal database crash", span.errors[0].Error())
	assert.Equal(t, "/crash", span.attrs["http.route"].AsString())
}

func TestMiddleware_PanicUnmatchedRoute(t *testing.T) {
	gin.SetMode(gin.TestMode)
	span := &mockSpan{}

	r := gin.New()
	r.Use(recovery.Middleware())
	r.NoRoute(func(c *gin.Context) {
		c.Request = c.Request.WithContext(trace.ContextWithSpan(c.Request.Context(), span))
		panic("panic on empty fullpath")
	})

	req := httptest.NewRequest("GET", "/unhandled", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, 500, rec.Code)
	assert.Equal(t, codes.Error, span.status)
	assert.Equal(t, "unmatched", span.attrs["http.route"].AsString())
}
