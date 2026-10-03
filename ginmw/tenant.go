// SPDX-License-Identifier: MIT

package ginmw

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// Default tenant extraction constants.
const (
	DefaultTenantContextKey = "tenant_id"
	DefaultTenantHeader     = "x-tenant-id"
	DefaultTenantQuery      = "tenant_id"
)

type tenantOptions struct {
	contextKey   string
	headerName   string
	queryParam   string
	bypassRoles  map[string]bool
	errorMessage string
}

// TenantOption configures RequireTenant middleware behavior.
type TenantOption func(*tenantOptions)

// WithTenantContextKey sets the context key used to store/retrieve the tenant ID.
func WithTenantContextKey(key string) TenantOption {
	return func(o *tenantOptions) {
		o.contextKey = key
	}
}

// WithTenantHeader sets the HTTP request header used for tenant ID extraction.
func WithTenantHeader(header string) TenantOption {
	return func(o *tenantOptions) {
		o.headerName = header
	}
}

// WithTenantQuery sets the URL query parameter used for tenant ID extraction.
func WithTenantQuery(param string) TenantOption {
	return func(o *tenantOptions) {
		o.queryParam = param
	}
}

// WithBypassRoles defines roles that are permitted to bypass the mandatory tenant check
// (e.g. cross-tenant platform administrators).
func WithBypassRoles(roles ...string) TenantOption {
	return func(o *tenantOptions) {
		for _, r := range roles {
			o.bypassRoles[strings.ToUpper(strings.TrimSpace(r))] = true
		}
	}
}

// RequireTenant ensures that the incoming request has a non-empty tenant identifier.
// The tenant ID is resolved from:
// 1. Existing Gin context key (e.g. set by prior auth middleware)
// 2. JWT claims (if present in context)
// 3. HTTP header (defaults to "x-tenant-id")
// 4. URL query parameter (defaults to "tenant_id")
//
// If the tenant ID is missing and the user does not hold a bypass role, it aborts
// with HTTP 400 Bad Request and code "TENANT_REQUIRED".
func RequireTenant(opts ...TenantOption) gin.HandlerFunc {
	cfg := &tenantOptions{
		contextKey:   DefaultTenantContextKey,
		headerName:   DefaultTenantHeader,
		queryParam:   DefaultTenantQuery,
		bypassRoles:  make(map[string]bool),
		errorMessage: "Tenant ID required via header (" + DefaultTenantHeader + ") or query parameter (" + DefaultTenantQuery + ")",
	}
	for _, opt := range opts {
		if opt != nil {
			opt(cfg)
		}
	}

	return func(c *gin.Context) {
		tenantID := ExtractTenantID(c, cfg.contextKey, cfg.headerName, cfg.queryParam)

		if tenantID == "" {
			// Check if caller holds an allowed bypass role
			if claims := GetClaims(c); claims != nil {
				for _, r := range claims.Roles {
					if cfg.bypassRoles[strings.ToUpper(strings.TrimSpace(r))] {
						c.Next()
						return
					}
				}
			}

			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "TENANT_REQUIRED",
					"message": cfg.errorMessage,
				},
			})
			return
		}

		c.Set(cfg.contextKey, tenantID)
		c.Next()
	}
}

// ExtractTenantID retrieves the active tenant identifier from context, claims, headers, or query.
func ExtractTenantID(c *gin.Context, contextKey, headerName, queryParam string) string {
	if contextKey == "" {
		contextKey = DefaultTenantContextKey
	}
	if headerName == "" {
		headerName = DefaultTenantHeader
	}
	if queryParam == "" {
		queryParam = DefaultTenantQuery
	}

	// 1. Direct Gin context lookup
	if val, exists := c.Get(contextKey); exists {
		if s, ok := val.(string); ok && strings.TrimSpace(s) != "" {
			return strings.TrimSpace(s)
		}
	}

	// 2. JWT claims lookup
	if claims := GetClaims(c); claims != nil && claims.TenantID.String() != "00000000-0000-0000-0000-000000000000" && claims.TenantID.String() != "" {
		return claims.TenantID.String()
	}

	// 3. Request Header lookup
	if h := strings.TrimSpace(c.GetHeader(headerName)); h != "" {
		return h
	}

	// 4. URL query lookup
	return strings.TrimSpace(c.Query(queryParam))
}
