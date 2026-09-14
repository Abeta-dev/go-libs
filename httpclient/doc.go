// SPDX-License-Identifier: MIT

// Package httpclient provides an enterprise-grade, composed resilient HTTP client
// and http.RoundTripper middleware stack for Go microservices.
//
// # Architectural Middleware Ordering
//
// In production distributed systems, resilience primitives must be composed in a
// strict, mathematically sound order. Any other order introduces resource leaks,
// false circuit trips, or thundering herds:
//
//  1. Rate Limiting (Outermost)
//     - Drops excess traffic immediately before consuming downstream circuit budget
//     or spending network/retry cycles.
//
//  2. Circuit Breaker
//     - Fails fast immediately if downstream is unhealthy (StateOpen).
//     - Prevents cascading failures and prevents retry loops from hammering dead backends.
//
//  3. Retry Policy
//     - Retries transient errors (e.g. 502/503/504 or network blips) with exponential
//     backoff and full jitter.
//     - Wraps individual attempts.
//
//  4. Per-Attempt Timeout (Innermost)
//     - Bounds the duration of each individual network attempt.
//     - Ensures slow calls fail fast so subsequent retry attempts have remaining time budget.
//
//  5. Inner Transport
//     - Standard http.RoundTripper (defaulting to http.DefaultTransport).
package httpclient
