// SPDX-License-Identifier: MIT

package logger

import (
	"context"
	"log/slog"
	"regexp"
	"strings"
)

// DefaultMask is the default placeholder used to replace sensitive log values.
const DefaultMask = "[REDACTED]"

// RedactOption configures a RedactingHandler.
type RedactOption func(*redactOptions)

type redactOptions struct {
	keys     []string
	patterns []*regexp.Regexp
	mask     string
}

// WithRedactedKeys adds sensitive key names to mask (case-insensitive substring or exact match).
func WithRedactedKeys(keys ...string) RedactOption {
	return func(o *redactOptions) {
		for _, k := range keys {
			if strings.TrimSpace(k) != "" {
				o.keys = append(o.keys, strings.ToLower(strings.TrimSpace(k)))
			}
		}
	}
}

// WithRedactedPattern adds a regular expression hook that masks matching substrings in string values and messages.
func WithRedactedPattern(pattern *regexp.Regexp) RedactOption {
	return func(o *redactOptions) {
		if pattern != nil {
			o.patterns = append(o.patterns, pattern)
		}
	}
}

// WithMask overrides the default mask string ("[REDACTED]").
func WithMask(mask string) RedactOption {
	return func(o *redactOptions) {
		if mask != "" {
			o.mask = mask
		}
	}
}

// DefaultRedactedKeys contains commonly recognized sensitive attribute keys.
var DefaultRedactedKeys = []string{
	"password",
	"passwd",
	"token",
	"authorization",
	"secret",
	"ssn",
	"card",
	"cvv",
	"api_key",
	"apikey",
	"private_key",
}

// RedactingHandler is an slog.Handler wrapper that masks sensitive keys and pattern matches.
type RedactingHandler struct {
	next     slog.Handler
	keys     []string
	patterns []*regexp.Regexp
	mask     string
}

// NewRedactingHandler creates a RedactingHandler wrapping next.
func NewRedactingHandler(next slog.Handler, opts ...RedactOption) *RedactingHandler {
	cfg := redactOptions{
		keys: append([]string{}, DefaultRedactedKeys...),
		mask: DefaultMask,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(&cfg)
		}
	}
	return &RedactingHandler{
		next:     next,
		keys:     cfg.keys,
		patterns: cfg.patterns,
		mask:     cfg.mask,
	}
}

// Enabled reports whether the handler handles records at the given level.
func (h *RedactingHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.next.Enabled(ctx, level)
}

// Handle processes the log record, masking any sensitive keys and regex pattern matches.
func (h *RedactingHandler) Handle(ctx context.Context, r slog.Record) error {
	msg := r.Message
	for _, re := range h.patterns {
		if re.MatchString(msg) {
			msg = re.ReplaceAllString(msg, h.mask)
		}
	}

	newRecord := slog.NewRecord(r.Time, r.Level, msg, r.PC)
	r.Attrs(func(a slog.Attr) bool {
		newRecord.AddAttrs(h.redactAttr(a))
		return true
	})
	return h.next.Handle(ctx, newRecord)
}

// WithAttrs returns a new RedactingHandler with the attributes redacted and attached.
func (h *RedactingHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	redacted := make([]slog.Attr, len(attrs))
	for i, a := range attrs {
		redacted[i] = h.redactAttr(a)
	}
	return &RedactingHandler{
		next:     h.next.WithAttrs(redacted),
		keys:     h.keys,
		patterns: h.patterns,
		mask:     h.mask,
	}
}

// WithGroup returns a new RedactingHandler with the group attached.
func (h *RedactingHandler) WithGroup(name string) slog.Handler {
	return &RedactingHandler{
		next:     h.next.WithGroup(name),
		keys:     h.keys,
		patterns: h.patterns,
		mask:     h.mask,
	}
}

func (h *RedactingHandler) isSensitiveKey(key string) bool {
	lowerKey := strings.ToLower(key)
	for _, k := range h.keys {
		if strings.Contains(lowerKey, k) {
			return true
		}
	}
	return false
}

func (h *RedactingHandler) redactAttr(a slog.Attr) slog.Attr {
	if a.Value.Kind() == slog.KindGroup {
		groupAttrs := a.Value.Group()
		redacted := make([]slog.Attr, len(groupAttrs))
		for i, child := range groupAttrs {
			redacted[i] = h.redactAttr(child)
		}
		return slog.Attr{
			Key:   a.Key,
			Value: slog.GroupValue(redacted...),
		}
	}

	// 1. Check if key matches sensitive key list
	if h.isSensitiveKey(a.Key) {
		return slog.String(a.Key, h.mask)
	}

	// 2. Check string value against patterns
	if a.Value.Kind() == slog.KindString && len(h.patterns) > 0 {
		str := a.Value.String()
		for _, re := range h.patterns {
			if re.MatchString(str) {
				str = re.ReplaceAllString(str, h.mask)
			}
		}
		return slog.String(a.Key, str)
	}

	return a
}
