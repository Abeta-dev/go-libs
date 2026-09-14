// SPDX-License-Identifier: MIT

package ginmw_test

import (
	"bytes"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/umesh0492/go-libs/ginmw"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

func TestRecovery_SafeRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(ginmw.Recovery())
	r.GET("/safe", func(c *gin.Context) {
		c.String(http.StatusOK, "all good")
	})

	req := httptest.NewRequest(http.MethodGet, "/safe", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "all good", w.Body.String())
}

func TestRecovery_PanicError_SpanAndRoute(t *testing.T) {
	gin.SetMode(gin.TestMode)
	span := &otelMockSpan{}

	var logBuf bytes.Buffer
	customLogger := slog.New(slog.NewJSONHandler(&logBuf, nil))

	r := gin.New()
	r.Use(ginmw.Recovery(ginmw.WithRecoveryLogger(customLogger), nil))
	r.GET("/api/v1/users/:id", func(c *gin.Context) {
		c.Request = c.Request.WithContext(trace.ContextWithSpan(c.Request.Context(), span))
		panic(errors.New("critical database failure"))
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/users/42", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, w.Body.String(), "Internal Server Error")

	// Verify span status and recorded error
	assert.Equal(t, codes.Error, span.status)
	assert.Contains(t, span.statusDesc, "critical database failure")
	assert.Len(t, span.errors, 1)
	assert.Equal(t, "critical database failure", span.errors[0].Error())

	// Verify route attribute is low-cardinality template
	assert.Equal(t, "/api/v1/users/:id", span.attrs["http.route"].AsString())
	assert.Equal(t, int64(500), span.attrs["http.status_code"].AsInt64())

	// Verify logger output
	assert.Contains(t, logBuf.String(), "PANIC RECOVERED")
	assert.Contains(t, logBuf.String(), "critical database failure")
}

func TestRecovery_PanicString_UnmatchedRoute(t *testing.T) {
	gin.SetMode(gin.TestMode)
	span := &otelMockSpan{}

	r := gin.New()
	r.Use(ginmw.Recovery())
	r.NoRoute(func(c *gin.Context) {
		c.Request = c.Request.WithContext(trace.ContextWithSpan(c.Request.Context(), span))
		panic("panic with string payload")
	})

	req := httptest.NewRequest(http.MethodPost, "/unhandled/path", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Equal(t, codes.Error, span.status)
	assert.Contains(t, span.statusDesc, "panic with string payload")
	assert.Len(t, span.errors, 1)
	assert.Equal(t, "panic: panic with string payload", span.errors[0].Error())
	assert.Equal(t, "unmatched", span.attrs["http.route"].AsString())
}
