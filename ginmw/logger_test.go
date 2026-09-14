// SPDX-License-Identifier: MIT

package ginmw_test

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/umesh0492/go-libs/ginmw"
)

func TestLogger_StructuredLoggingAndRequestID(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var buf bytes.Buffer
	customLogger := slog.New(slog.NewJSONHandler(&buf, nil))

	r := gin.New()
	r.Use(ginmw.Logger(ginmw.WithLogger(customLogger)))
	r.GET("/items", func(c *gin.Context) {
		c.Set("request_id", "req-test-999")
		c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/items", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var logEntry map[string]any
	err := json.Unmarshal(buf.Bytes(), &logEntry)
	assert.NoError(t, err)

	assert.Equal(t, "HTTP request handled", logEntry["msg"])
	assert.Equal(t, "req-test-999", logEntry["request_id"])
	assert.Equal(t, "GET", logEntry["method"])
	assert.Equal(t, "/items", logEntry["path"])
	assert.Equal(t, float64(200), logEntry["status"])
	assert.NotEmpty(t, logEntry["client_ip"])
	assert.NotNil(t, logEntry["latency"])
}

func TestLogger_DefaultAndContextRequestID(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := gin.New()
	r.Use(ginmw.RequestID())
	r.Use(ginmw.Logger(nil))
	r.GET("/ping", func(c *gin.Context) {
		c.String(http.StatusOK, "pong")
	})

	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	req.Header.Set("X-Request-ID", "inbound-trace-123")
	w := httptest.NewRecorder()

	assert.NotPanics(t, func() {
		r.ServeHTTP(w, req)
	})
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "inbound-trace-123", w.Header().Get("X-Request-ID"))
}
