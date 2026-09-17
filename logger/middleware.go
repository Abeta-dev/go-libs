// SPDX-License-Identifier: MIT

package logger

import (
	"log/slog"
	"net/http"
	"strings"
	"time"
)

// DefaultRequestIDHeader is the default HTTP header inspected for trace/request correlation.
const DefaultRequestIDHeader = "X-Request-ID"

// MiddlewareOptions configures HTTP request logging.
type MiddlewareOptions struct {
	Logger          *slog.Logger
	RequestIDHeader string
	ExtraAttributes func(r *http.Request) []slog.Attr
}

// MiddlewareOption sets configuration options for logger HTTP middleware.
type MiddlewareOption func(*MiddlewareOptions)

// WithLogger sets the slog.Logger instance to use.
func WithLogger(l *slog.Logger) MiddlewareOption {
	return func(o *MiddlewareOptions) {
		if l != nil {
			o.Logger = l
		}
	}
}

// WithRequestIDHeader specifies the HTTP header name for request correlation.
func WithRequestIDHeader(header string) MiddlewareOption {
	return func(o *MiddlewareOptions) {
		if header != "" {
			o.RequestIDHeader = header
		}
	}
}

// WithExtraAttributes provides custom slog attributes extracted from the request.
func WithExtraAttributes(fn func(r *http.Request) []slog.Attr) MiddlewareOption {
	return func(o *MiddlewareOptions) {
		o.ExtraAttributes = fn
	}
}

type statusRecorder struct {
	http.ResponseWriter
	statusCode   int
	bytesWritten int64
}

func (r *statusRecorder) WriteHeader(code int) {
	r.statusCode = code
	r.ResponseWriter.WriteHeader(code)
}

func (r *statusRecorder) Write(b []byte) (int, error) {
	n, err := r.ResponseWriter.Write(b)
	r.bytesWritten += int64(n)
	return n, err
}

// isAllowedCorrelationHeader guards against extracting and logging arbitrary or sensitive headers.
// Only standard, non-sensitive request correlation headers are permitted.
func isAllowedCorrelationHeader(h string) bool {
	switch strings.ToLower(strings.TrimSpace(h)) {
	case "x-request-id", "x-correlation-id", "x-trace-id", "request-id", "traceparent":
		return true
	default:
		return false
	}
}

// sanitizeHeaderForLogging validates and sanitizes untrusted HTTP header values
// before injecting into log records. Only safe alphanumeric and delimiter characters
// (a-z, A-Z, 0-9, -, _, ., :, /) up to 128 characters are accepted, preventing log injection
// and cleartext logging of sensitive header data.
func sanitizeHeaderForLogging(raw string) string {
	if raw == "" {
		return ""
	}
	const maxLen = 128
	if len(raw) > maxLen {
		raw = raw[:maxLen]
	}
	clean := make([]byte, 0, len(raw))
	for i := 0; i < len(raw); i++ {
		b := raw[i]
		if (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9') ||
			b == '-' || b == '_' || b == '.' || b == ':' || b == '/' {
			clean = append(clean, b)
		} else {
			return ""
		}
	}
	return string(clean)
}

// Middleware returns standard net/http middleware that logs every HTTP request and response,
// recording method, path, status, duration, bytes written, and request ID.
// It also injects a context-scoped logger into r.Context() containing the request ID.
func Middleware(opts ...MiddlewareOption) func(http.Handler) http.Handler {
	cfg := MiddlewareOptions{
		RequestIDHeader: DefaultRequestIDHeader,
	}

	for _, opt := range opts {
		opt(&cfg)
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			baseLogger := cfg.Logger
			if baseLogger == nil {
				baseLogger = FromContext(r.Context())
			}

			var reqID string
			if isAllowedCorrelationHeader(cfg.RequestIDHeader) {
				reqID = sanitizeHeaderForLogging(r.Header.Get(cfg.RequestIDHeader))
			}
			reqLogger := baseLogger
			if reqID != "" {
				reqLogger = baseLogger.With(slog.String("request_id", reqID))
			}

			ctx := WithContext(r.Context(), reqLogger)
			r = r.WithContext(ctx)

			start := time.Now()
			rec := &statusRecorder{
				ResponseWriter: w,
				statusCode:     http.StatusOK,
			}

			next.ServeHTTP(rec, r)

			duration := time.Since(start)

			attrs := []slog.Attr{
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
				slog.Int("status", rec.statusCode),
				slog.Float64("duration_ms", float64(duration.Microseconds())/1000.0),
				slog.Int64("bytes", rec.bytesWritten),
				slog.String("remote_ip", r.RemoteAddr),
			}

			if reqID != "" {
				attrs = append(attrs, slog.String("request_id", reqID))
			}

			if cfg.ExtraAttributes != nil {
				attrs = append(attrs, cfg.ExtraAttributes(r)...)
			}

			level := slog.LevelInfo
			if rec.statusCode >= 500 {
				level = slog.LevelError
			} else if rec.statusCode >= 400 {
				level = slog.LevelWarn
			}

			reqLogger.LogAttrs(ctx, level, "http request completed", attrs...)
		})
	}
}
