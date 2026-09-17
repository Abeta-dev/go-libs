// SPDX-License-Identifier: MIT

package logger_test

import (
	"bytes"
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/umesh0492/go-libs/logger"
)

type logCaptureHandler struct {
	records []slog.Record
}

func (h *logCaptureHandler) Enabled(context.Context, slog.Level) bool { return true }
func (h *logCaptureHandler) Handle(_ context.Context, r slog.Record) error {
	h.records = append(h.records, r)
	return nil
}
func (h *logCaptureHandler) WithAttrs([]slog.Attr) slog.Handler { return h }
func (h *logCaptureHandler) WithGroup(string) slog.Handler      { return h }

func TestMiddleware_Logging(t *testing.T) {
	capture := &logCaptureHandler{}
	testLogger := slog.New(capture)

	mw := logger.Middleware(
		logger.WithLogger(testLogger),
		logger.WithRequestIDHeader("X-Correlation-ID"),
		logger.WithExtraAttributes(func(r *http.Request) []slog.Attr {
			return []slog.Attr{slog.String("custom_tag", "val")}
		}),
	)

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify context logger has request_id
		ctxLog := logger.FromContext(r.Context())
		assert.NotNil(t, ctxLog)

		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte("hello-world"))
	}))

	req := httptest.NewRequest(http.MethodPost, "/items", bytes.NewBufferString("payload"))
	req.Header.Set("X-Correlation-ID", "corr-12345")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusCreated, rec.Code)
	assert.Equal(t, "hello-world", rec.Body.String())

	require.Len(t, capture.records, 1)
	record := capture.records[0]
	assert.Equal(t, slog.LevelInfo, record.Level)
	assert.Equal(t, "http request completed", record.Message)

	foundReqID := false
	foundStatus := false
	record.Attrs(func(a slog.Attr) bool {
		if a.Key == "request_id" && a.Value.String() == "corr-12345" {
			foundReqID = true
		}
		if a.Key == "status" && a.Value.Int64() == int64(http.StatusCreated) {
			foundStatus = true
		}
		return true
	})
	assert.True(t, foundReqID)
	assert.True(t, foundStatus)
}

func TestMiddleware_Levels(t *testing.T) {
	t.Run("4xx logs warn", func(t *testing.T) {
		capture := &logCaptureHandler{}
		mw := logger.Middleware(logger.WithLogger(slog.New(capture)))

		handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
		}))

		req := httptest.NewRequest(http.MethodGet, "/fail", nil)
		handler.ServeHTTP(httptest.NewRecorder(), req)

		require.Len(t, capture.records, 1)
		assert.Equal(t, slog.LevelWarn, capture.records[0].Level)
	})

	t.Run("5xx logs error", func(t *testing.T) {
		capture := &logCaptureHandler{}
		mw := logger.Middleware(logger.WithLogger(slog.New(capture)))

		handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))

		req := httptest.NewRequest(http.MethodGet, "/error", nil)
		handler.ServeHTTP(httptest.NewRecorder(), req)

		require.Len(t, capture.records, 1)
		assert.Equal(t, slog.LevelError, capture.records[0].Level)
	})

	t.Run("invalid or malicious request id sanitized", func(t *testing.T) {
		capture := &logCaptureHandler{}
		mw := logger.Middleware(logger.WithLogger(slog.New(capture)))

		handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		// Header containing injection or invalid characters
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("X-Request-ID", "invalid\r\nid\nwith$pecial#chars")
		handler.ServeHTTP(httptest.NewRecorder(), req)

		require.Len(t, capture.records, 1)
		foundReqID := false
		capture.records[0].Attrs(func(a slog.Attr) bool {
			if a.Key == "request_id" {
				foundReqID = true
			}
			return true
		})
		assert.False(t, foundReqID, "invalid header must be discarded and not logged")
	})

	t.Run("oversized request id truncated and sanitized", func(t *testing.T) {
		capture := &logCaptureHandler{}
		mw := logger.Middleware(logger.WithLogger(slog.New(capture)))

		handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		// Header > 128 characters
		longID := string(bytes.Repeat([]byte("a"), 200))
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("X-Request-ID", longID)
		handler.ServeHTTP(httptest.NewRecorder(), req)

		require.Len(t, capture.records, 1)
		var recordedID string
		capture.records[0].Attrs(func(a slog.Attr) bool {
			if a.Key == "request_id" {
				recordedID = a.Value.String()
			}
			return true
		})
		assert.Equal(t, 128, len(recordedID))
	})
}
