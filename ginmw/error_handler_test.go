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
	"github.com/umesh0492/go-libs/apperror"
	"github.com/umesh0492/go-libs/ginmw"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

func TestErrorHandler_NoErrors(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(ginmw.ErrorHandler())
	r.GET("/ok", func(c *gin.Context) {
		c.String(http.StatusOK, "success")
	})

	req := httptest.NewRequest(http.MethodGet, "/ok", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "success", w.Body.String())
}

func TestErrorHandler_AppErrorWithLogger(t *testing.T) {
	gin.SetMode(gin.TestMode)
	span := &otelMockSpan{}

	var logBuf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logBuf, nil))

	r := gin.New()
	r.Use(ginmw.ErrorHandler(ginmw.WithErrorHandlerLogger(logger), nil))
	r.GET("/api/v1/products/:id", func(c *gin.Context) {
		c.Request = c.Request.WithContext(trace.ContextWithSpan(c.Request.Context(), span))
		_ = c.Error(apperror.NotFound("product not found"))
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/products/item-99", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Contains(t, w.Body.String(), "NOT_FOUND")
	assert.Contains(t, w.Body.String(), "product not found")

	// Verify span status and recorded error
	assert.Equal(t, codes.Error, span.status)
	assert.Contains(t, span.statusDesc, "product not found")
	assert.Len(t, span.errors, 1)
	assert.Equal(t, "/api/v1/products/:id", span.attrs["http.route"].AsString())
	assert.Equal(t, int64(404), span.attrs["http.status_code"].AsInt64())

	// Verify logger output
	assert.Contains(t, logBuf.String(), "HTTP error handled")
	assert.Contains(t, logBuf.String(), "product not found")
}

func TestErrorHandler_GenericErrorWithoutLogger(t *testing.T) {
	gin.SetMode(gin.TestMode)
	span := &otelMockSpan{}

	r := gin.New()
	r.Use(ginmw.ErrorHandler())
	r.GET("/generic-fail", func(c *gin.Context) {
		c.Request = c.Request.WithContext(trace.ContextWithSpan(c.Request.Context(), span))
		_ = c.Error(errors.New("unexpected database drop"))
	})

	req := httptest.NewRequest(http.MethodGet, "/generic-fail", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, w.Body.String(), "INTERNAL_ERROR")
	assert.Contains(t, w.Body.String(), "unexpected database drop")

	assert.Equal(t, codes.Error, span.status)
	assert.Equal(t, "unexpected database drop", span.statusDesc)
	assert.Len(t, span.errors, 1)
	assert.Equal(t, int64(500), span.attrs["http.status_code"].AsInt64())
}

func TestErrorHandler_AlreadyWrittenResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)
	span := &otelMockSpan{}

	r := gin.New()
	r.Use(ginmw.ErrorHandler())
	r.GET("/custom-write", func(c *gin.Context) {
		c.Request = c.Request.WithContext(trace.ContextWithSpan(c.Request.Context(), span))
		c.String(http.StatusTeapot, "custom body")
		_ = c.Error(errors.New("custom error after write"))
	})

	req := httptest.NewRequest(http.MethodGet, "/custom-write", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Written body should not be overwritten by ErrorFromDomain
	assert.Equal(t, http.StatusTeapot, w.Code)
	assert.Equal(t, "custom body", w.Body.String())

	// Span should still record error and status code
	assert.Equal(t, codes.Error, span.status)
	assert.Equal(t, "custom error after write", span.statusDesc)
	assert.Len(t, span.errors, 1)
	assert.Equal(t, int64(http.StatusTeapot), span.attrs["http.status_code"].AsInt64())
}
