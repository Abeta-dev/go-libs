// SPDX-License-Identifier: MIT

package health_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/umesh0492/go-libs/health"
)

func TestLivenessHandler_AlwaysReturns200(t *testing.T) {
	r := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()
	health.LivenessHandler(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"status":"ok"`)
}

func TestReadinessHandler_ReturnsRuntimeInfo(t *testing.T) {
	r := httptest.NewRequest("GET", "/ready", nil)
	w := httptest.NewRecorder()
	health.ReadinessHandler(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "go_version")
}

func TestService_AllCheckersPass_Returns200(t *testing.T) {
	svc := health.New(
		health.WithStartTime(time.Now().Add(-10*time.Second)),
		health.WithChecker("db", func(_ context.Context) error { return nil }),
		health.WithRuntimeInfo(),
	)

	r := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()
	svc.Handler(w, r)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp health.Response
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	assert.Equal(t, "healthy", resp.Status)
	assert.Equal(t, "pass", resp.Checks["db"].Status)
	assert.NotEmpty(t, resp.Uptime)
	assert.NotNil(t, resp.Runtime)
}

func TestService_FailingChecker_Returns503(t *testing.T) {
	svc := health.New(
		health.WithChecker("cache", func(_ context.Context) error {
			return errors.New("connection refused")
		}),
	)

	r := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()
	svc.Handler(w, r)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)

	var resp health.Response
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	assert.Equal(t, "degraded", resp.Status)
	assert.Equal(t, "fail", resp.Checks["cache"].Status)
	assert.Contains(t, resp.Checks["cache"].Message, "connection refused")
}

func TestService_NoCheckers_Returns200(t *testing.T) {
	svc := health.New()
	r := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()
	svc.Handler(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestService_CacheControlHeaderSet(t *testing.T) {
	r := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()
	health.LivenessHandler(w, r)
	assert.Equal(t, "no-store", w.Header().Get("Cache-Control"))
}
