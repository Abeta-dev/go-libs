// SPDX-License-Identifier: MIT

package logger

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/umesh0492/go-libs/clock"
)

// Sampler defines the rate-limiting interface for sampling log entries.
type Sampler interface {
	Allow() bool
}

// SamplingOption configures a SamplingHandler.
type SamplingOption func(*samplingOptions)

type samplingOptions struct {
	clock       clock.Clock
	bypassLevel slog.Level
	keyFunc     func(r slog.Record) string
	sampler     Sampler
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
func WithSamplingKeyFunc(fn func(r slog.Record) string) SamplingOption {
	return func(o *samplingOptions) {
		o.keyFunc = fn
	}
}

// WithSampler configures a custom Sampler implementation.
func WithSampler(s Sampler) SamplingOption {
	return func(o *samplingOptions) {
		o.sampler = s
	}
}

// tokenBucketSampler provides an internal decoupled token bucket implementation.
type tokenBucketSampler struct {
	mu             sync.Mutex
	burst          int
	refillInterval time.Duration
	tokens         float64
	lastRefill     time.Time
	clk            clock.Clock
}

func newTokenBucketSampler(burst int, refillInterval time.Duration, clk clock.Clock) *tokenBucketSampler {
	if clk == nil {
		clk = clock.NewReal()
	}
	if burst <= 0 {
		burst = 1
	}
	if refillInterval <= 0 {
		refillInterval = time.Second
	}
	return &tokenBucketSampler{
		burst:          burst,
		refillInterval: refillInterval,
		tokens:         float64(burst),
		lastRefill:     clk.Now(),
		clk:            clk,
	}
}

func (s *tokenBucketSampler) Allow() bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := s.clk.Now()
	elapsed := now.Sub(s.lastRefill)
	if elapsed > 0 {
		delta := float64(elapsed) / float64(s.refillInterval)
		s.tokens += delta
		if s.tokens > float64(s.burst) {
			s.tokens = float64(s.burst)
		}
		s.lastRefill = now
	}

	if s.tokens >= 1.0 {
		s.tokens -= 1.0
		return true
	}
	return false
}

// SamplingHandler wraps an existing slog.Handler to rate-limit / sample log messages.
// Logs at or above the bypass level (default: WARN) are never sampled or dropped.
type SamplingHandler struct {
	next        slog.Handler
	sampler     Sampler
	bypassLevel slog.Level
	keyFunc     func(r slog.Record) string
}

// NewSamplingHandler creates a SamplingHandler with burst capacity and refill interval.
// When burst capacity is exhausted, non-bypass logs (e.g. DEBUG, INFO) are dropped until tokens replenish.
// If no custom sampler is configured via WithSampler, an internal token-bucket sampler is used.
func NewSamplingHandler(next slog.Handler, burst int, refillInterval time.Duration, opts ...SamplingOption) *SamplingHandler {
	cfg := samplingOptions{
		bypassLevel: slog.LevelWarn,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(&cfg)
		}
	}

	sampler := cfg.sampler
	if sampler == nil {
		sampler = newTokenBucketSampler(burst, refillInterval, cfg.clock)
	}

	return &SamplingHandler{
		next:        next,
		sampler:     sampler,
		bypassLevel: cfg.bypassLevel,
		keyFunc:     cfg.keyFunc,
	}
}

// Enabled reports whether the handler handles records at the given level.
func (h *SamplingHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.next.Enabled(ctx, level)
}

// Handle processes the log record. If the record level is below the bypass level,
// it checks the sampler. If rate-limited, the record is dropped silently (returning nil).
func (h *SamplingHandler) Handle(ctx context.Context, r slog.Record) error {
	if r.Level >= h.bypassLevel {
		return h.next.Handle(ctx, r)
	}
	if !h.sampler.Allow() {
		return nil
	}
	return h.next.Handle(ctx, r)
}

// WithAttrs returns a new SamplingHandler whose attributes are appended to the underlying handler.
func (h *SamplingHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &SamplingHandler{
		next:        h.next.WithAttrs(attrs),
		sampler:     h.sampler,
		bypassLevel: h.bypassLevel,
		keyFunc:     h.keyFunc,
	}
}

// WithGroup returns a new SamplingHandler with a group appended to the underlying handler.
func (h *SamplingHandler) WithGroup(name string) slog.Handler {
	return &SamplingHandler{
		next:        h.next.WithGroup(name),
		sampler:     h.sampler,
		bypassLevel: h.bypassLevel,
		keyFunc:     h.keyFunc,
	}
}
