// SPDX-License-Identifier: MIT

// Package ginmw provides a production-hardened Gin middleware pipeline adhering to
// strict execution topology: RequestID, SecurityHeaders, CORS, Recovery, Logger,
// Telemetry, RateLimit, BodyLimit, Auth, RBAC, and Idempotency.
package ginmw
