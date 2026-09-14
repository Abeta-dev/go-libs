// SPDX-License-Identifier: MIT

package ratelimit_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"

	"github.com/umesh0492/go-libs/ratelimit"
)

func ExampleNewGlobal() {
	handler := ratelimit.NewGlobal()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/resource", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	fmt.Println("Status:", rec.Code)
	// Output:
	// Status: 200
}

func ExampleNewAuth() {
	handler := ratelimit.NewAuth()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("authenticated"))
	}))

	req := httptest.NewRequest(http.MethodPost, "/auth/login", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	fmt.Println("Status:", rec.Code)
	// Output:
	// Status: 200
}

func ExampleNewWithLimiter() {
	limiter := ratelimit.NewSlidingWindow(100, 60_000_000_000) // 100 req/min
	middleware := ratelimit.NewWithLimiter(limiter)

	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("allowed"))
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/orders", nil)
	req.RemoteAddr = "198.51.100.25:443"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	fmt.Println("Allowed:", rec.Code == http.StatusOK)
	// Output:
	// Allowed: true
}
