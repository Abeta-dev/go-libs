// SPDX-License-Identifier: MIT

package telemetry_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/umesh0492/go-libs/telemetry"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace/noop"
)

func TestInitProviderAndStartSpan(t *testing.T) {
	tp := noop.NewTracerProvider()
	telemetry.InitProvider(tp)

	assert.Equal(t, tp, otel.GetTracerProvider())

	ctx, span := telemetry.StartSpan(context.Background(), "test-tracer", "test-span")
	assert.NotNil(t, ctx)
	assert.NotNil(t, span)
	span.End()
}

func TestNewTracerProvider(t *testing.T) {
	// Record existing global tracer provider
	globalBefore := otel.GetTracerProvider()

	tp, cleanup, err := telemetry.NewTracerProvider(
		telemetry.WithServiceName("order-service"),
		telemetry.WithServiceVersion("v1.2.0"),
		telemetry.WithEnvironment("production"),
		telemetry.WithSampleRate(1.0),
	)
	assert.NoError(t, err)
	assert.NotNil(t, tp)
	assert.NotNil(t, cleanup)
	assert.Equal(t, "order-service", tp.Config().ServiceName)
	assert.Equal(t, "v1.2.0", tp.Config().ServiceVersion)
	assert.Equal(t, "production", tp.Config().Environment)
	assert.Equal(t, 1.0, tp.Config().SampleRate)

	// Verify global state was NOT mutated
	assert.Equal(t, globalBefore, otel.GetTracerProvider())

	// Verify Tracer creation and span creation
	tracer := tp.Tracer("order-tracer")
	assert.NotNil(t, tracer)

	ctx, span := tracer.Start(context.Background(), "process-order")
	assert.NotNil(t, ctx)
	assert.NotNil(t, span)
	span.End()

	// Verify cleanup function runs without error
	err = cleanup(context.Background())
	assert.NoError(t, err)
}

func TestNewTracerProvider_DefaultServiceName(t *testing.T) {
	tp, cleanup, err := telemetry.NewTracerProvider()
	assert.NoError(t, err)
	assert.NotNil(t, tp)
	assert.Equal(t, "unknown-service", tp.Config().ServiceName)
	assert.NotNil(t, cleanup)
	assert.NoError(t, cleanup(context.Background()))

	// Test nil receiver fallback
	var nilTP *telemetry.TracerProvider
	assert.Equal(t, telemetry.Config{}, nilTP.Config())
	tracer := nilTP.Tracer("fallback-tracer")
	assert.NotNil(t, tracer)

	// Test explicit empty service name fallback
	tpEmpty, _, err := telemetry.NewTracerProvider(telemetry.WithServiceName(""))
	assert.NoError(t, err)
	assert.Equal(t, "unknown-service", tpEmpty.Config().ServiceName)
}
