// SPDX-License-Identifier: MIT

// Package idempotency provides two-phase atomic idempotency locking and response caching
// for HTTP mutating operations via the Idempotency-Key header, preventing duplicate side effects.
//
// Memory Characteristics:
// MemoryStore provides in-memory caching for single-instance applications and testing.
// By default, completed responses are retained in memory. To prevent unbounded memory growth,
// callers can configure WithResponseTTL to expire completed responses after a duration and
// WithMaxEntries to enforce an upper bound on cache size with automatic eviction of expired
// and oldest entries. For distributed, multi-instance production environments, use a persistent
// backing store such as Redis or PostgreSQL.
package idempotency
