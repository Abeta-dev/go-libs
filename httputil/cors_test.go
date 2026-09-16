// SPDX-License-Identifier: MIT

package httputil_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/umesh0492/go-libs/httputil"
)

func TestCORS_AllowedOrigins(t *testing.T) {
	cfg := httputil.DefaultCORSConfig("https://app.example.com", "*.sub.example.com", "https://api.test.com")
	mw := httputil.CORS(cfg)

	nextCalled := false
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))

	t.Run("exact match allowed", func(t *testing.T) {
		nextCalled = false
		req := httptest.NewRequest(http.MethodGet, "/data", nil)
		req.Header.Set("Origin", "https://app.example.com")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)
		assert.True(t, nextCalled)
		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, "https://app.example.com", rec.Header().Get("Access-Control-Allow-Origin"))
		assert.Equal(t, "true", rec.Header().Get("Access-Control-Allow-Credentials"))
		assert.Contains(t, rec.Header().Get("Access-Control-Allow-Methods"), "GET")
	})

	t.Run("wildcard suffix match allowed", func(t *testing.T) {
		nextCalled = false
		req := httptest.NewRequest(http.MethodGet, "/data", nil)
		req.Header.Set("Origin", "https://tenant.sub.example.com")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)
		assert.True(t, nextCalled)
		assert.Equal(t, "https://tenant.sub.example.com", rec.Header().Get("Access-Control-Allow-Origin"))
	})

	t.Run("disallowed origin", func(t *testing.T) {
		nextCalled = false
		req := httptest.NewRequest(http.MethodGet, "/data", nil)
		req.Header.Set("Origin", "https://malicious.org")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)
		assert.True(t, nextCalled)
		assert.Empty(t, rec.Header().Get("Access-Control-Allow-Origin"))
	})

	t.Run("no origin header", func(t *testing.T) {
		nextCalled = false
		req := httptest.NewRequest(http.MethodGet, "/data", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)
		assert.True(t, nextCalled)
		assert.Empty(t, rec.Header().Get("Access-Control-Allow-Origin"))
	})

	t.Run("preflight options request returns 204 without next", func(t *testing.T) {
		nextCalled = false
		req := httptest.NewRequest(http.MethodOptions, "/data", nil)
		req.Header.Set("Origin", "https://app.example.com")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)
		assert.False(t, nextCalled, "preflight OPTIONS should not call downstream handler")
		assert.Equal(t, http.StatusNoContent, rec.Code)
		assert.Equal(t, "https://app.example.com", rec.Header().Get("Access-Control-Allow-Origin"))
	})

	t.Run("star origin allowed", func(t *testing.T) {
		starCfg := httputil.CORSConfig{
			AllowedOrigins: []string{"*"},
		}
		starMw := httputil.CORS(starCfg)
		starHandler := starMw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequest(http.MethodGet, "/data", nil)
		req.Header.Set("Origin", "https://any.site.io")
		rec := httptest.NewRecorder()
		starHandler.ServeHTTP(rec, req)
		assert.Equal(t, "https://any.site.io", rec.Header().Get("Access-Control-Allow-Origin"))
	})
}
