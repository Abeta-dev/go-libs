// SPDX-License-Identifier: MIT

package ginmw_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/umesh0492/go-libs/ginmw"
)

func TestREDMetrics_Tracking(t *testing.T) {
	tracker := &ginmw.MetricsTracker{}
	r := gin.New()
	r.Use(ginmw.REDMetrics(tracker))

	r.GET("/ok", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})
	r.GET("/bad", func(c *gin.Context) {
		c.String(http.StatusBadRequest, "bad")
	})
	r.GET("/err", func(c *gin.Context) {
		c.String(http.StatusInternalServerError, "err")
	})
	r.GET("/metrics", ginmw.MetricsEndpoint(tracker))

	// Send requests
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/ok", nil))

	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/bad", nil))

	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/err", nil))

	if tracker.RequestsTotal.Load() != 3 {
		t.Fatalf("expected 3 total requests, got: %d", tracker.RequestsTotal.Load())
	}
	if tracker.Status2xx.Load() != 1 {
		t.Fatalf("expected 1 2xx request, got: %d", tracker.Status2xx.Load())
	}
	if tracker.Status4xx.Load() != 1 {
		t.Fatalf("expected 1 4xx request, got: %d", tracker.Status4xx.Load())
	}
	if tracker.Status5xx.Load() != 1 {
		t.Fatalf("expected 1 5xx request, got: %d", tracker.Status5xx.Load())
	}
	if tracker.ErrorsTotal.Load() != 2 {
		t.Fatalf("expected 2 total errors, got: %d", tracker.ErrorsTotal.Load())
	}

	// Test Prometheus text exposition
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 from /metrics, got: %d", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "http_requests_total") || !strings.Contains(body, "app_uptime_seconds") {
		t.Fatalf("expected prometheus metrics format, got: %s", body)
	}

	// Test JSON exposition
	w = httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/metrics?format=json", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 from /metrics?format=json, got: %d", w.Code)
	}
	var jsonResp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &jsonResp); err != nil {
		t.Fatalf("failed to decode JSON metrics: %v", err)
	}
	if _, ok := jsonResp["http_requests_total"]; !ok {
		t.Fatalf("missing http_requests_total in JSON: %+v", jsonResp)
	}
}
