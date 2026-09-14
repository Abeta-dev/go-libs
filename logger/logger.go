// SPDX-License-Identifier: MIT

// Package logger provides context-aware structured logging wrappers around
// the standard library's log/slog package.
package logger

import (
	"context"
	"log/slog"
	"os"
	"strings"
	"sync"
)

type contextKey struct{}

var (
	defaultLogger *slog.Logger
	defaultOnce   sync.Once
)

// Default returns a shared JSON slog.Logger configured from LOG_LEVEL
// (default: INFO) and writing to os.Stdout.
func Default() *slog.Logger {
	defaultOnce.Do(func() {
		var level slog.Level
		switch strings.ToUpper(os.Getenv("LOG_LEVEL")) {
		case "DEBUG":
			level = slog.LevelDebug
		case "WARN", "WARNING":
			level = slog.LevelWarn
		case "ERROR":
			level = slog.LevelError
		default:
			level = slog.LevelInfo
		}

		handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
			Level: level,
		})
		defaultLogger = slog.New(handler)
	})
	return defaultLogger
}

// WithContext returns a new context containing the given logger.
func WithContext(ctx context.Context, l *slog.Logger) context.Context {
	if l == nil {
		return ctx
	}
	return context.WithValue(ctx, contextKey{}, l)
}

// FromContext retrieves the logger stored in ctx. If no logger is found,
// it returns the Default() logger.
func FromContext(ctx context.Context) *slog.Logger {
	if ctx != nil {
		if l, ok := ctx.Value(contextKey{}).(*slog.Logger); ok && l != nil {
			return l
		}
	}
	return Default()
}

// WithField returns a new context whose logger has the given key-value field attached.
func WithField(ctx context.Context, key string, val any) context.Context {
	l := FromContext(ctx).With(slog.Any(key, val))
	return WithContext(ctx, l)
}

// WithFields returns a new context whose logger has the given key-value pairs attached.
func WithFields(ctx context.Context, args ...any) context.Context {
	l := FromContext(ctx).With(args...)
	return WithContext(ctx, l)
}
