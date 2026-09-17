// SPDX-License-Identifier: MIT

package recovery_test

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	mw := recovery.Middleware()
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("test panic")
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.Equal(t, "application/json; charset=utf-8", rec.Header().Get("Content-Type"))

	var body map[string]string
	err := json.Unmarshal(rec.Body.Bytes(), &body)
	require.NoError(t, err)
	assert.Equal(t, "Internal Server Error", body["error"])
	assert.NotEmpty(t, body["message"])
}

func TestWithLogger(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(os.Stderr, nil))
	mw := recovery.Middleware(recovery.WithLogger(logger))
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("test panic logger")
	}))

	req := httptest.NewRequest(http.MethodGet, "/test-logger", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestMiddlewareSafe(t *testing.T) {
	mw := recovery.Middleware()
	called := false
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))

	req := httptest.NewRequest(http.MethodGet, "/safe", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.True(t, called)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "ok", rec.Body.String())
}

func TestMiddleware_PanicErrorAndSpanRecording(t *testing.T) {
	span := &mockSpan{}
	mw := recovery.Middleware()
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic(errors.New("fatal database crash"))
	}))

	req := httptest.NewRequest(http.MethodGet, "/crash", nil)
	req = req.WithContext(trace.ContextWithSpan(req.Context(), span))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.Equal(t, codes.Error, span.status)
	assert.Len(t, span.errors, 1)
	assert.Equal(t, "fatal database crash", span.errors[0].Error())
	assert.Equal(t, "/crash", span.attrs["http.route"].AsString())
	assert.Equal(t, int64(http.StatusInternalServerError), span.attrs["http.status_code"].AsInt64())
}

func TestMiddleware_PanicUnmatchedRoute(t *testing.T) {
	span := &mockSpan{}
	mw := recovery.Middleware()
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("panic on empty path")
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.URL.Path = ""
	req = req.WithContext(trace.ContextWithSpan(req.Context(), span))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.Equal(t, codes.Error, span.status)
	assert.Equal(t, "unmatched", span.attrs["http.route"].AsString())
}
