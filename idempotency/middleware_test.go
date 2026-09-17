// SPDX-License-Identifier: MIT

package idempotency_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/umesh0492/go-libs/idempotency"
)

type mockStore struct {
	lockFunc   func(ctx context.Context, key string) (bool, *idempotency.Record, error)
	saveFunc   func(ctx context.Context, key string, response idempotency.Response) error
	unlockFunc func(ctx context.Context, key string) error
	deleteFunc func(ctx context.Context, key string) error
}

func (m *mockStore) Lock(ctx context.Context, key string) (bool, *idempotency.Record, error) {
	if m.lockFunc != nil {
		return m.lockFunc(ctx, key)
	}
	return false, nil, nil
}

func (m *mockStore) Save(ctx context.Context, key string, response idempotency.Response) error {
	if m.saveFunc != nil {
		return m.saveFunc(ctx, key, response)
	}
	return nil
}

func (m *mockStore) Unlock(ctx context.Context, key string) error {
	if m.unlockFunc != nil {
		return m.unlockFunc(ctx, key)
	}
	return nil
}

func (m *mockStore) Delete(ctx context.Context, key string) error {
	if m.deleteFunc != nil {
		return m.deleteFunc(ctx, key)
	}
	return nil
}

func TestMiddleware_SafeMethods_Bypass(t *testing.T) {
	store := idempotency.NewMemoryStore()
	mw := idempotency.Middleware(store)

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("safe-response"))
	}))

	for _, method := range []string{http.MethodGet, http.MethodHead, http.MethodOptions} {
		req := httptest.NewRequest(method, "/resource", nil)
		req.Header.Set("Idempotency-Key", "test-key")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
		if method != http.MethodHead {
			assert.Equal(t, "safe-response", rec.Body.String())
		}
	}
}

func TestMiddleware_MissingHeader(t *testing.T) {
	t.Run("not enforced passes through", func(t *testing.T) {
		store := idempotency.NewMemoryStore()
		mw := idempotency.Middleware(store)

		handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte("created"))
		}))

		req := httptest.NewRequest(http.MethodPost, "/resource", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusCreated, rec.Code)
		assert.Equal(t, "created", rec.Body.String())
	})

	t.Run("enforced returns 400 Bad Request", func(t *testing.T) {
		store := idempotency.NewMemoryStore()
		mw := idempotency.Middleware(store, idempotency.WithEnforceHeader(true))

		handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequest(http.MethodPost, "/resource", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusBadRequest, rec.Code)
		assert.Contains(t, rec.Body.String(), "missing idempotency key")
	})
}

func TestMiddleware_Lifecycle_SuccessAndReplay(t *testing.T) {
	store := idempotency.NewMemoryStore()
	mw := idempotency.Middleware(store)

	execCount := 0
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		execCount++
		w.Header().Set("X-Custom-Header", "custom-value")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"order_id":"12345"}`))
	}))

	// Request 1: Fresh execution
	req1 := httptest.NewRequest(http.MethodPost, "/orders", nil)
	req1.Header.Set("Idempotency-Key", "order-key-1")
	rec1 := httptest.NewRecorder()

	handler.ServeHTTP(rec1, req1)
	assert.Equal(t, http.StatusCreated, rec1.Code)
	assert.Equal(t, "custom-value", rec1.Header().Get("X-Custom-Header"))
	assert.Equal(t, `{"order_id":"12345"}`, rec1.Body.String())
	assert.Equal(t, 1, execCount)

	// Request 2: Replay of completed idempotent request
	req2 := httptest.NewRequest(http.MethodPost, "/orders", nil)
	req2.Header.Set("Idempotency-Key", "order-key-1")
	rec2 := httptest.NewRecorder()

	handler.ServeHTTP(rec2, req2)
	assert.Equal(t, http.StatusCreated, rec2.Code)
	assert.Equal(t, "custom-value", rec2.Header().Get("X-Custom-Header"))
	assert.Equal(t, "HIT - Idempotency", rec2.Header().Get("X-Cache-Lookup"))
	assert.Equal(t, `{"order_id":"12345"}`, rec2.Body.String())
	assert.Equal(t, 1, execCount, "handler must not execute twice for same idempotency key")
}

func TestMiddleware_Conflict_InProgress(t *testing.T) {
	mock := &mockStore{
		lockFunc: func(ctx context.Context, key string) (bool, *idempotency.Record, error) {
			return true, &idempotency.Record{Status: idempotency.StatusInProgress}, nil
		},
	}

	mw := idempotency.Middleware(mock)
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodPost, "/orders", nil)
	req.Header.Set("Idempotency-Key", "in-progress-key")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusConflict, rec.Code)
	assert.Contains(t, rec.Body.String(), "idempotent operation already in progress")
}

func TestMiddleware_StoreError(t *testing.T) {
	mock := &mockStore{
		lockFunc: func(ctx context.Context, key string) (bool, *idempotency.Record, error) {
			return false, nil, errors.New("redis/db down")
		},
	}

	mw := idempotency.Middleware(mock)
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodPost, "/orders", nil)
	req.Header.Set("Idempotency-Key", "err-key")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.Contains(t, rec.Body.String(), "idempotency store error")
}

func TestMiddleware_ServerError_UnlocksKey(t *testing.T) {
	unlockCalled := false
	store := idempotency.NewMemoryStore()

	mw := idempotency.Middleware(store)
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("db connection lost"))
	}))

	req := httptest.NewRequest(http.MethodPost, "/orders", nil)
	req.Header.Set("Idempotency-Key", "retryable-fail-key")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)

	// Since 500 unlocks the key, a subsequent request must be allowed to acquire lock again
	exists, record, err := store.Lock(context.Background(), "retryable-fail-key")
	require.NoError(t, err)
	assert.False(t, exists, "key should be unlocked after 5xx error")
	assert.Nil(t, record)
	_ = unlockCalled
}

func TestMiddleware_Panic_UnlocksKey(t *testing.T) {
	store := idempotency.NewMemoryStore()
	mw := idempotency.Middleware(store)

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("unhandled crash")
	}))

	req := httptest.NewRequest(http.MethodPost, "/orders", nil)
	req.Header.Set("Idempotency-Key", "panic-key")
	rec := httptest.NewRecorder()

	assert.Panics(t, func() {
		handler.ServeHTTP(rec, req)
	})

	// Key must be unlocked so retry can occur
	exists, _, err := store.Lock(context.Background(), "panic-key")
	require.NoError(t, err)
	assert.False(t, exists)
}

func TestMiddleware_CustomOptions(t *testing.T) {
	store := idempotency.NewMemoryStore()
	mw := idempotency.Middleware(
		store,
		idempotency.WithHeaderName("X-Custom-Idempotency"),
		idempotency.WithIgnoredMethods(http.MethodPatch),
		idempotency.WithStatusCodeMatcher(func(code int) bool {
			return code == http.StatusCreated
		}),
	)

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok-not-created"))
	}))

	// PATCH is in ignored methods
	req := httptest.NewRequest(http.MethodPatch, "/orders", nil)
	req.Header.Set("X-Custom-Idempotency", "custom-key")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)

	// POST with custom header, returns 200 which is not cached per custom matcher
	req2 := httptest.NewRequest(http.MethodPost, "/orders", nil)
	req2.Header.Set("X-Custom-Idempotency", "custom-key")
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)
	assert.Equal(t, http.StatusOK, rec2.Code)
}
