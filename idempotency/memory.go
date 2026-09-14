// SPDX-License-Identifier: MIT

package idempotency

import (
	"context"
	"sync"
	"time"
)

type memoryRecord struct {
	Status    Status
	Response  *Response
	UpdatedAt time.Time
}

// MemoryStore is a thread-safe, in-memory implementation of Store.
// It is primarily intended for testing and single-instance deployments.
// For production deployments with multiple instances, use a distributed store.
type MemoryStore struct {
	mu          sync.Mutex
	data        map[string]*memoryRecord
	lockTTL     time.Duration
	responseTTL time.Duration
	maxEntries  int
}

// Option configures MemoryStore.
type Option func(*MemoryStore)

// WithLockTTL sets the maximum duration an in-progress lock is considered valid.
// If an operation has been in progress longer than ttl, it is treated as expired and can be re-locked.
func WithLockTTL(ttl time.Duration) Option {
	return func(s *MemoryStore) {
		s.lockTTL = ttl
	}
}

// WithResponseTTL sets the retention duration for completed cached responses.
// Expired responses are automatically evicted and eligible for re-execution.
func WithResponseTTL(ttl time.Duration) Option {
	return func(s *MemoryStore) {
		s.responseTTL = ttl
	}
}

// WithMaxEntries sets the upper bound on the number of records retained in memory.
// When capacity is reached, expired and least-recently-updated records are evicted.
func WithMaxEntries(maxEntries int) Option {
	return func(s *MemoryStore) {
		s.maxEntries = maxEntries
	}
}

// NewMemoryStore creates a new MemoryStore.
func NewMemoryStore(opts ...Option) *MemoryStore {
	s := &MemoryStore{
		data: make(map[string]*memoryRecord),
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// Lock implements Store.Lock.
func (s *MemoryStore) Lock(ctx context.Context, key string) (bool, *Record, error) {
	if err := ctx.Err(); err != nil {
		return false, nil, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	if rec, exists := s.data[key]; exists {
		// If lock has timed out while still in progress, allow re-acquiring
		if s.lockTTL > 0 && rec.Status == StatusInProgress && now.Sub(rec.UpdatedAt) > s.lockTTL {
			rec.UpdatedAt = now
			return false, nil, nil
		}

		// If response retention TTL has expired on completed record, evict it
		if s.responseTTL > 0 && rec.Status == StatusCompleted && now.Sub(rec.UpdatedAt) > s.responseTTL {
			delete(s.data, key)
			s.data[key] = &memoryRecord{
				Status:    StatusInProgress,
				UpdatedAt: now,
			}
			return false, nil, nil
		}

		// Return a copy to prevent race conditions on mutation
		recCopy := &Record{
			Status: rec.Status,
		}
		if rec.Response != nil {
			recCopy.Response = deepCopyResponse(rec.Response)
		}
		return true, recCopy, nil
	}

	// Evict before inserting if max capacity reached
	if s.maxEntries > 0 && len(s.data) >= s.maxEntries {
		s.evictExpired(now)
		if len(s.data) >= s.maxEntries {
			s.evictOldest()
		}
	}

	// Create new record in progress
	s.data[key] = &memoryRecord{
		Status:    StatusInProgress,
		UpdatedAt: now,
	}

	return false, nil, nil
}

// Save implements Store.Save.
func (s *MemoryStore) Save(ctx context.Context, key string, response Response) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	rec, exists := s.data[key]
	if !exists {
		if s.maxEntries > 0 && len(s.data) >= s.maxEntries {
			s.evictExpired(now)
			if len(s.data) >= s.maxEntries {
				s.evictOldest()
			}
		}
		rec = &memoryRecord{}
		s.data[key] = rec
	}

	rec.Status = StatusCompleted
	rec.UpdatedAt = now
	rec.Response = deepCopyResponse(&response)
	return nil
}

// Unlock implements Store.Unlock.
func (s *MemoryStore) Unlock(ctx context.Context, key string) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if rec, exists := s.data[key]; exists {
		if rec.Status == StatusInProgress {
			delete(s.data, key)
		}
	}
	return nil
}

// Delete implements Store.Delete.
func (s *MemoryStore) Delete(ctx context.Context, key string) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.data, key)
	return nil
}

// Len returns the current number of stored records.
func (s *MemoryStore) Len() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.data)
}

// EvictExpired removes all expired in-progress locks and expired completed responses.
// Returns the count of evicted records.
func (s *MemoryStore) EvictExpired() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.evictExpired(time.Now())
}

func (s *MemoryStore) evictExpired(now time.Time) int {
	evicted := 0
	for k, rec := range s.data {
		if s.responseTTL > 0 && rec.Status == StatusCompleted && now.Sub(rec.UpdatedAt) > s.responseTTL {
			delete(s.data, k)
			evicted++
		} else if s.lockTTL > 0 && rec.Status == StatusInProgress && now.Sub(rec.UpdatedAt) > s.lockTTL {
			delete(s.data, k)
			evicted++
		}
	}
	return evicted
}

func (s *MemoryStore) evictOldest() {
	var oldestKey string
	var oldestTime time.Time
	first := true
	for k, rec := range s.data {
		if first || rec.UpdatedAt.Before(oldestTime) {
			oldestKey = k
			oldestTime = rec.UpdatedAt
			first = false
		}
	}
	if oldestKey != "" {
		delete(s.data, oldestKey)
	}
}

func deepCopyResponse(res *Response) *Response {
	if res == nil {
		return nil
	}
	copyRes := &Response{
		StatusCode: res.StatusCode,
		Body:       append([]byte(nil), res.Body...),
	}
	if res.Headers != nil {
		copyRes.Headers = make(map[string][]string, len(res.Headers))
		for k, v := range res.Headers {
			copyRes.Headers[k] = append([]string(nil), v...)
		}
	}
	return copyRes
}
