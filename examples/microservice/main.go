// SPDX-License-Identifier: MIT

// Package main provides a production-ready reference microservice blueprint
// demonstrating the composition of go-libs modules into a unified HTTP service.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/umesh0492/go-libs/apperror"
	"github.com/umesh0492/go-libs/cache"
	"github.com/umesh0492/go-libs/circuitbreaker"
	"github.com/umesh0492/go-libs/clock"
	"github.com/umesh0492/go-libs/env"
	"github.com/umesh0492/go-libs/ginmw"
	"github.com/umesh0492/go-libs/health"
	"github.com/umesh0492/go-libs/httpclient"
	"github.com/umesh0492/go-libs/httputil"
	"github.com/umesh0492/go-libs/logger"
	"github.com/umesh0492/go-libs/pagination"
	"github.com/umesh0492/go-libs/recovery"
	"github.com/umesh0492/go-libs/retry"
	"github.com/umesh0492/go-libs/shutdown"
	"github.com/umesh0492/go-libs/sliceutil"
	"github.com/umesh0492/go-libs/timeutil"
	"github.com/umesh0492/go-libs/workerpool"
)

// Config encapsulates environment-driven configuration for the microservice.
type Config struct {
	Port              string
	ReadHeaderTimeout time.Duration
	WorkerPoolSize    int
	WorkerQueueCap    int
	ShutdownTimeout   time.Duration
	QuoteServiceURL   string
}

// LoadConfig reads configuration using type-safe env parsers with defaults.
func LoadConfig() Config {
	return Config{
		Port:              env.String("PORT", ":8080"),
		ReadHeaderTimeout: env.Duration("READ_HEADER_TIMEOUT", 5*time.Second),
		WorkerPoolSize:    env.Int("WORKER_POOL_SIZE", 4),
		WorkerQueueCap:    env.Int("WORKER_QUEUE_CAP", 100),
		ShutdownTimeout:   env.Duration("SHUTDOWN_TIMEOUT", 10*time.Second),
		QuoteServiceURL:   env.String("QUOTE_SERVICE_URL", ""),
	}
}

// Item represents a sample domain entity.
type Item struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Category string `json:"category"`
}

// Quote represents external pricing data fetched through circuitbreaker & retry.
type Quote struct {
	Symbol     string    `json:"symbol"`
	Price      float64   `json:"price"`
	FetchedAt  time.Time `json:"fetched_at"`
	ValidUntil time.Time `json:"valid_until"`
}

// AtomicCounter implements metrics.Counter using sync/atomic.
type AtomicCounter struct {
	val atomic.Uint64
}

// Inc increments counter by 1.
func (c *AtomicCounter) Inc() { c.val.Add(1) }

// Add increments counter by delta.
func (c *AtomicCounter) Add(delta float64) {
	if delta <= 0 {
		return
	}
	c.val.Add(uint64(delta))
}

// Value returns current counter value.
func (c *AtomicCounter) Value() uint64 { return c.val.Load() }

// AtomicGauge implements metrics.Gauge using sync/atomic.
type AtomicGauge struct {
	val atomic.Int64
}

// Set replaces gauge value.
func (g *AtomicGauge) Set(value float64) { g.val.Store(int64(value)) }

// Add adjusts gauge value by delta.
func (g *AtomicGauge) Add(delta float64) { g.val.Add(int64(delta)) }

// Value returns current gauge value.
func (g *AtomicGauge) Value() int64 { return g.val.Load() }

// AppMetrics aggregates instrumentation across application components.
type AppMetrics struct {
	BreakerRequests   *AtomicCounter
	BreakerFailures   *AtomicCounter
	BreakerState      *AtomicGauge
	PoolSubmitted     *AtomicCounter
	PoolCompleted     *AtomicCounter
	PoolDropped       *AtomicCounter
	PoolPanics        *AtomicCounter
	PoolQueueSize     *AtomicGauge
	CacheHits         *AtomicCounter
	CacheMisses       *AtomicCounter
	CacheEvictions    *AtomicCounter
	HTTPRequestsTotal *AtomicCounter
	HTTPResponses2xx  *AtomicCounter
	HTTPResponses4xx  *AtomicCounter
	HTTPResponses5xx  *AtomicCounter
	HTTPDurationSumMs *AtomicCounter
}

// NewAppMetrics initializes zero-valued atomic metric collectors.
func NewAppMetrics() *AppMetrics {
	return &AppMetrics{
		BreakerRequests:   &AtomicCounter{},
		BreakerFailures:   &AtomicCounter{},
		BreakerState:      &AtomicGauge{},
		PoolSubmitted:     &AtomicCounter{},
		PoolCompleted:     &AtomicCounter{},
		PoolDropped:       &AtomicCounter{},
		PoolPanics:        &AtomicCounter{},
		PoolQueueSize:     &AtomicGauge{},
		CacheHits:         &AtomicCounter{},
		CacheMisses:       &AtomicCounter{},
		CacheEvictions:    &AtomicCounter{},
		HTTPRequestsTotal: &AtomicCounter{},
		HTTPResponses2xx:  &AtomicCounter{},
		HTTPResponses4xx:  &AtomicCounter{},
		HTTPResponses5xx:  &AtomicCounter{},
		HTTPDurationSumMs: &AtomicCounter{},
	}
}

// App encapsulates all runtime dependencies for the reference microservice.
type App struct {
	Config     Config
	Router     *gin.Engine
	Cache      *cache.TypedCache[[]Item]
	Pool       *workerpool.Pool
	Health     *health.Service
	Shutdown   *shutdown.Manager
	Logger     *slog.Logger
	Breaker    *circuitbreaker.ConsecutiveBreaker
	Metrics    *AppMetrics
	HTTPClient *http.Client
	Clock      clock.Clock
	FetchQuote func(ctx context.Context, symbol string) (Quote, error)
	HTTPServer *http.Server
}

// AppOption configures optional App dependencies.
type AppOption func(*appOptions)

type appOptions struct {
	clk           clock.Clock
	httpClient    *http.Client
	loggerHandler slog.Handler
}

// WithClock sets a custom clock source (e.g. clock.FakeClock) for deterministic testing.
func WithClock(c clock.Clock) AppOption {
	return func(o *appOptions) {
		if c != nil {
			o.clk = c
		}
	}
}

// WithHTTPClient overrides the default resilient HTTP client.
func WithHTTPClient(client *http.Client) AppOption {
	return func(o *appOptions) {
		if client != nil {
			o.httpClient = client
		}
	}
}

// WithLoggerHandler configures the underlying slog.Handler before wrapping with logger.NewRedactingHandler.
func WithLoggerHandler(h slog.Handler) AppOption {
	return func(o *appOptions) {
		if h != nil {
			o.loggerHandler = h
		}
	}
}

// NewApp initializes and wires all application components together.
func NewApp(opts ...AppOption) *App {
	cfg := LoadConfig()

	appOpts := appOptions{
		clk: clock.NewReal(),
	}
	for _, opt := range opts {
		if opt != nil {
			opt(&appOpts)
		}
	}

	baseHandler := appOpts.loggerHandler
	if baseHandler == nil {
		baseHandler = logger.Default().Handler()
	}
	redactingHandler := logger.NewRedactingHandler(
		baseHandler,
		logger.WithRedactedKeys("password", "token", "secret", "authorization"),
	)
	appLogger := slog.New(redactingHandler)

	appMetrics := NewAppMetrics()

	// 1. In-memory typed cache with metrics instrumentation
	itemCache := cache.NewTypedCache[[]Item](
		cache.WithMetrics[[]Item](cache.Metrics{
			Hits:      appMetrics.CacheHits,
			Misses:    appMetrics.CacheMisses,
			Evictions: appMetrics.CacheEvictions,
		}),
	)

	// 2. Bounded worker pool with panic recovery and metrics
	pool := workerpool.New(
		cfg.WorkerPoolSize,
		cfg.WorkerQueueCap,
		workerpool.WithLogger(appLogger),
		workerpool.WithMetrics(workerpool.Metrics{
			Submitted: appMetrics.PoolSubmitted,
			Completed: appMetrics.PoolCompleted,
			Dropped:   appMetrics.PoolDropped,
			Panics:    appMetrics.PoolPanics,
			QueueSize: appMetrics.PoolQueueSize,
		}),
	)

	// 3. Outbound Circuit Breaker (5 failures, 15s reset) with metrics
	cb := circuitbreaker.NewConsecutiveBreaker(
		5,
		15*time.Second,
		circuitbreaker.WithMetrics(circuitbreaker.Metrics{
			Requests: appMetrics.BreakerRequests,
			Failures: appMetrics.BreakerFailures,
			State:    appMetrics.BreakerState,
		}),
		circuitbreaker.WithOnStateChange(func(from, to circuitbreaker.State) {
			appLogger.Warn("circuit breaker state transition",
				slog.String("from", from.String()),
				slog.String("to", to.String()),
			)
		}),
	)

	// 4. Resilient HTTP Client composing Circuit Breaker, Retries, and Timeouts
	httpClient := appOpts.httpClient
	if httpClient == nil {
		httpClient = httpclient.New(
			httpclient.WithCircuitBreaker(cb),
			httpclient.WithRetry(retry.Config{
				Attempts:    3,
				InitialWait: 20 * time.Millisecond,
				MaxWait:     100 * time.Millisecond,
				Strategy:    retry.ExponentialJitter,
			}),
			httpclient.WithTotalTimeout(2*time.Second),
			httpclient.WithClock(appOpts.clk),
		)
	}

	// 5. Health service with dependency checker
	healthSvc := health.New(
		health.WithChecker("cache", func(ctx context.Context) error {
			if itemCache == nil {
				return errors.New("item cache not initialized")
			}
			return nil
		}),
		health.WithChecker("workerpool", func(ctx context.Context) error {
			if pool == nil {
				return errors.New("workerpool not initialized")
			}
			return nil
		}),
	)

	// 6. Graceful shutdown manager
	sm := shutdown.New(cfg.ShutdownTimeout)
	sm.Register("workerpool", func(ctx context.Context) error {
		pool.StopWait()
		return nil
	})
	sm.Register("cache", func(ctx context.Context) error {
		itemCache.Close()
		return nil
	})

	// 7. Configure Gin router with 12-stage middleware pipeline
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()

	r.Use(
		ginmw.RequestID(),
		ginmw.SecurityHeaders(),
		ginmw.CORS([]string{"*"}),
		recovery.Middleware(recovery.WithLogger(appLogger)),
		ginmw.Logger(),
		ginmw.Telemetry("order-service"),
		ginmw.GlobalRateLimit(),
		ginmw.LimitBodyDefault(),
		func(c *gin.Context) {
			start := appOpts.clk.Now()
			c.Next()
			durationMs := appOpts.clk.Since(start).Milliseconds()
			appMetrics.HTTPRequestsTotal.Add(1)
			appMetrics.HTTPDurationSumMs.Add(float64(durationMs))
			status := c.Writer.Status()
			switch {
			case status >= 200 && status < 300:
				appMetrics.HTTPResponses2xx.Add(1)
			case status >= 400 && status < 500:
				appMetrics.HTTPResponses4xx.Add(1)
			case status >= 500:
				appMetrics.HTTPResponses5xx.Add(1)
			}
		},
	)

	app := &App{
		Config:     cfg,
		Router:     r,
		Cache:      itemCache,
		Pool:       pool,
		Health:     healthSvc,
		Shutdown:   sm,
		Logger:     appLogger,
		Breaker:    cb,
		Metrics:    appMetrics,
		HTTPClient: httpClient,
		Clock:      appOpts.clk,
	}

	app.FetchQuote = func(ctx context.Context, symbol string) (Quote, error) {
		if app.Config.QuoteServiceURL != "" {
			return app.FetchQuoteHTTP(ctx, app.Config.QuoteServiceURL, symbol)
		}
		// Default simulated external quote fetcher using configured clock and timeutil business days
		now := app.Clock.Now().UTC()
		return Quote{
			Symbol:     symbol,
			Price:      142.50,
			FetchedAt:  now,
			ValidUntil: timeutil.AddBusinessDays(now, 1),
		}, nil
	}

	app.registerRoutes()
	return app
}

func (a *App) registerRoutes() {
	// Health probes
	a.Router.GET("/health/live", gin.WrapF(health.LivenessHandler))
	a.Router.GET("/health/ready", gin.WrapF(health.ReadinessHandler))
	a.Router.GET("/health", gin.WrapF(a.Health.Handler))

	// Observability & Metrics
	a.Router.GET("/metrics", a.handleMetrics)

	// API endpoints
	v1 := a.Router.Group("/api/v1")
	v1.GET("/items", a.handleGetItems)
	v1.GET("/items/:id", a.handleGetItemByID)
	v1.GET("/quotes", a.handleGetQuote)
	v1.POST("/tasks", a.handleCreateTask)
}

func (a *App) handleMetrics(c *gin.Context) {
	accept := c.GetHeader("Accept")
	if strings.Contains(accept, "text/plain") || strings.Contains(accept, "openmetrics") || c.Query("format") == "prometheus" {
		c.Header("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
		var sb strings.Builder
		sb.WriteString("# HELP app_circuit_breaker_requests_total Total number of requests through circuit breaker\n")
		sb.WriteString("# TYPE app_circuit_breaker_requests_total counter\n")
		fmt.Fprintf(&sb, "app_circuit_breaker_requests_total %d\n\n", a.Metrics.BreakerRequests.Value())

		sb.WriteString("# HELP app_circuit_breaker_failures_total Total number of failed requests through circuit breaker\n")
		sb.WriteString("# TYPE app_circuit_breaker_failures_total counter\n")
		fmt.Fprintf(&sb, "app_circuit_breaker_failures_total %d\n\n", a.Metrics.BreakerFailures.Value())

		sb.WriteString("# HELP app_circuit_breaker_state Current state of circuit breaker (0=Closed, 1=HalfOpen, 2=Open)\n")
		sb.WriteString("# TYPE app_circuit_breaker_state gauge\n")
		fmt.Fprintf(&sb, "app_circuit_breaker_state %d\n\n", a.Metrics.BreakerState.Value())

		sb.WriteString("# HELP app_workerpool_tasks_submitted_total Total number of tasks submitted to worker pool\n")
		sb.WriteString("# TYPE app_workerpool_tasks_submitted_total counter\n")
		fmt.Fprintf(&sb, "app_workerpool_tasks_submitted_total %d\n\n", a.Metrics.PoolSubmitted.Value())

		sb.WriteString("# HELP app_workerpool_tasks_completed_total Total number of tasks completed by worker pool\n")
		sb.WriteString("# TYPE app_workerpool_tasks_completed_total counter\n")
		fmt.Fprintf(&sb, "app_workerpool_tasks_completed_total %d\n\n", a.Metrics.PoolCompleted.Value())

		sb.WriteString("# HELP app_workerpool_tasks_dropped_total Total number of tasks dropped by worker pool\n")
		sb.WriteString("# TYPE app_workerpool_tasks_dropped_total counter\n")
		fmt.Fprintf(&sb, "app_workerpool_tasks_dropped_total %d\n\n", a.Metrics.PoolDropped.Value())

		sb.WriteString("# HELP app_workerpool_queue_size Current queue size of worker pool\n")
		sb.WriteString("# TYPE app_workerpool_queue_size gauge\n")
		fmt.Fprintf(&sb, "app_workerpool_queue_size %d\n\n", a.Metrics.PoolQueueSize.Value())

		sb.WriteString("# HELP app_cache_hits_total Total number of cache hits\n")
		sb.WriteString("# TYPE app_cache_hits_total counter\n")
		fmt.Fprintf(&sb, "app_cache_hits_total %d\n\n", a.Metrics.CacheHits.Value())

		sb.WriteString("# HELP app_cache_misses_total Total number of cache misses\n")
		sb.WriteString("# TYPE app_cache_misses_total counter\n")
		fmt.Fprintf(&sb, "app_cache_misses_total %d\n\n", a.Metrics.CacheMisses.Value())

		sb.WriteString("# HELP app_cache_evictions_total Total number of cache evictions\n")
		sb.WriteString("# TYPE app_cache_evictions_total counter\n")
		fmt.Fprintf(&sb, "app_cache_evictions_total %d\n\n", a.Metrics.CacheEvictions.Value())

		sb.WriteString("# HELP http_requests_total Total number of HTTP requests processed\n")
		sb.WriteString("# TYPE http_requests_total counter\n")
		fmt.Fprintf(&sb, "http_requests_total{status=\"2xx\"} %d\n", a.Metrics.HTTPResponses2xx.Value())
		fmt.Fprintf(&sb, "http_requests_total{status=\"4xx\"} %d\n", a.Metrics.HTTPResponses4xx.Value())
		fmt.Fprintf(&sb, "http_requests_total{status=\"5xx\"} %d\n\n", a.Metrics.HTTPResponses5xx.Value())

		sb.WriteString("# HELP http_request_duration_ms_total Total duration of HTTP requests in milliseconds\n")
		sb.WriteString("# TYPE http_request_duration_ms_total counter\n")
		fmt.Fprintf(&sb, "http_request_duration_ms_total %d\n", a.Metrics.HTTPDurationSumMs.Value())

		c.String(http.StatusOK, sb.String())
		return
	}

	httputil.OK(c.Writer, map[string]any{
		"http": map[string]any{
			"requests_total":    a.Metrics.HTTPRequestsTotal.Value(),
			"responses_2xx":     a.Metrics.HTTPResponses2xx.Value(),
			"responses_4xx":     a.Metrics.HTTPResponses4xx.Value(),
			"responses_5xx":     a.Metrics.HTTPResponses5xx.Value(),
			"duration_ms_total": a.Metrics.HTTPDurationSumMs.Value(),
		},
		"circuit_breaker": map[string]any{
			"requests": a.Metrics.BreakerRequests.Value(),
			"failures": a.Metrics.BreakerFailures.Value(),
			"state":    a.Metrics.BreakerState.Value(),
		},
		"worker_pool": map[string]any{
			"submitted":  a.Metrics.PoolSubmitted.Value(),
			"completed":  a.Metrics.PoolCompleted.Value(),
			"dropped":    a.Metrics.PoolDropped.Value(),
			"queue_size": a.Metrics.PoolQueueSize.Value(),
		},
		"cache": map[string]any{
			"hits":      a.Metrics.CacheHits.Value(),
			"misses":    a.Metrics.CacheMisses.Value(),
			"evictions": a.Metrics.CacheEvictions.Value(),
		},
	})
}

func (a *App) handleGetItems(c *gin.Context) {
	params := pagination.Parse(c.Request)
	cacheKey := fmt.Sprintf("items:page:%d:limit:%d", params.Page, params.Limit)

	items, err := a.Cache.GetOrFetch(c.Request.Context(), cacheKey, 2*time.Minute, func(ctx context.Context) ([]Item, error) {
		res := make([]Item, 0, params.Limit)
		for i := 0; i < params.Limit; i++ {
			idx := params.Offset() + i + 1
			if idx > 100 {
				break
			}
			category := "standard"
			if idx%2 == 0 {
				category = "premium"
			}
			res = append(res, Item{
				ID:       fmt.Sprintf("item-%d", idx),
				Name:     fmt.Sprintf("Product %d", idx),
				Category: category,
			})
		}
		return res, nil
	})
	if err != nil {
		httputil.Error(c.Writer, err.Error(), http.StatusInternalServerError)
		return
	}

	// Filter results dynamically using idiomatic Go loop
	categoryFilter := c.Query("category")
	filtered := items
	if categoryFilter != "" {
		filtered = make([]Item, 0, len(items))
		for _, it := range items {
			if strings.EqualFold(it.Category, categoryFilter) {
				filtered = append(filtered, it)
			}
		}
	}

	// Demonstrate sliceutil.Chunk for batching/chunking items
	if chunkSizeStr := c.Query("chunk_size"); chunkSizeStr != "" {
		if size, err := strconv.Atoi(chunkSizeStr); err == nil && size > 0 {
			chunks := sliceutil.Chunk(filtered, size)
			httputil.OK(c.Writer, map[string]any{
				"chunks":     chunks,
				"num_chunks": len(chunks),
				"total":      len(filtered),
			})
			return
		}
	}

	resp := pagination.NewTypedResponse(filtered, 100, params)
	httputil.OK(c.Writer, resp)
}

func (a *App) handleGetItemByID(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		httputil.ErrorFromDomain(c.Writer, apperror.BadRequest("missing item id"))
		return
	}

	// In real applications, this queries database; here we demonstrate canonical apperror mapping
	if !strings.HasPrefix(id, "item-") {
		httputil.ErrorFromDomain(c.Writer, apperror.NotFound(fmt.Sprintf("item %q not found", id)))
		return
	}

	item := Item{
		ID:       id,
		Name:     fmt.Sprintf("Product %s", strings.TrimPrefix(id, "item-")),
		Category: "standard",
	}
	httputil.OK(c.Writer, item)
}

// FetchQuoteHTTP fetches quote data from an external HTTP quote service using the resilient a.HTTPClient.
func (a *App) FetchQuoteHTTP(ctx context.Context, quoteServiceURL, symbol string) (Quote, error) {
	targetURL := fmt.Sprintf("%s/quotes?symbol=%s", strings.TrimRight(quoteServiceURL, "/"), symbol)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, targetURL, nil)
	if err != nil {
		return Quote{}, err
	}
	resp, err := a.HTTPClient.Do(req)
	if err != nil {
		return Quote{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return Quote{}, fmt.Errorf("quote upstream returned status %d", resp.StatusCode)
	}

	var q Quote
	if err := json.NewDecoder(resp.Body).Decode(&q); err != nil {
		return Quote{}, err
	}
	if q.FetchedAt.IsZero() {
		q.FetchedAt = a.Clock.Now().UTC()
	}
	if q.ValidUntil.IsZero() {
		q.ValidUntil = timeutil.AddBusinessDays(q.FetchedAt, 1)
	}
	return q, nil
}

func (a *App) handleGetQuote(c *gin.Context) {
	symbol := c.Query("symbol")
	if symbol == "" {
		httputil.ErrorFromDomain(c.Writer, apperror.BadRequest("symbol query parameter is required"))
		return
	}

	// Fetch quote protected by Circuit Breaker and Exponential Jitter Retries.
	// Demonstrates using a.FetchQuote (which leverages a.HTTPClient when an upstream service is configured).
	var quote Quote
	err := a.Breaker.Execute(c.Request.Context(), func() error {
		res, retryErr := retry.DoWithResult(c.Request.Context(), retry.Config{
			Attempts:    3,
			InitialWait: 20 * time.Millisecond,
			MaxWait:     100 * time.Millisecond,
			Strategy:    retry.ExponentialJitter,
		}, func(ctx context.Context) (Quote, error) {
			return a.FetchQuote(ctx, symbol)
		})
		if retryErr != nil {
			return retryErr
		}
		quote = res
		return nil
	})

	if err != nil {
		if errors.Is(err, circuitbreaker.ErrCircuitOpen) {
			httputil.ErrorFromDomain(c.Writer, apperror.Unavailable("external quote service is currently unavailable"))
			return
		}
		httputil.ErrorFromDomain(c.Writer, apperror.Wrap(err, apperror.CodeInternal, "failed to fetch quote"))
		return
	}

	if quote.ValidUntil.IsZero() {
		baseTime := quote.FetchedAt
		if baseTime.IsZero() {
			baseTime = a.Clock.Now().UTC()
		}
		quote.ValidUntil = timeutil.AddBusinessDays(baseTime, 1)
	}

	httputil.OK(c.Writer, quote)
}

type createTaskRequest struct {
	Payload string `json:"payload" binding:"required"`
}

func (a *App) handleCreateTask(c *gin.Context) {
	var req createTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httputil.ValidationError(c.Writer, err.Error())
		return
	}

	// Use logger from context
	reqLogger := logger.FromContext(c.Request.Context())

	// Submit async work to bounded workerpool
	if err := a.Pool.Submit(func() {
		reqLogger.Info("executing async background task", slog.String("payload", req.Payload))
	}); err != nil {
		httputil.Error(c.Writer, err.Error(), http.StatusServiceUnavailable)
		return
	}

	httputil.Created(c.Writer, map[string]string{
		"status":  "queued",
		"payload": req.Payload,
	})
}

func main() {
	app := NewApp()

	srv := &http.Server{
		Addr:              app.Config.Port,
		Handler:           app.Router,
		ReadHeaderTimeout: app.Config.ReadHeaderTimeout,
	}
	app.HTTPServer = srv

	app.Shutdown.Register("http_server", func(ctx context.Context) error {
		return srv.Shutdown(ctx)
	})

	slog.Info("starting reference microservice", slog.String("addr", app.Config.Port))
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server fatal error", slog.Any("error", err))
		}
	}()

	if err := app.Shutdown.Wait(); err != nil {
		slog.Error("shutdown error", slog.Any("error", err))
	}
}
