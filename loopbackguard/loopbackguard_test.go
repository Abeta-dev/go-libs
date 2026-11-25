// SPDX-License-Identifier: MIT

package loopbackguard

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type errReader struct{}

func (errReader) Read([]byte) (int, error) { return 0, errors.New("entropy exhausted") }

func TestValidateLoopbackAddr(t *testing.T) {
	valid := []string{
		"127.0.0.1:8080",
		"127.0.0.1:0",
		"127.0.1.5:9090",
		"127.0.0.1",
		"[::1]:8080",
		"::1",
		"localhost:3000",
		"LOCALHOST",
	}
	for _, addr := range valid {
		if err := ValidateLoopbackAddr(addr); err != nil {
			t.Errorf("ValidateLoopbackAddr(%q) unexpected error: %v", addr, err)
		}
	}

	invalid := []struct {
		addr    string
		wantErr error
	}{
		{"", ErrInvalidAddress},
		{"   ", ErrInvalidAddress},
		{"127.0.0.1:badport", ErrInvalidAddress},
		{"127.0.0.1:70000", ErrInvalidAddress},
		{"[bad:ipv6", ErrInvalidAddress},
		{":8080", ErrNonLoopbackAddress},
		{"0.0.0.0:8080", ErrNonLoopbackAddress},
		{"[::]:8080", ErrNonLoopbackAddress},
		{"2001:db8::1", ErrNonLoopbackAddress},
		{"192.168.1.10:8080", ErrNonLoopbackAddress},
		{"example.com:8080", ErrNonLoopbackAddress},
	}
	for _, tc := range invalid {
		err := ValidateLoopbackAddr(tc.addr)
		if !errors.Is(err, tc.wantErr) {
			t.Errorf("ValidateLoopbackAddr(%q) = %v, want %v", tc.addr, err, tc.wantErr)
		}
	}
}

func TestGenerateCSRFToken_ErrorBranch(t *testing.T) {
	tok, err := GenerateCSRFToken()
	if err != nil || len(tok) != DefaultCSRFTokenBytes*2 {
		t.Fatalf("GenerateCSRFToken failed: err=%v len=%d", err, len(tok))
	}

	if _, err := New(WithRandReader(errReader{})); err == nil {
		t.Fatal("expected error when entropy source fails")
	}
}

func TestGuard_MiddlewareDefenseInDepth(t *testing.T) {
	g, err := New(
		nil,
		WithAllowedHosts("bff.local", " "),
		WithAllowedOrigins("http://localhost:5173", " "),
		WithCSRFHeader("X-CSRF-Token"),
		WithExemptPaths("/api/bootstrap", " "),
		WithSafeMethods("GET", "HEAD", "OPTIONS"),
		WithRequireLoopbackPeer(true),
	)
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	if len(g.Token()) != DefaultCSRFTokenBytes*2 {
		t.Fatalf("unexpected token length %d", len(g.Token()))
	}
	if g.VerifyToken("") || g.VerifyToken("wrong") || !g.VerifyToken(g.Token()) {
		t.Fatal("VerifyToken failed basic checks")
	}

	okHandler := g.Middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	makeReq := func(method, target, host string) *http.Request {
		r := httptest.NewRequest(method, target, nil)
		r.Host = host
		r.RemoteAddr = "127.0.0.1:54321"
		return r
	}

	// 1. Non-loopback peer rejected
	t.Run("non_loopback_peer_blocked", func(t *testing.T) {
		r := makeReq(http.MethodGet, "/", "127.0.0.1:8080")
		r.RemoteAddr = "192.168.1.50:9999"
		rec := httptest.NewRecorder()
		okHandler.ServeHTTP(rec, r)
		if rec.Code != http.StatusForbidden || !strings.Contains(rec.Body.String(), "NON_LOOPBACK_PEER") {
			t.Fatalf("expected NON_LOOPBACK_PEER 403, got %d %s", rec.Code, rec.Body.String())
		}
	})

	// 2. DNS-rebinding Host allowlist
	t.Run("dns_rebinding_host_check", func(t *testing.T) {
		for _, badHost := range []string{"", ":8080", "evil.example.com:8080", "192.168.1.5:8080", "[bad", "a:b:c"} {
			rec := httptest.NewRecorder()
			okHandler.ServeHTTP(rec, makeReq(http.MethodGet, "/", badHost))
			if rec.Code != http.StatusForbidden || !strings.Contains(rec.Body.String(), "DNS_REBINDING_BLOCKED") {
				t.Fatalf("host %q: expected DNS_REBINDING_BLOCKED 403, got %d %s", badHost, rec.Code, rec.Body.String())
			}
		}
		for _, goodHost := range []string{"localhost:8080", "127.0.0.1:8080", "[::1]:8080", "::1", "bff.local:8080", "localhost"} {
			rec := httptest.NewRecorder()
			okHandler.ServeHTTP(rec, makeReq(http.MethodGet, "/", goodHost))
			if rec.Code != http.StatusOK {
				t.Fatalf("host %q: expected 200, got %d %s", goodHost, rec.Code, rec.Body.String())
			}
		}
	})

	// 3. Origin Same-Origin & allowlist check
	t.Run("origin_check", func(t *testing.T) {
		// Exact Same-Origin passes
		rSame := makeReq(http.MethodGet, "/", "127.0.0.1:8080")
		rSame.Header.Set("Origin", "http://127.0.0.1:8080")
		recSame := httptest.NewRecorder()
		okHandler.ServeHTTP(recSame, rSame)
		if recSame.Code != http.StatusOK {
			t.Fatalf("same-origin expected 200, got %d", recSame.Code)
		}

		// Explicitly allowed Origin passes
		rAllowed := makeReq(http.MethodGet, "/", "127.0.0.1:8080")
		rAllowed.Header.Set("Origin", "http://localhost:5173")
		recAllowed := httptest.NewRecorder()
		okHandler.ServeHTTP(recAllowed, rAllowed)
		if recAllowed.Code != http.StatusOK {
			t.Fatalf("allowed origin expected 200, got %d", recAllowed.Code)
		}

		// Evil / invalid / non-http scheme Origin blocked
		for _, badOrigin := range []string{"https://evil.com", "null", "javascript://127.0.0.1:8080", "http://[::1"} {
			rBad := makeReq(http.MethodGet, "/", "127.0.0.1:8080")
			rBad.Header.Set("Origin", badOrigin)
			recBad := httptest.NewRecorder()
			okHandler.ServeHTTP(recBad, rBad)
			if recBad.Code != http.StatusForbidden || !strings.Contains(recBad.Body.String(), "ORIGIN_MISMATCH") {
				t.Fatalf("origin %q: expected ORIGIN_MISMATCH 403, got %d", badOrigin, recBad.Code)
			}
		}
	})

	// 4. Sec-Fetch-Site checks
	t.Run("sec_fetch_site_checks", func(t *testing.T) {
		// Mutating cross-site or same-site blocked
		for _, site := range []string{"cross-site", "same-site"} {
			r := makeReq(http.MethodPost, "/api/write", "127.0.0.1:8080")
			r.Header.Set("Sec-Fetch-Site", site)
			r.Header.Set(DefaultCSRFHeader, g.Token())
			rec := httptest.NewRecorder()
			okHandler.ServeHTTP(rec, r)
			if rec.Code != http.StatusForbidden || !strings.Contains(rec.Body.String(), "CROSS_SITE_BLOCKED") {
				t.Fatalf("POST Sec-Fetch-Site=%q: expected 403, got %d", site, rec.Code)
			}
		}

		// Safe GET cross-site navigate allowed when not strict, subresource blocked
		rNav := makeReq(http.MethodGet, "/", "127.0.0.1:8080")
		rNav.Header.Set("Sec-Fetch-Site", "cross-site")
		rNav.Header.Set("Sec-Fetch-Mode", "navigate")
		recNav := httptest.NewRecorder()
		okHandler.ServeHTTP(recNav, rNav)
		if recNav.Code != http.StatusOK {
			t.Fatalf("expected top-level GET navigate 200, got %d", recNav.Code)
		}

		rSub := makeReq(http.MethodGet, "/api/data", "127.0.0.1:8080")
		rSub.Header.Set("Sec-Fetch-Site", "cross-site")
		rSub.Header.Set("Sec-Fetch-Mode", "cors")
		recSub := httptest.NewRecorder()
		okHandler.ServeHTTP(recSub, rSub)
		if recSub.Code != http.StatusForbidden {
			t.Fatalf("expected cross-site subresource 403, got %d", recSub.Code)
		}

		// Safe GET same-site allowed
		rSameSite := makeReq(http.MethodGet, "/", "127.0.0.1:8080")
		rSameSite.Header.Set("Sec-Fetch-Site", "same-site")
		recSS := httptest.NewRecorder()
		okHandler.ServeHTTP(recSS, rSameSite)
		if recSS.Code != http.StatusOK {
			t.Fatalf("expected safe GET same-site 200, got %d", recSS.Code)
		}

		// Strict Sec-Fetch-Site blocks cross-site navigate too
		strictGuard, _ := New(WithStrictSecFetchSite(true), WithDisableCSRF())
		strictH := strictGuard.Middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		recStrict := httptest.NewRecorder()
		strictH.ServeHTTP(recStrict, rNav)
		if recStrict.Code != http.StatusForbidden {
			t.Fatalf("expected strict cross-site navigate 403, got %d", recStrict.Code)
		}
		// Disabled CSRF allows POST with valid Host/Same-Origin
		rPostNoCSRF := makeReq(http.MethodPost, "/api/x", "127.0.0.1:8080")
		recNoCSRF := httptest.NewRecorder()
		strictH.ServeHTTP(recNoCSRF, rPostNoCSRF)
		if recNoCSRF.Code != http.StatusOK {
			t.Fatalf("expected 200 when CSRF disabled, got %d", recNoCSRF.Code)
		}
	})

	// 5. CSRF token validation & exempt paths
	t.Run("csrf_token_and_exempt_paths", func(t *testing.T) {
		// Missing CSRF -> 403
		rMiss := makeReq(http.MethodPost, "/api/write", "127.0.0.1:8080")
		recMiss := httptest.NewRecorder()
		okHandler.ServeHTTP(recMiss, rMiss)
		if recMiss.Code != http.StatusForbidden || !strings.Contains(recMiss.Body.String(), "CSRF_INVALID") {
			t.Fatalf("expected CSRF_INVALID 403, got %d", recMiss.Code)
		}

		// Valid CSRF -> 200
		rValid := makeReq(http.MethodPost, "/api/write", "127.0.0.1:8080")
		rValid.Header.Set(DefaultCSRFHeader, g.Token())
		rValid.Header.Set("Sec-Fetch-Site", "same-origin")
		recValid := httptest.NewRecorder()
		okHandler.ServeHTTP(recValid, rValid)
		if recValid.Code != http.StatusOK {
			t.Fatalf("expected 200 with valid CSRF token, got %d", recValid.Code)
		}

		// Exempt path -> 200 without CSRF token
		rExempt := makeReq(http.MethodPost, "/api/bootstrap", "127.0.0.1:8080")
		recExempt := httptest.NewRecorder()
		okHandler.ServeHTTP(recExempt, rExempt)
		if recExempt.Code != http.StatusOK {
			t.Fatalf("expected 200 for exempt path, got %d", recExempt.Code)
		}

		// Explicit token option
		gExplicit, err := New(WithCSRFToken("fixed-test-token"))
		if err != nil || gExplicit.Token() != "fixed-test-token" {
			t.Fatalf("WithCSRFToken failed: err=%v token=%q", err, gExplicit.Token())
		}
	})
}
