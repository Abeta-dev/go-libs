// SPDX-License-Identifier: MIT

package jwks

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

// Key represents a single cryptographic key entry in a JWKS document.
type Key struct {
	KID string `json:"kid"`
	KTY string `json:"kty"`
	Alg string `json:"alg"`
	Crv string `json:"crv"`
	X   string `json:"x"`
	Y   string `json:"y"`
}

// Set represents a collection of JWKS keys.
type Set struct {
	Keys []Key `json:"keys"`
}

// Resolver manages in-memory caching and retrieval of public keys from a JWKS URL.
type Resolver struct {
	mu             sync.RWMutex
	cache          map[string]*ecdsa.PublicKey
	lastFetch      time.Time
	cacheTTL       time.Duration
	httpClient     *http.Client
	defaultJWKSURL string
}

// DefaultResolver is the package-level default JWKS resolver instance.
var DefaultResolver = NewResolver()

// NewResolver creates a new JWKS public key resolver with sensible defaults.
func NewResolver() *Resolver {
	return &Resolver{
		cache:      make(map[string]*ecdsa.PublicKey),
		cacheTTL:   2 * time.Hour,
		httpClient: &http.Client{Timeout: 5 * time.Second},
	}
}

// GetECPublicKey retrieves or caches the ECDSA P-256 public key for the specified key ID (kid).
func (r *Resolver) GetECPublicKey(kid, jwksURL string) (*ecdsa.PublicKey, error) {
	r.mu.RLock()
	key, exists := r.cache[kid]
	cachedTime := r.lastFetch
	r.mu.RUnlock()

	if exists && time.Since(cachedTime) < r.cacheTTL {
		return key, nil
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	// Double check under write lock
	if key, exists = r.cache[kid]; exists && time.Since(r.lastFetch) < r.cacheTTL {
		return key, nil
	}

	if jwksURL == "" {
		jwksURL = r.defaultJWKSURL
	}
	if jwksURL == "" {
		if envURL := os.Getenv("AUTH_JWKS_URL"); envURL != "" {
			jwksURL = strings.TrimSpace(envURL)
		} else if envBase := os.Getenv("AUTH_ISSUER_URL"); envBase != "" {
			jwksURL = fmt.Sprintf("%s/.well-known/jwks.json", strings.TrimSuffix(strings.TrimSpace(envBase), "/"))
		} else {
			return nil, fmt.Errorf("failed to fetch JWKS: no jwksURL provided and AUTH_JWKS_URL/AUTH_ISSUER_URL not set")
		}
	}

	resp, err := r.httpClient.Get(jwksURL)
	if err != nil {
		if key != nil {
			return key, nil // Fallback to stale cached key if network is temporarily unreachable
		}
		return nil, fmt.Errorf("failed to fetch JWKS from %s: %w", jwksURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch JWKS: received HTTP %d from %s", resp.StatusCode, jwksURL)
	}

	var set Set
	if err := json.NewDecoder(resp.Body).Decode(&set); err != nil {
		return nil, fmt.Errorf("failed to decode JWKS payload: %w", err)
	}

	for _, k := range set.Keys {
		if k.KTY == "EC" && k.Crv == "P-256" {
			xBytes, errX := base64.RawURLEncoding.DecodeString(k.X)
			yBytes, errY := base64.RawURLEncoding.DecodeString(k.Y)
			if errX == nil && errY == nil {
				pubKey := &ecdsa.PublicKey{
					Curve: elliptic.P256(),
					X:     new(big.Int).SetBytes(xBytes),
					Y:     new(big.Int).SetBytes(yBytes),
				}
				r.cache[k.KID] = pubKey
			}
		}
	}
	r.lastFetch = time.Now()

	if found, ok := r.cache[kid]; ok {
		return found, nil
	}
	// Fallback to first available P-256 EC key if kid was omitted or changed
	for _, k := range r.cache {
		return k, nil
	}
	return nil, fmt.Errorf("public key with kid '%s' not found in JWKS", kid)
}

// GetECPublicKey fetches from the DefaultResolver.
func GetECPublicKey(kid, jwksURL string) (*ecdsa.PublicKey, error) {
	return DefaultResolver.GetECPublicKey(kid, jwksURL)
}
