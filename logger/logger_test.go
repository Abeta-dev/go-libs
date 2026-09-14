// SPDX-License-Identifier: MIT

package logger_test

import (
	"bytes"
	"context"
	"log/slog"
	"regexp"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/umesh0492/go-libs/clock"
	"github.com/umesh0492/go-libs/logger"
)

func TestDefault(t *testing.T) {
	l := logger.Default()
	require.NotNil(t, l)
}

func TestDefault_EnvLevels(t *testing.T) {
	l := logger.FromContext(context.TODO())
	assert.NotNil(t, l)
}

func TestWithContext_And_FromContext(t *testing.T) {
	var buf bytes.Buffer
	custom := slog.New(slog.NewJSONHandler(&buf, nil))

	ctx := logger.WithContext(context.Background(), custom)
	retrieved := logger.FromContext(ctx)
	assert.Same(t, custom, retrieved)

	// empty context returns default
	assert.NotNil(t, logger.FromContext(context.TODO()))

	// context without logger returns default
	assert.NotNil(t, logger.FromContext(context.Background()))

	// WithContext with nil logger returns original ctx
	orig := context.Background()
	assert.Equal(t, orig, logger.WithContext(orig, nil))
}

func TestWithField(t *testing.T) {
	ctx := logger.WithField(context.Background(), "request_id", "req-123")
	l := logger.FromContext(ctx)
	assert.NotNil(t, l)
}

func TestWithFields(t *testing.T) {
	ctx := logger.WithFields(context.Background(), "user_id", "u-456", "tenant_id", "t-789")
	l := logger.FromContext(ctx)
	assert.NotNil(t, l)
}

func TestSamplingHandler_RateLimitingAndBurst(t *testing.T) {
	var buf bytes.Buffer
	baseHandler := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})

	fc := clock.NewFake()
	// Burst 2, refill 1 token every 100ms
	sampler := logger.NewSamplingHandler(
		baseHandler,
		2,
		100*time.Millisecond,
		logger.WithSamplingClock(fc),
	)
	log := slog.New(sampler)

	// First 2 INFO logs allowed (burst = 2)
	log.Info("info-1")
	log.Info("info-2")
	assert.Contains(t, buf.String(), "info-1")
	assert.Contains(t, buf.String(), "info-2")

	// 3rd and 4th INFO logs dropped
	log.Info("info-3")
	log.Info("info-4")
	assert.NotContains(t, buf.String(), "info-3")
	assert.NotContains(t, buf.String(), "info-4")

	// WARN and ERROR logs bypass sampling and are never dropped
	log.Warn("warn-1")
	log.Error("error-1")
	assert.Contains(t, buf.String(), "warn-1")
	assert.Contains(t, buf.String(), "error-1")

	// Advance clock by 100ms -> 1 token replenishes
	fc.Add(100 * time.Millisecond)
	log.Info("info-5")
	assert.Contains(t, buf.String(), "info-5")

	// Next log dropped again
	log.Info("info-6")
	assert.NotContains(t, buf.String(), "info-6")

	// Test Enabled, WithAttrs, and WithGroup
	assert.True(t, sampler.Enabled(context.Background(), slog.LevelDebug))
	subSampler := sampler.WithAttrs([]slog.Attr{slog.String("component", "auth")})
	assert.NotNil(t, subSampler)
	grpSampler := sampler.WithGroup("req_meta")
	assert.NotNil(t, grpSampler)
}

func TestSamplingHandler_CustomBypassLevel(t *testing.T) {
	var buf bytes.Buffer
	baseHandler := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})

	fc := clock.NewFake()
	sampler := logger.NewSamplingHandler(
		baseHandler,
		1,
		time.Minute,
		logger.WithSamplingClock(fc),
		logger.WithBypassLevel(slog.LevelError),
	)
	log := slog.New(sampler)

	// 1 token consumed
	log.Warn("warn-1")
	assert.Contains(t, buf.String(), "warn-1")

	// Next WARN is now dropped because bypass level is ERROR
	log.Warn("warn-2")
	assert.NotContains(t, buf.String(), "warn-2")

	// ERROR still passes
	log.Error("error-1")
	assert.Contains(t, buf.String(), "error-1")
}

func TestRedactingHandler_SensitiveKeysAndNestedGroups(t *testing.T) {
	var buf bytes.Buffer
	baseHandler := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})

	redactor := logger.NewRedactingHandler(
		baseHandler,
		logger.WithRedactedKeys("session_id", "credit_card"),
		logger.WithMask("[SECRET]"),
	)
	log := slog.New(redactor)

	log.Info("user login",
		slog.String("username", "alice"),
		slog.String("password", "supersecret123"),
		slog.String("api_key", "key-999"),
		slog.String("session_id", "sess-abc"),
		slog.Group("nested",
			slog.String("token", "tok-xyz"),
			slog.String("safe", "visible"),
		),
	)

	out := buf.String()
	assert.Contains(t, out, `"username":"alice"`)
	assert.Contains(t, out, `"password":"[SECRET]"`)
	assert.Contains(t, out, `"api_key":"[SECRET]"`)
	assert.Contains(t, out, `"session_id":"[SECRET]"`)
	assert.Contains(t, out, `"token":"[SECRET]"`)
	assert.Contains(t, out, `"safe":"visible"`)
	assert.NotContains(t, out, "supersecret123")
	assert.NotContains(t, out, "key-999")
	assert.NotContains(t, out, "sess-abc")
	assert.NotContains(t, out, "tok-xyz")

	// Test WithAttrs and WithGroup
	withAttrs := redactor.WithAttrs([]slog.Attr{
		slog.String("auth_token", "secret-token"),
		slog.String("version", "v1"),
	})
	assert.NotNil(t, withAttrs)
	withGroup := redactor.WithGroup("audit")
	assert.NotNil(t, withGroup)
	assert.True(t, redactor.Enabled(context.Background(), slog.LevelInfo))
}

func TestRedactingHandler_RegexPattern(t *testing.T) {
	var buf bytes.Buffer
	baseHandler := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})

	ccPattern := regexp.MustCompile(`\b\d{4}-\d{4}-\d{4}-\d{4}\b`)
	redactor := logger.NewRedactingHandler(
		baseHandler,
		logger.WithRedactedPattern(ccPattern),
	)
	log := slog.New(redactor)

	log.Info("payment received for 1234-5678-9012-3456",
		slog.String("payment_info", "card number is 9876-5432-1098-7654"),
		slog.String("note", "plain note"),
	)

	out := buf.String()
	assert.Contains(t, out, "payment received for [REDACTED]")
	assert.Contains(t, out, "card number is [REDACTED]")
	assert.Contains(t, out, "plain note")
	assert.NotContains(t, out, "1234-5678-9012-3456")
	assert.NotContains(t, out, "9876-5432-1098-7654")
}
