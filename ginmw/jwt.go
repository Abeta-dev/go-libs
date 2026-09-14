// SPDX-License-Identifier: MIT

// Package ginmw provides HTTP middleware for JWT authentication and RBAC.
package ginmw

import (
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// Claims represents the JWT payload for generic multi-tenant authorization.
type Claims struct {
	UserID   uuid.UUID      `json:"user_id"`
	TenantID uuid.UUID      `json:"tenant_id,omitempty"`
	Email    string         `json:"email,omitempty"`
	Roles    []string       `json:"roles,omitempty"`
	Custom   map[string]any `json:"custom,omitempty"`

	jwt.RegisteredClaims
}

// PrimaryRole returns the first role in Roles, or empty string.
// Use only during the migration window where single-role code paths exist.
func (c *Claims) PrimaryRole() string {
	if len(c.Roles) == 0 {
		return ""
	}
	return c.Roles[0]
}

// HasRole returns true if any of the claim's roles matches the given role name.
func (c *Claims) HasRole(role string) bool {
	for _, r := range c.Roles {
		if r == role {
			return true
		}
	}
	return false
}

var (
	jwtSecretMu sync.RWMutex
	jwtSecret   []byte
)

func getJWTSecret() []byte {
	jwtSecretMu.RLock()
	defer jwtSecretMu.RUnlock()
	if len(jwtSecret) == 0 {
		return nil
	}
	cp := make([]byte, len(jwtSecret))
	copy(cp, jwtSecret)
	return cp
}

// SetJWTSecret configures the signing secret. Must be called once at startup.
// Returns an error if the provided secret is empty.
func SetJWTSecret(secret string) error {
	jwtSecretMu.Lock()
	defer jwtSecretMu.Unlock()
	if strings.TrimSpace(secret) == "" {
		jwtSecret = nil
		return errors.New("jwt secret cannot be empty")
	}
	jwtSecret = []byte(secret)
	return nil
}

// GenerateToken mints a signed JWT valid for the specified duration (in minutes).
// Sets ExpiresAt and IssuedAt while preserving caller RegisteredClaims (Issuer, Subject, Audience, NotBefore, ID).
func GenerateToken(claims *Claims, timeInMins int64) (string, error) {
	secret := getJWTSecret()
	if len(secret) == 0 {
		return "", errors.New("jwt secret is uninitialized")
	}
	now := time.Now()
	claims.ExpiresAt = jwt.NewNumericDate(now.Add(time.Duration(timeInMins) * time.Minute))
	claims.IssuedAt = jwt.NewNumericDate(now)

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(secret)
}

type authOptions struct {
	allowQueryToken bool
}

// AuthOption configures AuthMiddleware behavior.
type AuthOption func(*authOptions)

// WithAllowQueryToken allows extracting tokens from the "token" query parameter.
// This should only be used for endpoints that cannot use the Authorization header (e.g. WebSockets).
func WithAllowQueryToken() AuthOption {
	return func(o *authOptions) {
		o.allowQueryToken = true
	}
}

// AuthMiddleware validates the Bearer token and injects Claims into the gin context.
// Returns machine-readable error codes in the JSON body for deterministic client handling.
func AuthMiddleware(opts ...AuthOption) gin.HandlerFunc {
	var cfg authOptions
	for _, opt := range opts {
		opt(&cfg)
	}

	return func(c *gin.Context) {
		secret := getJWTSecret()
		if len(secret) == 0 {
			c.AbortWithStatusJSON(401, gin.H{"error": "JWT secret is uninitialized", "code": "UNAUTHORIZED"})
			return
		}

		tokenStr := extractToken(c, cfg.allowQueryToken)
		if tokenStr == "" {
			c.AbortWithStatusJSON(401, gin.H{"error": "Authorization required", "code": "UNAUTHORIZED"})
			return
		}

		claims := &Claims{}
		token, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (interface{}, error) {
			return secret, nil
		}, jwt.WithValidMethods([]string{"HS256"}))
		if err != nil || !token.Valid {
			c.AbortWithStatusJSON(401, gin.H{"error": "Invalid or expired token", "code": "TOKEN_EXPIRED"})
			return
		}

		c.Set("claims", claims)
		c.Next()
	}
}

// GetClaims extracts JWT claims from the gin context.
// Returns nil if the request is unauthenticated (AuthMiddleware not in chain or token absent).
func GetClaims(c *gin.Context) *Claims {
	v, exists := c.Get("claims")
	if !exists {
		return nil
	}
	claims, _ := v.(*Claims)
	return claims
}

// extractToken extracts the JWT token from the request.
// By default, only Authorization: Bearer <token> is accepted.
// If allowQueryToken is true, it also checks the ?token= query parameter.
func extractToken(c *gin.Context, allowQueryToken bool) string {
	if h := c.GetHeader("Authorization"); h != "" {
		if parts := strings.SplitN(h, " ", 2); len(parts) == 2 && strings.EqualFold(parts[0], "bearer") {
			return parts[1]
		}
	}
	if allowQueryToken {
		return c.Query("token")
	}
	return ""
}
