// SPDX-License-Identifier: MIT

package securityheaders_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/umesh0492/go-libs/securityheaders"
)

func roundTrip(mw func(http.Handler) http.Handler) *httptest.ResponseRecorder {
	h := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	r := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

func TestDefault_SetsAllHeaders(t *testing.T) {
	w := roundTrip(securityheaders.Default)

	hdr := w.Header()
	assert.Equal(t, "DENY", hdr.Get("X-Frame-Options"))
	assert.Equal(t, "nosniff", hdr.Get("X-Content-Type-Options"))
	assert.Equal(t, "1; mode=block", hdr.Get("X-XSS-Protection"))
	assert.Equal(t, "strict-origin-when-cross-origin", hdr.Get("Referrer-Policy"))
	assert.Contains(t, hdr.Get("Strict-Transport-Security"), "max-age=63072000")
	assert.Contains(t, hdr.Get("Strict-Transport-Security"), "includeSubDomains")
	assert.NotEmpty(t, hdr.Get("Permissions-Policy"))
	assert.NotEmpty(t, hdr.Get("Content-Security-Policy"))
	assert.Equal(t, "no-store", hdr.Get("Cache-Control"))
	assert.Equal(t, "api", hdr.Get("Server"), "Server banner must be masked")
}

func TestNew_CustomServerName(t *testing.T) {
	mw := securityheaders.New(securityheaders.WithServerName("core-platform"))
	w := roundTrip(mw)
	assert.Equal(t, "core-platform", w.Header().Get("Server"))
}

func TestNew_CustomCSP(t *testing.T) {
	csp := "default-src 'self'; script-src 'self'"
	mw := securityheaders.New(securityheaders.WithCSP(csp))
	w := roundTrip(mw)
	assert.Equal(t, csp, w.Header().Get("Content-Security-Policy"))
}

func TestNew_CustomHSTSMaxAge(t *testing.T) {
	mw := securityheaders.New(securityheaders.WithHSTSMaxAge(31536000))
	w := roundTrip(mw)
	assert.Contains(t, w.Header().Get("Strict-Transport-Security"), "max-age=31536000")
}

func TestNew_CustomPermissionsPolicy(t *testing.T) {
	policy := "camera=()"
	mw := securityheaders.New(securityheaders.WithPermissionsPolicy(policy))
	w := roundTrip(mw)
	assert.Equal(t, policy, w.Header().Get("Permissions-Policy"))
}

func TestDefault_PropagatesResponseCode(t *testing.T) {
	h := securityheaders.Default(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	}))
	r := httptest.NewRequest("POST", "/", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	assert.Equal(t, http.StatusCreated, w.Code)
}

func TestNew_EmptyOptionsFallbackToDefault(t *testing.T) {
	mw := securityheaders.New(
		securityheaders.WithServerName(""),
		securityheaders.WithHSTSMaxAge(0),
		securityheaders.WithCSP(""),
		securityheaders.WithPermissionsPolicy(""),
	)
	w := roundTrip(mw)
	hdr := w.Header()
	assert.Equal(t, securityheaders.DefaultConfig.ServerName, hdr.Get("Server"))
	assert.Contains(t, hdr.Get("Strict-Transport-Security"), "max-age=63072000")
	assert.Equal(t, securityheaders.DefaultConfig.CSP, hdr.Get("Content-Security-Policy"))
	assert.Equal(t, securityheaders.DefaultConfig.PermissionsPolicy, hdr.Get("Permissions-Policy"))
}
