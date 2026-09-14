// SPDX-License-Identifier: MIT

package ginmw

import (
	"context"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/umesh0492/go-libs/rbaccontext"
)

// PermissionProvider abstracts the source of truth for Role-Based Access Control.
//
// GetPermissionsForUser returns the union of all permissions across every active
// role assigned to that user within the given account context.
// Implementations are expected to resolve permissions dynamically per-request
// to guarantee immediate invalidation when role assignments change.
type PermissionProvider interface {
	GetPermissionsForUser(ctx context.Context, userID, tenantID uuid.UUID) ([]string, error)
}

// RBAC loads the union of permissions for the authenticated user and stores them
// in the request context via rbaccontext.WithPermissions.
// Must run AFTER AuthMiddleware in the middleware chain.
func RBAC(provider PermissionProvider) gin.HandlerFunc {
	return func(c *gin.Context) {
		claims := GetClaims(c)
		ctx := c.Request.Context()

		if claims == nil {
			// Unauthenticated: inject empty permission set — all Can() calls return false.
			c.Request = c.Request.WithContext(rbaccontext.WithPermissions(ctx, []string{}))
			c.Next()
			return
		}

		perms, err := provider.GetPermissionsForUser(ctx, claims.UserID, claims.TenantID)
		if err != nil {
			// Fail closed — never grant access on DB error.
			c.AbortWithStatusJSON(500, gin.H{"error": "Permission check failed", "code": "RBAC_ERROR"})
			return
		}

		c.Request = c.Request.WithContext(rbaccontext.WithPermissions(ctx, perms))
		c.Next()
	}
}

// RequirePermission aborts with 403 if the caller lacks the given "resource:action" permission.
// Use as an inline middleware on individual routes for fine-grained control.
//
// Example:
//
//	r.POST("/invoices/:id/approve", ginmw.RequirePermission("invoices:approve"), handler.ApproveInvoice)
func RequirePermission(perm string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !rbaccontext.Can(c.Request.Context(), perm) {
			c.AbortWithStatusJSON(403, gin.H{"error": "Forbidden", "code": "FORBIDDEN", "required": perm})
			return
		}
		c.Next()
	}
}

// RequireAnyPermission aborts with 403 unless the caller holds at least one of the given permissions.
func RequireAnyPermission(perms ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !rbaccontext.CanAny(c.Request.Context(), perms...) {
			c.AbortWithStatusJSON(403, gin.H{"error": "Forbidden", "code": "FORBIDDEN"})
			return
		}
		c.Next()
	}
}

// RequireRole aborts with 403 unless at least one of the caller's JWT roles matches.
// Prefer RequirePermission for most guards; use RequireRole only for coarse route-group protection.
func RequireRole(roles ...string) gin.HandlerFunc {
	allowed := toSet(roles)
	return func(c *gin.Context) {
		claims := GetClaims(c)
		if claims == nil {
			c.AbortWithStatusJSON(403, gin.H{"error": "Forbidden", "code": "FORBIDDEN"})
			return
		}
		for _, r := range claims.Roles {
			if allowed[r] {
				c.Next()
				return
			}
		}
		c.AbortWithStatusJSON(403, gin.H{"error": "Forbidden: insufficient role", "code": "FORBIDDEN"})
	}
}

// AdminOnly restricts route access to administrative roles.
// If no roles are provided, it defaults to "ADMIN", "SUPERADMIN", and "PLATFORM_ADMIN".
func AdminOnly(roles ...string) gin.HandlerFunc {
	if len(roles) == 0 {
		roles = []string{"ADMIN", "SUPERADMIN", "PLATFORM_ADMIN"}
	}
	return RequireRole(roles...)
}

// toSet converts a slice of strings into a map for O(1) lookup.
func toSet(ss []string) map[string]bool {
	m := make(map[string]bool, len(ss))
	for _, s := range ss {
		m[s] = true
	}
	return m
}
