// SPDX-License-Identifier: MIT

package workerpool

import (
	"context"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/umesh0492/go-libs/metrics"
)

func TestPool_Internal_NilMetricsFallback(t *testing.T) {
	p := New(1, 1, WithMetrics(Metrics{}))
	defer p.StopWait()

	assert.NotNil(t, p.metrics.Submitted)
	assert.NotNil(t, p.metrics.Completed)
	assert.NotNil(t, p.metrics.Dropped)
	assert.NotNil(t, p.metrics.Panics)
	assert.NotNil(t, p.metrics.QueueSize)
}

func TestPool_Internal_ClosedChannelBeforeStateUpdate(t *testing.T) {
	p := &Pool{
		tasks:  make(chan func(), 1),
		closed: make(chan struct{}),
		logger: slog.Default(),
		metrics: Metrics{
			Submitted: metrics.NoopCounter{},
			Completed: metrics.NoopCounter{},
			Dropped:   metrics.NoopCounter{},
			Panics:    metrics.NoopCounter{},
			QueueSize: metrics.NoopGauge{},
		},
	}
	close(p.closed)

	// p.isClosed is false, but p.closed is closed.
	// Verifies lines in Submit when channel is closed before mutex/state update.
	err := p.Submit(func() {})
	assert.ErrorIs(t, err, ErrPoolClosed)

	// Verifies lines in SubmitContext when channel is closed before mutex/state update.
	err = p.SubmitContext(context.Background(), func() {})
	assert.ErrorIs(t, err, ErrPoolClosed)

	// Verifies line in IsClosed when channel is closed before p.isClosed flag update.
	assert.True(t, p.IsClosed())
}
