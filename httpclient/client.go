// SPDX-License-Identifier: MIT

package httpclient

import (
	"net/http"

	"github.com/umesh0492/go-libs/clock"
	"github.com/umesh0492/go-libs/metrics"
)

// New creates an *http.Client configured with the resilient RoundTripper middleware stack.
func New(opts ...Option) *http.Client {
	return &http.Client{
		Transport: NewRoundTripper(opts...),
	}
}

// NewRoundTripper creates an http.RoundTripper configured with the resilient middleware stack.
func NewRoundTripper(opts ...Option) http.RoundTripper {
	cfg := options{
		transport: http.DefaultTransport,
		metrics: Metrics{
			Requests: metrics.NoopCounter{},
			Failures: metrics.NoopCounter{},
			Duration: metrics.NoopHistogram{},
		},
	}
	for _, opt := range opts {
		if opt != nil {
			opt(&cfg)
		}
	}
	if cfg.clock == nil {
		cfg.clock = clock.NewReal()
	}
	if cfg.metrics.Requests == nil {
		cfg.metrics.Requests = metrics.NoopCounter{}
	}
	if cfg.metrics.Failures == nil {
		cfg.metrics.Failures = metrics.NoopCounter{}
	}
	if cfg.metrics.Duration == nil {
		cfg.metrics.Duration = metrics.NoopHistogram{}
	}
	if cfg.transport == nil {
		cfg.transport = http.DefaultTransport
	}
	return &resilientTransport{opts: cfg}
}
