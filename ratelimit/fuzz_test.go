// SPDX-License-Identifier: MIT

package ratelimit

import (
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
)

func FuzzRealIP(f *testing.F) {
	seeds := []struct {
		xff        string
		xri        string
		remoteAddr string
	}{
		{"203.0.113.195", "", "10.0.0.1:1234"},
		{"203.0.113.195, 70.41.3.18, 150.172.238.178", "198.51.100.1", "127.0.0.1:8080"},
		{"invalid-ip", "198.51.100.1", "127.0.0.1:8080"},
		{"", "2001:db8::1", "[2001:db8::1]:1234"},
		{"", "", "192.168.1.1:80"},
		{"", "", "192.168.1.1"},
		{"", "", "invalid-remote-addr"},
		{"  10.0.0.1  , 1.2.3.4", "", "10.0.0.2:443"},
		{"\x00\xff", "\r\n", ":::1"},
	}

	for _, s := range seeds {
		f.Add(s.xff, s.xri, s.remoteAddr)
	}

	trusted := ParseTrustedProxies("10.0.0.0/8", "127.0.0.1/32")

	f.Fuzz(func(t *testing.T, xff, xri, remoteAddr string) {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		if xff != "" {
			req.Header.Set("X-Forwarded-For", xff)
		}
		if xri != "" {
			req.Header.Set("X-Real-IP", xri)
		}
		req.RemoteAddr = remoteAddr

		expectedDirect := ExtractIP(remoteAddr)

		// 1. Without trusted proxies: safe by default, MUST ignore spoofed headers
		ipNoProxies := RealIP(req)
		if ipNoProxies != expectedDirect {
			t.Fatalf("spoofed headers not ignored without trusted proxy: got %q, want %q", ipNoProxies, expectedDirect)
		}

		// 2. With trusted proxies: if direct connection is NOT trusted, MUST ignore spoofed headers
		ipWithProxies := RealIP(req, trusted...)
		directIP := net.ParseIP(expectedDirect)
		if directIP != nil && !IsTrusted(directIP, trusted) {
			if ipWithProxies != expectedDirect {
				t.Fatalf("spoofed headers not ignored when direct IP is untrusted: got %q, want %q", ipWithProxies, expectedDirect)
			}
		}

		// 3. Verify middleware handlers never panic with arbitrary inputs
		handlerUntrusted := New(10, 0)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		rec1 := httptest.NewRecorder()
		handlerUntrusted.ServeHTTP(rec1, req)

		handlerTrusted := New(10, 0, WithTrustedProxies("10.0.0.0/8", "127.0.0.1/32"))(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		rec2 := httptest.NewRecorder()
		handlerTrusted.ServeHTTP(rec2, req)
	})
}
