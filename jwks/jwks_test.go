// SPDX-License-Identifier: MIT

package jwks_test

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/umesh0492/go-libs/jwks"
)

func testExtractXY(t *testing.T, pub *ecdsa.PublicKey) (string, string) {
	t.Helper()
	ecdhKey, err := pub.ECDH()
	if err != nil {
		t.Fatalf("failed to get ECDH key: %v", err)
	}
	b := ecdhKey.Bytes()
	x := base64.RawURLEncoding.EncodeToString(b[1:33])
	y := base64.RawURLEncoding.EncodeToString(b[33:65])
	return x, y
}

func TestJWKS_GetECPublicKey(t *testing.T) {
	// Generate test ECDSA key
	privKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("failed to generate test ecdsa key: %v", err)
	}

	xStr, yStr := testExtractXY(t, &privKey.PublicKey)

	mockJWKS := jwks.Set{
		Keys: []jwks.Key{
			{
				KID: "test-kid-1",
				KTY: "EC",
				Alg: "ES256",
				Crv: "P-256",
				X:   xStr,
				Y:   yStr,
			},
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(mockJWKS)
	}))
	defer server.Close()

	resolver := jwks.NewResolver()
	pubKey, err := resolver.GetECPublicKey("test-kid-1", server.URL)
	if err != nil {
		t.Fatalf("failed to get public key: %v", err)
	}

	if !pubKey.Equal(&privKey.PublicKey) {
		t.Fatal("retrieved public key does not match expected")
	}

	// Verify caching (second call doesn't error even if server is closed)
	server.Close()
	cachedKey, err := resolver.GetECPublicKey("test-kid-1", server.URL)
	if err != nil {
		t.Fatalf("expected cached key, got error: %v", err)
	}
	if !cachedKey.Equal(pubKey) {
		t.Fatal("cached key mismatch")
	}
}

func TestJWKS_EnvFallbacks(t *testing.T) {
	privKey, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	xStr, yStr := testExtractXY(t, &privKey.PublicKey)
	mockJWKS := jwks.Set{
		Keys: []jwks.Key{
			{
				KID: "env-kid",
				KTY: "EC",
				Alg: "ES256",
				Crv: "P-256",
				X:   xStr,
				Y:   yStr,
			},
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(mockJWKS)
	}))
	defer server.Close()

	// 1. AUTH_JWKS_URL
	os.Setenv("AUTH_JWKS_URL", server.URL)
	defer os.Unsetenv("AUTH_JWKS_URL")

	resolver := jwks.NewResolver()
	key, err := resolver.GetECPublicKey("env-kid", "")
	if err != nil || key == nil {
		t.Fatalf("expected key resolved from AUTH_JWKS_URL, got err: %v", err)
	}

	// 2. AUTH_ISSUER_URL
	os.Unsetenv("AUTH_JWKS_URL")
	os.Setenv("AUTH_ISSUER_URL", server.URL)
	defer os.Unsetenv("AUTH_ISSUER_URL")

	resolver2 := jwks.NewResolver()
	key2, err2 := resolver2.GetECPublicKey("env-kid", "")
	if err2 != nil || key2 == nil {
		t.Fatalf("expected key resolved from AUTH_ISSUER_URL, got err: %v", err2)
	}

	// 3. No URL and no env vars
	os.Unsetenv("AUTH_ISSUER_URL")
	resolver3 := jwks.NewResolver()
	_, err3 := resolver3.GetECPublicKey("env-kid", "")
	if err3 == nil {
		t.Fatal("expected error when no URL and no env vars set")
	}
}

func TestJWKS_ErrorCases(t *testing.T) {
	// 500 Internal Server Error
	server500 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server500.Close()

	resolver := jwks.NewResolver()
	_, err := resolver.GetECPublicKey("kid-1", server500.URL)
	if err == nil {
		t.Fatal("expected error for HTTP 500 response")
	}

	// Malformed JSON
	serverBadJSON := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("{invalid-json"))
	}))
	defer serverBadJSON.Close()

	_, err = resolver.GetECPublicKey("kid-1", serverBadJSON.URL)
	if err == nil {
		t.Fatal("expected error for malformed JSON")
	}

	// Empty keys
	serverEmpty := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(jwks.Set{})
	}))
	defer serverEmpty.Close()

	_, err = resolver.GetECPublicKey("kid-missing", serverEmpty.URL)
	if err == nil {
		t.Fatal("expected error when key not found in JWKS")
	}

	// Invalid URL
	_, err = resolver.GetECPublicKey("kid-1", "http://[::1]:namedport")
	if err == nil {
		t.Fatal("expected error for invalid URL")
	}
}

func TestJWKS_PackageLevelHelper(t *testing.T) {
	privKey, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	xStr, yStr := testExtractXY(t, &privKey.PublicKey)
	mockJWKS := jwks.Set{
		Keys: []jwks.Key{
			{
				KID: "pkg-kid",
				KTY: "EC",
				Alg: "ES256",
				Crv: "P-256",
				X:   xStr,
				Y:   yStr,
			},
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(mockJWKS)
	}))
	defer server.Close()

	key, err := jwks.GetECPublicKey("pkg-kid", server.URL)
	if err != nil || key == nil {
		t.Fatalf("expected package-level GetECPublicKey to succeed, got: %v", err)
	}
}
