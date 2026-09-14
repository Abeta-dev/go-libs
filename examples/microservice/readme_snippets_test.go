// SPDX-License-Identifier: MIT

package main_test

import (
	"bytes"
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/umesh0492/go-libs/apperror"
	"github.com/umesh0492/go-libs/cache"
	"github.com/umesh0492/go-libs/circuitbreaker"
	"github.com/umesh0492/go-libs/clock"
	"github.com/umesh0492/go-libs/env"
	"github.com/umesh0492/go-libs/httpclient"
	"github.com/umesh0492/go-libs/logger"
	"github.com/umesh0492/go-libs/ratelimit"
	"github.com/umesh0492/go-libs/retry"
	"github.com/umesh0492/go-libs/timeutil"
	"github.com/umesh0492/go-libs/workerpool"
)

type UserProfile struct {
	ID   int
	Name string
}

func TestHeadlineSnippetsCompileAndRun(t *testing.T) {
	ctx := context.Background()

	// 1. retry
	cfg := retry.Config{
		Attempts:    3,
		InitialWait: 10 * time.Millisecond,
		MaxWait:     50 * time.Millisecond,
		Strategy:    retry.ExponentialJitter,
	}
	errRetry := retry.Do(ctx, cfg, func(ctx context.Context) error {
		return nil
	})
	if errRetry != nil {
		t.Fatalf("retry failed: %v", errRetry)
	}

	// 2. circuitbreaker
	cb := circuitbreaker.NewConsecutiveBreaker(3, 500*time.Millisecond)
	errCB := cb.Execute(ctx, func() error {
		return nil
	})
	if errCB != nil {
		t.Fatalf("circuitbreaker failed: %v", errCB)
	}

	// 3. ratelimit
	limiter := ratelimit.NewTokenBucket(100, time.Minute)
	if !limiter.Allow("tenant-client-ip") {
		t.Fatalf("token bucket allow failed")
	}
	mw := ratelimit.New(100, time.Minute)
	dummyHandler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	dummyHandler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("ratelimit middleware status: %d", rec.Code)
	}

	// 4. cache
	c := cache.NewTypedCache[UserProfile](
		cache.WithCapacity[UserProfile](10_000),
		cache.WithEvictionPolicy[UserProfile](cache.EvictionSampledLRU),
	)
	profile, errCache := c.GetOrFetch(ctx, "user:123", 5*time.Minute, func(ctx context.Context) (UserProfile, error) {
		return UserProfile{ID: 123, Name: "Alice"}, nil
	})
	if errCache != nil || profile.ID != 123 {
		t.Fatalf("cache GetOrFetch failed: %v, profile=%+v", errCache, profile)
	}

	// 5. workerpool
	wp := workerpool.New(4, 100)
	errWP := wp.Submit(func() {})
	if errWP != nil {
		t.Fatalf("workerpool submit failed: %v", errWP)
	}
	wp.StopWait()

	// 6. env
	port := env.Int("PORT", 8080)
	dbTimeout := env.Duration("DB_TIMEOUT", 5*time.Second)
	isProduction := env.Bool("PRODUCTION", true)
	appName := env.String("APP_NAME", "order-service")
	if port != 8080 || dbTimeout != 5*time.Second || !isProduction || appName != "order-service" {
		t.Fatalf("env parsing failed")
	}

	// 7. apperror
	findUser := func(id string) (*UserProfile, error) {
		return nil, apperror.NotFound("user not found")
	}
	_, errApp := findUser("123")
	if !apperror.Is(errApp, apperror.CodeNotFound) {
		t.Fatalf("expected CodeNotFound")
	}
	if errApp.(*apperror.Error).GetStatus() != 404 {
		t.Fatalf("expected status 404")
	}

	// 8. httpclient
	client := httpclient.New(
		httpclient.WithTotalTimeout(2*time.Second),
		httpclient.WithRetry(cfg),
	)
	if client == nil {
		t.Fatalf("httpclient.New returned nil")
	}

	// 9. timeutil
	friday := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
	monday := timeutil.AddBusinessDays(friday, 1)
	if monday.Weekday() != time.Monday {
		t.Fatalf("expected Monday, got %v", monday.Weekday())
	}

	// 10. clock
	fc := clock.NewFakeAt(friday)
	fc.Add(24 * time.Hour)
	if fc.Now().Weekday() != time.Saturday {
		t.Fatalf("expected Saturday, got %v", fc.Now().Weekday())
	}

	// 11. logger.NewRedactingHandler
	var logBuf bytes.Buffer
	redactor := logger.NewRedactingHandler(
		slog.NewJSONHandler(&logBuf, nil),
		logger.WithRedactedKeys("password", "token"),
	)
	redactingLogger := slog.New(redactor)
	redactingLogger.Info("credentials audit", slog.String("password", "secret123"))
	if !strings.Contains(logBuf.String(), "[REDACTED]") || strings.Contains(logBuf.String(), "secret123") {
		t.Fatalf("redacting handler failed: %s", logBuf.String())
	}
}
