// SPDX-License-Identifier: MIT

package requestid_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/umesh0492/go-libs/requestid"
)

func TestMiddleware_GeneratesIDWhenAbsent(t *testing.T) {
	var capturedID string
	h := requestid.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedID = requestid.FromContext(r.Context())
	}))

	r := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	assert.NotEmpty(t, capturedID, "should generate an ID when none supplied")
	assert.Equal(t, capturedID, w.Header().Get(requestid.Header), "response header must match context ID")
}

func TestMiddleware_PreservesClientSuppliedID(t *testing.T) {
	const clientID = "my-trace-id-123"

	var capturedID string
	h := requestid.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedID = requestid.FromContext(r.Context())
	}))

	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set(requestid.Header, clientID)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	assert.Equal(t, clientID, capturedID)
	assert.Equal(t, clientID, w.Header().Get(requestid.Header))
}

func TestMiddleware_UniquePerRequest(t *testing.T) {
	ids := make(map[string]bool)
	h := requestid.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ids[requestid.FromContext(r.Context())] = true
	}))

	for i := 0; i < 100; i++ {
		r := httptest.NewRequest("GET", "/", nil)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
	}

	assert.Len(t, ids, 100, "all 100 request IDs should be unique")
}

func TestFromContext_EmptyStringWhenMissing(t *testing.T) {
	id := requestid.FromContext(context.Background())
	assert.Empty(t, id)
}

func TestMiddleware_PropagatesResponseCode(t *testing.T) {
	h := requestid.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
	}))
	r := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	require.Equal(t, http.StatusAccepted, w.Code)
}
