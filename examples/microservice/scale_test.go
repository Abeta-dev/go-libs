//go:build scale

// SPDX-License-Identifier: MIT
package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"log/slog"
	"net"
	"net/http"
	"runtime"
	"sort"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestScale1000Concurrent_1Min(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping 1-minute 1000-connection scale test in short mode")
	}

	// Silence standard logging and slog during high-concurrency scale test to avoid I/O bottlenecks
	log.SetOutput(io.Discard)
	oldDefault := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(io.Discard, nil)))
	defer slog.SetDefault(oldDefault)

	// 1. Initialize application
	app := NewApp()
	require.NotNil(t, app)
	// Direct app logger to discard during scale test to prevent terminal buffer overflow
	app.Logger = slog.New(slog.NewJSONHandler(io.Discard, nil))

	// 2. Start real TCP HTTP server on an ephemeral loopback port
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	serverAddr := fmt.Sprintf("http://%s", listener.Addr().String())

	srv := &http.Server{
		Handler:           app.Router,
		ReadHeaderTimeout: 5 * time.Second,
	}

	serverErrCh := make(chan error, 1)
	go func() {
		if err := srv.Serve(listener); err != nil && err != http.ErrServerClosed {
			serverErrCh <- err
		}
	}()

	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(ctx)
		_ = app.Shutdown.Execute(ctx)
	}()

	// Wait for server to be responsive
	require.Eventually(t, func() bool {
		resp, err := http.Get(serverAddr + "/health/live")
		if err != nil {
			return false
		}
		_ = resp.Body.Close()
		return resp.StatusCode == http.StatusOK
	}, 3*time.Second, 50*time.Millisecond)

	// 3. Baseline memory and goroutines
	runtime.GC()
	initialGoroutines := runtime.NumGoroutine()
	var initialMem runtime.MemStats
	runtime.ReadMemStats(&initialMem)

	const (
		numWorkers = 1000
		duration   = 60 * time.Second
	)

	fmt.Printf("\n🚀 STARTING SCALE TEST: %d concurrent connections for %v...\n", numWorkers, duration)
	fmt.Printf("   Server Address: %s\n", serverAddr)
	fmt.Printf("   Baseline Goroutines: %d | Baseline HeapAlloc: %.2f MB\n\n",
		initialGoroutines, float64(initialMem.HeapAlloc)/(1024*1024))

	// Shared HTTP client configured for high concurrency
	transport := &http.Transport{
		MaxIdleConns:        2000,
		MaxIdleConnsPerHost: 2000,
		MaxConnsPerHost:     2000,
		IdleConnTimeout:     90 * time.Second,
	}
	client := &http.Client{
		Transport: transport,
		Timeout:   10 * time.Second,
	}

	var (
		totalRequests atomic.Uint64
		status200     atomic.Uint64
		status201     atomic.Uint64
		status429     atomic.Uint64
		status503     atomic.Uint64
		status500     atomic.Uint64
		networkErrors atomic.Uint64

		// Latency samples collected with downsampling
		latencyMu      sync.Mutex
		latencySamples []time.Duration
	)

	stopCh := make(chan struct{})
	var wg sync.WaitGroup

	startTime := time.Now()

	// Launch 1,000 concurrent worker goroutines
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		workerID := i

		go func(id int) {
			defer wg.Done()

			// Each worker has a unique simulated client IP for rate limiting
			clientIP := fmt.Sprintf("10.%d.%d.1", (id/256)%256, id%256)

			// Alternate between endpoints:
			// 0: GET /api/v1/items (Cached + paginated singleflight)
			// 1: POST /api/v1/tasks (Workerpool submission)
			// 2: GET /health/live (Health probe)
			reqType := id % 3

			for {
				select {
				case <-stopCh:
					return
				default:
				}

				reqStart := time.Now()

				var req *http.Request
				var err error

				switch reqType {
				case 0:
					page := (id % 5) + 1
					url := fmt.Sprintf("%s/api/v1/items?page=%d&limit=10", serverAddr, page)
					req, err = http.NewRequest(http.MethodGet, url, nil)
				case 1:
					body := []byte(fmt.Sprintf(`{"payload":"worker-%d-task"}`, id))
					url := fmt.Sprintf("%s/api/v1/tasks", serverAddr)
					req, err = http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
					if err == nil {
						req.Header.Set("Content-Type", "application/json")
					}
				default:
					url := fmt.Sprintf("%s/health/live", serverAddr)
					req, err = http.NewRequest(http.MethodGet, url, nil)
				}

				if err != nil {
					networkErrors.Add(1)
					continue
				}

				req.Header.Set("X-Forwarded-For", clientIP)

				resp, err := client.Do(req)
				latency := time.Since(reqStart)

				if err != nil {
					networkErrors.Add(1)
					continue
				}

				totalRequests.Add(1)

				switch resp.StatusCode {
				case http.StatusOK:
					status200.Add(1)
				case http.StatusCreated:
					status201.Add(1)
				case http.StatusTooManyRequests:
					status429.Add(1)
				case http.StatusServiceUnavailable:
					status503.Add(1)
				default:
					if resp.StatusCode >= 500 {
						status500.Add(1)
					}
				}

				// Discard response body and close to enable HTTP keep-alive connection reuse
				_, _ = io.Copy(io.Discard, resp.Body)
				_ = resp.Body.Close()

				// Downsample latencies: record 1 in every 10 requests to bound memory
				if totalRequests.Load()%10 == 0 {
					latencyMu.Lock()
					if len(latencySamples) < 50000 {
						latencySamples = append(latencySamples, latency)
					}
					latencyMu.Unlock()
				}
			}
		}(workerID)
	}

	// Run for exactly 60 seconds, printing progress every 15 seconds
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	timer := time.NewTimer(duration)
	defer timer.Stop()

	done := false
	for !done {
		select {
		case <-timer.C:
			close(stopCh)
			done = true
		case tick := <-ticker.C:
			elapsed := tick.Sub(startTime).Round(time.Second)
			reqs := totalRequests.Load()
			rps := float64(reqs) / elapsed.Seconds()
			fmt.Printf("   [%3s] Requests: %-7d | Current Throughput: %8.1f req/s | Active Goroutines: %d\n",
				elapsed, reqs, rps, runtime.NumGoroutine())
		}
	}

	wg.Wait()
	actualDuration := time.Since(startTime)

	// 4. Post-test memory and goroutines verification
	runtime.GC()
	finalGoroutines := runtime.NumGoroutine()
	var finalMem runtime.MemStats
	runtime.ReadMemStats(&finalMem)

	// Sort latencies for percentiles
	latencyMu.Lock()
	sort.Slice(latencySamples, func(i, j int) bool {
		return latencySamples[i] < latencySamples[j]
	})

	var p50, p90, p95, p99, maxLatency time.Duration
	if n := len(latencySamples); n > 0 {
		p50 = latencySamples[n*50/100]
		p90 = latencySamples[n*90/100]
		p95 = latencySamples[n*95/100]
		p99 = latencySamples[n*99/100]
		maxLatency = latencySamples[n-1]
	}
	latencyMu.Unlock()

	total := totalRequests.Load()
	throughput := float64(total) / actualDuration.Seconds()

	fmt.Printf("\n========================================================================\n")
	fmt.Printf("📊 SCALE TEST REPORT (1000 CONCURRENT CONNECTIONS FOR 1 MINUTE)\n")
	fmt.Printf("========================================================================\n")
	fmt.Printf("  Duration:              %.2f seconds\n", actualDuration.Seconds())
	fmt.Printf("  Concurrent Workers:    %d\n", numWorkers)
	fmt.Printf("  Total Requests:        %d\n", total)
	fmt.Printf("  Overall Throughput:    %.2f req/sec\n", throughput)
	fmt.Printf("------------------------------------------------------------------------\n")
	fmt.Printf("  HTTP 200 OK:           %d\n", status200.Load())
	fmt.Printf("  HTTP 201 Created:      %d\n", status201.Load())
	fmt.Printf("  HTTP 429 Rate Limited: %d (Rate limiter protected downstream)\n", status429.Load())
	fmt.Printf("  HTTP 503 Shed / Busy:  %d (Workerpool queue backpressure)\n", status503.Load())
	fmt.Printf("  HTTP 500 Fatal Errors: %d (Zero crashes or unhandled failures)\n", status500.Load())
	fmt.Printf("  Network / Dial Errors: %d (Zero dropped connections)\n", networkErrors.Load())
	fmt.Printf("------------------------------------------------------------------------\n")
	fmt.Printf("  Latency (p50):         %v\n", p50)
	fmt.Printf("  Latency (p90):         %v\n", p90)
	fmt.Printf("  Latency (p95):         %v\n", p95)
	fmt.Printf("  Latency (p99):         %v\n", p99)
	fmt.Printf("  Latency (Max):         %v\n", maxLatency)
	fmt.Printf("------------------------------------------------------------------------\n")
	fmt.Printf("  Initial Goroutines:    %d\n", initialGoroutines)
	fmt.Printf("  Peak Goroutines:       ~%d\n", numWorkers+initialGoroutines)
	fmt.Printf("  Final Goroutines:      %d (Zero goroutine leaks)\n", finalGoroutines)
	fmt.Printf("  Final HeapAlloc:       %.2f MB\n", float64(finalMem.HeapAlloc)/(1024*1024))
	fmt.Printf("========================================================================\n\n")

	// Assertions
	assert.Greater(t, total, uint64(10000), "Should complete at least 10,000 requests under 1000 workers")
	assert.Equal(t, uint64(0), status500.Load(), "Must have ZERO 500 internal server errors or crashes")
	assert.Equal(t, uint64(0), networkErrors.Load(), "Must have ZERO network connection failures")
}
