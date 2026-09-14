// SPDX-License-Identifier: MIT

package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

type Stats struct {
	TotalRequests atomic.Uint64
	Status200     atomic.Uint64
	Status404     atomic.Uint64
	Status429     atomic.Uint64
	Status500     atomic.Uint64
	Status503     atomic.Uint64
	OtherStatus   atomic.Uint64
	Errors        atomic.Uint64
}

func main() {
	targetRPM := flag.Int("rpm", 100000, "Target requests per minute (default 100,000)")
	duration := flag.Duration("duration", 15*time.Minute, "Duration of the traffic run (default 15m)")
	targetURL := flag.String("url", "http://localhost:8080", "Target base URL")
	concurrency := flag.Int("workers", 64, "Number of concurrent worker goroutines")
	flag.Parse()

	targetRPS := float64(*targetRPM) / 60.0
	intervalPerReq := time.Duration(float64(time.Second) / targetRPS)

	fmt.Printf("================================================================================\n")
	fmt.Printf("🚀 Starting High-Performance Golden Signals Traffic Generator\n")
	fmt.Printf("================================================================================\n")
	fmt.Printf("  Target Rate    : %d RPM (~%.1f requests/second)\n", *targetRPM, targetRPS)
	fmt.Printf("  Total Duration : %v\n", *duration)
	fmt.Printf("  Target Base URL: %s\n", *targetURL)
	fmt.Printf("  Worker Routines: %d\n", *concurrency)
	fmt.Printf("  Grafana UI     : http://localhost:3000/d/microservice-golden-signals/microservice-golden-signals-and-resilience\n")
	fmt.Printf("  Prometheus UI  : http://localhost:9090\n")
	fmt.Printf("================================================================================\n\n")

	ctx, cancel := context.WithTimeout(context.Background(), *duration)
	defer cancel()

	// Listen for interrupt signals to shut down gracefully
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		fmt.Println("\n⚠️  Termination signal received, draining traffic generator...")
		cancel()
	}()

	// Tuned high-throughput HTTP transport with keep-alive connection pooling
	transport := &http.Transport{
		MaxIdleConns:        500,
		MaxIdleConnsPerHost: 500,
		IdleConnTimeout:     90 * time.Second,
		DisableCompression:  true,
	}
	client := &http.Client{
		Transport: transport,
		Timeout:   2 * time.Second,
	}

	stats := &Stats{}
	startTime := time.Now()

	// Rate-pacing token bucket channel
	tokenChan := make(chan struct{}, 10000)
	go func() {
		ticker := time.NewTicker(intervalPerReq)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				select {
				case tokenChan <- struct{}{}:
				default:
					// Channel saturated, drop token to avoid burst
				}
			}
		}
	}()

	// Launch worker goroutines
	var wg sync.WaitGroup
	for w := 0; w < *concurrency; w++ {
		wg.Add(1)
		workerID := w
		go func() {
			defer wg.Done()
			// #nosec G404 -- synthetic load generator uses pseudorandom numbers for request distribution, not security
			rng := rand.New(rand.NewSource(time.Now().UnixNano() + int64(workerID*1000)))
			taskPayload := []byte(`{"payload":"mock background transaction"}`)

			for {
				select {
				case <-ctx.Done():
					return
				case <-tokenChan:
					// Dispatch request
					stats.TotalRequests.Add(1)

					// Roll random distribution for endpoint selection
					dice := rng.Intn(100)
					var req *http.Request
					var err error

					// Generate rotating simulated IP (10.0.0.1 to 10.0.7.255)
					simIP := fmt.Sprintf("10.0.%d.%d", rng.Intn(8), rng.Intn(256))
					// 1% of requests use a single abusive IP to showcase Rate Limiting (HTTP 429)
					if dice == 99 {
						simIP = "192.168.100.99"
					}

					switch {
					case dice < 60:
						// 60% Cache Reads (Fast path items)
						page := rng.Intn(5) + 1
						limit := 10
						req, err = http.NewRequestWithContext(ctx, http.MethodGet,
							fmt.Sprintf("%s/api/v1/items?page=%d&limit=%d", *targetURL, page, limit), nil)

					case dice < 75:
						// 15% Quote Service (Hits Circuit Breaker + Cache)
						symbols := []string{"AAPL", "GOOG", "MSFT", "AMZN"}
						sym := symbols[rng.Intn(len(symbols))]
						req, err = http.NewRequestWithContext(ctx, http.MethodGet,
							fmt.Sprintf("%s/api/v1/quotes?symbol=%s", *targetURL, sym), nil)

					case dice < 85:
						// 10% Kubernetes Health Probes (Liveness & Readiness)
						if dice%2 == 0 {
							req, err = http.NewRequestWithContext(ctx, http.MethodGet, *targetURL+"/health/live", nil)
						} else {
							req, err = http.NewRequestWithContext(ctx, http.MethodGet, *targetURL+"/health/ready", nil)
						}

					case dice < 92:
						// 7% Specific Item Details (Domain Entity lookup)
						itemID := rng.Intn(100) + 1
						req, err = http.NewRequestWithContext(ctx, http.MethodGet,
							fmt.Sprintf("%s/api/v1/items/item-%d", *targetURL, itemID), nil)

					case dice < 95:
						// 3% 404 Not Found (Tests AppError Domain Mapping & 4xx Error Signals)
						req, err = http.NewRequestWithContext(ctx, http.MethodGet,
							fmt.Sprintf("%s/api/v1/items/nonexistent-%d", *targetURL, rng.Intn(1000)), nil)

					case dice < 98:
						// 3% Worker Pool Tasks (Tests Async Queue Backlog & Task Processing)
						req, err = http.NewRequestWithContext(ctx, http.MethodPost,
							*targetURL+"/api/v1/tasks", bytes.NewReader(taskPayload))
						if req != nil {
							req.Header.Set("Content-Type", "application/json")
						}

					default:
						// 2% Upstream External Failures (Tests Circuit Breaker State & Fallbacks)
						req, err = http.NewRequestWithContext(ctx, http.MethodGet,
							*targetURL+"/api/v1/quotes?symbol=FAIL", nil)
					}

					if err != nil || req == nil {
						stats.Errors.Add(1)
						continue
					}

					// Set client IP & headers
					req.Header.Set("X-Forwarded-For", simIP)
					req.Header.Set("User-Agent", "GoldenSignals-LoadGen/1.1")

					resp, reqErr := client.Do(req)
					if reqErr != nil {
						stats.Errors.Add(1)
						continue
					}
					// Always drain and close response body to reuse TCP socket
					_, _ = io.Copy(io.Discard, resp.Body)
					_ = resp.Body.Close()

					switch resp.StatusCode {
					case http.StatusOK, http.StatusCreated:
						stats.Status200.Add(1)
					case http.StatusNotFound:
						stats.Status404.Add(1)
					case http.StatusTooManyRequests:
						stats.Status429.Add(1)
					case http.StatusInternalServerError:
						stats.Status500.Add(1)
					case http.StatusServiceUnavailable:
						stats.Status503.Add(1)
					default:
						stats.OtherStatus.Add(1)
					}
				}
			}
		}()
	}

	// Progress telemetry ticker (logs every 10 seconds)
	progressTicker := time.NewTicker(10 * time.Second)
	defer progressTicker.Stop()

	var lastTotal uint64
	lastReportTime := startTime

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case t := <-progressTicker.C:
				elapsed := t.Sub(startTime)
				windowDuration := t.Sub(lastReportTime).Seconds()
				currentTotal := stats.TotalRequests.Load()

				deltaReqs := currentTotal - lastTotal
				instRPS := float64(deltaReqs) / windowDuration
				instRPM := instRPS * 60.0

				lastTotal = currentTotal
				lastReportTime = t

				s200 := stats.Status200.Load()
				s404 := stats.Status404.Load()
				s429 := stats.Status429.Load()
				s503 := stats.Status503.Load()
				s500 := stats.Status500.Load()
				errs := stats.Errors.Load()

				fmt.Printf("[%02dm %02ds / %02dm 00s] Throughput: %7.0f RPM (%5.1f req/s) | Total: %8d | 2xx: %d | 404: %d | 429: %d | 500: %d | 503: %d | NetErr: %d\n",
					int(elapsed.Minutes()), int(elapsed.Seconds())%60,
					int(duration.Minutes()),
					instRPM, instRPS,
					currentTotal,
					s200, s404, s429, s500, s503, errs,
				)
			}
		}
	}()

	wg.Wait()

	totalDuration := time.Since(startTime)
	finalTotal := stats.TotalRequests.Load()
	avgRPM := float64(finalTotal) / totalDuration.Minutes()
	avgRPS := float64(finalTotal) / totalDuration.Seconds()

	fmt.Printf("\n================================================================================\n")
	fmt.Printf("🏁 Traffic Simulation Complete!\n")
	fmt.Printf("================================================================================\n")
	fmt.Printf("  Total Elapsed Time: %v\n", totalDuration.Round(time.Second))
	fmt.Printf("  Total Requests Sent: %d\n", finalTotal)
	fmt.Printf("  Average Throughput : %.0f RPM (%.1f req/s)\n", avgRPM, avgRPS)
	fmt.Printf("  Status Breakdown   :\n")
	fmt.Printf("    - 200 OK / 201 Created   : %8d (%5.1f%%)\n", stats.Status200.Load(), float64(stats.Status200.Load())/float64(finalTotal)*100)
	fmt.Printf("    - 404 Not Found (AppError): %8d (%5.1f%%)\n", stats.Status404.Load(), float64(stats.Status404.Load())/float64(finalTotal)*100)
	fmt.Printf("    - 429 Too Many Requests  : %8d (%5.1f%%)\n", stats.Status429.Load(), float64(stats.Status429.Load())/float64(finalTotal)*100)
	fmt.Printf("    - 503 Service Unavailable: %8d (%5.1f%%)\n", stats.Status503.Load(), float64(stats.Status503.Load())/float64(finalTotal)*100)
	fmt.Printf("    - 500 Internal Error     : %8d (%5.1f%%)\n", stats.Status500.Load(), float64(stats.Status500.Load())/float64(finalTotal)*100)
	fmt.Printf("    - Network / Socket Errors: %8d\n", stats.Errors.Load())
	fmt.Printf("================================================================================\n\n")
}
