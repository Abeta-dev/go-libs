// SPDX-License-Identifier: MIT

package ginmw

import (
	"fmt"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"
)

var processStartTime = time.Now()

// MetricsTracker holds thread-safe counters and gauges for HTTP request processing.
type MetricsTracker struct {
	RequestsTotal  atomic.Uint64
	ErrorsTotal    atomic.Uint64
	Status2xx      atomic.Uint64
	Status4xx      atomic.Uint64
	Status5xx      atomic.Uint64
	ActiveRequests atomic.Int64
	DurationSumMs  atomic.Int64
}

// DefaultMetrics is the package-level default metrics tracker.
var DefaultMetrics = &MetricsTracker{}

// REDMetrics returns a Gin middleware that records Rate, Errors, and Duration (RED) metrics.
func REDMetrics(tracker ...*MetricsTracker) gin.HandlerFunc {
	t := DefaultMetrics
	if len(tracker) > 0 && tracker[0] != nil {
		t = tracker[0]
	}

	return func(c *gin.Context) {
		start := time.Now()
		t.RequestsTotal.Add(1)
		t.ActiveRequests.Add(1)

		defer func() {
			t.ActiveRequests.Add(-1)
			duration := time.Since(start).Milliseconds()
			t.DurationSumMs.Add(duration)

			status := c.Writer.Status()
			switch {
			case status >= 200 && status < 300:
				t.Status2xx.Add(1)
			case status >= 400 && status < 500:
				t.Status4xx.Add(1)
				t.ErrorsTotal.Add(1)
			case status >= 500:
				t.Status5xx.Add(1)
				t.ErrorsTotal.Add(1)
			}
		}()

		c.Next()
	}
}

// MetricsEndpoint returns a Gin handler serving OpenMetrics Prometheus text format or JSON.
func MetricsEndpoint(tracker ...*MetricsTracker) gin.HandlerFunc {
	t := DefaultMetrics
	if len(tracker) > 0 && tracker[0] != nil {
		t = tracker[0]
	}

	return func(c *gin.Context) {
		accept := c.GetHeader("Accept")
		format := c.Query("format")
		uptime := int64(time.Since(processStartTime).Seconds())

		if strings.Contains(accept, "application/json") || format == "json" {
			c.JSON(http.StatusOK, gin.H{
				"uptime_seconds":        uptime,
				"http_requests_total":   t.RequestsTotal.Load(),
				"http_errors_total":     t.ErrorsTotal.Load(),
				"http_status_2xx_total": t.Status2xx.Load(),
				"http_status_4xx_total": t.Status4xx.Load(),
				"http_status_5xx_total": t.Status5xx.Load(),
				"http_active_requests":  t.ActiveRequests.Load(),
				"http_duration_sum_ms":  t.DurationSumMs.Load(),
			})
			return
		}

		c.Header("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
		var sb strings.Builder

		sb.WriteString("# HELP app_uptime_seconds Total service uptime in seconds\n")
		sb.WriteString("# TYPE app_uptime_seconds gauge\n")
		fmt.Fprintf(&sb, "app_uptime_seconds %d\n\n", uptime)

		sb.WriteString("# HELP http_requests_total Total number of HTTP requests processed\n")
		sb.WriteString("# TYPE http_requests_total counter\n")
		fmt.Fprintf(&sb, "http_requests_total %d\n\n", t.RequestsTotal.Load())

		sb.WriteString("# HELP http_errors_total Total number of failed HTTP requests\n")
		sb.WriteString("# TYPE http_errors_total counter\n")
		fmt.Fprintf(&sb, "http_errors_total %d\n\n", t.ErrorsTotal.Load())

		sb.WriteString("# HELP http_status_total HTTP responses partitioned by status class\n")
		sb.WriteString("# TYPE http_status_total counter\n")
		fmt.Fprintf(&sb, "http_status_total{class=\"2xx\"} %d\n", t.Status2xx.Load())
		fmt.Fprintf(&sb, "http_status_total{class=\"4xx\"} %d\n", t.Status4xx.Load())
		fmt.Fprintf(&sb, "http_status_total{class=\"5xx\"} %d\n\n", t.Status5xx.Load())

		sb.WriteString("# HELP http_active_requests Current number of in-flight HTTP requests\n")
		sb.WriteString("# TYPE http_active_requests gauge\n")
		fmt.Fprintf(&sb, "http_active_requests %d\n\n", t.ActiveRequests.Load())

		sb.WriteString("# HELP http_duration_sum_ms Total request processing time in milliseconds\n")
		sb.WriteString("# TYPE http_duration_sum_ms counter\n")
		fmt.Fprintf(&sb, "http_duration_sum_ms %d\n", t.DurationSumMs.Load())

		c.String(http.StatusOK, sb.String())
	}
}
