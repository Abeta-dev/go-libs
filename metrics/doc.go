// SPDX-License-Identifier: MIT

// Package metrics defines framework-agnostic instrumentation interfaces (Counter,
// Gauge, Histogram) with zero-allocation no-op implementations, enabling pluggable
// metrics exporters without coupling core packages to specific monitoring backends.
package metrics
