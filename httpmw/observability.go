// SPDX-License-Identifier: MIT

package httpmw

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"runtime/debug"
	"strings"
	"sync/atomic"
	"time"

	"github.com/abeta-dev/go-libs/logger"
	"github.com/abeta-dev/go-libs/ratelimit"
	"github.com/abeta-dev/go-libs/requestid"
)

// Default tenant extraction constants (parity with ginmw).
const (
	DefaultTenantHeader = "x-tenant-id"
	DefaultTenantQuery  = "tenant_id"
)

type tenantContextKey struct{}

// statusResponseWriter captures status code and bytes written while preserving
// http.Flusher and Unwrap() for SSE and http.ResponseController compatibility.
type statusResponseWriter struct {
	http.ResponseWriter
	statusCode    int
	bytesWritten  int64
	headerWritten bool
}

func (w *statusResponseWriter) WriteHeader(code int) {
	if !w.headerWritten {
		w.statusCode = code
		w.headerWritten = true
		w.ResponseWriter.WriteHeader(code)
	}
}

func (w *statusResponseWriter) Write(p []byte) (int, error) {
	if !w.headerWritten {
		w.WriteHeader(http.StatusOK)
	}
	n, err := w.ResponseWriter.Write(p)
	w.bytesWritten += int64(n)
	return n, err
}

func (w *statusResponseWriter) Flush() {
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func (w *statusResponseWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}

// ── Recovery (pure stdlib, zero OTel) ─────────────────────────────────────────

type recoveryConfig struct {
	logger *slog.Logger
}

// RecoveryOption configures the stdlib Recovery middleware.
type RecoveryOption func(*recoveryConfig)

// WithRecoveryLogger overrides the structured logger used for recovered panics.
func WithRecoveryLogger(l *slog.Logger) RecoveryOption {
	return func(c *recoveryConfig) {
		if l != nil {
			c.logger = l
		}
	}
}

// Recovery traps panics in downstream net/http handlers, logs structured stack
// traces via slog, and returns a safe HTTP 500 JSON response without pulling
// OpenTelemetry or Gin dependencies.
func Recovery(opts ...RecoveryOption) Middleware {
	cfg := recoveryConfig{
		logger: slog.New(slog.NewJSONHandler(os.Stderr, nil)),
	}
	for _, opt := range opts {
		if opt != nil {
			opt(&cfg)
		}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rec := recover(); rec != nil {
					route := r.Pattern
					if route == "" {
						route = r.URL.Path
					}
					cfg.logger.Error("PANIC RECOVERED",
						slog.Any("error", rec),
						slog.String("request_id", requestid.FromContext(r.Context())),
						slog.String("route", route),
						slog.String("path", r.URL.Path),
						slog.String("stack", string(debug.Stack())),
					)
					writeJSONStatus(w, http.StatusInternalServerError,
						`{"error":"Internal Server Error","code":"INTERNAL_ERROR"}`)
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

// ── Logger ────────────────────────────────────────────────────────────────────

type loggerConfig struct {
	logger         *slog.Logger
	trustedProxies []*net.IPNet
}

// LoggerOption configures the stdlib Logger middleware.
type LoggerOption func(*loggerConfig)

// WithLogger sets a custom structured slog.Logger for HTTP access logging.
func WithLogger(l *slog.Logger) LoggerOption {
	return func(c *loggerConfig) {
		c.logger = l
	}
}

// WithTrustedProxies configures trusted proxy CIDRs for RealIP extraction in access logs.
func WithTrustedProxies(cidrs ...string) LoggerOption {
	nets := ratelimit.ParseTrustedProxies(cidrs...)
	return func(c *loggerConfig) {
		c.trustedProxies = append(c.trustedProxies, nets...)
	}
}

// Logger enriches r.Context() with a request-scoped slog.Logger carrying
// request_id and emits structured access logs upon request completion.
func Logger(opts ...LoggerOption) Middleware {
	var cfg loggerConfig
	for _, opt := range opts {
		if opt != nil {
			opt(&cfg)
		}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			baseLogger := cfg.logger
			if baseLogger == nil {
				baseLogger = logger.FromContext(r.Context())
			}

			reqID := requestid.FromContext(r.Context())
			if reqID != "" {
				r = r.WithContext(logger.WithContext(r.Context(), baseLogger.With(slog.String("request_id", reqID))))
			}

			sw := &statusResponseWriter{ResponseWriter: w, statusCode: http.StatusOK}
			next.ServeHTTP(sw, r)

			baseLogger.Info("HTTP request handled",
				slog.String("request_id", reqID),
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
				slog.Int("status", sw.statusCode),
				slog.Int64("bytes", sw.bytesWritten),
				slog.Duration("latency", time.Since(start)),
				slog.String("client_ip", ratelimit.RealIP(r, cfg.trustedProxies...)),
			)
		})
	}
}

// ── RED Metrics & OpenMetrics Endpoint ────────────────────────────────────────

var processStartTime = time.Now()

// MetricsTracker holds lock-free atomic counters and gauges for HTTP RED metrics.
type MetricsTracker struct {
	RequestsTotal  atomic.Uint64
	ErrorsTotal    atomic.Uint64
	Status2xx      atomic.Uint64
	Status4xx      atomic.Uint64
	Status5xx      atomic.Uint64
	ActiveRequests atomic.Int64
	DurationSumMs  atomic.Int64
}

// DefaultMetrics is the package-level default HTTP RED metrics tracker.
var DefaultMetrics = &MetricsTracker{}

// REDMetrics records Rate, Errors, and Duration (RED) metrics for stdlib net/http handlers.
func REDMetrics(tracker ...*MetricsTracker) Middleware {
	t := DefaultMetrics
	if len(tracker) > 0 && tracker[0] != nil {
		t = tracker[0]
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			t.RequestsTotal.Add(1)
			t.ActiveRequests.Add(1)

			sw := &statusResponseWriter{ResponseWriter: w, statusCode: http.StatusOK}
			defer func() {
				t.ActiveRequests.Add(-1)
				t.DurationSumMs.Add(time.Since(start).Milliseconds())
				switch {
				case sw.statusCode >= 200 && sw.statusCode < 300:
					t.Status2xx.Add(1)
				case sw.statusCode >= 400 && sw.statusCode < 500:
					t.Status4xx.Add(1)
					t.ErrorsTotal.Add(1)
				case sw.statusCode >= 500:
					t.Status5xx.Add(1)
					t.ErrorsTotal.Add(1)
				}
			}()

			next.ServeHTTP(sw, r)
		})
	}
}

// MetricsEndpoint returns a standard net/http HandlerFunc exposing RED metrics
// in OpenMetrics Prometheus text format or JSON (?format=json or Accept: application/json).
func MetricsEndpoint(tracker ...*MetricsTracker) http.HandlerFunc {
	t := DefaultMetrics
	if len(tracker) > 0 && tracker[0] != nil {
		t = tracker[0]
	}

	return func(w http.ResponseWriter, r *http.Request) {
		uptime := int64(time.Since(processStartTime).Seconds())
		if strings.Contains(r.Header.Get("Accept"), "application/json") || r.URL.Query().Get("format") == "json" {
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"uptime_seconds":        uptime,
				"http_requests_total":   t.RequestsTotal.Load(),
				"http_errors_total":     t.ErrorsTotal.Load(),
				"http_status_2xx_total": t.Status2xx.Load(),
				"http_status_4xx_total": t.Status4xx.Load(),
				"http_status_5xx_total": t.Status5xx.Load(),
				"http_active_requests":  t.ActiveRequests.Load(),
				"http_duration_sum_ms":  t.DurationSumMs.Load(),
			})
			return
		}

		w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
		var sb strings.Builder
		fmt.Fprintf(&sb, "# HELP app_uptime_seconds Total service uptime in seconds\n# TYPE app_uptime_seconds gauge\napp_uptime_seconds %d\n\n", uptime)
		fmt.Fprintf(&sb, "# HELP http_requests_total Total number of HTTP requests processed\n# TYPE http_requests_total counter\nhttp_requests_total %d\n\n", t.RequestsTotal.Load())
		fmt.Fprintf(&sb, "# HELP http_errors_total Total number of failed HTTP requests\n# TYPE http_errors_total counter\nhttp_errors_total %d\n\n", t.ErrorsTotal.Load())
		fmt.Fprintf(&sb, "# HELP http_status_total HTTP responses partitioned by status class\n# TYPE http_status_total counter\nhttp_status_total{class=\"2xx\"} %d\nhttp_status_total{class=\"4xx\"} %d\nhttp_status_total{class=\"5xx\"} %d\n\n",
			t.Status2xx.Load(), t.Status4xx.Load(), t.Status5xx.Load())
		fmt.Fprintf(&sb, "# HELP http_active_requests Current number of in-flight HTTP requests\n# TYPE http_active_requests gauge\nhttp_active_requests %d\n\n", t.ActiveRequests.Load())
		fmt.Fprintf(&sb, "# HELP http_duration_sum_ms Total request processing time in milliseconds\n# TYPE http_duration_sum_ms counter\nhttp_duration_sum_ms %d\n", t.DurationSumMs.Load())

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(sb.String()))
	}
}

// ── Tenant Enforcement ────────────────────────────────────────────────────────

type tenantConfig struct {
	headerName string
	queryParam string
}

// TenantOption configures RequireTenant middleware.
type TenantOption func(*tenantConfig)

// WithTenantHeader overrides the HTTP header inspected for tenant IDs.
func WithTenantHeader(header string) TenantOption {
	return func(c *tenantConfig) {
		if trimmed := strings.TrimSpace(header); trimmed != "" {
			c.headerName = trimmed
		}
	}
}

// WithTenantQuery overrides the URL query parameter inspected for tenant IDs.
func WithTenantQuery(param string) TenantOption {
	return func(c *tenantConfig) {
		if trimmed := strings.TrimSpace(param); trimmed != "" {
			c.queryParam = trimmed
		}
	}
}

// WithTenantContext binds tenantID to ctx for downstream handlers.
func WithTenantContext(ctx context.Context, tenantID string) context.Context {
	return context.WithValue(ctx, tenantContextKey{}, strings.TrimSpace(tenantID))
}

// TenantIDFromContext retrieves the tenant ID stored in ctx, or "" if absent.
func TenantIDFromContext(ctx context.Context) string {
	id, _ := ctx.Value(tenantContextKey{}).(string)
	return id
}

// RequireTenant resolves a non-empty tenant identifier from context, request
// header (default "x-tenant-id"), or query parameter (default "tenant_id"),
// binds it into r.Context(), or aborts with HTTP 400 TENANT_REQUIRED.
func RequireTenant(opts ...TenantOption) Middleware {
	cfg := tenantConfig{
		headerName: DefaultTenantHeader,
		queryParam: DefaultTenantQuery,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(&cfg)
		}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tenantID := TenantIDFromContext(r.Context())
			if tenantID == "" {
				tenantID = strings.TrimSpace(r.Header.Get(cfg.headerName))
			}
			if tenantID == "" {
				tenantID = strings.TrimSpace(r.URL.Query().Get(cfg.queryParam))
			}
			if tenantID == "" {
				writeJSONStatus(w, http.StatusBadRequest,
					`{"success":false,"error":{"code":"TENANT_REQUIRED","message":"Tenant ID required"}}`)
				return
			}
			next.ServeHTTP(w, r.WithContext(WithTenantContext(r.Context(), tenantID)))
		})
	}
}
