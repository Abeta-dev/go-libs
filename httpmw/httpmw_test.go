// SPDX-License-Identifier: MIT

package httpmw_test

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/abeta-dev/go-libs/compress"
	"github.com/abeta-dev/go-libs/etag"
	"github.com/abeta-dev/go-libs/httpmw"
	"github.com/abeta-dev/go-libs/httputil"
	"github.com/abeta-dev/go-libs/loopbackguard"
	"github.com/abeta-dev/go-libs/rbaccontext"
	"github.com/abeta-dev/go-libs/securityheaders"
	"github.com/abeta-dev/go-libs/sse"
)

func TestChainAndCoreMiddlewares(t *testing.T) {
	var logBuf bytes.Buffer
	customLog := slog.New(slog.NewJSONHandler(&logBuf, nil))
	tracker := &httpmw.MetricsTracker{}

	guard, err := loopbackguard.New(loopbackguard.WithCSRFToken("desktop-csrf"))
	if err != nil {
		t.Fatal(err)
	}

	pipeline := httpmw.Use(
		nil,
		httpmw.RequestID(),
		httpmw.SecurityHeaders(
			securityheaders.WithDisableHSTS(),
			securityheaders.WithCacheControl("private, no-cache"),
		),
		httpmw.LoopbackGuard(guard),
		httpmw.LoopbackGuard(nil),
		httpmw.LimitBody(1<<20),
		httpmw.LimitBodyDefault(),
		httpmw.LimitBodyAuth(),
		httpmw.CORS("http://127.0.0.1:8080"),
		httpmw.CORSWithConfig(httputil.DefaultCORSConfig("http://127.0.0.1:8080")),
		httpmw.Compress(),
		httpmw.ETag(etag.WithAppVersion("1.0.0")),
		httpmw.REDMetrics(tracker),
		httpmw.Logger(nil, httpmw.WithLogger(customLog), httpmw.WithTrustedProxies("127.0.0.1/32")),
		httpmw.Recovery(nil, httpmw.WithRecoveryLogger(customLog)),
		httpmw.RateLimit(100, time.Minute),
		httpmw.GlobalRateLimit(),
		httpmw.AuthRateLimit(),
	)

	h := pipeline(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		if u, ok := w.(interface{ Unwrap() http.ResponseWriter }); !ok || u.Unwrap() == nil {
			t.Error("expected Unwrap()")
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.WriteHeader(http.StatusAccepted) // duplicate ignored
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	req.Host = "127.0.0.1:8080"
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK || rec.Header().Get("X-Request-ID") == "" {
		t.Fatalf("pipeline failed: code=%d reqID=%q", rec.Code, rec.Header().Get("X-Request-ID"))
	}
	if tracker.RequestsTotal.Load() != 1 || tracker.Status2xx.Load() != 1 {
		t.Fatalf("metrics mismatch: total=%d 2xx=%d", tracker.RequestsTotal.Load(), tracker.Status2xx.Load())
	}
	if !strings.Contains(logBuf.String(), "HTTP request handled") {
		t.Fatalf("expected access log, got %q", logBuf.String())
	}

	// Default logger (no WithLogger) path
	defaultLogH := httpmw.Logger()(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("ok"))
	}))
	recDef := httptest.NewRecorder()
	defaultLogH.ServeHTTP(recDef, httptest.NewRequest(http.MethodGet, "/default-log", nil))
	if recDef.Code != http.StatusOK {
		t.Fatalf("default logger failed: %d", recDef.Code)
	}
}

func TestRecoveryREDMetricsAndEndpoint(t *testing.T) {
	var logBuf bytes.Buffer
	recLog := slog.New(slog.NewJSONHandler(&logBuf, nil))
	tracker := &httpmw.MetricsTracker{}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /panic", func(_ http.ResponseWriter, _ *http.Request) {
		panic("boom")
	})
	mux.HandleFunc("GET /bad", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	})
	mux.HandleFunc("GET /ok", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("ok"))
	})

	h := httpmw.Chain(mux, httpmw.REDMetrics(tracker), httpmw.Recovery(httpmw.WithRecoveryLogger(recLog)))

	// Panic -> 500 JSON + 5xx metric
	recPanic := httptest.NewRecorder()
	h.ServeHTTP(recPanic, httptest.NewRequest(http.MethodGet, "/panic", nil))
	if recPanic.Code != http.StatusInternalServerError || !strings.Contains(logBuf.String(), "PANIC RECOVERED") {
		t.Fatalf("recovery failed: code=%d log=%q", recPanic.Code, logBuf.String())
	}

	// Panic on unmatched route (empty r.Pattern)
	hUnmatched := httpmw.Recovery(httpmw.WithRecoveryLogger(recLog))(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		panic("unmatched-boom")
	}))
	recUnmatched := httptest.NewRecorder()
	hUnmatched.ServeHTTP(recUnmatched, httptest.NewRequest(http.MethodGet, "/raw-panic", nil))
	if recUnmatched.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on unmatched panic, got %d", recUnmatched.Code)
	}

	// 4xx and 2xx metrics
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/bad", nil))
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/ok", nil))

	if tracker.Status5xx.Load() != 1 || tracker.Status4xx.Load() != 1 || tracker.Status2xx.Load() != 1 || tracker.ErrorsTotal.Load() != 2 {
		t.Fatalf("unexpected RED counters: 2xx=%d 4xx=%d 5xx=%d err=%d",
			tracker.Status2xx.Load(), tracker.Status4xx.Load(), tracker.Status5xx.Load(), tracker.ErrorsTotal.Load())
	}

	// DefaultMetrics middleware + endpoint
	httpmw.REDMetrics()(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})).ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/def", nil))
	recDefEp := httptest.NewRecorder()
	httpmw.MetricsEndpoint()(recDefEp, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if recDefEp.Code != http.StatusOK || !strings.Contains(recDefEp.Body.String(), "http_requests_total") {
		t.Fatalf("default MetricsEndpoint failed: code=%d", recDefEp.Code)
	}

	// Prometheus text exposition
	recProm := httptest.NewRecorder()
	httpmw.MetricsEndpoint(tracker)(recProm, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if recProm.Code != http.StatusOK || !strings.Contains(recProm.Body.String(), `http_status_total{class="5xx"} 1`) {
		t.Fatalf("prometheus metrics failed: %s", recProm.Body.String())
	}

	// JSON exposition
	recJSON := httptest.NewRecorder()
	reqJSON := httptest.NewRequest(http.MethodGet, "/metrics?format=json", nil)
	httpmw.MetricsEndpoint(tracker)(recJSON, reqJSON)
	var snap map[string]any
	if err := json.Unmarshal(recJSON.Body.Bytes(), &snap); err != nil || snap["http_requests_total"] != float64(3) {
		t.Fatalf("json metrics failed: err=%v body=%s", err, recJSON.Body.String())
	}
}

func TestPermissionsTenantSSEAndStaticHelpers(t *testing.T) {
	// 1. RequirePermission & RequireAnyPermission
	permH := httpmw.Chain(
		http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }),
		httpmw.RequirePermission("invoices:write"),
		httpmw.RequireAnyPermission("invoices:write", "admin:*"),
	)

	recDeny1 := httptest.NewRecorder()
	permH.ServeHTTP(recDeny1, httptest.NewRequest(http.MethodPost, "/inv", nil))
	if recDeny1.Code != http.StatusForbidden {
		t.Fatalf("expected 403 without permission, got %d", recDeny1.Code)
	}

	reqAllow := httptest.NewRequest(http.MethodPost, "/inv", nil)
	reqAllow = reqAllow.WithContext(rbaccontext.WithPermissions(reqAllow.Context(), []string{"invoices:write"}))
	recAllow := httptest.NewRecorder()
	permH.ServeHTTP(recAllow, reqAllow)
	if recAllow.Code != http.StatusOK {
		t.Fatalf("expected 200 with permission, got %d", recAllow.Code)
	}

	anyDeny := httpmw.RequireAnyPermission("billing:admin")(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	recDeny2 := httptest.NewRecorder()
	anyDeny.ServeHTTP(recDeny2, reqAllow)
	if recDeny2.Code != http.StatusForbidden {
		t.Fatalf("expected 403 on RequireAnyPermission miss, got %d", recDeny2.Code)
	}

	// 2. RequireTenant (missing, header, query, existing context)
	var gotTenant string
	tenantH := httpmw.RequireTenant(
		nil,
		httpmw.WithTenantHeader("X-Org-ID"),
		httpmw.WithTenantQuery("org_id"),
	)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotTenant = httpmw.TenantIDFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	recMissing := httptest.NewRecorder()
	tenantH.ServeHTTP(recMissing, httptest.NewRequest(http.MethodGet, "/t", nil))
	if recMissing.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 when tenant missing, got %d", recMissing.Code)
	}

	reqHdr := httptest.NewRequest(http.MethodGet, "/t", nil)
	reqHdr.Header.Set("X-Org-ID", "org-1")
	recHdr := httptest.NewRecorder()
	tenantH.ServeHTTP(recHdr, reqHdr)
	if recHdr.Code != http.StatusOK || gotTenant != "org-1" {
		t.Fatalf("header tenant failed: code=%d tenant=%q", recHdr.Code, gotTenant)
	}

	recQ := httptest.NewRecorder()
	tenantH.ServeHTTP(recQ, httptest.NewRequest(http.MethodGet, "/t?org_id=org-2", nil))
	if recQ.Code != http.StatusOK || gotTenant != "org-2" {
		t.Fatalf("query tenant failed: code=%d tenant=%q", recQ.Code, gotTenant)
	}

	reqCtx := httptest.NewRequest(http.MethodGet, "/t", nil)
	reqCtx = reqCtx.WithContext(httpmw.WithTenantContext(reqCtx.Context(), "org-ctx"))
	recCtx := httptest.NewRecorder()
	tenantH.ServeHTTP(recCtx, reqCtx)
	if recCtx.Code != http.StatusOK || gotTenant != "org-ctx" {
		t.Fatalf("context tenant failed: code=%d tenant=%q", recCtx.Code, gotTenant)
	}

	// 3. SSE & Precompressed static file server helpers
	recSSE := httptest.NewRecorder()
	httpmw.SSE(func(_ context.Context, s *sse.Stream) error {
		return s.SendData("tok", "hello")
	})(recSSE, httptest.NewRequest(http.MethodGet, "/sse", nil))
	if recSSE.Code != http.StatusOK || !strings.Contains(recSSE.Body.String(), "data: hello") {
		t.Fatalf("SSE helper failed: %s", recSSE.Body.String())
	}

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte("<h1>spa</h1>"), 0o600); err != nil {
		t.Fatal(err)
	}
	recDisk := httptest.NewRecorder()
	httpmw.PrecompressedFileServer(dir).ServeHTTP(recDisk, httptest.NewRequest(http.MethodGet, "/", nil))
	if recDisk.Code != http.StatusOK || recDisk.Body.String() != "<h1>spa</h1>" {
		t.Fatalf("PrecompressedFileServer failed: %d", recDisk.Code)
	}

	recFS := httptest.NewRecorder()
	httpmw.PrecompressedFS(fstest.MapFS{"app.js": {Data: []byte("1")}}, compress.WithImmutableCacheControl("public")).
		ServeHTTP(recFS, httptest.NewRequest(http.MethodGet, "/app.js", nil))
	if recFS.Code != http.StatusOK || recFS.Body.String() != "1" {
		t.Fatalf("PrecompressedFS failed: %d", recFS.Code)
	}
}
