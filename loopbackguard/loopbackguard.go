// SPDX-License-Identifier: MIT

package loopbackguard

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
)

const (
	// DefaultCSRFHeader is the default HTTP header carrying the per-boot CSRF token.
	DefaultCSRFHeader = "X-CSRF-Token"
	// DefaultCSRFTokenBytes is the entropy length in bytes for ephemeral tokens (256-bit).
	DefaultCSRFTokenBytes = 32
)

var (
	// ErrInvalidAddress is returned when a listen address cannot be parsed as host or host:port.
	ErrInvalidAddress = errors.New("loopbackguard: invalid listen address")
	// ErrNonLoopbackAddress is returned when a listen address resolves outside loopback interfaces.
	ErrNonLoopbackAddress = errors.New("loopbackguard: listen address must bind to a loopback interface")
)

var randReader io.Reader = rand.Reader

// ValidateLoopbackAddr verifies that addr binds exclusively to a loopback interface
// (localhost, 127.0.0.0/8, or ::1), optionally with a valid TCP port (0..65535).
// Wildcard binds (":8080", "0.0.0.0:8080", "[::]:8080") and non-loopback hosts are rejected.
func ValidateLoopbackAddr(addr string) error {
	trimmed := strings.TrimSpace(addr)
	if trimmed == "" {
		return ErrInvalidAddress
	}

	host := trimmed
	if strings.Contains(trimmed, ":") {
		h, portStr, err := net.SplitHostPort(trimmed)
		if err != nil {
			// Accept bare IPv6 loopback literal "::1" without brackets/port.
			if ip, parseErr := netip.ParseAddr(trimmed); parseErr == nil {
				if ip.IsLoopback() {
					return nil
				}
				return fmt.Errorf("%w: %s", ErrNonLoopbackAddress, trimmed)
			}
			return fmt.Errorf("%w: %w", ErrInvalidAddress, err)
		}
		port, err := strconv.Atoi(portStr)
		if err != nil || port < 0 || port > 65535 {
			return fmt.Errorf("%w: invalid port %q", ErrInvalidAddress, portStr)
		}
		host = h
	}

	if host == "" {
		return fmt.Errorf("%w: empty host binds all interfaces", ErrNonLoopbackAddress)
	}
	if strings.EqualFold(host, "localhost") {
		return nil
	}
	ip, err := netip.ParseAddr(host)
	if err != nil || !ip.IsLoopback() {
		return fmt.Errorf("%w: %s", ErrNonLoopbackAddress, host)
	}
	return nil
}

// GenerateCSRFToken generates a cryptographically random 256-bit (64-hex-char)
// per-boot ephemeral CSRF token using crypto/rand.
func GenerateCSRFToken() (string, error) {
	return generateCSRFToken(randReader)
}

func generateCSRFToken(r io.Reader) (string, error) {
	buf := make([]byte, DefaultCSRFTokenBytes)
	if _, err := io.ReadFull(r, buf); err != nil {
		return "", fmt.Errorf("loopbackguard: generate csrf token: %w", err)
	}
	return hex.EncodeToString(buf), nil
}

type config struct {
	allowedHosts        map[string]struct{}
	allowedOrigins      map[string]struct{}
	csrfToken           string
	csrfHeader          string
	disableCSRF         bool
	safeMethods         map[string]struct{}
	exemptPrefixes      []string
	strictSecFetchSite  bool
	requireLoopbackPeer bool
	randReader          io.Reader
}

// Option configures a Guard instance.
type Option func(*config)

// WithAllowedHosts adds additional hostnames (without port) to the DNS-rebinding Host allowlist.
// Loopback hosts ("localhost", "127.0.0.1", "::1", and 127.0.0.0/8) are always allowed.
func WithAllowedHosts(hosts ...string) Option {
	return func(c *config) {
		for _, h := range hosts {
			if normalized := strings.ToLower(strings.TrimSpace(h)); normalized != "" {
				c.allowedHosts[normalized] = struct{}{}
			}
		}
	}
}

// WithAllowedOrigins registers additional explicitly trusted Origin values
// (e.g. "http://localhost:5173" for local dev servers) in addition to exact Same-Origin matches.
func WithAllowedOrigins(origins ...string) Option {
	return func(c *config) {
		for _, o := range origins {
			if normalized := strings.ToLower(strings.TrimRight(strings.TrimSpace(o), "/")); normalized != "" {
				c.allowedOrigins[normalized] = struct{}{}
			}
		}
	}
}

// WithCSRFToken configures an explicit CSRF token instead of auto-generating a
// fresh per-boot CSPRNG token.
func WithCSRFToken(token string) Option {
	return func(c *config) {
		c.csrfToken = strings.TrimSpace(token)
	}
}

// WithCSRFHeader overrides the HTTP header inspected for CSRF tokens.
// Defaults to DefaultCSRFHeader ("X-CSRF-Token").
func WithCSRFHeader(header string) Option {
	return func(c *config) {
		if trimmed := strings.TrimSpace(header); trimmed != "" {
			c.csrfHeader = trimmed
		}
	}
}

// WithDisableCSRF disables CSRF token verification while preserving DNS-rebinding
// Host allowlist and Sec-Fetch-Site / Origin checks.
func WithDisableCSRF() Option {
	return func(c *config) {
		c.disableCSRF = true
	}
}

// WithSafeMethods overrides HTTP methods exempt from CSRF token verification.
// Defaults to GET, HEAD, OPTIONS, and TRACE.
func WithSafeMethods(methods ...string) Option {
	return func(c *config) {
		m := make(map[string]struct{}, len(methods))
		for _, method := range methods {
			if trimmed := strings.ToUpper(strings.TrimSpace(method)); trimmed != "" {
				m[trimmed] = struct{}{}
			}
		}
		if len(m) > 0 {
			c.safeMethods = m
		}
	}
}

// WithExemptPaths configures URL path prefixes exempt from CSRF token verification
// (DNS-rebinding Host and Same-Origin checks still apply).
func WithExemptPaths(prefixes ...string) Option {
	return func(c *config) {
		for _, p := range prefixes {
			if trimmed := strings.TrimSpace(p); trimmed != "" {
				c.exemptPrefixes = append(c.exemptPrefixes, trimmed)
			}
		}
	}
}

// WithStrictSecFetchSite rejects Sec-Fetch-Site: cross-site even on top-level
// GET/HEAD navigation requests (Sec-Fetch-Mode: navigate).
func WithStrictSecFetchSite(strict bool) Option {
	return func(c *config) {
		c.strictSecFetchSite = strict
	}
}

// WithRequireLoopbackPeer enforces that r.RemoteAddr originates from a loopback IP.
func WithRequireLoopbackPeer(require bool) Option {
	return func(c *config) {
		c.requireLoopbackPeer = require
	}
}

// WithRandReader overrides the randomness source used when auto-generating the
// per-boot ephemeral CSRF token. Defaults to crypto/rand.Reader.
func WithRandReader(r io.Reader) Option {
	return func(c *config) {
		if r != nil {
			c.randReader = r
		}
	}
}

// Guard enforces loopback DNS-rebinding protection, Same-Origin checks, and
// constant-time per-boot ephemeral CSRF token validation.
type Guard struct {
	cfg   config
	token []byte
}

// New constructs a Guard with an automatically generated 256-bit per-boot ephemeral
// CSRF token (unless WithCSRFToken or WithDisableCSRF is provided).
func New(opts ...Option) (*Guard, error) {
	cfg := config{
		allowedHosts:   make(map[string]struct{}),
		allowedOrigins: make(map[string]struct{}),
		csrfHeader:     DefaultCSRFHeader,
		safeMethods: map[string]struct{}{
			http.MethodGet:     {},
			http.MethodHead:    {},
			http.MethodOptions: {},
			http.MethodTrace:   {},
		},
	}
	for _, opt := range opts {
		if opt != nil {
			opt(&cfg)
		}
	}

	tokenStr := cfg.csrfToken
	if !cfg.disableCSRF && tokenStr == "" {
		reader := cfg.randReader
		if reader == nil {
			reader = randReader
		}
		var err error
		tokenStr, err = generateCSRFToken(reader)
		if err != nil {
			return nil, err
		}
	}

	return &Guard{
		cfg:   cfg,
		token: []byte(tokenStr),
	}, nil
}

// Token returns the active per-boot ephemeral CSRF token (or empty string when CSRF is disabled).
func (g *Guard) Token() string {
	return string(g.token)
}

// VerifyToken checks candidate against the guard's active CSRF token using
// constant-time comparison. Empty tokens always return false.
func (g *Guard) VerifyToken(candidate string) bool {
	if len(g.token) == 0 || len(candidate) == 0 {
		return false
	}
	return subtle.ConstantTimeCompare(g.token, []byte(candidate)) == 1
}

// Middleware returns a standard net/http middleware enforcing loopback security guards.
func (g *Guard) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if g.cfg.requireLoopbackPeer && !isLoopbackPeer(r.RemoteAddr) {
			writeForbidden(w, "NON_LOOPBACK_PEER", "requests must originate from loopback")
			return
		}
		if !g.isAllowedHost(r.Host) {
			writeForbidden(w, "DNS_REBINDING_BLOCKED", "forbidden Host header")
			return
		}
		if !g.isAllowedOrigin(r) {
			writeForbidden(w, "ORIGIN_MISMATCH", "cross-origin request blocked")
			return
		}
		if !g.isAllowedSecFetchSite(r) {
			writeForbidden(w, "CROSS_SITE_BLOCKED", "cross-site request blocked")
			return
		}
		if g.requiresCSRF(r) && !g.VerifyToken(strings.TrimSpace(r.Header.Get(g.cfg.csrfHeader))) {
			writeForbidden(w, "CSRF_INVALID", "invalid or missing CSRF token")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func writeForbidden(w http.ResponseWriter, code, msg string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusForbidden)
	_, _ = w.Write([]byte(`{"error":"` + msg + `","code":"` + code + `"}`))
}

func isLoopbackPeer(remoteAddr string) bool {
	host := strings.TrimSpace(remoteAddr)
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	ip, err := netip.ParseAddr(host)
	return err == nil && ip.IsLoopback()
}

func extractHostname(hostHeader string) (string, bool) {
	trimmed := strings.TrimSpace(hostHeader)
	if trimmed == "" {
		return "", false
	}
	if strings.Contains(trimmed, ":") {
		h, _, err := net.SplitHostPort(trimmed)
		if err != nil {
			// Allow bare IPv6 literal without port (e.g. "::1")
			if ip, parseErr := netip.ParseAddr(trimmed); parseErr == nil {
				return strings.ToLower(ip.String()), true
			}
			return "", false
		}
		return strings.ToLower(h), h != ""
	}
	return strings.ToLower(trimmed), true
}

func (g *Guard) isAllowedHost(hostHeader string) bool {
	host, ok := extractHostname(hostHeader)
	if !ok {
		return false
	}
	if host == "localhost" {
		return true
	}
	if _, ok := g.cfg.allowedHosts[host]; ok {
		return true
	}
	ip, err := netip.ParseAddr(host)
	return err == nil && ip.IsLoopback()
}

func (g *Guard) isAllowedOrigin(r *http.Request) bool {
	origin := strings.TrimSpace(r.Header.Get("Origin"))
	if origin == "" {
		return true
	}
	normOrigin := strings.ToLower(strings.TrimRight(origin, "/"))
	if _, ok := g.cfg.allowedOrigins[normOrigin]; ok {
		return true
	}
	parsed, err := url.Parse(normOrigin)
	if err != nil || parsed.Host == "" {
		return false
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return false
	}
	return strings.EqualFold(parsed.Host, strings.TrimSpace(r.Host))
}

func (g *Guard) isAllowedSecFetchSite(r *http.Request) bool {
	site := strings.ToLower(strings.TrimSpace(r.Header.Get("Sec-Fetch-Site")))
	if site == "" || site == "same-origin" || site == "none" {
		return true
	}
	_, isSafe := g.cfg.safeMethods[r.Method]
	if !isSafe {
		return false
	}
	if site == "cross-site" {
		if g.cfg.strictSecFetchSite {
			return false
		}
		return strings.EqualFold(strings.TrimSpace(r.Header.Get("Sec-Fetch-Mode")), "navigate")
	}
	return true
}

func (g *Guard) requiresCSRF(r *http.Request) bool {
	if g.cfg.disableCSRF {
		return false
	}
	if _, safe := g.cfg.safeMethods[r.Method]; safe {
		return false
	}
	for _, prefix := range g.cfg.exemptPrefixes {
		if strings.HasPrefix(r.URL.Path, prefix) {
			return false
		}
	}
	return true
}
