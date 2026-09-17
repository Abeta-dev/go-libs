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

func TestMiddleware_SanitizesHeaderInjectionAndGarbage(t *testing.T) {
	invalidInputs := []string{
		"evil\r\nInjected-Header: val",
		"id with spaces",
		"id<script>alert(1)</script>",
		"id;DROP TABLE users;--",
		"id$var",
		"id#hash",
		"id%20encoded",
		string(make([]byte, 129)), // > 128 chars
	}

	for _, badID := range invalidInputs {
		t.Run("sanitizes: "+badID, func(t *testing.T) {
			var capturedID string
			h := requestid.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				capturedID = requestid.FromContext(r.Context())
			}))

			r := httptest.NewRequest("GET", "/", nil)
			r.Header.Set(requestid.Header, badID)
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)

			assert.NotEmpty(t, capturedID)
			assert.NotEqual(t, badID, capturedID, "invalid/garbage header should be replaced")
			assert.Equal(t, capturedID, w.Header().Get(requestid.Header))
			// Ensure generated replacement is valid UUID length (36 chars)
			assert.Len(t, capturedID, 36)
		})
	}

	// Valid inputs with allowed characters (alphanumeric, hyphen, underscore, dot, colon, slash)
	validInputs := []string{
		"trace-123_456.789:abc/xyz",
		"SIMPLE-ID",
		"12345",
		"a",
	}
	for _, goodID := range validInputs {
		t.Run("preserves: "+goodID, func(t *testing.T) {
			var capturedID string
			h := requestid.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				capturedID = requestid.FromContext(r.Context())
			}))

			r := httptest.NewRequest("GET", "/", nil)
			r.Header.Set(requestid.Header, goodID)
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)

			assert.Equal(t, goodID, capturedID, "valid header should be preserved")
			assert.Equal(t, goodID, w.Header().Get(requestid.Header))
		})
	}
}
