// SPDX-License-Identifier: MIT

package metrics_test

import (
	"testing"

	"github.com/umesh0492/go-libs/metrics"
)

// Verify interface satisfaction at compile time.
var _ metrics.Counter = metrics.NoopCounter{}
var _ metrics.Gauge = metrics.NoopGauge{}
var _ metrics.Histogram = metrics.NoopHistogram{}

func TestNoopCounter(t *testing.T) {
	c := metrics.NoopCounter{}
	c.Inc()
	c.Add(10.0)
	// no panics — noop implementations are always valid
}

func TestNoopGauge(t *testing.T) {
	g := metrics.NoopGauge{}
	g.Set(42.0)
	g.Add(-1.0)
}

func TestNoopHistogram(t *testing.T) {
	h := metrics.NoopHistogram{}
	h.Observe(0.005)
	h.Observe(1.234)
}
