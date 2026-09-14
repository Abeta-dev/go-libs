// SPDX-License-Identifier: MIT

package logger

import (
	"context"
	"log/slog"
	"time"

	"github.com/umesh0492/go-libs/clock"
	"github.com/umesh0492/go-libs/ratelimit"
)

// SamplingOption configures a SamplingHandler.
type SamplingOption func(*samplingOptions)

type samplingOptions struct {
	clock       clock.Clock
	bypassLevel slog.Level
	keyFunc     func(r slog.Record) string
}

// WithSamplingClock configures the clock used for sampling rate calculations.
func WithSamplingClock(c clock.Clock) SamplingOption {
	return func(o *samplingOptions) {
		o.clock = c
	}
}

// WithBypassLevel sets the minimum log level that always bypasses sampling (default: slog.LevelWarn).
// For instance, slog.LevelWarn and slog.LevelError are never dropped.
func WithBypassLevel(level slog.Level) SamplingOption {
	return func(o *samplingOptions) {
		o.bypassLevel = level
	}
}

// WithSamplingKeyFunc configures a key extraction function for per-key sampling (e.g., per message or logger name).
// If not specified, a global token bucket is shared across all sampled log entries.
func WithSamplingKeyFunc(fn func(r slog.Record) string) SamplingOption {
	return func(o *samplingOptions) {
		o.keyFunc = fn
	}
}

// SamplingHandler wraps an existing slog.Handler to rate-limit / sample log messages.
// Logs at or above the bypass level (default: WARN) are never sampled or dropped.
type SamplingHandler struct {
	next        slog.Handler
	limiter     *ratelimit.TokenBucketLimiter
	bypassLevel slog.Level
	keyFunc     func(r slog.Record) string
}

// NewSamplingHandler creates a SamplingHandler with burst capacity and refill interval.
// When burst capacity is exhausted, non-bypass logs (e.g. DEBUG, INFO) are dropped until tokens replenish.
func NewSamplingHandler(next slog.Handler, burst int, refillInterval time.Duration, opts ...SamplingOption) *SamplingHandler {
	cfg := samplingOptions{
		bypassLevel: slog.LevelWarn,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(&cfg)
		}
	}
	var limiterOpts []ratelimit.Option
	if cfg.clock != nil {
		limiterOpts = append(limiterOpts, ratelimit.WithClock(cfg.clock))
	}
	limiter := ratelimit.NewTokenBucket(burst, refillInterval, limiterOpts...)
	return &SamplingHandler{
		next:        next,
		limiter:     limiter,
		bypassLevel: cfg.bypassLevel,
		keyFunc:     cfg.keyFunc,
	}
}

// Enabled reports whether the handler handles records at the given level.
func (h *SamplingHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.next.Enabled(ctx, level)
}

// Handle processes the log record. If the record level is below the bypass level,
// it checks the token bucket. If rate-limited, the record is dropped silently (returning nil).
func (h *SamplingHandler) Handle(ctx context.Context, r slog.Record) error {
	if r.Level >= h.bypassLevel {
		return h.next.Handle(ctx, r)
	}
	key := "global"
	if h.keyFunc != nil {
		key = h.keyFunc(r)
	}
	if !h.limiter.Allow(key) {
		return nil
	}
	return h.next.Handle(ctx, r)
}

// WithAttrs returns a new SamplingHandler whose attributes are appended to the underlying handler.
func (h *SamplingHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &SamplingHandler{
		next:        h.next.WithAttrs(attrs),
		limiter:     h.limiter,
		bypassLevel: h.bypassLevel,
		keyFunc:     h.keyFunc,
	}
}

// WithGroup returns a new SamplingHandler with a group appended to the underlying handler.
func (h *SamplingHandler) WithGroup(name string) slog.Handler {
	return &SamplingHandler{
		next:        h.next.WithGroup(name),
		limiter:     h.limiter,
		bypassLevel: h.bypassLevel,
		keyFunc:     h.keyFunc,
	}
}
