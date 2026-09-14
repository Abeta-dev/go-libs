// SPDX-License-Identifier: MIT

// Package cache provides a thread-safe in-memory cache featuring generic key-value
// storage (TypedCache), singleflight stampede suppression, TTL expiration, and
// configurable eviction policies (EvictionSampledLRU, EvictionLRU, EvictionLFU, EvictionFIFO).
package cache
