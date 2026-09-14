// SPDX-License-Identifier: MIT

package ginmw_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/umesh0492/go-libs/ginmw"
	"github.com/umesh0492/go-libs/idempotency"
)

// mockStore for testing errors and specific states
type mockStore struct {
	lockErr   error
	saveErr   error
	unlockErr error
	exists    bool
	record    *idempotency.Record
	saved     bool
	unlocked  bool
	deleted   bool
}

func (m *mockStore) Lock(ctx context.Context, key string) (bool, *idempotency.Record, error) {
	return m.exists, m.record, m.lockErr
}

func (m *mockStore) Save(ctx context.Context, key string, response idempotency.Response) error {
	m.saved = true
	return m.saveErr
}

func (m *mockStore) Unlock(ctx context.Context, key string) error {
	m.unlocked = true
	return m.unlockErr
}

func (m *mockStore) Delete(ctx context.Context, key string) error {
	m.deleted = true
	return nil
}

func TestIdempotency_NoHeader(t *testing.T) {
	gin.SetMode(gin.TestMode)
	store := idempotency.NewMemoryStore()
	r := gin.New()
	r.Use(ginmw.Idempotency(store))
	r.GET("/", func(c *gin.Context) {
		c.String(http.StatusOK, "OK")
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "OK", w.Body.String())
}

func TestIdempotency_FirstRequestAndReplay(t *testing.T) {
	gin.SetMode(gin.TestMode)
	store := idempotency.NewMemoryStore()
	r := gin.New()
	r.Use(ginmw.Idempotency(store))

	count := 0
	r.GET("/", func(c *gin.Context) {
		count++
		c.Header("X-Custom", "test")
		c.String(http.StatusCreated, "Created %d", count)
	})

	// First request
	req1 := httptest.NewRequest(http.MethodGet, "/", nil)
	req1.Header.Set("Idempotency-Key", "key1")
	w1 := httptest.NewRecorder()
	r.ServeHTTP(w1, req1)

	assert.Equal(t, http.StatusCreated, w1.Code)
	assert.Equal(t, "Created 1", w1.Body.String())
	assert.Equal(t, "test", w1.Header().Get("X-Custom"))

	// Second request (should replay)
	req2 := httptest.NewRequest(http.MethodGet, "/", nil)
	req2.Header.Set("Idempotency-Key", "key1")
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)

	assert.Equal(t, http.StatusCreated, w2.Code)
	assert.Equal(t, "Created 1", w2.Body.String()) // Notice count didn't increase
	assert.Equal(t, "test", w2.Header().Get("X-Custom"))
	assert.Equal(t, 1, count)
}

func TestIdempotency_InProgress(t *testing.T) {
	gin.SetMode(gin.TestMode)
	store := &mockStore{
		exists: true,
		record: &idempotency.Record{Status: idempotency.StatusInProgress},
	}
	r := gin.New()
	r.Use(ginmw.Idempotency(store))
	r.GET("/", func(c *gin.Context) {})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Idempotency-Key", "key2")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusConflict, w.Code)
	assert.Contains(t, w.Body.String(), "idempotent operation already in progress")
}

func TestIdempotency_StoreError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	store := &mockStore{
		lockErr: errors.New("db down"),
	}
	r := gin.New()
	r.Use(ginmw.Idempotency(store))
	r.GET("/", func(c *gin.Context) {})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Idempotency-Key", "key3")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, w.Body.String(), "idempotency store error")
}

func TestIdempotency_ServerErrorsNotSaved(t *testing.T) {
	gin.SetMode(gin.TestMode)
	store := &mockStore{
		exists: false,
	}
	r := gin.New()
	r.Use(ginmw.Idempotency(store))
	r.GET("/", func(c *gin.Context) {
		c.Status(http.StatusInternalServerError)
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Idempotency-Key", "key4")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.False(t, store.saved, "5xx responses should not be saved")
	assert.True(t, store.unlocked, "5xx responses should unlock the key")
}

func TestIdempotency_PanicUnlocks(t *testing.T) {
	gin.SetMode(gin.TestMode)
	store := &mockStore{
		exists: false,
	}
	r := gin.New()
	r.Use(ginmw.Idempotency(store))
	r.GET("/", func(c *gin.Context) {
		panic("something went horribly wrong")
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Idempotency-Key", "panic-key")
	w := httptest.NewRecorder()

	assert.Panics(t, func() {
		r.ServeHTTP(w, req)
	})

	assert.True(t, store.unlocked, "panic should unlock the key")
}

func TestIdempotency_WriteString(t *testing.T) {
	gin.SetMode(gin.TestMode)
	store := idempotency.NewMemoryStore()
	r := gin.New()
	r.Use(ginmw.Idempotency(store))
	r.GET("/", func(c *gin.Context) {
		c.Writer.WriteString("Hello, WriteString")
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Idempotency-Key", "write-string-key")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, "Hello, WriteString", w.Body.String())

	// Replay
	req2 := httptest.NewRequest(http.MethodGet, "/", nil)
	req2.Header.Set("Idempotency-Key", "write-string-key")
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)

	assert.Equal(t, "Hello, WriteString", w2.Body.String())
}

func TestIdempotency_UnlockError_OnPanic(t *testing.T) {
	gin.SetMode(gin.TestMode)
	store := &mockStore{
		unlockErr: errors.New("failed to release lock"),
	}
	r := gin.New()
	r.Use(ginmw.Idempotency(store))
	r.GET("/", func(c *gin.Context) {
		panic("boom")
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Idempotency-Key", "panic-unlock-err")
	w := httptest.NewRecorder()

	assert.Panics(t, func() {
		r.ServeHTTP(w, req)
	})

	assert.True(t, store.unlocked)
}

func TestIdempotency_UnlockError_OnIncomplete(t *testing.T) {
	gin.SetMode(gin.TestMode)
	store := &mockStore{
		unlockErr: errors.New("failed to release lock"),
	}
	r := gin.New()
	r.Use(ginmw.Idempotency(store))
	r.GET("/", func(c *gin.Context) {
		c.Status(http.StatusInternalServerError)
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Idempotency-Key", "incomplete-unlock-err")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.True(t, store.unlocked)
}

func TestIdempotency_SaveError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	store := &mockStore{
		saveErr: errors.New("failed to save response"),
	}
	r := gin.New()
	r.Use(ginmw.Idempotency(store))
	r.GET("/", func(c *gin.Context) {
		c.String(http.StatusOK, "saved-content")
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Idempotency-Key", "save-err-key")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "saved-content", w.Body.String())
	assert.True(t, store.saved)
	assert.True(t, store.unlocked, "store should unlock when save fails")
}
