// SPDX-License-Identifier: MIT

package ginmw_test

import (
	"context"
	"errors"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/umesh0492/go-libs/ginmw"
	"github.com/umesh0492/go-libs/rbaccontext"
)

// MockProvider implements ginmw.PermissionProvider
type MockProvider struct {
	Perms map[string][]string // userID:accountID -> permissions
	Err   error
}

func (m *MockProvider) GetPermissionsForUser(ctx context.Context, userID, tenantID uuid.UUID) ([]string, error) {
	if m.Err != nil {
		return nil, m.Err
	}
	key := userID.String() + ":" + tenantID.String()
	return m.Perms[key], nil
}

func setupEngine() *gin.Engine {
	gin.SetMode(gin.TestMode)
	return gin.New()
}

func TestRBAC(t *testing.T) {
	uID := uuid.New()
	aID := uuid.New()
	key := uID.String() + ":" + aID.String()

	t.Run("Unauthenticated Request", func(t *testing.T) {
		r := setupEngine()
		r.Use(ginmw.RBAC(&MockProvider{}))
		r.GET("/test", func(c *gin.Context) {
			assert.False(t, rbaccontext.Can(c.Request.Context(), "foo:bar"))
			c.Status(200)
		})
		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, 200, w.Code)
	})

	t.Run("Authenticated Claims Found", func(t *testing.T) {
		provider := &MockProvider{
			Perms: map[string][]string{key: {"foo:bar"}},
		}
		r := setupEngine()
		r.Use(func(c *gin.Context) {
			c.Set("claims", &ginmw.Claims{
				UserID:   uID,
				TenantID: aID,
				Roles:    []string{"admin"},
			})
		})
		r.Use(ginmw.RBAC(provider))
		r.GET("/test", func(c *gin.Context) {
			assert.True(t, rbaccontext.Can(c.Request.Context(), "foo:bar"))
			assert.False(t, rbaccontext.Can(c.Request.Context(), "foo:baz"))
			c.Status(200)
		})
		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, 200, w.Code)
	})

	t.Run("Provider Error", func(t *testing.T) {
		provider := &MockProvider{Err: errors.New("db error")}
		r := setupEngine()
		r.Use(func(c *gin.Context) {
			c.Set("claims", &ginmw.Claims{
				UserID:   uID,
				TenantID: aID,
			})
		})
		r.Use(ginmw.RBAC(provider))
		r.GET("/test", func(c *gin.Context) { c.Status(200) })
		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, 500, w.Code)
	})
}

func TestRequireRole(t *testing.T) {
	r := setupEngine()
	r.Use(func(c *gin.Context) {
		c.Set("claims", &ginmw.Claims{Roles: []string{"superuser"}})
	})
	r.Use(ginmw.RequireRole("superuser", "manager"))
	r.GET("/test", func(c *gin.Context) { c.Status(200) })

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)
}

func TestRequireRole_Forbidden(t *testing.T) {
	r := setupEngine()
	r.Use(func(c *gin.Context) {
		c.Set("claims", &ginmw.Claims{Roles: []string{"guest"}})
	})
	r.Use(ginmw.RequireRole("superuser", "manager"))
	r.GET("/test", func(c *gin.Context) { c.Status(200) })

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, 403, w.Code)
}

func TestAdminOnly(t *testing.T) {
	t.Run("Default Roles - ADMIN", func(t *testing.T) {
		r := setupEngine()
		r.Use(func(c *gin.Context) {
			c.Set("claims", &ginmw.Claims{Roles: []string{"ADMIN"}})
		})
		r.Use(ginmw.AdminOnly())
		r.GET("/test", func(c *gin.Context) { c.Status(200) })

		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, 200, w.Code)
	})

	t.Run("Default Roles - SUPERADMIN", func(t *testing.T) {
		r := setupEngine()
		r.Use(func(c *gin.Context) {
			c.Set("claims", &ginmw.Claims{Roles: []string{"SUPERADMIN"}})
		})
		r.Use(ginmw.AdminOnly())
		r.GET("/test", func(c *gin.Context) { c.Status(200) })

		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, 200, w.Code)
	})

	t.Run("Default Roles - PLATFORM_ADMIN", func(t *testing.T) {
		r := setupEngine()
		r.Use(func(c *gin.Context) {
			c.Set("claims", &ginmw.Claims{Roles: []string{"PLATFORM_ADMIN"}})
		})
		r.Use(ginmw.AdminOnly())
		r.GET("/test", func(c *gin.Context) { c.Status(200) })

		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, 200, w.Code)
	})

	t.Run("Default Roles - Rejected", func(t *testing.T) {
		r := setupEngine()
		r.Use(func(c *gin.Context) {
			c.Set("claims", &ginmw.Claims{Roles: []string{"VIEWER"}})
		})
		r.Use(ginmw.AdminOnly())
		r.GET("/test", func(c *gin.Context) { c.Status(200) })

		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, 403, w.Code)
	})

	t.Run("Custom Roles Specified", func(t *testing.T) {
		r := setupEngine()
		r.Use(func(c *gin.Context) {
			c.Set("claims", &ginmw.Claims{Roles: []string{"ORG_SECURITY_ADMIN"}})
		})
		r.Use(ginmw.AdminOnly("ORG_SECURITY_ADMIN"))
		r.GET("/test", func(c *gin.Context) { c.Status(200) })

		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, 200, w.Code)
	})
}

func TestRequireRole_NoClaims(t *testing.T) {
	r := setupEngine()
	// No claims set
	r.Use(ginmw.RequireRole("superuser"))
	r.GET("/test", func(c *gin.Context) { c.Status(200) })

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, 403, w.Code)
}

func TestRequirePermission(t *testing.T) {
	t.Run("Allowed", func(t *testing.T) {
		r := setupEngine()
		r.Use(func(c *gin.Context) {
			c.Request = c.Request.WithContext(rbaccontext.WithPermissions(c.Request.Context(), []string{"users:read"}))
		})
		r.Use(ginmw.RequirePermission("users:read"))
		r.GET("/test", func(c *gin.Context) { c.Status(200) })

		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, 200, w.Code)
	})

	t.Run("Forbidden", func(t *testing.T) {
		r := setupEngine()
		r.Use(func(c *gin.Context) {
			c.Request = c.Request.WithContext(rbaccontext.WithPermissions(c.Request.Context(), []string{"users:read"}))
		})
		r.Use(ginmw.RequirePermission("users:write"))
		r.GET("/test", func(c *gin.Context) { c.Status(200) })

		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, 403, w.Code)
	})
}

func TestRequireAnyPermission(t *testing.T) {
	t.Run("Allowed", func(t *testing.T) {
		r := setupEngine()
		r.Use(func(c *gin.Context) {
			c.Request = c.Request.WithContext(rbaccontext.WithPermissions(c.Request.Context(), []string{"users:read"}))
		})
		r.Use(ginmw.RequireAnyPermission("users:read", "users:write"))
		r.GET("/test", func(c *gin.Context) { c.Status(200) })

		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, 200, w.Code)
	})

	t.Run("Forbidden", func(t *testing.T) {
		r := setupEngine()
		r.Use(func(c *gin.Context) {
			c.Request = c.Request.WithContext(rbaccontext.WithPermissions(c.Request.Context(), []string{"none"}))
		})
		r.Use(ginmw.RequireAnyPermission("users:read", "users:write"))
		r.GET("/test", func(c *gin.Context) { c.Status(200) })

		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, 403, w.Code)
	})
}
