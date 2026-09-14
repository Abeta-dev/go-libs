// SPDX-License-Identifier: MIT

// Package securityheaders provides an HTTP middleware that sets defensive
// security headers on every response. Apply it as the outermost middleware so
// no response can bypass it.
//
// Headers set:
//   - X-Frame-Options: DENY                (clickjacking)
//   - X-Content-Type-Options: nosniff      (MIME sniffing)
//   - X-XSS-Protection: 1; mode=block      (legacy XSS filter)
//   - Referrer-Policy: strict-origin-...   (leak prevention)
//   - Strict-Transport-Security            (HTTPS enforcement, 2 years)
//   - Permissions-Policy                   (feature restrictions)
//   - Content-Security-Policy              (configurable)
//   - Cache-Control: no-store              (prevent caching of API responses)
//   - Server: <custom>                     (removes Go version fingerprint)
//
// Usage:
//
//	// Default (strict CSP, no framing, HSTS 2 years)
//	router.Use(securityheaders.Default)
//
//	// Custom CSP
//	router.Use(securityheaders.New(
//	    securityheaders.WithCSP("default-src 'self'"),
//	    securityheaders.WithServerName("my-api"),
//	))
package securityheaders

import (
	"fmt"
	"net/http"
)

// Config controls which header values are applied.
type Config struct {
	// ServerName replaces the default Go server banner. Defaults to "api".
	ServerName string
	// HSTS max-age in seconds. Defaults to 63072000 (2 years).
	HSTSMaxAge int
	// CSP is the Content-Security-Policy header value.
	// Defaults to a strict deny-all policy appropriate for REST APIs.
	CSP string
	// PermissionsPolicy value. Defaults to denying geo/mic/camera.
	PermissionsPolicy string
}

// Option configures defensive HTTP security headers.
type Option func(*Config)

// WithServerName sets a custom Server response header.
func WithServerName(name string) Option {
	return func(c *Config) {
		c.ServerName = name
	}
}

// WithHSTSMaxAge sets the max-age in seconds for Strict-Transport-Security.
func WithHSTSMaxAge(maxAge int) Option {
	return func(c *Config) {
		c.HSTSMaxAge = maxAge
	}
}

// WithCSP sets the Content-Security-Policy header value.
func WithCSP(policy string) Option {
	return func(c *Config) {
		c.CSP = policy
	}
}

// WithPermissionsPolicy sets the Permissions-Policy header value.
func WithPermissionsPolicy(policy string) Option {
	return func(c *Config) {
		c.PermissionsPolicy = policy
	}
}

// DefaultConfig is the recommended configuration for REST API services.
var DefaultConfig = Config{
	ServerName:        "api",
	HSTSMaxAge:        63072000, // 2 years
	CSP:               "default-src 'none'; frame-ancestors 'none';",
	PermissionsPolicy: "geolocation=(), microphone=(), camera=()",
}

// Default is a ready-to-use middleware with DefaultConfig.
// This is the one-liner option for most microservices.
var Default = New()

// New returns a middleware configured by the provided options.
// If no options are specified, DefaultConfig values are applied.
func New(opts ...Option) func(http.Handler) http.Handler {
	cfg := DefaultConfig
	for _, opt := range opts {
		opt(&cfg)
	}
	if cfg.ServerName == "" {
		cfg.ServerName = DefaultConfig.ServerName
	}
	if cfg.HSTSMaxAge == 0 {
		cfg.HSTSMaxAge = DefaultConfig.HSTSMaxAge
	}
	if cfg.CSP == "" {
		cfg.CSP = DefaultConfig.CSP
	}
	if cfg.PermissionsPolicy == "" {
		cfg.PermissionsPolicy = DefaultConfig.PermissionsPolicy
	}

	hsts := fmt.Sprintf("max-age=%d; includeSubDomains; preload", cfg.HSTSMaxAge)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			h := w.Header()
			h.Set("X-Frame-Options", "DENY")
			h.Set("X-Content-Type-Options", "nosniff")
			h.Set("X-XSS-Protection", "1; mode=block")
			h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
			h.Set("Strict-Transport-Security", hsts)
			h.Set("Permissions-Policy", cfg.PermissionsPolicy)
			h.Set("Content-Security-Policy", cfg.CSP)
			h.Set("Cache-Control", "no-store")
			h.Set("Pragma", "no-cache")
			h.Set("Server", cfg.ServerName)
			next.ServeHTTP(w, r)
		})
	}
}
