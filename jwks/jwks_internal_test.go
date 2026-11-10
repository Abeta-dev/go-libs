// SPDX-License-Identifier: MIT

package jwks

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestResolver_InternalBranches(t *testing.T) {
	privKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("failed to generate key: %v", err)
	}
	ecdhKey, err := privKey.PublicKey.ECDH()
	if err != nil {
		t.Fatalf("failed to get ECDH key: %v", err)
	}
	b := ecdhKey.Bytes()
	xStr := base64.RawURLEncoding.EncodeToString(b[1:33])
	yStr := base64.RawURLEncoding.EncodeToString(b[33:65])

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(Set{
			Keys: []Key{
				{
					KID: "bad-b64",
					KTY: "EC",
					Crv: "P-256",
					X:   "!!!invalid-base64!!!",
					Y:   yStr,
				},
				{
					KID: "good-kid",
					KTY: "EC",
					Crv: "P-256",
					X:   xStr,
					Y:   yStr,
				},
			},
		})
	}))

	r := NewResolver()
	r.defaultJWKSURL = server.URL

	// 1. Initial fetch via defaultJWKSURL and skip bad-b64 key
	pub, err := r.GetECPublicKey("good-kid", "")
	if err != nil || pub == nil {
		t.Fatalf("expected good-kid via defaultJWKSURL, got err: %v", err)
	}

	// 2. Double-check under write lock: first timeSince call (in getCachedKey) reports expired,
	// second timeSince call (under write lock) reports fresh (concurrent refresh completed).
	origTimeSince := timeSince
	var calls atomic.Int32
	timeSince = func(t time.Time) time.Duration {
		if calls.Add(1) == 1 {
			return 3 * time.Hour
		}
		return 0
	}
	pubDoubleCheck, err := r.GetECPublicKey("good-kid", "")
	timeSince = origTimeSince
	if err != nil || pubDoubleCheck == nil {
		t.Fatalf("expected key from write-lock double-check, got err: %v", err)
	}

	// 3. Expire cache TTL and close server to hit network error + stale cache fallback
	r.lastFetch = time.Now().Add(-3 * time.Hour)
	server.Close()

	stalePub, err := r.GetECPublicKey("good-kid", "")
	if err != nil || stalePub == nil {
		t.Fatalf("expected stale cached key on network failure, got err: %v", err)
	}
}
