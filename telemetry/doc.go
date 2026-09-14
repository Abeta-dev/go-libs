// SPDX-License-Identifier: MIT

// Package telemetry initializes OpenTelemetry tracing providers and propagators,
// providing process-global setup (InitProvider) and instance-scoped providers
// (NewTracerProvider) without global mutation, plus span creation helpers (StartSpan).
package telemetry
