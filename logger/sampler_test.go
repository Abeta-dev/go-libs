// SPDX-License-Identifier: MIT

package logger_test

import (
	"bytes"
	"log/slog"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/umesh0492/go-libs/clock"
	"github.com/umesh0492/go-libs/logger"
)

type mockCustomSampler struct {
	allowed atomic.Bool
}

func (m *mockCustomSampler) Allow() bool {
	return m.allowed.Load()
}

func TestSamplingHandler_WithCustomSampler(t *testing.T) {
	var buf bytes.Buffer
	baseHandler := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})

	mockSampler := &mockCustomSampler{}
	mockSampler.allowed.Store(true)

	handler := logger.NewSamplingHandler(
		baseHandler,
		1,
		time.Second,
		logger.WithSampler(mockSampler),
	)
	log := slog.New(handler)

	log.Info("msg-1")
	assert.Contains(t, buf.String(), "msg-1")

	// Disallow through custom sampler
	mockSampler.allowed.Store(false)
	log.Info("msg-2")
	assert.NotContains(t, buf.String(), "msg-2")

	// Bypass level (Warn, Error) still passes
	log.Warn("warn-msg")
	log.Error("error-msg")
	assert.Contains(t, buf.String(), "warn-msg")
	assert.Contains(t, buf.String(), "error-msg")
}

func TestSamplingHandler_InternalSamplerDefault(t *testing.T) {
	var buf bytes.Buffer
	baseHandler := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})

	// Burst 2, refill 1s, default real clock
	handler := logger.NewSamplingHandler(baseHandler, 2, time.Second)
	log := slog.New(handler)

	log.Info("allowed-1")
	log.Info("allowed-2")
	log.Info("dropped-3")

	assert.Contains(t, buf.String(), "allowed-1")
	assert.Contains(t, buf.String(), "allowed-2")
	assert.NotContains(t, buf.String(), "dropped-3")
}

func TestSamplingHandler_EdgeCaseZeroParameters(t *testing.T) {
	var buf bytes.Buffer
	baseHandler := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})

	// Burst <= 0 and refill <= 0 should default safely to burst=1, refill=1s
	handler := logger.NewSamplingHandler(baseHandler, 0, 0)
	log := slog.New(handler)

	log.Info("first-log")
	log.Info("second-log")

	assert.Contains(t, buf.String(), "first-log")
	assert.NotContains(t, buf.String(), "second-log")
}

func TestSamplingHandler_ConcurrentSampling(t *testing.T) {
	var buf bytes.Buffer
	baseHandler := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})

	fc := clock.NewFake()
	handler := logger.NewSamplingHandler(
		baseHandler,
		100,
		time.Millisecond,
		logger.WithSamplingClock(fc),
	)
	log := slog.New(handler)

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				log.Info("concurrent-log")
			}
		}()
	}
	wg.Wait()

	require.NotEmpty(t, buf.String())
}
