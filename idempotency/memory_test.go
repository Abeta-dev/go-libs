// SPDX-License-Identifier: MIT

package idempotency

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestMemoryStore_LockAndSave(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()
	key := "test-key-1"

	// 1. Initial Lock
	exists, rec, err := store.Lock(ctx, key)
	assert.NoError(t, err)
	assert.False(t, exists)
	assert.Nil(t, rec)

	// 2. Lock while in progress
	exists, rec, err = store.Lock(ctx, key)
	assert.NoError(t, err)
	assert.True(t, exists)
	assert.Equal(t, StatusInProgress, rec.Status)
	assert.Nil(t, rec.Response)

	// 3. Save
	res := Response{
		StatusCode: 201,
		Headers:    map[string][]string{"Content-Type": {"application/json"}},
		Body:       []byte(`{"status":"ok"}`),
	}
	err = store.Save(ctx, key, res)
	assert.NoError(t, err)

	// 4. Lock after completion
	exists, rec, err = store.Lock(ctx, key)
	assert.NoError(t, err)
	assert.True(t, exists)
	assert.Equal(t, StatusCompleted, rec.Status)
	assert.NotNil(t, rec.Response)
	assert.Equal(t, 201, rec.Response.StatusCode)
	assert.Equal(t, []byte(`{"status":"ok"}`), rec.Response.Body)
	assert.Equal(t, "application/json", rec.Response.Headers["Content-Type"][0])
}

func TestMemoryStore_SaveWithoutLock(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()
	key := "test-key-2"

	res := Response{
		StatusCode: 200,
	}
	err := store.Save(ctx, key, res)
	assert.NoError(t, err)

	exists, rec, err := store.Lock(ctx, key)
	assert.NoError(t, err)
	assert.True(t, exists)
	assert.Equal(t, StatusCompleted, rec.Status)
	assert.Equal(t, 200, rec.Response.StatusCode)
}

func TestMemoryStore_Unlock(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()
	key := "test-key-unlock"

	// Lock key
	exists, _, err := store.Lock(ctx, key)
	assert.NoError(t, err)
	assert.False(t, exists)

	// Unlock releases in-progress lock
	err = store.Unlock(ctx, key)
	assert.NoError(t, err)

	// Re-locking should now succeed as a new lock
	exists, _, err = store.Lock(ctx, key)
	assert.NoError(t, err)
	assert.False(t, exists)

	// Complete the operation
	err = store.Save(ctx, key, Response{StatusCode: 200})
	assert.NoError(t, err)

	// Unlocking completed operation should NOT delete completed record
	err = store.Unlock(ctx, key)
	assert.NoError(t, err)

	exists, rec, err := store.Lock(ctx, key)
	assert.NoError(t, err)
	assert.True(t, exists)
	assert.Equal(t, StatusCompleted, rec.Status)
}

func TestMemoryStore_Delete(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()
	key := "test-key-delete"

	_ = store.Save(ctx, key, Response{StatusCode: 200})

	err := store.Delete(ctx, key)
	assert.NoError(t, err)

	exists, _, err := store.Lock(ctx, key)
	assert.NoError(t, err)
	assert.False(t, exists)
}

func TestMemoryStore_LockTTL(t *testing.T) {
	ttl := 30 * time.Millisecond
	store := NewMemoryStore(WithLockTTL(ttl))
	ctx := context.Background()
	key := "test-key-ttl"

	exists, _, err := store.Lock(ctx, key)
	assert.NoError(t, err)
	assert.False(t, exists)

	// While TTL active, it's still in progress
	exists, rec, err := store.Lock(ctx, key)
	assert.NoError(t, err)
	assert.True(t, exists)
	assert.Equal(t, StatusInProgress, rec.Status)

	// Wait for TTL to expire
	time.Sleep(40 * time.Millisecond)

	// After TTL expires, Lock acquires the lock again
	exists, _, err = store.Lock(ctx, key)
	assert.NoError(t, err)
	assert.False(t, exists)
}

func TestMemoryStore_ContextCancellation(t *testing.T) {
	store := NewMemoryStore()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, _, err := store.Lock(ctx, "k")
	assert.ErrorIs(t, err, context.Canceled)

	err = store.Save(ctx, "k", Response{StatusCode: 200})
	assert.ErrorIs(t, err, context.Canceled)

	err = store.Unlock(ctx, "k")
	assert.ErrorIs(t, err, context.Canceled)

	err = store.Delete(ctx, "k")
	assert.ErrorIs(t, err, context.Canceled)
}

func TestMemoryStore_Adversarial_ConcurrentLockAndUnlock(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()
	key := "contended-key"

	var wg sync.WaitGroup
	workers := 100

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			for j := 0; j < 20; j++ {
				exists, _, err := store.Lock(ctx, key)
				if err != nil {
					return
				}
				if !exists {
					// Acquired lock: randomly save or unlock
					if workerID%2 == 0 {
						_ = store.Save(ctx, key, Response{
							StatusCode: 200,
							Body:       []byte("done"),
						})
					} else {
						_ = store.Unlock(ctx, key)
					}
				}
			}
		}(i)
	}

	wg.Wait()
}

func TestMemoryStore_ResponseTTL(t *testing.T) {
	ttl := 30 * time.Millisecond
	store := NewMemoryStore(WithResponseTTL(ttl))
	ctx := context.Background()
	key := "test-key-resp-ttl"

	// 1. Initial lock and save
	exists, _, err := store.Lock(ctx, key)
	assert.NoError(t, err)
	assert.False(t, exists)

	err = store.Save(ctx, key, Response{
		StatusCode: 200,
		Body:       []byte("cached-data"),
	})
	assert.NoError(t, err)
	assert.Equal(t, 1, store.Len())

	// 2. Immediate read: within TTL, response is returned
	exists, rec, err := store.Lock(ctx, key)
	assert.NoError(t, err)
	assert.True(t, exists)
	assert.Equal(t, StatusCompleted, rec.Status)
	assert.Equal(t, []byte("cached-data"), rec.Response.Body)

	// 3. Sleep past TTL
	time.Sleep(45 * time.Millisecond)

	// 4. Lock after TTL: expired response is evicted, lock re-acquired
	exists, rec, err = store.Lock(ctx, key)
	assert.NoError(t, err)
	assert.False(t, exists)
	assert.Nil(t, rec)
}

func TestMemoryStore_EvictExpired(t *testing.T) {
	store := NewMemoryStore(
		WithLockTTL(20*time.Millisecond),
		WithResponseTTL(20*time.Millisecond),
	)
	ctx := context.Background()

	// Add in-progress lock
	_, _, err := store.Lock(ctx, "lock-key")
	assert.NoError(t, err)

	// Add completed response
	_, _, err = store.Lock(ctx, "resp-key")
	assert.NoError(t, err)
	err = store.Save(ctx, "resp-key", Response{StatusCode: 200})
	assert.NoError(t, err)

	assert.Equal(t, 2, store.Len())

	// Before expiry, evict should remove 0
	evicted := store.EvictExpired()
	assert.Equal(t, 0, evicted)
	assert.Equal(t, 2, store.Len())

	// Wait past expiry
	time.Sleep(30 * time.Millisecond)

	evicted = store.EvictExpired()
	assert.Equal(t, 2, evicted)
	assert.Equal(t, 0, store.Len())
}

func TestMemoryStore_MaxEntries(t *testing.T) {
	store := NewMemoryStore(WithMaxEntries(2))
	ctx := context.Background()

	_, _, err := store.Lock(ctx, "k1")
	assert.NoError(t, err)
	_ = store.Save(ctx, "k1", Response{StatusCode: 200})

	time.Sleep(5 * time.Millisecond)
	_, _, err = store.Lock(ctx, "k2")
	assert.NoError(t, err)
	_ = store.Save(ctx, "k2", Response{StatusCode: 200})

	assert.Equal(t, 2, store.Len())

	time.Sleep(5 * time.Millisecond)
	// Inserting 3rd key should evict oldest (k1)
	_, _, err = store.Lock(ctx, "k3")
	assert.NoError(t, err)
	assert.LessOrEqual(t, store.Len(), 2)

	// k1 was oldest and should have been evicted
	exists, _, err := store.Lock(ctx, "k1")
	assert.NoError(t, err)
	assert.False(t, exists, "k1 should have been evicted due to max entries capacity")
}

func TestMemoryStore_SaveAtCapacityAndNilCopy(t *testing.T) {
	store := NewMemoryStore(WithMaxEntries(1))
	ctx := context.Background()

	err := store.Save(ctx, "k1", Response{StatusCode: 200})
	assert.NoError(t, err)

	err = store.Save(ctx, "k2", Response{StatusCode: 201})
	assert.NoError(t, err)

	assert.Nil(t, deepCopyResponse(nil))
}
