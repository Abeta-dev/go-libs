// SPDX-License-Identifier: MIT

package ratelimit

import (
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// TestRealIP_DirectUntrustedClients verifies that when a request comes directly from an
// untrusted client (not in trusted proxies), all spoofed headers (X-Forwarded-For, X-Real-IP)
// are strictly ignored and RemoteAddr is returned.
func TestRealIP_DirectUntrustedClients(t *testing.T) {
	trusted := ParseTrustedProxies("10.0.0.0/8", "127.0.0.1/32", "172.16.0.0/12")

	tests := []struct {
		name       string
		remoteAddr string
		xff        string
		xri        string
		trusted    []*net.IPNet
		want       string
	}{
		{
			name:       "untrusted direct client with spoofed XFF (no trusted proxies configured)",
			remoteAddr: "198.51.100.50:4321",
			xff:        "1.1.1.1",
			trusted:    nil,
			want:       "198.51.100.50",
		},
		{
			name:       "untrusted direct client with spoofed XFF (with trusted proxies configured)",
			remoteAddr: "198.51.100.50:4321",
			xff:        "1.1.1.1",
			trusted:    trusted,
			want:       "198.51.100.50",
		},
		{
			name:       "untrusted direct client with spoofed X-Real-IP",
			remoteAddr: "198.51.100.50:4321",
			xri:        "8.8.8.8",
			trusted:    trusted,
			want:       "198.51.100.50",
		},
		{
			name:       "untrusted direct client with both spoofed XFF and X-Real-IP",
			remoteAddr: "203.0.113.99:9999",
			xff:        "10.0.0.1, 192.168.1.1, 1.2.3.4",
			xri:        "9.9.9.9",
			trusted:    trusted,
			want:       "203.0.113.99",
		},
		{
			name:       "untrusted IPv6 direct client with spoofed headers",
			remoteAddr: "[2001:db8:cafe::1]:54321",
			xff:        "127.0.0.1",
			trusted:    trusted,
			want:       "2001:db8:cafe::1",
		},
		{
			name:       "untrusted direct client without port in RemoteAddr",
			remoteAddr: "203.0.113.100",
			xff:        "1.1.1.1",
			trusted:    trusted,
			want:       "203.0.113.100",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/secure", nil)
			req.RemoteAddr = tt.remoteAddr
			if tt.xff != "" {
				req.Header.Set("X-Forwarded-For", tt.xff)
			}
			if tt.xri != "" {
				req.Header.Set("X-Real-IP", tt.xri)
			}

			got := RealIP(req, tt.trusted...)
			if got != tt.want {
				t.Errorf("RealIP() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestRealIP_ProxyChainTraversal verifies RFC 7239 reverse-proxy traversal from right to left,
// correctly stopping at the first untrusted IP in the chain.
func TestRealIP_ProxyChainTraversal(t *testing.T) {
	// Proxies in our infrastructure: 10.0.0.0/8 (internal), 172.16.0.0/12 (ingress/edge)
	trusted := ParseTrustedProxies("10.0.0.0/8", "172.16.0.0/12", "127.0.0.1/32")

	tests := []struct {
		name       string
		remoteAddr string
		xff        string
		want       string
	}{
		{
			name:       "single hop: client -> trusted proxy -> app",
			remoteAddr: "10.0.0.1:8080",
			xff:        "203.0.113.195",
			want:       "203.0.113.195",
		},
		{
			name:       "multi hop: client -> trusted edge -> trusted internal -> app",
			remoteAddr: "10.0.0.1:8080",
			xff:        "203.0.113.195, 172.16.10.5",
			want:       "203.0.113.195",
		},
		{
			name:       "spoofed prepend: attacker pretends to come from 1.1.1.1 through trusted chain",
			remoteAddr: "10.0.0.1:8080",
			// Attacker 203.0.113.195 sent XFF "1.1.1.1, 2.2.2.2" to edge 172.16.10.5
			xff:  "1.1.1.1, 2.2.2.2, 203.0.113.195, 172.16.10.5",
			want: "203.0.113.195", // Traversal stops at 203.0.113.195 because it is untrusted!
		},
		{
			name:       "untrusted intermediate proxy in chain",
			remoteAddr: "10.0.0.1:8080",
			// Attacker proxy 198.51.100.99 forwarded to trusted edge 172.16.10.5 claiming client 1.1.1.1
			xff:  "1.1.1.1, 198.51.100.99, 172.16.10.5",
			want: "198.51.100.99", // Stops at untrusted intermediate
		},
		{
			name:       "whitespace and tabs in XFF entries",
			remoteAddr: "10.0.0.1:8080",
			xff:        "  203.0.113.195 \t, \t 172.16.10.5  ",
			want:       "203.0.113.195",
		},
		{
			name:       "entries with ports stripped",
			remoteAddr: "10.0.0.1:8080",
			xff:        "203.0.113.195:12345, 172.16.10.5:80",
			want:       "203.0.113.195",
		},
		{
			name:       "all proxies in XFF are trusted -> returns leftmost valid IP",
			remoteAddr: "10.0.0.1:8080",
			xff:        "10.0.0.3, 10.0.0.2, 172.16.10.5",
			want:       "10.0.0.3",
		},
		{
			name:       "XFF with invalid IP in chain skips invalid entry safely",
			remoteAddr: "10.0.0.1:8080",
			xff:        "203.0.113.195, not-an-ip, 172.16.10.5",
			want:       "203.0.113.195",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api", nil)
			req.RemoteAddr = tt.remoteAddr
			req.Header.Set("X-Forwarded-For", tt.xff)

			got := RealIP(req, trusted...)
			if got != tt.want {
				t.Errorf("RealIP() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestRealIP_Adversarial_CyclingSpoofedIPs_DirectClient demonstrates that an attacker
// cannot bypass rate limiting by rotating X-Forwarded-For headers when connecting directly.
func TestRealIP_Adversarial_CyclingSpoofedIPs_DirectClient(t *testing.T) {
	// Rate limit: 2 req/min, trusting only internal proxy 10.0.0.1
	mw := New(2, time.Minute, WithTrustedProxies("10.0.0.1/32"))
	h := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	attackerIP := "198.51.100.77"

	for i := 1; i <= 5; i++ {
		req := httptest.NewRequest(http.MethodPost, "/login", nil)
		req.RemoteAddr = attackerIP + ":54321"
		// Attacker cycles different spoofed IPs on each request
		req.Header.Set("X-Forwarded-For", fmt.Sprintf("1.1.1.%d", i))
		req.Header.Set("X-Real-IP", fmt.Sprintf("2.2.2.%d", i))

		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)

		if i <= 2 {
			if rec.Code != http.StatusOK {
				t.Fatalf("request %d: expected 200, got %d", i, rec.Code)
			}
		} else {
			// Requests 3, 4, 5 must be blocked because the rate limiter tracks attackerIP!
			if rec.Code != http.StatusTooManyRequests {
				t.Fatalf("request %d: expected 429 RATE_LIMITED, got %d (attacker bypassed rate limiter with spoofed IP!)", i, rec.Code)
			}
		}
	}
}

// TestRealIP_Adversarial_CyclingSpoofedIPs_BehindProxy demonstrates that an attacker
// sending requests through a trusted reverse proxy cannot bypass rate limits by prepending spoofed IPs.
func TestRealIP_Adversarial_CyclingSpoofedIPs_BehindProxy(t *testing.T) {
	mw := New(2, time.Minute, WithTrustedProxies("10.0.0.1/32"))
	h := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	proxyAddr := "10.0.0.1:443"
	realAttackerIP := "203.0.113.88"

	for i := 1; i <= 5; i++ {
		req := httptest.NewRequest(http.MethodPost, "/login", nil)
		req.RemoteAddr = proxyAddr
		// Trusted proxy appends attacker's IP to XFF:
		req.Header.Set("X-Forwarded-For", fmt.Sprintf("fake-%d.example.com, %s", i, realAttackerIP))

		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)

		if i <= 2 {
			if rec.Code != http.StatusOK {
				t.Fatalf("request %d: expected 200, got %d", i, rec.Code)
			}
		} else {
			if rec.Code != http.StatusTooManyRequests {
				t.Fatalf("request %d: expected 429, got %d (attacker bypassed rate limiter behind proxy!)", i, rec.Code)
			}
		}
	}
}

// TestRealIP_Adversarial_DuplicateHeaders verifies handling of multiple X-Forwarded-For header lines.
func TestRealIP_Adversarial_DuplicateHeaders(t *testing.T) {
	trusted := ParseTrustedProxies("10.0.0.1/32")

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.0.0.1:1234"
	// Multiple header lines in HTTP request:
	req.Header.Add("X-Forwarded-For", "spoofed.client.1")
	req.Header.Add("X-Forwarded-For", "203.0.113.123")

	got := RealIP(req, trusted...)
	if got != "203.0.113.123" {
		t.Errorf("RealIP(duplicate headers) = %q, want 203.0.113.123", got)
	}
}

// TestRealIP_EdgeCases tests edge cases like nil request, empty strings, and IPv6 formatting.
func TestRealIP_EdgeCases(t *testing.T) {
	if got := RealIP(nil); got != "" {
		t.Errorf("RealIP(nil) = %q, want empty string", got)
	}

	reqEmpty := httptest.NewRequest(http.MethodGet, "/", nil)
	reqEmpty.RemoteAddr = ""
	if got := RealIP(reqEmpty); got != "" {
		t.Errorf("RealIP(empty RemoteAddr) = %q, want empty string", got)
	}

	// IPv6 bracketed RemoteAddr with port
	reqIPv6 := httptest.NewRequest(http.MethodGet, "/", nil)
	reqIPv6.RemoteAddr = "[::1]:8080"
	if got := RealIP(reqIPv6); got != "::1" {
		t.Errorf("RealIP([::1]:8080) = %q, want ::1", got)
	}

	// IPv6 bracketed RemoteAddr without port
	reqIPv6NoPort := httptest.NewRequest(http.MethodGet, "/", nil)
	reqIPv6NoPort.RemoteAddr = "[2001:db8::1]"
	if got := RealIP(reqIPv6NoPort); got != "2001:db8::1" {
		t.Errorf("RealIP([2001:db8::1]) = %q, want 2001:db8::1", got)
	}
}

// TestTrustedProxies_TypeAndHelpers tests the helper types, options, and methods.
func TestTrustedProxies_TypeAndHelpers(t *testing.T) {
	tp := TrustedProxies{"127.0.0.1/8", "10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16", "::1/128"}
	nets := tp.IPNets()
	if len(nets) != 5 {
		t.Fatalf("expected 5 IPNets, got %d", len(nets))
	}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.0.0.1:1234"
	req.Header.Set("X-Forwarded-For", "203.0.113.50")

	// tp.RealIP
	if got := tp.RealIP(req); got != "203.0.113.50" {
		t.Errorf("tp.RealIP() = %q, want 203.0.113.50", got)
	}

	// RealIPWithProxies
	if got := RealIPWithProxies(req, "10.0.0.0/8"); got != "203.0.113.50" {
		t.Errorf("RealIPWithProxies() = %q, want 203.0.113.50", got)
	}

	// WithTrustedIPNets
	mw := New(10, time.Minute, WithTrustedIPNets(nets...))
	if mw == nil {
		t.Fatal("WithTrustedIPNets returned nil middleware")
	}

	// WithDefaultTrustedProxies
	mwDefault := New(10, time.Minute, WithDefaultTrustedProxies())
	if mwDefault == nil {
		t.Fatal("WithDefaultTrustedProxies returned nil middleware")
	}

	// KeyFuncWithTrustedProxies
	kf := KeyFuncWithTrustedProxies("10.0.0.0/8")
	if key := kf(req); key != "203.0.113.50" {
		t.Errorf("KeyFuncWithTrustedProxies key = %q, want 203.0.113.50", key)
	}
}

// TestParseTrustedProxies_Comprehensive tests various valid and invalid CIDR notations.
func TestParseTrustedProxies_Comprehensive(t *testing.T) {
	tests := []struct {
		input string
		valid bool
	}{
		{"127.0.0.1/8", true},
		{"10.0.0.0/8", true},
		{"172.16.0.0/12", true},
		{"192.168.0.0/16", true},
		{"::1/128", true},
		{"192.168.1.1", true},
		{"::1", true},
		{"", false},
		{"invalid", false},
		{"256.256.256.256", false},
		{"10.0.0.0/33", false},
	}

	for _, tt := range tests {
		ipNet, err := ParseCIDRorIP(tt.input)
		if tt.valid && (err != nil || ipNet == nil) {
			t.Errorf("ParseCIDRorIP(%q) failed unexpectedly: %v", tt.input, err)
		}
		if !tt.valid && err == nil {
			t.Errorf("ParseCIDRorIP(%q) succeeded unexpectedly", tt.input)
		}
	}
}
