// SPDX-License-Identifier: MIT

package jwks

import (
	"context"
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
	if key := r.getCachedKey(kid); key != nil {
		return key, nil
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	// Double check under write lock
	if key, exists := r.cache[kid]; exists && time.Since(r.lastFetch) < r.cacheTTL {
		return key, nil
	}

	targetURL, err := r.resolveJWKSURL(jwksURL)
	if err != nil {
		return nil, err
	}

	if err := r.fetchAndCacheKeys(targetURL); err != nil {
		if cachedKey := r.cache[kid]; cachedKey != nil {
			return cachedKey, nil // Fallback to stale cached key if network is temporarily unreachable
		}
		return nil, err
	}

	if found, ok := r.cache[kid]; ok {
		return found, nil
	}
	// Fallback to first available P-256 EC key if kid was omitted or changed
	for _, k := range r.cache {
		return k, nil
	}
	return nil, fmt.Errorf("public key with kid '%s' not found in JWKS", kid)
}

func (r *Resolver) getCachedKey(kid string) *ecdsa.PublicKey {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if key, exists := r.cache[kid]; exists && time.Since(r.lastFetch) < r.cacheTTL {
		return key
	}
	return nil
}

func (r *Resolver) resolveJWKSURL(jwksURL string) (string, error) {
	if jwksURL != "" {
		return jwksURL, nil
	}
	if r.defaultJWKSURL != "" {
		return r.defaultJWKSURL, nil
	}
	if envURL := os.Getenv("AUTH_JWKS_URL"); envURL != "" {
		return strings.TrimSpace(envURL), nil
	}
	if envBase := os.Getenv("AUTH_ISSUER_URL"); envBase != "" {
		return fmt.Sprintf("%s/.well-known/jwks.json", strings.TrimSuffix(strings.TrimSpace(envBase), "/")), nil
	}
	return "", fmt.Errorf("failed to fetch JWKS: no jwksURL provided and AUTH_JWKS_URL/AUTH_ISSUER_URL not set")
}

func (r *Resolver) fetchAndCacheKeys(targetURL string) error {
	//nolint:gosec // G704: targetURL is an operator-configured endpoint for JWKS public keys
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, targetURL, http.NoBody)
	if err != nil {
		return fmt.Errorf("failed to create JWKS request: %w", err)
	}
	resp, err := r.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to fetch JWKS from %s: %w", targetURL, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to fetch JWKS: received HTTP %d from %s", resp.StatusCode, targetURL)
	}

	var set Set
	if err := json.NewDecoder(resp.Body).Decode(&set); err != nil {
		return fmt.Errorf("failed to decode JWKS payload: %w", err)
	}

	for _, k := range set.Keys {
		if k.KTY == "EC" && k.Crv == "P-256" {
			if pubKey := parseECP256Key(k); pubKey != nil {
				r.cache[k.KID] = pubKey
			}
		}
	}
	r.lastFetch = time.Now()
	return nil
}

func parseECP256Key(k Key) *ecdsa.PublicKey {
	xBytes, errX := base64.RawURLEncoding.DecodeString(k.X)
	yBytes, errY := base64.RawURLEncoding.DecodeString(k.Y)
	if errX != nil || errY != nil {
		return nil
	}
	//nolint:staticcheck // SA1019: JWKS standard defines EC public keys via raw big-endian coordinates
	return &ecdsa.PublicKey{
		Curve: elliptic.P256(),
		X:     new(big.Int).SetBytes(xBytes),
		Y:     new(big.Int).SetBytes(yBytes),
	}
}

// GetECPublicKey fetches from the DefaultResolver.
func GetECPublicKey(kid, jwksURL string) (*ecdsa.PublicKey, error) {
	return DefaultResolver.GetECPublicKey(kid, jwksURL)
}
