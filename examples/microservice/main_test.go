// SPDX-License-Identifier: MIT

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/umesh0492/go-libs/circuitbreaker"
	"github.com/umesh0492/go-libs/clock"
	"github.com/umesh0492/go-libs/httpclient"
	"github.com/umesh0492/go-libs/logger"
	"github.com/umesh0492/go-libs/pagination"
	"github.com/umesh0492/go-libs/retry"
	"github.com/umesh0492/go-libs/timeutil"
)

func TestReferenceMicroservice(t *testing.T) {
	app := NewApp()
	require.NotNil(t, app)

	t.Run("Config Loading", func(t *testing.T) {
		os.Setenv("PORT", ":9090")
		os.Setenv("WORKER_POOL_SIZE", "8")
		defer func() {
			os.Unsetenv("PORT")
			os.Unsetenv("WORKER_POOL_SIZE")
		}()

		cfg := LoadConfig()
		assert.Equal(t, ":9090", cfg.Port)
		assert.Equal(t, 8, cfg.WorkerPoolSize)
	})

	t.Run("Health Endpoints", func(t *testing.T) {
		// Live check
		req := httptest.NewRequest(http.MethodGet, "/health/live", nil)
		rec := httptest.NewRecorder()
		app.Router.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)

		// Ready check
		req = httptest.NewRequest(http.MethodGet, "/health/ready", nil)
		rec = httptest.NewRecorder()
		app.Router.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)

		// Full health check
		req = httptest.NewRequest(http.MethodGet, "/health", nil)
		rec = httptest.NewRecorder()
		app.Router.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
	})

	t.Run("Get Items Paginated, Cached, and Filtered", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/items?page=1&limit=6", nil)
		rec := httptest.NewRecorder()
		app.Router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		assert.NotEmpty(t, rec.Header().Get("X-Request-ID"))
		assert.Equal(t, "DENY", rec.Header().Get("X-Frame-Options"))

		var body pagination.TypedResponse[Item]
		err := json.Unmarshal(rec.Body.Bytes(), &body)
		require.NoError(t, err)
		assert.Len(t, body.Items, 6)
		assert.Equal(t, 1, body.Page)
		assert.Equal(t, 6, body.Limit)
		assert.Equal(t, 100, body.Total)

		// Second request should hit cache
		rec2 := httptest.NewRecorder()
		app.Router.ServeHTTP(rec2, req)
		assert.Equal(t, http.StatusOK, rec2.Code)

		// Filter with sliceutil
		reqFilter := httptest.NewRequest(http.MethodGet, "/api/v1/items?page=1&limit=6&category=premium", nil)
		recFilter := httptest.NewRecorder()
		app.Router.ServeHTTP(recFilter, reqFilter)
		assert.Equal(t, http.StatusOK, recFilter.Code)

		var filteredBody pagination.TypedResponse[Item]
		err = json.Unmarshal(recFilter.Body.Bytes(), &filteredBody)
		require.NoError(t, err)
		for _, it := range filteredBody.Items {
			assert.Equal(t, "premium", it.Category)
		}

		// Chunking with sliceutil.Chunk
		reqChunk := httptest.NewRequest(http.MethodGet, "/api/v1/items?chunk_size=2", nil)
		recChunk := httptest.NewRecorder()
		app.Router.ServeHTTP(recChunk, reqChunk)
		assert.Equal(t, http.StatusOK, recChunk.Code)

		var chunkResp struct {
			Chunks    [][]Item `json:"chunks"`
			NumChunks int      `json:"num_chunks"`
			Total     int      `json:"total"`
		}
		err = json.Unmarshal(recChunk.Body.Bytes(), &chunkResp)
		require.NoError(t, err)
		assert.Greater(t, chunkResp.NumChunks, 0)
		for _, chunk := range chunkResp.Chunks {
			assert.LessOrEqual(t, len(chunk), 2)
		}
	})

	t.Run("Get Item by ID - AppError Mapping", func(t *testing.T) {
		// Valid item
		req := httptest.NewRequest(http.MethodGet, "/api/v1/items/item-42", nil)
		rec := httptest.NewRecorder()
		app.Router.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)

		var it Item
		err := json.Unmarshal(rec.Body.Bytes(), &it)
		require.NoError(t, err)
		assert.Equal(t, "item-42", it.ID)
		assert.Equal(t, "Product 42", it.Name)

		// Non-existent item (apperror.NotFound -> HTTP 404)
		reqNotFound := httptest.NewRequest(http.MethodGet, "/api/v1/items/unknown-xyz", nil)
		recNotFound := httptest.NewRecorder()
		app.Router.ServeHTTP(recNotFound, reqNotFound)
		assert.Equal(t, http.StatusNotFound, recNotFound.Code)

		var errResp map[string]string
		err = json.Unmarshal(recNotFound.Body.Bytes(), &errResp)
		require.NoError(t, err)
		assert.Equal(t, "NOT_FOUND", errResp["code"])
	})

	t.Run("Quotes with Circuit Breaker and Retry", func(t *testing.T) {
		// Missing symbol param -> 400 Bad Request
		badReq := httptest.NewRequest(http.MethodGet, "/api/v1/quotes", nil)
		badRec := httptest.NewRecorder()
		app.Router.ServeHTTP(badRec, badReq)
		assert.Equal(t, http.StatusBadRequest, badRec.Code)

		// Successful fetch with retry
		var attempts atomic.Int32
		app.FetchQuote = func(ctx context.Context, symbol string) (Quote, error) {
			att := attempts.Add(1)
			if att < 2 {
				return Quote{}, errors.New("temporary connection error")
			}
			return Quote{
				Symbol:    symbol,
				Price:     195.75,
				FetchedAt: time.Now().UTC(),
			}, nil
		}

		req := httptest.NewRequest(http.MethodGet, "/api/v1/quotes?symbol=GOOGL", nil)
		rec := httptest.NewRecorder()
		app.Router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		var q Quote
		err := json.Unmarshal(rec.Body.Bytes(), &q)
		require.NoError(t, err)
		assert.Equal(t, "GOOGL", q.Symbol)
		assert.Equal(t, 195.75, q.Price)
		assert.False(t, q.ValidUntil.IsZero())
		assert.Equal(t, timeutil.AddBusinessDays(q.FetchedAt, 1), q.ValidUntil)
		assert.Equal(t, int32(2), attempts.Load()) // Retried once and succeeded!

		// Permanent failure tripping circuit breaker
		app.FetchQuote = func(ctx context.Context, symbol string) (Quote, error) {
			return Quote{}, errors.New("upstream service down")
		}

		// Trip the breaker (maxFailures = 5)
		for i := 0; i < 6; i++ {
			r := httptest.NewRequest(http.MethodGet, "/api/v1/quotes?symbol=FAIL", nil)
			w := httptest.NewRecorder()
			app.Router.ServeHTTP(w, r)
		}

		// Subsequent request should fail immediately with 503 UNAVAILABLE due to open circuit
		openReq := httptest.NewRequest(http.MethodGet, "/api/v1/quotes?symbol=FAIL", nil)
		openRec := httptest.NewRecorder()
		app.Router.ServeHTTP(openRec, openReq)
		assert.Equal(t, http.StatusServiceUnavailable, openRec.Code)

		var unavailableResp map[string]string
		err = json.Unmarshal(openRec.Body.Bytes(), &unavailableResp)
		require.NoError(t, err)
		assert.Equal(t, "UNAVAILABLE", unavailableResp["code"])
	})

	t.Run("Create Task Async", func(t *testing.T) {
		body := bytes.NewBufferString(`{"payload":"process-order-456"}`)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/tasks", body)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		app.Router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusCreated, rec.Code)

		// Invalid payload
		badReq := httptest.NewRequest(http.MethodPost, "/api/v1/tasks", bytes.NewBufferString(`{}`))
		badReq.Header.Set("Content-Type", "application/json")
		badRec := httptest.NewRecorder()
		app.Router.ServeHTTP(badRec, badReq)
		assert.Equal(t, http.StatusBadRequest, badRec.Code)
	})

	t.Run("Observability Metrics Endpoint JSON", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
		rec := httptest.NewRecorder()
		app.Router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		var metricsResp map[string]any
		err := json.Unmarshal(rec.Body.Bytes(), &metricsResp)
		require.NoError(t, err)
		assert.Contains(t, metricsResp, "circuit_breaker")
		assert.Contains(t, metricsResp, "worker_pool")
		assert.Contains(t, metricsResp, "cache")
		assert.Contains(t, metricsResp, "http")
	})

	t.Run("Observability Metrics Endpoint Prometheus Text Format", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
		req.Header.Set("Accept", "text/plain; version=0.0.4")
		rec := httptest.NewRecorder()
		app.Router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, "text/plain; version=0.0.4; charset=utf-8", rec.Header().Get("Content-Type"))
		body := rec.Body.String()
		assert.Contains(t, body, "# HELP app_circuit_breaker_requests_total")
		assert.Contains(t, body, "# TYPE app_circuit_breaker_requests_total counter")
		assert.Contains(t, body, "app_circuit_breaker_state")
		assert.Contains(t, body, "app_cache_hits_total")
		assert.Contains(t, body, "app_workerpool_queue_size")
		assert.Contains(t, body, "http_requests_total")
		assert.Contains(t, body, "http_request_duration_ms_total")
	})

	t.Run("Observability Metrics Endpoint Query Format Prometheus", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/metrics?format=prometheus", nil)
		rec := httptest.NewRecorder()
		app.Router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		body := rec.Body.String()
		assert.Contains(t, body, "app_circuit_breaker_requests_total")
	})

	t.Run("Shutdown Manager Execution", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		err := app.Shutdown.Execute(ctx)
		assert.NoError(t, err)
	})

	t.Run("AtomicCounter Add Guard", func(t *testing.T) {
		counter := &AtomicCounter{}
		counter.Add(10.5)
		assert.Equal(t, uint64(10), counter.Value())

		// Zero delta should have no effect
		counter.Add(0)
		assert.Equal(t, uint64(10), counter.Value())

		// Negative delta should be ignored and not wrap underflow
		counter.Add(-1)
		assert.Equal(t, uint64(10), counter.Value())

		counter.Add(-50.5)
		assert.Equal(t, uint64(10), counter.Value())
	})

	t.Run("Quote ValidUntil with timeutil.AddBusinessDays", func(t *testing.T) {
		// Test default fetcher computing business days
		defaultApp := NewApp()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/quotes?symbol=AAPL", nil)
		rec := httptest.NewRecorder()
		defaultApp.Router.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)

		var q Quote
		err := json.Unmarshal(rec.Body.Bytes(), &q)
		require.NoError(t, err)
		assert.Equal(t, "AAPL", q.Symbol)
		assert.False(t, q.FetchedAt.IsZero())
		assert.False(t, q.ValidUntil.IsZero())
		assert.Equal(t, timeutil.AddBusinessDays(q.FetchedAt, 1), q.ValidUntil)

		// Test weekend rollover (Friday to Monday)
		friday := time.Date(2026, 5, 1, 14, 0, 0, 0, time.UTC) // Friday
		fc := clock.NewFakeAt(friday)
		weekendApp := NewApp(WithClock(fc))

		reqFriday := httptest.NewRequest(http.MethodGet, "/api/v1/quotes?symbol=MSFT", nil)
		recFriday := httptest.NewRecorder()
		weekendApp.Router.ServeHTTP(recFriday, reqFriday)
		assert.Equal(t, http.StatusOK, recFriday.Code)

		var qFriday Quote
		err = json.Unmarshal(recFriday.Body.Bytes(), &qFriday)
		require.NoError(t, err)
		assert.Equal(t, time.Friday, qFriday.FetchedAt.Weekday())
		assert.Equal(t, time.Monday, qFriday.ValidUntil.Weekday())
		assert.Equal(t, friday.AddDate(0, 0, 3), qFriday.ValidUntil)
	})

	t.Run("Sensitive Attributes Redacted via logger.RedactingHandler", func(t *testing.T) {
		var logBuf bytes.Buffer
		bufHandler := slog.NewJSONHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelDebug})

		// Initialize app with logging handler wrapped by NewRedactingHandler
		appWithLogger := NewApp(WithLoggerHandler(bufHandler))
		require.NotNil(t, appWithLogger)
		assert.IsType(t, &logger.RedactingHandler{}, appWithLogger.Logger.Handler())

		// Log record containing sensitive attributes
		appWithLogger.Logger.Info("user authentication audit",
			slog.String("username", "developer1"),
			slog.String("password", "super-secret-pass-1234"),
			slog.String("token", "jwt-token-val-9999"),
			slog.String("secret", "topsecret-api-key"),
			slog.String("authorization", "Bearer my-jwt-access-token"),
		)

		logOutput := logBuf.String()
		assert.Contains(t, logOutput, `[REDACTED]`)
		assert.Contains(t, logOutput, `"username":"developer1"`)
		assert.NotContains(t, logOutput, "super-secret-pass-1234")
		assert.NotContains(t, logOutput, "jwt-token-val-9999")
		assert.NotContains(t, logOutput, "topsecret-api-key")
		assert.NotContains(t, logOutput, "my-jwt-access-token")
	})

	t.Run("Deterministic Time Testing with clock.FakeClock", func(t *testing.T) {
		startTime := time.Date(2026, 6, 1, 9, 30, 0, 0, time.UTC) // Monday morning
		fakeClock := clock.NewFakeAt(startTime)

		testApp := NewApp(WithClock(fakeClock))
		assert.Equal(t, startTime, testApp.Clock.Now())

		// First quote at Monday 09:30 UTC -> valid until Tuesday 09:30 UTC
		req := httptest.NewRequest(http.MethodGet, "/api/v1/quotes?symbol=TSLA", nil)
		rec := httptest.NewRecorder()
		testApp.Router.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)

		var q1 Quote
		err := json.Unmarshal(rec.Body.Bytes(), &q1)
		require.NoError(t, err)
		assert.Equal(t, startTime, q1.FetchedAt)
		assert.Equal(t, time.Date(2026, 6, 2, 9, 30, 0, 0, time.UTC), q1.ValidUntil)

		// Advance clock by 48 hours without real-world sleeping
		fakeClock.Add(48 * time.Hour)
		expectedNow := startTime.Add(48 * time.Hour) // Wednesday
		assert.Equal(t, expectedNow, testApp.Clock.Now())

		rec2 := httptest.NewRecorder()
		testApp.Router.ServeHTTP(rec2, req)
		assert.Equal(t, http.StatusOK, rec2.Code)

		var q2 Quote
		err = json.Unmarshal(rec2.Body.Bytes(), &q2)
		require.NoError(t, err)
		assert.Equal(t, expectedNow, q2.FetchedAt)
		assert.Equal(t, time.Date(2026, 6, 4, 9, 30, 0, 0, time.UTC), q2.ValidUntil) // Thursday
	})

	t.Run("HTTPClient Functions with Circuit Breaker and Retry", func(t *testing.T) {
		// 1. Verify retry on transient upstream failure
		var attempts atomic.Int32
		mockUpstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			att := attempts.Add(1)
			if att == 1 {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"symbol":"NVDA","price":125.75}`))
		}))
		defer mockUpstream.Close()

		ctx := context.Background()
		testApp := NewApp()

		// Call upstream via resilient HTTP client
		q, err := testApp.FetchQuoteHTTP(ctx, mockUpstream.URL, "NVDA")
		require.NoError(t, err)
		assert.Equal(t, "NVDA", q.Symbol)
		assert.Equal(t, 125.75, q.Price)
		assert.Equal(t, int32(2), attempts.Load()) // Retried after first 500 and succeeded!
		assert.False(t, q.ValidUntil.IsZero())

		// 2. Verify circuit breaker trips after repeated upstream failures
		failingUpstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusServiceUnavailable)
		}))
		defer failingUpstream.Close()

		cb := circuitbreaker.NewConsecutiveBreaker(3, 5*time.Second)
		clientWithBreaker := httpclient.New(
			httpclient.WithCircuitBreaker(cb),
			httpclient.WithRetry(retry.Config{Attempts: 1}),
			httpclient.WithTotalTimeout(1*time.Second),
		)

		breakerApp := NewApp(WithHTTPClient(clientWithBreaker))

		// Trip breaker with consecutive failures
		for i := 0; i < 4; i++ {
			_, _ = breakerApp.FetchQuoteHTTP(ctx, failingUpstream.URL, "FAIL")
		}

		// Subsequent request must fail immediately with ErrCircuitOpen
		_, errBreaker := breakerApp.FetchQuoteHTTP(ctx, failingUpstream.URL, "FAIL")
		require.Error(t, errBreaker)
		assert.True(t, errors.Is(errBreaker, circuitbreaker.ErrCircuitOpen) || cb.State() == circuitbreaker.StateOpen)
	})
}
