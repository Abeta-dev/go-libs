// SPDX-License-Identifier: MIT

// Package httpmw provides a unified, lightweight standard-library net/http
// middleware suite with full parity to ginmw (RequestID, LimitBody, Compress,
// PrecompressedFileServer, ETag, SecurityHeaders, LoopbackGuard, CORS,
// Recovery, Logger, REDMetrics, MetricsEndpoint, RateLimit, RBAC permissions,
// RequireTenant, and SSE) while pulling zero Gin, pgx, or OpenTelemetry
// dependencies — ideal for stdlib net/http BFFs and desktop binaries.
package httpmw
