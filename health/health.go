// SPDX-License-Identifier: MIT

// Package health provides a generic, zero-dependency HTTP health check handler
// for Go microservices. It exposes liveness, readiness, and a detailed system
// info endpoint that any service can embed.
//
// Usage:
//
//	import "github.com/umesh0492/go-libs/health"
//
//	// Minimal — liveness only
//	router.Get("/health", health.LivenessHandler)
//
//	// Full detail — with custom checkers
//	checker := health.New(
//	     health.WithStartTime(startTime),
//	     health.WithChecker("database", dbChecker),
//	)
//	router.Get("/health", checker.Handler)
package health

import (
	"context"
	"encoding/json"
	"net/http"
	"runtime"
	"sync"
	"time"
)

// ── Response shapes ────────────────────────────────────────────────────────────

// Response is the standard health check JSON payload.
type Response struct {
	Status    string                 `json:"status"` // "healthy" | "degraded" | "unhealthy"
	Timestamp time.Time              `json:"timestamp"`
	Uptime    string                 `json:"uptime,omitempty"`
	Checks    map[string]CheckResult `json:"checks,omitempty"`
	Runtime   *RuntimeInfo           `json:"runtime,omitempty"`
}

// CheckResult represents the result of one named dependency check.
type CheckResult struct {
	Status  string `json:"status"` // "pass" | "fail"
	Message string `json:"message,omitempty"`
}

// RuntimeInfo exposes Go runtime statistics.
type RuntimeInfo struct {
	GoVersion  string `json:"go_version"`
	Goroutines int    `json:"goroutines"`
	NumCPU     int    `json:"num_cpu"`
	AllocMB    uint64 `json:"mem_alloc_mb"`
	SysMB      uint64 `json:"mem_sys_mb"`
	GCRuns     uint32 `json:"gc_runs"`
}

// ── Checker interface ──────────────────────────────────────────────────────────

// Checker is implemented by any dependency that can report its health.
// func checkDB(ctx context.Context) error { return pool.Ping(ctx) }
type Checker func(ctx context.Context) error

// ── Checker configuration ──────────────────────────────────────────────────────

// Service is the main health service with pluggable dependency checkers.
type Service struct {
	startTime   time.Time
	checkers    map[string]Checker
	withRuntime bool
}

// Option configures a Service.
type Option func(*Service)

// WithStartTime records the service start time for uptime calculation.
func WithStartTime(t time.Time) Option {
	return func(s *Service) { s.startTime = t }
}

// WithChecker registers a named dependency health checker.
// Multiple checkers can be registered; all run in parallel.
func WithChecker(name string, fn Checker) Option {
	return func(s *Service) { s.checkers[name] = fn }
}

// WithRuntimeInfo includes Go runtime statistics in the response.
func WithRuntimeInfo() Option {
	return func(s *Service) { s.withRuntime = true }
}

// New creates a configured health Service.
func New(opts ...Option) *Service {
	svc := &Service{
		startTime: time.Now(),
		checkers:  make(map[string]Checker),
	}
	for _, opt := range opts {
		opt(svc)
	}
	return svc
}

// Handler is an http.HandlerFunc that returns the full health response.
// Returns 200 if all checks pass, 503 if any fail.
func (s *Service) Handler(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	resp := Response{
		Status:    "healthy",
		Timestamp: time.Now().UTC(),
		Checks:    make(map[string]CheckResult),
	}

	if !s.startTime.IsZero() {
		resp.Uptime = time.Since(s.startTime).Round(time.Second).String()
	}

	// Run all checkers in parallel
	type checkOutput struct {
		name string
		res  CheckResult
		pass bool
	}

	resultsChan := make(chan checkOutput, len(s.checkers))
	var wg sync.WaitGroup

	for name, fn := range s.checkers {
		wg.Add(1)
		go func(checkName string, checkFn Checker) {
			defer wg.Done()
			if err := checkFn(ctx); err != nil {
				resultsChan <- checkOutput{name: checkName, res: CheckResult{Status: "fail", Message: err.Error()}, pass: false}
			} else {
				resultsChan <- checkOutput{name: checkName, res: CheckResult{Status: "pass"}, pass: true}
			}
		}(name, fn)
	}

	wg.Wait()
	close(resultsChan)

	allPass := true
	for out := range resultsChan {
		resp.Checks[out.name] = out.res
		if !out.pass {
			allPass = false
		}
	}

	if !allPass {
		resp.Status = "degraded"
	}

	if s.withRuntime {
		resp.Runtime = getRuntimeInfo()
	}

	status := http.StatusOK
	if !allPass {
		status = http.StatusServiceUnavailable
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(resp)
}

// ── Standalone handlers ────────────────────────────────────────────────────────

// LivenessHandler returns 200 OK with {"status":"ok"} — no dependency checks.
// Kubernetes liveness probes should point here. It NEVER returns 503.
func LivenessHandler(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}

// ReadinessHandler returns 200 OK with runtime stats. No dependency checks.
// Use as a readiness probe before registering with load balancers.
func ReadinessHandler(w http.ResponseWriter, _ *http.Request) {
	resp := Response{
		Status:    "ready",
		Timestamp: time.Now().UTC(),
		Runtime:   getRuntimeInfo(),
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(resp)
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func getRuntimeInfo() *RuntimeInfo {
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)
	return &RuntimeInfo{
		GoVersion:  runtime.Version(),
		Goroutines: runtime.NumGoroutine(),
		NumCPU:     runtime.NumCPU(),
		AllocMB:    mem.Alloc / 1024 / 1024,
		SysMB:      mem.Sys / 1024 / 1024,
		GCRuns:     mem.NumGC,
	}
}
