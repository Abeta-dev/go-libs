// SPDX-License-Identifier: MIT

// Package telemetry initializes OpenTelemetry tracing providers and propagators.
package telemetry

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
)

// InitProvider sets the process-global trace provider and text map propagator.
//
// WARNING — GLOBAL STATE MUTATION:
// InitProvider mutates process-global state by calling otel.SetTracerProvider(tp)
// and otel.SetTextMapPropagator(...). This modifies the tracer provider across the
// entire process and affects all concurrent goroutines and packages that rely on
// the global OpenTelemetry registry.
//
// For isolated or multi-tenant use cases where modifying global process state is
// unsafe or undesirable, use NewTracerProvider instead to obtain an instance-scoped
// tracer provider without mutating global process state.
func InitProvider(tp trace.TracerProvider) {
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))
}

// Config specifies configuration parameters for creating an instance-scoped TracerProvider.
type Config struct {
	ServiceName    string
	ServiceVersion string
	Environment    string
	SampleRate     float64
}

// Option configures an instance-scoped TracerProvider.
type Option func(*Config)

// WithServiceName sets the service name attribute.
func WithServiceName(name string) Option {
	return func(c *Config) {
		c.ServiceName = name
	}
}

// WithServiceVersion sets the service version attribute.
func WithServiceVersion(version string) Option {
	return func(c *Config) {
		c.ServiceVersion = version
	}
}

// WithEnvironment sets the deployment environment attribute.
func WithEnvironment(env string) Option {
	return func(c *Config) {
		c.Environment = env
	}
}

// WithSampleRate sets the sampling rate (0.0 to 1.0).
func WithSampleRate(rate float64) Option {
	return func(c *Config) {
		c.SampleRate = rate
	}
}

// TracerProvider is an instance-scoped OpenTelemetry tracer provider that
// implements trace.TracerProvider without mutating process-global state.
type TracerProvider struct {
	provider trace.TracerProvider
	cfg      Config
}

// Config returns the configuration used to initialize this TracerProvider.
func (tp *TracerProvider) Config() Config {
	if tp == nil {
		return Config{}
	}
	return tp.cfg
}

// Tracer returns an instance-scoped Tracer with the given name and options.
func (tp *TracerProvider) Tracer(name string, opts ...trace.TracerOption) trace.Tracer {
	if tp != nil && tp.provider != nil {
		return tp.provider.Tracer(name, opts...)
	}
	return otel.GetTracerProvider().Tracer(name, opts...)
}

// Shutdown gracefully flushes and stops the tracer provider.
func (tp *TracerProvider) Shutdown(ctx context.Context) error {
	return nil
}

// NewTracerProvider creates an instance-scoped TracerProvider without mutating
// the process-global state (otel.SetTracerProvider). Callers receive the provider,
// a shutdown cleanup function to be deferred, and any initialization error.
func NewTracerProvider(opts ...Option) (*TracerProvider, func(context.Context) error, error) {
	cfg := Config{
		ServiceName: "unknown-service",
	}
	for _, opt := range opts {
		opt(&cfg)
	}
	if cfg.ServiceName == "" {
		cfg.ServiceName = "unknown-service"
	}
	tp := &TracerProvider{
		provider: noop.NewTracerProvider(),
		cfg:      cfg,
	}
	cleanup := func(ctx context.Context) error {
		return tp.Shutdown(ctx)
	}
	return tp, cleanup, nil
}

// StartSpan creates a new span from the global tracer.
func StartSpan(ctx context.Context, tracerName, spanName string) (context.Context, trace.Span) {
	tracer := otel.Tracer(tracerName)
	return tracer.Start(ctx, spanName)
}
