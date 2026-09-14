// SPDX-License-Identifier: MIT

package ginmw

import (
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/umesh0492/go-libs/logger"
	"github.com/umesh0492/go-libs/requestid"
)

// LoggerOption defines functional configuration for the Logger middleware.
type LoggerOption func(*loggerConfig)

type loggerConfig struct {
	logger *slog.Logger
}

// WithLogger sets a custom structured logger for the Logger middleware.
func WithLogger(l *slog.Logger) LoggerOption {
	return func(cfg *loggerConfig) {
		cfg.logger = l
	}
}

// Logger returns a Gin middleware that records structured access logs
// with request-ID correlation, method, path, status, latency, and client IP.
func Logger(opts ...LoggerOption) gin.HandlerFunc {
	cfg := &loggerConfig{}
	for _, opt := range opts {
		if opt != nil {
			opt(cfg)
		}
	}

	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		latency := time.Since(start)

		reqID := c.GetString("request_id")
		if reqID == "" {
			reqID = requestid.FromContext(c.Request.Context())
		}

		l := cfg.logger
		if l == nil {
			l = logger.FromContext(c.Request.Context())
		}

		l.Info("HTTP request handled",
			slog.String("request_id", reqID),
			slog.String("method", c.Request.Method),
			slog.String("path", c.Request.URL.Path),
			slog.Int("status", c.Writer.Status()),
			slog.Duration("latency", latency),
			slog.String("client_ip", c.ClientIP()),
		)
	}
}
